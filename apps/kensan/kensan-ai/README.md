# Kensan AI

Kensan アプリケーションの AI サービス。Python (FastAPI) + Gemini API によるエージェントベースの対話・レビュー機能を提供する。

詳細なアーキテクチャは [ARCHITECTURE.md](ARCHITECTURE.md) を参照。

---

## 主な機能

- **エージェント対話** - Direct Tools (39個) を使った DB 直接操作つきチャット（SSE ストリーミング）
- **Read/Write 分離** - 書き込み操作は提案→承認の 2 ステップ
- **動的ツール選択** - メッセージ意図に基づき必要なツールのみを選択しトークン節約
- **週次レビュー** - 構造化された振り返りレポート生成
- **ファクト自動抽出** - 会話からユーザーの好み・習慣・スキルを非同期抽出
- **コンテキスト管理** - 時刻ベースの状況検出、A/B テスト、変数置換
- **セマンティック検索** - pgvector による埋め込みベクトル検索
- **外部情報取得** - Tavily API による Web 検索・ページ取得

---

## セットアップ

### Docker（推奨）

```bash
# プロジェクトルートから全サービス起動
make up
```

### ローカル開発

```bash
cd kensan-ai
pip install -e .
uvicorn kensan_ai.main:app --reload --port 8089
```

### テスト

```bash
cd kensan-ai
pytest
pytest --cov=kensan_ai  # カバレッジ付き
```

---

## API エンドポイント

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/health` | ヘルスチェック |
| POST | `/agent/stream` | エージェントストリーミング（SSE） |
| POST | `/agent/approve` | 書き込みアクションの承認・実行 |
| GET | `/conversations` | 会話一覧 |
| GET | `/conversations/{id}` | 会話詳細 |
| POST | `/interactions/{id}/feedback` | フィードバック送信 |
| GET | `/prompts` | AIコンテキスト一覧 |
| GET | `/prompts/{id}` | AIコンテキスト詳細 |
| PATCH | `/prompts/{id}` | AIコンテキスト更新（自動バージョン作成） |
| GET | `/prompts/{id}/versions` | バージョン履歴一覧 |
| GET | `/prompts/{id}/versions/{version_number}` | 特定バージョン取得 |
| POST | `/prompts/{id}/rollback/{version_number}` | 指定バージョンにロールバック |

---

## 環境変数

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `AI_PROVIDER` | `google` | AI プロバイダー |
| `GOOGLE_API_KEY` | - | Google GenAI API キー |
| `GOOGLE_MODEL` | `gemini-2.0-flash` | Google モデル |
| `EMBEDDING_PROVIDER` | `gemini` | 埋め込みプロバイダー |
| `GEMINI_EMBEDDING_MODEL` | `gemini-embedding-001` | Gemini 埋め込みモデル |
| `DB_HOST` | `localhost` | PostgreSQL ホスト |
| `DB_PORT` | `5432` | PostgreSQL ポート |
| `DB_USER` | `kensan` | PostgreSQL ユーザー |
| `DB_PASSWORD` | `kensan` | PostgreSQL パスワード |
| `DB_NAME` | `kensan` | PostgreSQL データベース名 |
| `JWT_SECRET` | `dev-secret-key` | JWT シークレット |
| `TAVILY_API_KEY` | - | Tavily API キー（Web 検索用） |
| `OTEL_ENABLED` | `false` | OpenTelemetry 有効化 |
| `OTEL_COLLECTOR_URL` | `localhost:4318` | OTel Collector エンドポイント |
| `LAKEHOUSE_ENABLED` | `false` | Lakehouse 書き込み有効化 |
