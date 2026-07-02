# Kensan Lakehouse

Apache Iceberg + Polaris REST Catalog によるデータレイクハウス基盤。
Kensanアプリケーションの PostgreSQL データと Loki ログを Medallion Architecture（Bronze → Silver → Gold）で段階的に加工し、DuckDB でアドホック分析を行う。

## アーキテクチャ

```
PostgreSQL ──batch──▶ Bronze ──PyArrow──▶ Silver ──PyArrow──▶ Gold
(Kensan DB)           (生データ)           (整形済み)           (集計済み)
                         │                    │                   │
                         └────────────────────┴───────────────────┘
                                              │
                                     ┌────────▼────────┐
                                     │     DuckDB      │
                                     │  (アドホック分析) │
                                     └─────────────────┘
```

### インフラ構成

| コンポーネント | 役割 | ポート |
|--------------|------|--------|
| PostgreSQL | Kensanアプリ DB（データソース） | 5432 |
| MinIO | S3互換オブジェクトストレージ（Parquetファイル格納） | 9000 (API) / 9001 (Console) |
| Polaris | Iceberg REST Catalog（テーブルメタデータ管理） | 8181 (API) / 8182 (Management) |
| Dremio CE | SQL クエリエンジン（Web UI + Arrow Flight SQL） | 9047 (UI) / 31010 (ODBC) / 32010 (Flight SQL) |
| OpenMetadata | メタデータカタログ（データディスカバリ・リネージ） | 8585 (UI/API) / 8586 (Admin) |
| OpenMetadata Postgres | OpenMetadata 専用 DB | 5433 |
| Elasticsearch | OpenMetadata 検索エンジン | 9200 |
| Airflow (Ingestion) | OpenMetadata メタデータ収集 | 8080 |

### ツール

| ツール | 役割 |
|--------|------|
| PyIceberg | テーブル管理、ingestion、変換（Bronze→Silver→Gold） |
| PyArrow | データ変換・型処理 |
| DuckDB | アドホッククエリ、データ確認 |
| uv | Python依存関係管理 |

## セットアップ

```bash
cd lakehouse

# 1. Python依存関係インストール
make install

# 2. インフラ起動（PostgreSQL + MinIO + Polaris）
make up

# 3. Icebergテーブル作成
make init

# 4. パイプライン実行（Bronze → Silver → Gold）
make pipeline
```

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `make up` | PostgreSQL, MinIO, Polaris を起動 |
| `make down` | 全コンテナを停止 |
| `make install` | Python依存関係をインストール |
| `make init` | Polaris Catalog にIcebergテーブルを作成 |
| `make ingest` | PostgreSQL → Bronze（バッチ取り込み） |
| `make transform` | Bronze → Silver（整形・算出値追加） |
| `make aggregate` | Silver → Gold（週次集計） |
| `make pipeline` | ingest + transform + aggregate を一括実行 |
| `make query` | DuckDB インタラクティブシェル起動 |
| `make summary` | Gold/Silverテーブルのサマリーを表示 |
| `make dremio` | Dremio CE を起動（Web UI: http://localhost:9047） |
| `make openmetadata` | OpenMetadata を起動（UI: http://localhost:8585） |
| `make health` | 各サービスのヘルスチェック |
| `make logs` | コンテナログをtail |
| `make clean` | コンテナとボリュームを削除 |

## テーブル設計

### Bronze層（生データ）

PostgreSQLのデータをそのまま格納。`_ingested_at` カラムを付与。

| テーブル | ソース | パーティション | 取り込み方式 |
|---------|--------|---------------|-------------|
| `bronze.time_entries_raw` | time_entries | `month(start_datetime)` | PG batch |
| `bronze.tasks_raw` | tasks | なし | PG batch |
| `bronze.notes_raw` | notes | なし | PG batch |
| `bronze.ai_interactions_raw` | ai_interactions | `month(created_at)` | PG batch |
| `bronze.ai_facts_raw` | user_facts | なし | PG batch |
| `bronze.ai_reviews_raw` | ai_review_reports | なし | PG batch |
| `bronze.ai_contexts_raw` | ai_contexts | なし | PG batch |
| `bronze.tags_raw` | tags | なし | PG batch |
| `bronze.note_tags_raw` | note_tags | なし | PG batch (always full) |
| `bronze.external_tool_results_raw` | kensan-ai直接書込 | `month(executed_at)` | Direct append |
| `bronze.ai_explorer_events_raw` | Loki (AI agent ログ) | `month(timestamp)` | Loki batch (5分) |

**データフロー 3系統:**
1. **PG → Bronze (batch)**: `ingest_postgres.py` による差分バッチ取り込み（日次）
2. **kensan-ai → Bronze (direct)**: 外部ツール結果を kensan-ai の LakehouseWriter が fire & forget で直接 append
3. **Loki → Bronze (batch)**: `ingest_loki.py` による AI エージェントログの 5分バッチ取り込み

**差分取り込み:**
- `.ingestion_state.json` に各テーブルの最終取り込み時刻を記録
- **mutable テーブル** (time_entries, tasks, notes, ai_contexts): 変更があれば全件 overwrite、なければスキップ
- **append-only テーブル** (ai_interactions, ai_facts, ai_reviews): 前回以降の新規行のみ append
- 初回実行時（ステートファイルなし）は全テーブル全件 overwrite

### Maintenance（定期メンテナンス）

| アセット | 説明 | スケジュール |
|---------|------|------------|
| `reindex_note_chunks` | `index_status='pending'` のノートに対してチャンク分割＋embedding生成を kensan-ai 経由で実行 | 10分ごと (`*/10 * * * *`) |
| `generate_weekly_reviews` | 全アクティブユーザーの週次レビューを kensan-ai 経由で自動生成 | 毎週月曜 3:00 AM (`0 3 * * 1`) |
| `run_prompt_optimization` | プロンプト品質評価＋自動最適化バッチを kensan-ai 経由で実行 | 毎週月曜 3:10 AM (`10 3 * * 1`) |

### Silver層（整形済み）

クリーニング、不要カラム除去、算出値の追加。

| テーブル | 主な変換 | パーティション |
|---------|---------|---------------|
| `silver.time_entries` | `duration_minutes` 算出、`date` 抽出 | `month(date)` |
| `silver.tasks` | `is_subtask` 判定（parent_task_id の有無） | なし |
| `silver.notes` | `content_length` 算出、本文除去 | なし |
| `silver.emotion_segments` | 日記ノートからLLMで感情抽出（valence/energy/stress/keywords） | なし |
| `silver.ai_interactions` | `date` 抽出、ツール情報解析、`tokens_total` 算出 | `month(date)` |
| `silver.ai_token_usage` | 日別×situation別のトークン使用量集計 | なし |
| `silver.tag_usage_profile` | タグ使用統計（件数、共起、トレンド） | なし |
| `silver.user_trait_segments` | LLMによる性格特性抽出（work_style, strengths等） | なし |
| `silver.ai_explorer_interactions` | trace_idでグルーピング、user_id伝播、サマリー構築 | なし |
| `silver.ai_explorer_events` | user_id伝播 + event_order付与 | なし |

### Gold層（集計済み）

分析用の集計テーブル。

| テーブル | 内容 | 集計軸 |
|---------|------|--------|
| `gold.weekly_summary` | 週次の時間・タスク・ノート集計 | user_id × week_start |
| `gold.goal_progress` | ゴール別の週次進捗 | user_id × goal_name × week_start |
| `gold.ai_usage_weekly` | AI使用量の週次集計（トークン、ツール利用分布） | user_id × week_start |
| `gold.ai_quality_weekly` | AI品質の週次集計（評価、ファクト数、レビュー有無） | user_id × week_start |
| `gold.emotion_weekly` | 感情の週次集計（平均valence/energy/stress、タスク相関、トレンド） | user_id × week_start |
| `gold.user_interest_profile` | タグベースの関心プロファイル（top_tags, emerging, fading, clusters） | user_id × week_start |
| `gold.user_trait_profile` | 性格プロファイル集約（work_style, strengths, challenges等） | user_id |

## DuckDBクエリ

### インタラクティブシェル

```bash
make query
```

全Icebergテーブルが自動で登録される。使えるテーブル名：

- `bronze_time_entries`, `bronze_tasks`, `bronze_notes`
- `bronze_ai_interactions`, `bronze_ai_facts`, `bronze_ai_reviews`, `bronze_ai_contexts`
- `bronze_tags`, `bronze_note_tags`
- `bronze_external_tool_results`
- `silver_time_entries`, `silver_tasks`, `silver_notes`
- `silver_ai_interactions`, `silver_ai_token_usage`
- `silver_emotion_segments`, `silver_tag_usage_profile`, `silver_user_trait_segments`
- `gold_weekly_summary`, `gold_goal_progress`
- `gold_ai_usage_weekly`, `gold_ai_quality_weekly`
- `gold_emotion_weekly`, `gold_user_interest_profile`, `gold_user_trait_profile`

```sql
D> SELECT goal_name, sum(duration_minutes)/60 AS hours
   FROM silver_time_entries GROUP BY goal_name ORDER BY hours DESC;
```

### サマリー表示

```bash
make summary
```

### クエリ例（queries/examples.sql）

```sql
-- ゴール別の合計時間
SELECT goal_name, sum(duration_minutes) / 60.0 AS total_hours
FROM silver_time_entries
GROUP BY goal_name
ORDER BY total_hours DESC;

-- 日別の作業時間
SELECT date, sum(duration_minutes) AS total_minutes
FROM silver_time_entries
GROUP BY date ORDER BY date DESC LIMIT 30;

-- タスク完了率
SELECT completed, count(*) FROM silver_tasks GROUP BY completed;

-- ノートタイプ別の文字数
SELECT type, count(*), round(avg(content_length)) AS avg_length
FROM silver_notes GROUP BY type;

-- AI: 週次トークン消費量
SELECT week_start, interaction_count, tokens_total,
       situation_distribution_json
FROM gold_ai_usage_weekly ORDER BY week_start DESC;

-- AI: situation別のトークン使用量
SELECT date, situation, interaction_count, tokens_total
FROM silver_ai_token_usage ORDER BY date DESC LIMIT 30;

-- AI: Web検索の利用頻度
SELECT tool_name, count(*) AS cnt
FROM bronze_external_tool_results GROUP BY tool_name;

-- AI: 品質メトリクス（評価ありのインタラクション）
SELECT week_start, rated_count, round(avg_rating, 2) AS avg_rating,
       fact_count, review_generated
FROM gold_ai_quality_weekly ORDER BY week_start DESC;
```

## Dremio CE セットアップ

Dremio は Polaris + MinIO 上の Iceberg テーブルに SQL でアクセスするためのクエリエンジン。

### 起動

```bash
make dremio
# → http://localhost:9047
```

### 初回セットアップ

1. http://localhost:9047 にアクセスし、admin アカウントを作成
2. Sources → Add Source → **Arctic (Iceberg REST)** を選択
3. 以下を設定:

| 項目 | 値 |
|------|-----|
| Name | `lakehouse` |
| Catalog URI | `http://polaris:8181/api/catalog` |
| Authentication | OAuth2 (`root:s3cr3t`) |

**Storage** セクション:

| 項目 | 値 |
|------|-----|
| AWS Root Path | `kensan-lakehouse` |
| AWS Access Key | `kensan` |
| AWS Secret Key | `kensan-minio` |
| Encrypt connection | OFF |

**Connection Properties** に以下を追加:

| Key | Value |
|-----|-------|
| `fs.s3a.endpoint` | `minio:9000` |
| `fs.s3a.path.style.access` | `true` |
| `dremio.s3.compat` | `true` |
| `fs.s3a.connection.ssl.enabled` | `false` |

4. Save すると bronze/silver/gold テーブルが表示される

### クエリ例

SQL Runner で実行:

```sql
SELECT * FROM lakehouse.bronze.time_entries_raw LIMIT 10;
SELECT goal_name, SUM(duration_minutes)/60 AS hours
FROM lakehouse.silver.time_entries GROUP BY goal_name ORDER BY hours DESC;
```

## OpenMetadata セットアップ

OpenMetadata はメタデータカタログツール。PostgreSQL/Dremio/Polaris のテーブルメタデータを自動収集し、データディスカバリ・リネージを提供する。

### 起動

```bash
make openmetadata
# → OpenMetadata UI: http://localhost:8585
# → Airflow UI:      http://localhost:8080
```

### 初回セットアップ

1. http://localhost:8585 にアクセス（admin@open-metadata.org / admin）
2. Settings → Services → Databases → Add New Service で接続追加:

**Kensan PostgreSQL:**

| 項目 | 値 |
|------|-----|
| Service Type | Postgres |
| Host | postgres |
| Port | 5432 |
| Username | kensan |
| Password | kensan |
| Database | kensan |

**Dremio:**

| 項目 | 値 |
|------|-----|
| Service Type | Dremio |
| Host | dremio |
| Port | 9047 |

3. 各サービスで Ingestion Pipeline を作成 → Run してメタデータが収集されることを確認

### メモリ見積もり（追加分 ~3-4GB）

| サービス | ヒープ/メモリ |
|----------|-------------|
| OpenMetadata Server | 1GB |
| Elasticsearch | 512MB |
| OpenMetadata Postgres | ~256MB |
| Ingestion (Airflow) | ~1-2GB |

## Polaris Catalog 設定

Polaris は Apache Iceberg REST Catalog 仕様に準拠。OAuth2 `client_credentials` フローで認証。

```yaml
polaris:
  image: apache/polaris:latest
  environment:
    POLARIS_BOOTSTRAP_CREDENTIALS: "POLARIS,root,s3cr3t"
    AWS_REGION: us-east-1
    AWS_ACCESS_KEY_ID: kensan
    AWS_SECRET_ACCESS_KEY: kensan-minio
  volumes:
    - ./polaris-config/application.properties:/deployments/config/application.properties:ro
```

`application.properties` で `SKIP_CREDENTIAL_SUBSCOPING_INDIRECTION=true` を設定し、MinIO との互換性を確保（STS 不要）。

初回セットアップ: `python3 catalog/bootstrap_polaris.py` で catalog 作成 → `python3 catalog/init_catalog.py` でテーブル作成。

PyIceberg 接続:
```python
catalog = load_catalog("polaris", type="rest",
    uri="http://localhost:8181/api/catalog",
    credential="root:s3cr3t",
    scope="PRINCIPAL_ROLE:ALL",
    warehouse="kensan-lakehouse")
```

## DuckDB + Iceberg の注意点

DuckDB の `iceberg_scan()` は S3 上の Iceberg テーブルを直接読めるが、Polaris Catalog 経由のパス構造（UUID入りディレクトリ）と互換性の問題がある。

本プロジェクトでは **PyIceberg でArrowに読み込み → DuckDB に登録** するアプローチを採用：

```python
# PyIceberg → Arrow → DuckDB
arrow_table = catalog.load_table("silver.time_entries").scan().to_arrow()
con.register("silver_time_entries", arrow_table)
con.sql("SELECT * FROM silver_time_entries")
```

将来的にDuckDB の Iceberg REST Catalog サポートが改善されれば、直接クエリに移行可能。

## ディレクトリ構成

```
lakehouse/
├── .env                        # 接続設定
├── .gitignore
├── Makefile                    # コマンド一覧
├── pyproject.toml              # Python依存関係
├── README.md                   # このファイル
├── catalog/
│   ├── config.py               # Polaris/S3/PostgreSQL接続設定
│   ├── bootstrap_polaris.py    # Polaris catalog/principal 初期化
│   └── init_catalog.py         # Bronze/Silver/Goldテーブル定義
├── pipelines/
│   ├── bronze/
│   │   ├── ingest_postgres.py  # PostgreSQL → Bronze
│   │   └── ingest_loki.py      # Loki → Bronze (AI Explorer)
│   ├── silver/
│   │   ├── transform.py        # Bronze → Silver
│   │   ├── emotion_extractor.py # 日記感情抽出 (LLM)
│   │   ├── tag_profiler.py     # タグ使用統計集計
│   │   ├── trait_extractor.py  # 性格特性抽出 (LLM)
│   │   └── explorer_transform.py # Bronze Explorer → Silver Explorer
│   ├── gold/
│   │   └── aggregate.py        # Silver → Gold
│   └── maintenance/
│       ├── compaction.py       # Iceberg compaction & snapshot expiry
│       ├── reindex_chunks.py   # kensan-ai チャンクインデックス呼び出し
│       ├── weekly_review.py    # kensan-ai 週次レビュー自動生成呼び出し
│       └── prompt_optimization.py  # kensan-ai プロンプト自動最適化呼び出し
└── queries/
    ├── examples.sql            # SQLクエリ例
    └── query.py                # DuckDBインタラクティブシェル
```

## ロードマップ

| Phase | 内容 | 状態 |
|-------|------|------|
| 1 | MinIO + Polaris + PyIceberg + DuckDB基盤 | ✅ 完了 |
| 1.5 | AI Data Lakehouse統合（Bronze/Silver/Gold AI テーブル、外部ツール連携） | ✅ 完了 |
| 2 | 差分取り込み（ステートファイル、append-only/mutable テーブル分離） | ✅ 完了 |
| 2.5 | Dremio CE によるインタラクティブ SQL クエリ | ✅ 完了 |
| 2.6 | OpenMetadata によるメタデータカタログ | ✅ 完了 |
| 3 | k8s デプロイ、Trino 追加、OTel Collector 連携 | 未着手 |
| 4 | Gold層を kensan-ai のコンテキストとして利用（感情・関心・性格プロファイル） | ✅ 完了 |
