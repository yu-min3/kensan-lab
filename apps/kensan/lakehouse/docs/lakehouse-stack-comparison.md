# Lakehouse スタック比較

## 確定コンポーネント

以下は両プランで共通。選定済み。

| コンポーネント | 役割 | k8s目安メモリ |
|--------------|------|-------------|
| **Dagster** (webserver + daemon + user code) | ETLオーケストレーション + リネージ + データ品質 | 2.5GB |
| **Dagster PostgreSQL** | Dagster メタデータDB | 512MB |
| **dbt** (dagster-dbt統合) | SQL変換 (silver/gold層) | Dagster内で実行 |
| **Nessie** | Iceberg カタログ (Git-like branching) | 512MB |
| **MinIO** | S3互換オブジェクトストレージ | 512MB |
| | **小計** | **4.5GB** |

### Dagster の役割

パイプライン管理の中枢。以下を**1つのUI**で統合管理する。

| 機能 | 詳細 |
|------|------|
| **スケジュール管理** | cron式でパイプラインを定期実行。UIからON/OFF・手動実行も可能 |
| **Asset管理** | テーブル単位（Software-Defined Assets）で依存関係を宣言。bronze→silver→goldの流れが自動グラフ化 |
| **リネージ** | Asset間の依存関係がそのままリネージ図になる。コードを書くだけで自動生成 |
| **データ品質** | Freshness checks（鮮度監視）、Asset checks（カスタム品質チェック）を組込みで実行 |
| **実行履歴** | 全Assetの実行結果（成功/失敗/所要時間）を時系列で保持 |
| **リトライ** | 失敗時の自動リトライポリシー設定、UIからの手動再実行 |
| **センサー** | イベント駆動実行（新ファイル到着、外部トリガー等） |
| **アラート** | Slack/Email通知（失敗時、SLA違反時） |
| **k8s連携** | `dagster-k8s` で各Assetのマテリアライズをk8s Jobとして実行 |

### dbt の役割

silver/gold層の変換をSQLで宣言的に定義する。

| 機能 | 詳細 |
|------|------|
| **SQLベース変換** | SELECT文でモデルを定義。Pythonスクリプト不要 |
| **依存関係管理** | `ref()` でモデル間の依存を宣言。Dagsterが実行順序を自動解決 |
| **テスト** | `not_null`, `unique`, `accepted_values` 等のデータテストを定義可能 |
| **ドキュメント生成** | モデルの説明・カラム説明からドキュメントを自動生成 |
| **Dagster統合** | `dagster-dbt` でdbtモデルが自動的にDagster Assetになる |

### Nessie の役割

Icebergテーブルのメタデータをバージョン管理する。

| 機能 | 詳細 |
|------|------|
| **Git-likeブランチ** | `main` から `dev` ブランチを切り、検証後にマージ。本番データを壊すリスクなし |
| **コミット履歴** | テーブルへの変更がコミットとして記録される |
| **タグ** | 特定時点にタグを打ち、後から参照可能（Git tagと同じ） |
| **マルチテーブルトランザクション** | 複数テーブルの変更を1コミットでアトミックに反映 |
| **タイムトラベル** | 任意の過去の状態にクエリ可能 |
| **バックエンド** | In-memory, PostgreSQL, RocksDB, MongoDB, DynamoDB 等から選択 |

> **注意**: Dremioは将来的にNessieの機能をApache Polaris（incubating）に統合し、Nessieを廃止する計画を発表している。ただし時期は未定で、当面はNessieが利用可能。

---

## 残る選択: SQL + 分析UI層

### Option A: Dremio 単体

```
Dagster ──→ Iceberg/Nessie/MinIO ──→ Dremio (クエリエンジン + カタログUI + SQL Editor)
```

### Option B: Trino + Superset

```
Dagster ──→ Iceberg/Nessie/MinIO ──→ Trino (クエリエンジン) ──→ Superset (BI + SQL Lab)
```

---

## Dremio 詳細

### アーキテクチャ

- **エンジン**: Apache Arrow ベースの列指向インメモリエンジン (Sabot)
- **実行最適化**: Gandiva (LLVM JITコンパイル) でCPU SIMD命令を活用。Java実行比で最大70倍高速
- **ノード構成**: Coordinator + Executor。Zookeeperでノード管理
- **キャッシュ**: Columnar Cloud Cache (C3) でリモートストレージのデータをローカルSSDにキャッシュ

### できること

| カテゴリ | 機能 | 詳細 |
|---------|------|------|
| **SQL実行** | SQLエディタ | ブラウザ上で構文ハイライト付きのSQLを実行。結果はインラインで表示 |
| | ANSI SQL準拠 | SELECT, JOIN, Window関数, CTE, PIVOT/UNPIVOT, サブクエリ |
| | Iceberg DML | INSERT, UPDATE, DELETE, MERGE（Icebergテーブルのみ） |
| | タイムトラベル | スナップショットID指定で過去のデータにクエリ |
| | UDF | ユーザー定義関数（スカラー/テーブル、v22.0以降） |
| **データカタログ** | テーブル一覧 | ソース・スペース・フォルダ階層でブラウズ。カラム名・型をUI上で確認 |
| | 検索 | メタデータ・Wiki・タグの全文検索 |
| | データプレビュー | ワンクリックでテーブルの中身をプレビュー |
| | Wiki | データセットごとにMarkdownドキュメント（最大10万文字） |
| | タグ | データセットにタグ付け。検索・フィルタリングに利用 |
| | リネージ | データの出自と変換経路を可視化 |
| **高速化** | Reflections | 透過的マテリアライズドビュー。Raw（ソート/パーティション最適化コピー）とAggregation（事前集計）の2種類 |
| | 自動Reflections | クエリパターンを分析し、最適なReflectionを自動作成・管理 |
| | 結果キャッシュ | 同一クエリの結果をキャッシュして即時返却 |
| **Nessie連携** | ネイティブ統合 | UIからブランチ切り替え、タグ作成、テーブル操作が可能 |
| **接続プロトコル** | JDBC / ODBC | 標準ドライバ付属 |
| | Arrow Flight | 大量データ転送時にODBC/JDBC比で10-100倍高速 |
| | REST API (v3) | カタログ操作、SQL実行、ジョブ管理、Reflection管理、ユーザー管理 |
| **BI連携** | 外部ツール | Tableau, Power BI をUIからワンクリック起動 |

### できないこと

| カテゴリ | 詳細 |
|---------|------|
| **ダッシュボード** | チャート・グラフの作成機能なし。クエリ結果はテーブル表示のみ |
| **アクセス制御 (OSS)** | 全ユーザーが実質admin権限。ロール分離・行レベル/カラムレベルセキュリティはEnterprise版のみ |
| **外部認証 (OSS)** | LDAP, OIDC, SAML, Okta は Enterprise版のみ。ローカルユーザー/パスワードのみ |
| **ETLオーケストレーション** | パイプラインのスケジュール実行・依存管理は不可（Dagsterが担当） |
| **データ品質テスト** | プロファイリングはあるが、品質ルール定義・アラートは不可 |

### 対応データソース（主要）

| カテゴリ | ソース |
|---------|-------|
| オブジェクトストレージ | S3 (MinIO含む), Azure Blob/ADLS, GCS, HDFS |
| RDBMS | PostgreSQL, MySQL, SQL Server, Oracle, Db2 |
| クラウドDWH | Snowflake, Redshift, BigQuery, Synapse |
| NoSQL | MongoDB, Elasticsearch, Druid |
| レイクハウスカタログ | Nessie, Hive Metastore, AWS Glue, Iceberg REST, Unity Catalog |

### k8sリソース

| コンポーネント | メモリ |
|--------------|-------|
| Dremio (Coordinator + Executor) | 4GB |
| **小計** | **4GB** |

---

## Trino 詳細

### アーキテクチャ

- **エンジン**: Java ベースの分散SQLクエリエンジン。パイプライン型インメモリ実行
- **ノード構成**: Coordinator（1台、クエリ解析・スケジュール）+ Worker（N台、タスク実行）
- **最適化**: コストベースオプティマイザ (CBO)、動的フィルタリング、述語プッシュダウン
- **耐障害性**: Project Tardigrade でタスク/クエリレベルのリトライ対応（バッチ向け）
- **スピル**: メモリ超過時にローカルディスクへスピル（OOM回避）

### できること

| カテゴリ | 機能 | 詳細 |
|---------|------|------|
| **SQL実行** | ANSI SQL準拠 | SELECT, JOIN, Window関数, CTE (再帰含む), PIVOT/UNPIVOT, MATCH_RECOGNIZE, GROUPING SETS |
| | Iceberg DML | INSERT, UPDATE, DELETE, MERGE (v400+) |
| | タイムトラベル | `FOR TIMESTAMP AS OF`, `FOR VERSION AS OF` |
| | テーブルメンテナンス | `EXECUTE optimize`（ファイルコンパクション）, `expire_snapshots`, `remove_orphan_files` |
| | メタデータテーブル | `$snapshots`, `$manifests`, `$partitions`, `$history`, `$files` |
| | 統計収集 | `ANALYZE` コマンドでCBO用統計を収集 |
| **フェデレーション** | クロスソースJOIN | 40以上のコネクタ間で単一クエリでJOIN可能（例: Iceberg + PostgreSQL + MongoDB） |
| | プッシュダウン | RDBMS向けに述語・結合・集約をソース側にプッシュダウン（Dremioより高度） |
| **Nessie連携** | カタログタイプ | `iceberg.catalog.type=nessie` で設定。ブランチ/タグ指定はセッションプロパティで切り替え |
| **Icebergカタログ** | 複数対応 | Nessie, Hive Metastore, AWS Glue, REST Catalog, JDBC, Hadoop |
| **接続プロトコル** | JDBC | 公式ドライバあり |
| | Python | `trino` パッケージ (DB-API 2.0) + SQLAlchemy対応 |
| | CLI | 公式CLIツール |
| | dbt | `dbt-trino` アダプタ（本番利用可能） |
| **リソース管理** | リソースグループ | クエリの優先度・同時実行数・メモリ制限を階層的に設定 |
| **認証** | 多様 | パスワード, LDAP, Kerberos, OAuth2/OIDC, JWT, クライアント証明書（全てOSSで利用可） |
| **認可** | 柔軟 | ファイルベースACL, OPA, Apache Ranger, カスタムプラグイン。カラム/行レベルフィルタリング対応 |

### できないこと

| カテゴリ | 詳細 |
|---------|------|
| **Web UI** | ジョブ監視のみ（クエリ一覧、実行プラン、ワーカー状態）。SQLエディタ・データブラウジング機能なし |
| **データカタログ** | テーブル一覧・メタデータ管理・Wiki・タグ等の機能なし |
| **ダッシュボード** | チャート・グラフの作成機能なし |
| **データプレビュー** | UIからのワンクリックプレビューなし（SQL実行が必要） |
| **高速化レイヤー** | Reflections相当の透過的マテリアライズドビューなし。クエリ結果キャッシュもなし |
| **ODBC** | 公式ODBCドライバなし（Starburst商用版のみ） |

### 対応コネクタ（主要、40以上）

| カテゴリ | ソース |
|---------|-------|
| テーブルフォーマット | Iceberg, Delta Lake, Hudi, Hive |
| RDBMS | PostgreSQL, MySQL, SQL Server, Oracle, MariaDB, ClickHouse |
| クラウドDWH | Redshift, BigQuery |
| NoSQL | MongoDB, Cassandra, Elasticsearch/OpenSearch, Redis |
| ストリーミング | Kafka, Kinesis |
| その他 | Prometheus, Google Sheets, Pinot, Druid, Kudu |

> **Trinoの最大の強み**: フェデレーション。異なるデータソースを単一SQLで横断クエリできる。これはDremioも対応しているが、Trinoはコネクタ数と プッシュダウンの深さで上回る。

### k8sリソース

| コンポーネント | メモリ |
|--------------|-------|
| Trino Coordinator | 2GB |
| Trino Worker (×1) | 2GB |
| **小計** | **4GB** |

---

## Superset 詳細

### アーキテクチャ

- **エンジン**: Python (Flask) ベースのBIプラットフォーム
- **構成**: Webサーバー + Celery Worker (非同期クエリ) + Redis (キャッシュ/キュー) + PostgreSQL (メタデータ)
- **接続方式**: SQLAlchemy経由で各種DBに接続。Trino用ドライバ (`sqlalchemy-trino`) あり

### できること

| カテゴリ | 機能 | 詳細 |
|---------|------|------|
| **SQL Lab** | インタラクティブSQL | ブラウザ上でSQLを実行。結果のテーブル表示、CSV/Excelエクスポート |
| | クエリ履歴 | 過去のクエリ一覧、再実行、共有 |
| | スキーマブラウザ | 接続先DBのテーブル・カラム一覧を左ペインで確認 |
| | テンプレート | Jinjaテンプレートでパラメータ化クエリ |
| **ダッシュボード** | チャート作成 | 40以上のチャートタイプ（棒、折れ線、円、散布図、ヒートマップ、地図、ツリーマップ、サンキー等） |
| | ダッシュボード | 複数チャートを1画面に配置。フィルタ連動、自動リフレッシュ |
| | ドリルダウン | チャートからデータ詳細への掘り下げ |
| | 共有 | ダッシュボードのURL共有、埋め込み |
| **データセット管理** | 定義 | テーブル/ビューをデータセットとして登録。メトリクス・カラム説明を定義 |
| | Semantic Layer | 計算カラム、集計メトリクスを定義して再利用 |
| **認証/認可** | 多様 | OAuth, LDAP, OIDC, DB認証。ロールベースアクセス制御（データソース/ダッシュボード単位） |
| **アラート** | レポート | スケジュールでダッシュボードをEmail/Slack送信 |
| | アラート | SQLクエリ結果に基づく条件アラート |

### できないこと

| カテゴリ | 詳細 |
|---------|------|
| **クエリエンジン** | 自身ではクエリを実行しない。接続先DB（Trino等）に委譲 |
| **データカタログ** | テーブルメタデータの自動収集・管理機能なし |
| **リネージ** | データの出自追跡機能なし |
| **ETL** | データ変換・パイプライン管理機能なし |
| **テーブル定義同期** | 接続先DBのスキーマ変更は手動でデータセットを再同期する必要あり |
| **Nessieブランチ操作** | UIからのブランチ操作不可（Trino SQL経由で可能） |

### k8sリソース

| コンポーネント | メモリ |
|--------------|-------|
| Superset Web | 768MB |
| Celery Worker | 256MB |
| Redis | 256MB |
| Superset PostgreSQL | 256MB |
| **小計** | **~1.5GB** |

---

## 総合比較

### 機能マトリクス

| 機能 | Option A: Dremio | Option B: Trino + Superset |
|------|:----------------:|:--------------------------:|
| **アドホックSQL実行** | ○ SQL Runner | ○ SQL Lab |
| **テーブル一覧・カラム確認** | ○ カタログUI (ワンクリック) | △ SQL Lab (SHOW COLUMNS) |
| **データプレビュー** | ○ ワンクリック | △ SQL実行が必要 |
| **Wiki / ドキュメント** | ○ Markdownで記述可能 | ✕ |
| **タグ / 検索** | ○ 全文検索対応 | ✕ |
| **データリネージ** | ○ (カタログ内) | ✕ (Dagster側で管理) |
| **ダッシュボード作成** | ✕ | ○ 40+チャートタイプ |
| **チャート・グラフ** | ✕ | ○ |
| **スケジュールレポート** | ✕ | ○ Email/Slack送信 |
| **SQLアラート** | ✕ | ○ 条件ベースアラート |
| **Reflections (透過的高速化)** | ○ Raw + Aggregation | ✕ |
| **結果キャッシュ** | ○ | ✕ (Superset側に簡易キャッシュあり) |
| **Nessieブランチ操作 (UI)** | ○ ネイティブ | ✕ (SQL経由のみ) |
| **フェデレーション (多データソース)** | ○ (コネクタ数は中程度) | ◎ (40+コネクタ、プッシュダウン深い) |
| **認証 (OSS)** | △ ローカルユーザーのみ | ○ Trino: LDAP/OIDC/JWT, Superset: OAuth/LDAP |
| **認可 (OSS)** | ✕ 全員admin | ○ Trino: ファイルACL/OPA, Superset: RBAC |
| **JDBC** | ○ | ○ |
| **ODBC** | ○ 付属 | △ Starburst商用版のみ |
| **Arrow Flight** | ○ (高速データ転送) | ✕ |

### 非機能比較

| 観点 | Option A: Dremio | Option B: Trino + Superset |
|------|:----------------:|:--------------------------:|
| **追加コンポーネント数** | 1 | 4 (Trino, Superset, Redis, PG) |
| **追加メモリ** | 4GB | 5.5GB |
| **合計メモリ (確定分含む)** | **~8.5GB** | **~10GB** |
| **合計コンポーネント数** | **6** | **9** |
| **初期セットアップ** | 簡単 (Nessie接続のみ) | 中 (Trino→Nessie + Superset→Trino + データセット定義) |
| **スキーマ変更時の対応** | 自動反映 (メタデータリフレッシュ) | 手動 (Supersetデータセット再同期) |
| **Helm Chart** | 公式あり | 両方公式あり |
| **運用時のUI数** | 2 (Dagster + Dremio) | 3 (Dagster + Superset + Trino監視) |

### ポートフォリオ・キャリア比較

| 観点 | Option A: Dremio | Option B: Trino + Superset |
|------|:----------------:|:--------------------------:|
| **市場認知度** | 中 (レイクハウス界隈) | 高 (両方とも広く普及) |
| **エンタープライズ採用** | 中 | 高 (特にTrino) |
| **成果物の見栄え** | △ テーブル表示のみ | ○ ダッシュボードを見せられる |
| **Snowflake経験との相乗** | 低 | 高 (分散SQLエンジンの理解を示せる) |
| **転職市場での価値** | ニッチ | 高 (Trino/Superset は求人で見かける) |
| **技術選定を語れるか** | ○ (統合型の利点を説明できる) | ○ (分離型の利点を説明できる) |

---

## メリット・デメリットまとめ

### Option A: Dremio

| メリット | デメリット |
|---------|----------|
| **1コンポーネント**で SQL + カタログ + 高速化が完結 | ダッシュボード機能がなく、最終成果物が「クエリ結果の表」止まり |
| Nessieとネイティブ統合。UIからブランチ操作可能 | OSS版は全ユーザーがadmin権限。アクセス制御なし |
| Reflections で透過的にクエリ高速化（OSSで利用可） | 外部認証 (LDAP/OIDC) は Enterprise版のみ |
| Arrow Flight で高速データ転送 | 市場認知度がTrinoより低い |
| Wiki/タグでデータドキュメント管理可能 | Nessie→Polaris統合の先行き不透明感 |
| メモリ消費が少ない (4GB) | フェデレーションのコネクタ数・プッシュダウンはTrinoに劣る |

### Option B: Trino + Superset

| メリット | デメリット |
|---------|----------|
| ダッシュボードで視覚的な成果物を作れる | コンポーネント4つ追加。障害点が増える |
| Trino + Superset 両方の経験が履歴書に書ける | Supersetのテーブル定義は手動同期が必要 |
| 40+コネクタで将来の拡張性が高い | Nessie操作はSQL経由のみ（UIでのブランチ操作不可） |
| OSSでもLDAP/OIDC/OPA等の認証認可が完備 | メモリ1.5GB追加 |
| 本業Snowflakeとの対比で技術選定を語れる | Reflections相当の透過的高速化機能なし |
| Trinoのプッシュダウンが強力（RDBMS連携時） | 運用UI が3画面に分散 |
| スケジュールレポート・アラートが使える | Supersetの初期設定がやや面倒 |

---

## AI エージェント (Claude Code) との親和性

このプロジェクトでは、人間 (yu-min) が UI で探索的分析を行い、**Claude Code が ETL ジョブの実装・状況確認・トラブルシュートを担当する**。
各コンポーネントが Claude Code からプログラマティックに操作できるかは、技術選定の重要な軸となる。

### 確定コンポーネントの AI エージェント対応

| コンポーネント | CLI / API | Claude Code でできること |
|--------------|-----------|------------------------|
| **Dagster** | `dagster` CLI + GraphQL API | Asset定義の実装 (Python)、`dagster dev` でローカル実行、`dagster asset materialize` で手動トリガー、GraphQL APIでジョブ状態確認・実行履歴取得 |
| **dbt** | `dbt` CLI | モデル定義 (SQL)、`dbt run` / `dbt test` / `dbt build` の実行、`dbt ls` でモデル一覧、`dbt docs generate` でドキュメント生成 |
| **Nessie** | REST API (OpenAPI) | `curl` / `httpie` でブランチ作成・マージ・コミット履歴確認、テーブル一覧取得。全操作がHTTP REST |
| **MinIO** | `mc` CLI (MinIO Client) | バケット操作、ファイル一覧、容量確認。S3互換なので `aws s3` CLI も利用可能 |
| **PyIceberg** | Python ライブラリ | テーブルスキーマ確認、スナップショット一覧、メタデータ操作をPythonスクリプトで実行 |

**評価: 全て優秀**。確定コンポーネントは全て CLI / API / Python ライブラリ経由で完全操作可能。
Claude Code がパイプラインの実装からデバッグまで自律的に行える。

### Option A: Dremio の AI エージェント対応

| 操作 | 方法 | Claude Code での実行例 |
|------|------|----------------------|
| **SQL実行** | REST API (`POST /api/v3/sql`) | `curl -X POST http://localhost:9047/api/v3/sql -d '{"sql":"SELECT ..."}' -H "Authorization: Bearer <token>"` → ジョブIDが返る → `GET /api/v3/job/{id}/results` で結果取得 |
| **ジョブ状態確認** | REST API (`GET /api/v3/job/{id}`) | ジョブの成功/失敗/実行中を確認。実行時間、読み取り行数等の詳細も取得可 |
| **カタログ操作** | REST API (`/api/v3/catalog`) | テーブル一覧、カラム定義、Wiki/タグの取得・更新 |
| **Reflection管理** | REST API (`/api/v3/reflection`) | Reflectionの作成・状態確認・有効/無効切り替え |
| **認証トークン取得** | REST API (`POST /apiv2/login`) | ユーザー名/パスワードでログイン → トークン取得。またはPAT (Personal Access Token) を事前発行 |
| **テーブル定義確認** | REST API or JDBC | カタログAPIでスキーマ取得、または `trino` Python経由でSQLクエリ |
| **JDBC接続** | Python (`jaydebeapi` or `sqlalchemy-dremio`) | Python スクリプトからクエリ実行可能だが、ドライバのセットアップがやや手間 |

**課題**:
- REST API (v3) は充実しているが、**認証が2段階**（まずログインしてトークン取得、その後Bearer認証）
- JDBC 経由のPython接続は `jaydebeapi` (JVM依存) が必要で、環境構築が面倒
- Arrow Flight Python クライアント (`pyarrow.flight`) は高速だが、Dremio固有の認証ハンドリングが必要
- **公式CLIツールが存在しない**。全てREST APIを直接叩く必要がある

### Option B: Trino + Superset の AI エージェント対応

#### Trino

| 操作 | 方法 | Claude Code での実行例 |
|------|------|----------------------|
| **SQL実行** | `trino` CLI | `trino --server localhost:8080 --execute "SELECT * FROM iceberg.bronze.tasks_raw LIMIT 10"` |
| **SQL実行 (Python)** | `trino` パッケージ | `import trino; conn = trino.dbapi.connect(host='localhost'); cur = conn.cursor(); cur.execute("SELECT ...")` |
| **テーブル一覧** | CLI / Python | `SHOW TABLES FROM iceberg.bronze` |
| **カラム定義** | CLI / Python | `DESCRIBE iceberg.bronze.tasks_raw` または `SHOW COLUMNS FROM ...` |
| **Icebergメタデータ** | SQL | `SELECT * FROM iceberg.bronze."tasks_raw$snapshots"` でスナップショット一覧等 |
| **テーブルメンテナンス** | SQL | `ALTER TABLE ... EXECUTE optimize` (コンパクション)、`expire_snapshots`、`remove_orphan_files` |
| **統計収集** | SQL | `ANALYZE iceberg.bronze.tasks_raw` |
| **Nessieブランチ操作** | SQL | セッションプロパティでブランチ切り替え |
| **ジョブ状態確認** | REST API (`GET /v1/query/{id}`) | クエリの状態・統計をJSON取得 |

#### Superset

| 操作 | 方法 | Claude Code での実行例 |
|------|------|----------------------|
| **SQL実行** | REST API (`POST /api/v1/sqllab/execute`) | SQLをAPI経由で実行し結果取得 |
| **データセット管理** | REST API (`/api/v1/dataset/`) | データセットの作成・更新・同期 |
| **ダッシュボード管理** | REST API (`/api/v1/dashboard/`) | ダッシュボードの作成・更新・エクスポート/インポート |
| **チャート管理** | REST API (`/api/v1/chart/`) | チャートの作成・更新 |
| **認証** | REST API (`POST /api/v1/security/login`) | JWT トークン取得 |
| **CLI** | `superset` CLI | `superset import-dashboards`, `superset export-dashboards` でダッシュボードのJSON管理 |

### AI エージェント対応 比較表

| 観点 | Option A: Dremio | Option B: Trino + Superset |
|------|:----------------:|:--------------------------:|
| **公式CLI** | ✕ なし | ◎ `trino` CLI + `superset` CLI |
| **Python接続の容易さ** | △ JVM依存 or Arrow Flight | ◎ `pip install trino` で即利用 |
| **SQL実行のシンプルさ** | △ REST API 2段階認証 | ◎ CLI一行で実行可能 |
| **テーブル定義確認** | ○ REST API | ◎ `DESCRIBE` / `SHOW COLUMNS` |
| **ジョブ監視** | ○ REST API | ○ REST API |
| **ダッシュボード自動化** | ✕ (機能自体がない) | ○ REST API + CLI |
| **設定のコード管理** | △ REST APIでソース定義を取得可能 | ○ Trino: properties ファイル、Superset: CLI export |
| **環境構築の手間** | △ REST APIラッパーを自作する必要 | ○ 標準ツールで完結 |

### Claude Code の典型的なワークフロー

**Option A (Dremio) の場合**:
```bash
# 認証トークン取得
TOKEN=$(curl -s -X POST http://localhost:9047/apiv2/login \
  -H 'Content-Type: application/json' \
  -d '{"userName":"yu","password":"..."}' | jq -r '.token')

# SQL実行 (2ステップ: 投げる → 結果取得)
JOB_ID=$(curl -s -X POST http://localhost:9047/api/v3/sql \
  -H "Authorization: _dremio${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"sql":"SELECT count(*) FROM nessie.bronze.tasks_raw"}' | jq -r '.id')

curl -s http://localhost:9047/api/v3/job/${JOB_ID}/results \
  -H "Authorization: _dremio${TOKEN}"
```

**Option B (Trino) の場合**:
```bash
# SQL実行 (1行で完結)
trino --server localhost:8080 --execute \
  "SELECT count(*) FROM iceberg.bronze.tasks_raw"

# テーブル定義確認
trino --server localhost:8080 --execute \
  "DESCRIBE iceberg.bronze.tasks_raw"

# Icebergスナップショット確認
trino --server localhost:8080 --execute \
  "SELECT * FROM iceberg.bronze.\"tasks_raw\$snapshots\" ORDER BY committed_at DESC LIMIT 5"
```

### 結論: AI エージェント親和性では Option B が明確に優位

Trino は公式 CLI と Python パッケージが充実しており、Claude Code が**ワンライナーでSQL実行・テーブル確認・メンテナンス操作**を行える。
Dremio は REST API は存在するが、CLI がなく、認証が2段階で、Python接続にJVM依存があり、エージェントが自律的に操作するにはラッパースクリプトの整備が必要。

Dagster のパイプライン実装（Python/dbt）は両プランで同一なので差がない。
**差が出るのは「データの中身を確認する」「テーブルの状態を調べる」「問題を診断する」場面**で、ここで Trino の CLI が圧倒的に使いやすい。

---

## 全体構成図

### Option A: Dagster + Dremio (~8.5GB)

```
データソース              Dagster                     Iceberg              分析
──────────              ───────                     ───────              ────
                   ┌─────────────────┐
PostgreSQL ──┐     │ Bronze (Python)  │                              ┌──────────┐
Web API    ──┼────→│ Silver (dbt)     │──→ Nessie ──→ MinIO ──────→│  Dremio  │
RSS/Scrape ──┘     │ Gold   (dbt)     │                              │          │
                   │                  │                              │ SQL実行   │
                   │ スケジュール管理   │                              │ カタログ  │
                   │ リネージ          │                              │ Wiki/タグ │
                   │ データ品質        │                              │ Reflections│
                   └─────────────────┘                              └──────────┘
                       UI ①                                            UI ②
```

### Option B: Dagster + Trino + Superset (~10GB)

```
データソース              Dagster                     Iceberg              分析
──────────              ───────                     ───────              ────
                   ┌─────────────────┐
PostgreSQL ──┐     │ Bronze (Python)  │                              ┌──────────┐
Web API    ──┼────→│ Silver (dbt)     │──→ Nessie ──→ MinIO ──────→│  Trino   │
RSS/Scrape ──┘     │ Gold   (dbt)     │                              │(クエリ)  │
                   │                  │                              └────┬─────┘
                   │ スケジュール管理   │                                   │
                   │ リネージ          │                              ┌────▼─────┐
                   │ データ品質        │                              │ Superset │
                   └─────────────────┘                              │          │
                       UI ①                             UI ③(監視)   │ SQL Lab  │
                                                       Trino Web UI  │ ダッシュボード│
                                                                     │ アラート  │
                                                                     └──────────┘
                                                                        UI ②
```
