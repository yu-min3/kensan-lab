# Kensan（研鑽）

<p align="center">
  <img src="docs/design/kensan-logo-dark.svg" alt="Kensan Logo" width="200">
</p>

<p align="center">
  <strong>使うほど賢くなる、エンジニアのためのAIエージェント</strong>
</p>

<p align="center">
  日本語 | <a href="README.en.md">English</a>
</p>

<p align="center">
  <a href="#概要">概要</a> |
  <a href="#機能">機能</a> |
  <a href="#技術スタック">技術スタック</a> |
  <a href="#はじめかた">はじめかた</a> |
  <a href="#アーキテクチャ">アーキテクチャ</a> |
  <a href="#claude-code-による開発">Claude Code</a> |
  <a href="#プロジェクト構成">プロジェクト構成</a>
</p>

---

## 概要

Kensan は、エンジニアの自己研鑽を支援するパーソナル生産性プラットフォームです。目標管理・タイムブロック・学習ノート・AIチャットを統合し、39種類のツールを持つAIエージェントが進捗分析や計画提案を行います。

最大の特徴は**共進化**の仕組みです。エージェントは週次バッチで自身のプロンプトを Gemini で自己評価し、改善案を生成。ユーザーがブラインド A/B テストで採否を判断することで、使うほど賢くなるフィードバックループを実現しています。

---

## 機能

- **目標・タスク管理** - 年間目標 → マイルストーン → タスクの階層管理、カンバンボード
- **タイムブロック** - 日次・週次の時間計画をドラッグ&ドロップで作成
- **学習ノート** - TipTap ベースのリッチエディタ。日記・学習メモ・読書レビュー
- **AIチャット** - ADK ベースの Gemini エージェント。39ツール搭載、Read/Write 分離（読み取りは即実行、書き込みはユーザー承認）。Deferred Write Injection により、LLM の read ツール呼び出しをシグナルとして write ツールを動的に追加
- **プロンプト自己評価** - Gemini がエージェント自身のプロンプトを週次評価し、弱点の特定と改善版を自動生成
- **ブラインド A/B テスト** - 現行版と改善版を使い比べ、ユーザーが投票で採否を決定
- **AI 週次レビュー** - 1週間の行動データから振り返りレポートを自動生成
- **ファクト抽出** - 会話からユーザーの嗜好・習慣・スキルを自動抽出
- **分析ダッシュボード** - 目標への時間投資や生産性トレンドを可視化
- **データレイクハウス** - Medallion Architecture（Bronze/Silver/Gold）。Apache Iceberg + Dagster + Polaris REST Catalog
- **可観測性** - OpenTelemetry 完全対応（Grafana, Prometheus, Loki, Tempo）+ AI インタラクション・エクスプローラー

---

## 技術スタック

| レイヤー | 技術 |
|----------|------|
| **フロントエンド** | React 18, TypeScript, Vite, Zustand, Tailwind CSS 4, shadcn/ui, TipTap |
| **バックエンド** | Go 1.24, Chi, pgx, JWT 認証 |
| **AI サービス** | Python 3.12, FastAPI, Google ADK (Agent Development Kit), Gemini 2.0 Flash |
| **データベース** | PostgreSQL 16 + pgvector |
| **ストレージ** | MinIO（S3 互換オブジェクトストレージ） |
| **データ基盤** | Apache Iceberg, Dagster, Polaris REST Catalog |
| **可観測性** | OpenTelemetry, Grafana, Prometheus, Loki, Tempo |
| **インフラ** | Docker Compose, GCE (Google Compute Engine) |

---

## はじめかた

### 前提条件

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- [Google AI Studio API Key](https://aistudio.google.com/apikey)（Gemini 用）

### 1. リポジトリをクローン

```bash
git clone https://github.com/yu-min3/kensan-mockup.git
cd kensan-mockup
```

### 2. 環境変数を設定

```bash
cp .env.example .env
```

`.env` を編集して Google API キーを設定：

```bash
GOOGLE_API_KEY=your-google-api-key-here
```

### 3. 全サービスを起動

```bash
make up
```

Docker Compose で以下が起動します：
- フロントエンド: http://localhost:5173
- バックエンド API: http://localhost:8081-8091
- Grafana: http://localhost:3000

### 4. ログイン

デモアカウント：
- メール: `test@kensan.dev`
- パスワード: `password123`

### ローカル開発（Docker なし）

```bash
# フロントエンド
cd frontend
npm install
npm run dev

# バックエンド
cd backend && make build && make test

# AI サービス
cd kensan-ai
pip install -e .
uvicorn kensan_ai.main:app --reload --port 8089
```

---

## アーキテクチャ

**React SPA + Go マイクロサービス + Python AI サービス** の構成です。

ローカル開発では各サービスのポートに直接アクセスします。本番 (GCE) では nginx リバースプロキシが HTTPS で統一し、内部ポートは非公開です。

```
ローカル (make up)                    本番 GCE (make prod-up)
─────────────────                     ──────────────────────
Browser                               Browser
  ├── :5173  frontend                   └── :443 nginx (HTTPS)
  ├── :8081  user-service                     ├── /          → frontend
  ├── :8082  task-service                     ├── /api/v1/*  → Go services ×6
  ├── :8084  timeblock-service                └── /api/v1/agent/ → kensan-ai (SSE)
  ├── :8088  analytics-service
  ├── :8089  kensan-ai                 内部のみ: PostgreSQL, MinIO, Grafana,
  ├── :8090  memo-service                      Polaris, Dagster, OTel stack
  ├── :8091  note-service
  └── :3000  Grafana
```

### クリーンアーキテクチャ & マイクロサービス

各 Go サービスは**レイヤードアーキテクチャ**を厳格に適用しています。

```
Handler (HTTP) → Service (ビジネスロジック) → Repository (データアクセス) → PostgreSQL
```

- 各サービスは **300〜500行**。AI コーディングエージェントのコンテキストウィンドウに収まるサイズ
- 各境界に ISP ベースのインターフェースを配置し、テストと差し替えを容易に
- 新しいサービスの追加はコマンド1つ: `claude /new-service <name>`（後述）
- 共通関心事（認証ミドルウェア、エラーハンドリング、レスポンスエンベロープ）は `backend/shared/` に集約

この構造は **AI との協働開発** を前提に設計しています。Claude Code がサービス全体を読み、コントラクトを理解し、他のサービスへの副作用なく安全に変更できます。

### ドキュメント

詳細なアーキテクチャドキュメント：
- [技術ハイライト](docs/TECHNICAL_HIGHLIGHTS.md) - Agentic Loop、クリーンアーキテクチャ、Lakehouse フィードバックループ等の設計解説
- [全体アーキテクチャ](ARCHITECTURE.md)
- [バックエンド](backend/ARCHITECTURE.md)
- [フロントエンド](frontend/src/ARCHITECTURE.md)
- [AI サービス](kensan-ai/ARCHITECTURE.md)

---

## Claude Code による開発

Kensan は **AI コーディングエージェントと共に開発する**ことを前提に構築されています。[Claude Code](https://docs.anthropic.com/en/docs/claude-code) の設定ファイルにより、コーディング規約の強制、定型作業の自動化、ドキュメントの同期を実現しています。

### ルール（`.claude/rules/`）

7つのルールファイルがプロジェクトの規約を定義。Claude Code が自動的に読み込み、全てのやり取りで遵守します。

| ルール | 強制する内容 |
|--------|-------------|
| `backend-go.md` | レイヤードアーキテクチャ、サービスディレクトリ構造、ブートストラップパターン |
| `frontend-react.md` | コンポーネント階層、Zustand ストア、タイムゾーン処理 |
| `api-design.md` | レスポンスエンベロープ形式、エラーコード、URL パターン |
| `database.md` | マルチテナンシー（全テーブルに `user_id`）、UUID 主キー、マイグレーション命名規則 |
| `security.md` | JWT 認証、SQL パラメータ化、シークレットのハードコード禁止 |
| `testing.md` | テーブルドリブンテスト、インターフェースによるモック、マルチテナンシーのテストケース |
| `workflow.md` | 変更後のテスト自動実行、ARCHITECTURE.md の自動更新 |

### スキル（`.claude/skills/`）

6つのスラッシュコマンドで、プロジェクト規約に沿ったコードをスキャフォルド：

| コマンド | 機能 |
|---------|------|
| `/new-service <name>` | Go マイクロサービスの雛形を生成（cmd, handler, service, repository, Dockerfile, Makefile） |
| `/new-page <Page> <prefix>` | React ページをルート登録・ストア込みで作成 |
| `/new-endpoint <svc> <method> <path>` | API エンドポイントを handler, service, repository のセットで追加 |
| `/go-test` | Go テストを実行し、失敗時は自動修正 |
| `/build-check` | フロントエンド + バックエンドのビルドを並列実行 |
| `/code-review` | 未コミットの変更をプロジェクト規約に照らしてレビュー |

### ワークフロー自動化

`workflow.md` ルールにより、Claude Code は**指示がなくても**以下を自動実行します：

1. **テスト自動実行** - Go コードの変更で `make test`、フロントエンドの変更で `cd frontend && npm run build` を実行。失敗時は修正してから完了を報告
2. **ドキュメント自動更新** - 構造的な変更（新しいサービス、エンドポイント、ページ、スキーマ）があれば、該当する `ARCHITECTURE.md` を自動更新

高速なイテレーション中でも、テストは通り、ドキュメントは最新で、規約は遵守された状態が維持されます。

---

## プロジェクト構成

```
kensan-mockup/
├── frontend/             # React/TypeScript フロントエンド
├── backend/              # Go マイクロサービス
│   ├── services/         # 各サービスの実装
│   ├── shared/           # 共有ミドルウェア、認証、エラー処理
│   └── migrations/       # データベースマイグレーション
├── kensan-ai/            # Python AI サービス (ADK + Gemini 2.0 Flash)
├── lakehouse/            # データパイプライン (Dagster + Iceberg)
├── observability/        # 監視設定 (Grafana, Prometheus)
├── docs/                 # ドキュメント
│   ├── spec/             # API 仕様書
│   ├── adr/              # アーキテクチャ決定記録
│   ├── guides/           # セットアップ・開発ガイド
│   └── design/           # ブランドガイドライン・ロゴ
├── .claude/              # Claude Code 設定
│   ├── rules/            # 7つの規約ルールファイル
│   └── skills/           # 6つのスラッシュコマンド
├── e2e/                  # Playwright E2E テスト
├── k8s/                  # Kubernetes マニフェスト
├── deploy/               # 本番デプロイ設定
│   ├── nginx.conf        # nginx リバースプロキシ (HTTPS)
│   └── .env.prod.example # 本番環境変数テンプレート
├── scripts/gce-deploy.sh # GCE デプロイスクリプト
├── docker-compose.yml    # ローカル開発オーケストレーション
├── docker-compose.prod.yml # 本番 overlay (nginx, ポート非公開, CORS)
└── ARCHITECTURE.md       # 全体アーキテクチャドキュメント
```

---

## 環境変数

| 変数 | 必須 | デフォルト | 説明 |
|------|------|-----------|------|
| `GOOGLE_API_KEY` | Yes | - | Google GenAI API キー（Gemini 用） |
| `AI_PROVIDER` | No | `google` | AI プロバイダー |
| `GOOGLE_MODEL` | No | `gemini-2.0-flash` | Gemini モデル |
| `JWT_SECRET` | 本番のみ | `dev-secret-key-...` | JWT 署名鍵 |
| `DB_PASSWORD` | No | `kensan` | PostgreSQL パスワード |

全リストは `.env.example` を参照。

---

## ライセンス

本プロジェクトはハッカソン提出作品です。All rights reserved.
