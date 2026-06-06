# Kensan Lakehouse Architecture

_2026-02-04 時点の実装状況 (Dagster 統合版)_

---

## 1. 全体データフロー

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  DATA SOURCES                                                               │
│                                                                             │
│  ┌──────────────────┐         ┌──────────────────────┐                      │
│  │ PostgreSQL :5432  │         │ kensan-ai            │                      │
│  │ (Kensan本体DB)    │         │ (LakehouseWriter)    │                      │
│  └────────┬─────────┘         └──────────┬───────────┘                      │
│           │ psycopg batch                │ direct Parquet append             │
└───────────┼──────────────────────────────┼──────────────────────────────────┘
            │                              │
            ▼                              │
┌───────────────────────┐                  │
│ ingest_postgres.py    │                  │
│ (差分検知:             │                  │
│  .ingestion_state.json)│                  │
└───────────┬───────────┘                  │
            │ PyIceberg write              │
            ▼                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  ICEBERG TABLES  (Parquet on MinIO, cataloged by Nessie)                    │
│                                                                             │
│  ┌─ Bronze (Raw) ──────────────────────────────────────────────────────┐    │
│  │  time_entries_raw*   tasks_raw          notes_raw                   │    │
│  │  ai_interactions_raw*  ai_facts_raw     ai_reviews_raw              │    │
│  │  ai_contexts_raw      external_tool_results_raw*                    │    │
│  │                                              (* = month partition)  │    │
│  └────────────────────────────┬─────────────────────────────────────────┘    │
│               │ transform.py (PyArrow計算)                                  │
│               ▼                                                             │
│  ┌─ Silver (Cleaned) ──────────────────────────────────────────────────┐    │
│  │  time_entries (+date, +duration_minutes)*                           │    │
│  │  tasks (+is_subtask)                                                │    │
│  │  notes (+content_length)                                            │    │
│  │  ai_interactions (+tool_count, +tokens_total)*                      │    │
│  │  ai_token_usage (日次×situation集計)                                │    │
│  └────────────────────────────┬─────────────────────────────────────────┘    │
│               │ aggregate.py (週次集計)                                      │
│               ▼                                                             │
│  ┌─ Gold (Weekly Aggregation) ─────────────────────────────────────────┐    │
│  │  weekly_summary        goal_progress                                │    │
│  │  ai_usage_weekly       ai_quality_weekly                            │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
            │                    │                      │
            ▼                    ▼                      ▼
    ┌──────────────┐   ┌─────────────────┐   ┌──────────────────┐
    │  Option A    │   │  Option B       │   │  Local           │
    │  Dremio      │   │  Trino+Superset │   │  DuckDB          │
    │  :9047       │   │  :8085 / :8088  │   │  make query      │
    └──────────────┘   └─────────────────┘   └──────────────────┘
```

---

## 2. オーケストレーション (Dagster)

```
┌─────────────────────────────────────────────────────────────────┐
│  Dagster (docker-compose.common.yml に統合)                      │
│                                                                  │
│  ┌──────────────────┐   ┌──────────────────┐                     │
│  │ dagster-webserver │   │ dagster-daemon   │                     │
│  │ :3070 (Web UI)    │   │ (scheduler)      │                     │
│  └────────┬─────────┘   └────────┬─────────┘                     │
│           │ gRPC                  │ gRPC                          │
│           ▼                      ▼                               │
│  ┌────────────────────────────────┐   ┌────────────────────┐     │
│  │ dagster-user-code :4000        │   │ dagster-postgres   │     │
│  │ (Bronze 7 + Silver 5 + Gold 4) │   │ (メタデータDB)      │     │
│  └────────────────────────────────┘   └────────────────────┘     │
│                                                                  │
│  Asset グラフ:                                                    │
│    PG → bronze_*_raw → silver_* → gold_*                         │
│                                                                  │
│  Job: full_pipeline (全16アセット)                                 │
│  Schedule: daily_schedule (毎日02:00, 初期STOPPED)                │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. 共通基盤 (docker-compose.common.yml)

```
┌──────────────────────────────────────────────────────┐
│  lakehouse-net                                        │
│                                                       │
│  ┌─────────────┐   ┌────────────┐   ┌─────────────┐  │
│  │   MinIO      │   │ minio-init │   │   Nessie    │  │
│  │   :9000 API  │◄──│ バケット作成 │   │   :19120    │  │
│  │   :9001 UI   │   │ kensan-    │   │  REST       │  │
│  │              │   │ lakehouse  │   │  Catalog    │  │
│  │  volume:     │   └────────────┘   │             │  │
│  │  minio_data  │                    │  IN_MEMORY  │  │
│  └─────────────┘                    └─────────────┘  │
│                                                       │
└──────────────────────────────────────────────────────┘
```

---

## 4. クエリエンジン構成 (3つの選択肢)

### Option A: Dremio (`make dremio-up`)

```
┌──────────────────────────────────────────────┐
│  docker-compose.common.yml                    │
│  + docker-compose.dremio.yml                  │
│                                               │
│  ┌──────────┐                                 │
│  │  Dremio  │───Nessie REST──▶ Nessie :19120 │
│  │  :9047   │───S3 read──────▶ MinIO  :9000  │
│  │  :31010  │                                 │
│  │  :32010  │  volume: dremio_data            │
│  └──────────┘                                 │
└──────────────────────────────────────────────┘

ブラウザ ──▶ http://localhost:9047
  └─ Add Source ▶ Nessie
     Endpoint:  http://nessie:19120/api/v2
     Auth:      None
     S3:        http://minio:9000
     Key:       kensan / kensan-minio
     Path style: ON
```

### Option B: Trino + Superset (`make trino-up`)

```
┌────────────────────────────────────────────────────────┐
│  docker-compose.common.yml                              │
│  + docker-compose.trino.yml                             │
│                                                         │
│  ┌───────────┐                                          │
│  │  Trino    │──iceberg.properties──▶ Nessie :19120    │
│  │  :8085    │──S3 read─────────────▶ MinIO  :9000     │
│  └─────▲─────┘                                          │
│        │ trino://                                       │
│  ┌─────┴──────┐   ┌────────────┐   ┌────────────────┐  │
│  │  Superset  │──▶│   Redis    │   │  Superset PG   │  │
│  │  :8088     │──▶│  (cache)   │   │  (metadata)    │  │
│  │            │──▶│            │   │                 │  │
│  └────────────┘   └────────────┘   └────────────────┘  │
└────────────────────────────────────────────────────────┘

Trino Web UI ──▶ http://localhost:8085
Superset    ──▶ http://localhost:8088  (admin / admin)
  └─ Settings ▶ Database Connections ▶ + Database
     SQLAlchemy URI:  trino://trino@trino:8080/iceberg
     → SQL Lab で SHOW TABLES FROM iceberg.bronze
```

### Local: DuckDB (`make query`)

```
┌──────────────────────────────────────────────┐
│  Host (no Docker)                             │
│                                               │
│  ┌──────────┐   PyIceberg                     │
│  │ query.py │──────scan()──▶ Nessie :19120   │
│  │ DuckDB   │       │                         │
│  │ REPL     │◀──Arrow──┘    MinIO  :9000     │
│  └──────────┘                                 │
│                                               │
│  全17テーブルを自動登録                         │
│  SELECT * FROM gold_weekly_summary LIMIT 10;  │
└──────────────────────────────────────────────┘
```

---

## 5. テーブル一覧 & 変換関係

### Bronze → Silver 変換

| Bronze (Raw)                   | Silver (Cleaned)         | 追加カラム                          |
|--------------------------------|--------------------------|-------------------------------------|
| `time_entries_raw`             | `time_entries`           | `date`, `duration_minutes`          |
| `tasks_raw`                    | `tasks`                  | `is_subtask`                        |
| `notes_raw`                    | `notes`                  | `content_length` (content削除)      |
| `ai_interactions_raw`          | `ai_interactions`        | `date`, `tool_count`, `tool_names_json`, `tokens_total` |
| `ai_interactions_raw`          | `ai_token_usage`         | 日次×situation pre-aggregate        |
| `ai_facts_raw`                 | _(Gold直接参照)_         | -                                   |
| `ai_reviews_raw`               | _(Gold直接参照)_         | -                                   |
| `ai_contexts_raw`              | _(変換なし)_             | -                                   |
| `external_tool_results_raw`    | _(変換なし)_             | -                                   |

### Silver/Bronze → Gold 集計

| Gold (Weekly)          | ソーステーブル                              | 集計内容                                            |
|------------------------|---------------------------------------------|-----------------------------------------------------|
| `weekly_summary`       | silver.time_entries, tasks, notes            | total_minutes, task_count, completed, note/diary/learning count |
| `goal_progress`        | silver.time_entries                          | goal別 total_minutes, entry_count                   |
| `ai_usage_weekly`      | silver.ai_interactions                       | interaction_count, tokens, latency, situation/tool JSON |
| `ai_quality_weekly`    | silver.ai_interactions, bronze.facts/reviews | rated_count, avg_rating, fact_count, review_generated |

---

## 6. データ投入経路

| 経路        | ソース       | 方式                                                     | 宛先                              |
|-------------|-------------|----------------------------------------------------------|-----------------------------------|
| バッチ      | PostgreSQL  | `ingest_postgres.py` (差分: `.ingestion_state.json`)      | Bronze 7テーブル                   |
| 直接書込    | kensan-ai   | LakehouseWriter (Parquet append)                          | `bronze.external_tool_results_raw` |

**差分検知ロジック:**
- Mutableテーブル (time_entries, tasks, notes, ai_contexts): `updated_at > last_run` → 全件reload
- Append-onlyテーブル (ai_interactions, facts, reviews): `created_at > last_run` → 差分append
- 初回: EPOCH (2000-01-01) から全件取得

---

## 7. ポート一覧

| サービス       | ポート | 用途                  | 起動コマンド       |
|----------------|--------|-----------------------|--------------------|
| MinIO API      | 9000   | S3互換ストレージ       | `make common-up`   |
| MinIO Console  | 9001   | Web UI                | `make common-up`   |
| Nessie         | 19120  | Iceberg REST Catalog  | `make common-up`   |
| Dremio         | 9047   | Web UI + SQL          | `make dremio-up`   |
| Dremio ODBC    | 31010  | ODBC接続              | `make dremio-up`   |
| Dremio Flight  | 32010  | Arrow Flight SQL      | `make dremio-up`   |
| Trino          | 8085   | Web UI + SQL          | `make trino-up`    |
| Superset       | 8088   | ダッシュボード UI      | `make trino-up`    |
| Dagster Web    | 3070   | Dagster Web UI        | `make common-up`   |

---

## 8. Makeコマンド

```
# ── 共通基盤 ──────────────────────────
make common-up          # MinIO + Nessie 起動
make common-down        # 停止

# ── Option A: Dremio ──────────────────
make dremio-up          # 共通 + Dremio 起動
make dremio-down        # 停止

# ── Option B: Trino + Superset ────────
make trino-up           # 共通 + Trino + Superset 起動
make trino-down         # 停止

# ── パイプライン ──────────────────────
make init               # Icebergテーブル作成
make pipeline           # ingest → transform → aggregate
make query              # DuckDB REPL (全テーブル登録済)

# ── Dagster ──────────────────────────
make dagster-dev        # ローカル Dagster dev server (:3070)
make dagster-logs       # Dagster コンテナログ表示

# ── その他 ────────────────────────────
make health             # MinIO / Nessie ヘルスチェック
make clean              # 全volume削除
```

---

## 9. ファイル構成

```
lakehouse/
├── docker-compose.common.yml        # 共通: MinIO, Nessie, Dagster
├── docker-compose.dremio.yml        # Option A: Dremio
├── docker-compose.trino.yml         # Option B: Trino + Superset + Redis + PG
├── Dockerfile.dagster               # Dagster ユーザーコード用イメージ
├── dagster-docker.yaml               # Dagster インスタンス設定 (Docker用、ビルド時 dagster.yaml にリネーム)
├── workspace.yaml                   # Dagster ワークスペース (gRPC)
├── Makefile
├── pyproject.toml                   # Python deps (pyiceberg, pyarrow, duckdb, psycopg, dagster)
├── catalog/
│   ├── __init__.py                  # パッケージ化
│   ├── config.py                    # Nessie REST catalog + PG DSN 設定
│   └── init_catalog.py              # Icebergテーブル定義 & 作成
├── dagster_project/                 # Dagster Software-Defined Assets
│   ├── __init__.py                  # Definitions (全16アセット, ジョブ, スケジュール)
│   ├── resources.py                 # IcebergCatalog / PostgresDsn リソース
│   └── assets/
│       ├── bronze.py                # 7 Bronze Assets (ファクトリ生成)
│       ├── silver.py                # 5 Silver Assets
│       └── gold.py                  # 4 Gold Assets
├── pipelines/
│   ├── bronze/
│   │   └── ingest_postgres.py       # PostgreSQL → Bronze (差分取込)
│   ├── silver/
│   │   └── transform.py             # Bronze → Silver (クレンジング)
│   └── gold/
│       └── aggregate.py             # Silver → Gold (週次集計)
├── queries/
│   ├── query.py                     # DuckDB REPL (全テーブル登録)
│   └── examples.sql                 # サンプルクエリ
├── trino/
│   └── catalog/
│       └── iceberg.properties       # Trino Iceberg connector → Nessie
└── docs/
    ├── architecture-diagram.md      # ← このファイル
    ├── lakehouse-stack-comparison.md # Dremio vs Trino+Superset 比較
    └── catalog-comparison.md        # Nessie vs Polaris vs Unity
```
