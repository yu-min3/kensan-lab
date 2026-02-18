# Kensan 全体アーキテクチャ

Kensanは、エンジニアの自己改善を支援するパーソナル生産性アプリケーションです。時間管理、タスク管理、学習記録、AI週次レビューを統合し、目標達成をサポートします。

---

## 目次

1. [システム全体像](#システム全体像)
2. [技術スタック一覧](#技術スタック一覧)
3. [サービス構成](#サービス構成)
4. [画面構成とユーザーフロー](#画面構成とユーザーフロー)
5. [データフロー](#データフロー)
6. [認証・セキュリティ](#認証セキュリティ)
7. [データベース設計](#データベース設計)
8. [フロントエンドアーキテクチャ](#フロントエンドアーキテクチャ)
9. [バックエンドアーキテクチャ](#バックエンドアーキテクチャ)
10. [AIサービスアーキテクチャ](#aiサービスアーキテクチャ)
11. [Observability](#observability)
12. [詳細ドキュメント](#詳細ドキュメント)

---

## システム全体像

Kensanは **React SPA + Goマイクロサービス + Python AIサービス** の3層構成です。

```mermaid
graph TB
    subgraph "クライアント"
        Browser["ブラウザ<br/>React SPA"]
    end

    subgraph "バックエンドサービス群"
        direction TB
        subgraph "Go マイクロサービス (7サービス)"
            US["user-service<br/>:8081<br/>認証・設定"]
            TS["task-service<br/>:8082<br/>目標・タスク"]
            TBS["timeblock-service<br/>:8084<br/>時間管理"]
            RS["routine-service<br/>:8085<br/>ルーティン"]
            AS["analytics-service<br/>:8088<br/>分析"]
            MS["memo-service<br/>:8090<br/>メモ"]
            NS["note-service<br/>:8091<br/>ノート"]
        end

        subgraph "Python AIサービス"
            AI["kensan-ai<br/>:8089<br/>チャット・レビュー"]
        end
    end

    subgraph "データストア"
        PG[("PostgreSQL 16<br/>+ pgvector")]
        MinIO[("MinIO<br/>オブジェクトストレージ")]
    end

    subgraph "外部API"
        Gemini["Gemini API<br/>チャット・レビュー・抽出・埋め込み"]
    end

    Browser -->|"REST API<br/>JWT認証"| US
    Browser -->|"REST API"| TS
    Browser -->|"REST API"| TBS
    Browser -->|"REST API"| RS
    Browser -->|"REST API"| AS
    Browser -->|"REST API"| MS
    Browser -->|"REST API"| NS
    Browser -->|"REST API + SSE"| AI

    US --> PG
    TS --> PG
    TBS --> PG
    RS --> PG
    AS --> PG
    MS --> PG
    NS --> PG
    NS --> MinIO
    AI --> PG
    AI -->|読み取り専用| MinIO
    AI --> Gemini
```

### システムの特徴

- **マイクロサービス構成**: ドメインごとに独立したGoサービス（共有DB）
- **AIネイティブ**: Gemini APIによるチャット、週次レビュー、ファクト自動抽出
- **タイムゾーン対応**: DBはUTC保存、フロントエンドでローカル変換
- **マルチテナント**: 全テーブルに`user_id`カラムでデータ完全分離

---

## 技術スタック一覧

| レイヤー | 技術 | バージョン | 用途 |
|---------|------|----------|------|
| **フロントエンド** | React | 18.3 | UIフレームワーク |
| | TypeScript | 5.6 | 型システム |
| | Vite | 6.x | ビルドツール |
| | Zustand | 5.x | 状態管理 |
| | React Router | 7.x | ルーティング |
| | Tailwind CSS | 4.x | スタイリング |
| | shadcn/ui | - | UIコンポーネント |
| | TipTap | 3.16 | リッチテキストエディタ |
| | Recharts | 3.6 | チャート |
| **バックエンド** | Go | 1.24.0 | サービス実装 |
| | chi | v5.1.0 | HTTPルーター |
| | pgx | v5.7.2 | PostgreSQLドライバ |
| | slog + otelslog | Go標準 + v0.14.0 | 構造化ログ（OpenTelemetry連携） |
| | golang-jwt | v5.2.1 | JWT認証 |
| **AIサービス** | Python | 3.12+ | AIサービス実装 |
| | FastAPI | 0.115+ | Webフレームワーク |
| | asyncpg | 0.30+ | 非同期DBドライバ |
| | Google GenAI SDK | 1.0+ | Gemini API (チャット・埋め込み) |
| **インフラ** | PostgreSQL | 16 | メインDB + pgvector |
| | MinIO | - | オブジェクトストレージ (ノートコンテンツ) |
| | Docker Compose | - | ローカル開発 |

---

## サービス構成

### サービス一覧とドメイン責務

```mermaid
graph LR
    subgraph "認証・設定"
        US["user-service :8081"]
    end

    subgraph "タスク管理"
        TS["task-service :8082"]
    end

    subgraph "時間管理"
        TBS["timeblock-service :8084"]
        RS["routine-service :8085"]
    end

    subgraph "記録"
        NS["note-service :8091"]
        MS["memo-service :8090"]
    end

    subgraph "分析・AI"
        AS["analytics-service :8088"]
        AI["kensan-ai :8089"]
    end
```

| サービス | ポート | 言語 | ドメイン | 主な責務 |
|---------|--------|------|---------|---------|
| user-service | 8081 | Go | 認証・設定 | ユーザー登録、ログイン、JWT発行、ユーザー設定 |
| task-service | 8082 | Go | タスク管理 | 目標(Goal)、マイルストーン、タグ、タスクのCRUD |
| timeblock-service | 8084 | Go | 時間管理 | 予定(TimeBlock)、実績(TimeEntry)、タイマー |
| routine-service | 8085 | Go | ルーティン | 繰り返しタスクの管理 |
| analytics-service | 8088 | Go | 分析 | 週間/月間サマリー、目標進捗 |
| memo-service | 8090 | Go | メモ | クイックメモ（スクラッチパッド） |
| note-service | 8091 | Go | ノート | 日記、学習記録、一般ノート、読書レビュー |
| kensan-ai | 8089 | Python | AI | チャット、週次レビュー、ファクト抽出 |

### ドメインモデルの関係

```mermaid
erDiagram
    User ||--o| UserSettings : "設定"
    User ||--o{ Goal : "目標"
    Goal ||--o{ Milestone : "マイルストーン"
    Milestone ||--o{ Task : "タスク"
    Task ||--o{ Task : "サブタスク"
    Task }o--o{ Tag : "タグ付け"

    User ||--o{ TimeBlock : "予定"
    User ||--o{ TimeEntry : "実績"
    User ||--o| RunningTimer : "稼働中タイマー"

    User ||--o{ Note : "ノート"
    NoteType ||--o{ Note : "タイプ定義"
    Note }o--o{ Tag : "タグ付け"

    User ||--o{ Memo : "メモ"
    User ||--o{ RoutineTask : "ルーティン"

    User ||--o{ AIInteraction : "AI会話"
    User ||--o| UserMemory : "AIメモリ"
    User ||--o{ UserFact : "抽出ファクト"
    User ||--o{ AIReviewReport : "週次レビュー"
```

---

## 画面構成とユーザーフロー

### 画面一覧

```mermaid
graph TB
    subgraph "公開画面"
        Login["/login<br/>ログイン"]
        Setup["/settings<br/>初期設定"]
    end

    subgraph "日次ワークフロー"
        Daily["/ (DailyPage)<br/>今日の予定・実績タイムライン"]
        Briefing["/briefing<br/>朝のブリーフィング"]
        Reflection["/reflection<br/>夕方の振り返り"]
    end

    subgraph "計画・管理"
        Weekly["/weekly<br/>週間カレンダー"]
        Tasks["/tasks<br/>目標・タスク管理"]
        Routines["/routines<br/>ルーティン管理"]
    end

    subgraph "記録"
        Notes["/notes<br/>ノート一覧"]
        NoteEdit["/notes/:id<br/>ノート編集"]
    end

    subgraph "分析・AI"
        Analytics["/analytics<br/>分析レポート"]
        AIReview["/ai-review<br/>AI週次レビュー"]
        Interactions["/interactions<br/>AI Interaction Explorer"]
    end

    Login -->|認証成功| Daily
    Login -->|未設定| Setup
    Setup -->|設定完了| Daily
```

### 1日のユーザーフロー

```mermaid
flowchart LR
    subgraph "朝"
        B[ブリーフィング]
        P[予定の確認・調整]
    end

    subgraph "日中"
        T[タイマーで作業]
        TB[タイムブロック記録]
        M[メモ・ノート]
    end

    subgraph "夕方"
        R[振り返り]
        A[分析確認]
    end

    subgraph "週末"
        WR[週次レビュー]
    end

    B --> P --> T --> TB --> M --> R --> A
    A -.->|週次| WR
```

---

## データフロー

### 典型的なユーザー操作

```mermaid
sequenceDiagram
    participant U as ブラウザ
    participant Z as Zustandストア
    participant A as APIサービス
    participant H as HttpClient
    participant B as Goサービス
    participant DB as PostgreSQL

    U->>Z: アクション呼び出し (例: addTask)
    Z->>A: APIサービス呼び出し
    A->>H: HTTP リクエスト構築
    H->>H: JWT Authorizationヘッダー付与
    H->>B: POST /api/v1/tasks
    B->>B: JWT検証 → user_id抽出
    B->>DB: INSERT INTO tasks
    DB-->>B: 新規レコード
    B-->>H: JSON レスポンス {data, meta}
    H-->>A: dataフィールドを抽出
    A-->>Z: 変換済みエンティティ
    Z->>Z: 状態更新 (set)
    Z-->>U: 再レンダリング
```

### AIチャットのデータフロー

```mermaid
sequenceDiagram
    participant U as ブラウザ
    participant AI as kensan-ai
    participant Ctx as コンテキスト解決
    participant Agent as AgentRunner
    participant LLM as Gemini API
    participant Tools as ツールレジストリ
    participant DB as PostgreSQL

    U->>AI: POST /agent/stream {message}
    AI->>AI: JWT → user_id抽出
    AI->>Ctx: 状況検出 + コンテキスト読込
    Ctx->>DB: ai_contexts取得
    Ctx->>Ctx: {変数}をユーザーデータで置換
    Ctx-->>AI: システムプロンプト + ツール設定

    AI->>Agent: run(message, user_id)

    loop エージェントループ (最大10ターン)
        Agent->>LLM: messages + tools
        LLM-->>Agent: レスポンス

        alt ツール呼び出しあり
            Agent->>Agent: user_idを自動注入
            Agent->>Tools: execute_tool(name, args)
            Tools->>DB: SQLクエリ
            DB-->>Tools: 結果
            Tools-->>Agent: ツール結果
        else テキスト応答
            Agent-->>AI: 最終レスポンス
        end
    end

    AI->>DB: ai_interactionsに記録
    AI--)AI: 非同期: ファクト抽出 → プロフィール要約
    AI-->>U: SSE ストリーミング
```

### タイムゾーン変換フロー

```mermaid
flowchart TB
    subgraph "ユーザー操作"
        Input["ローカル時刻入力<br/>例: 2026-01-27 09:00 JST"]
    end

    subgraph "フロントエンド API層"
        Convert["localToUtcDatetime()<br/>→ 2026-01-27T00:00:00.000Z"]
    end

    subgraph "バックエンド"
        Store["TIMESTAMPTZ として UTC 保存"]
    end

    subgraph "フロントエンド 表示層"
        Display["getLocalTime()<br/>→ 09:00"]
    end

    subgraph "ユーザー表示"
        Output["ローカル時刻で表示<br/>09:00"]
    end

    Input --> Convert --> Store --> Display --> Output
```

DBはUTC保存、フロントエンドでローカル変換する設計。変換ユーティリティは `frontend/src/lib/timezone.ts` に集約。

---

## 認証・セキュリティ

### 認証フロー

```mermaid
sequenceDiagram
    participant B as ブラウザ
    participant US as user-service
    participant Other as 他サービス

    B->>US: POST /auth/login {email, password}
    US->>US: bcryptでパスワード検証
    US->>US: JWT生成 (HS256, 24時間有効)
    US-->>B: {token, user}

    B->>B: Zustand persist → localStorage

    B->>Other: GET /tasks<br/>Authorization: Bearer <token>
    Other->>Other: JWT検証 → user_id抽出
    Other-->>B: {data, meta}

    Note over B: 401応答時 → 自動ログアウト
```

### セキュリティ設計

| 項目 | 実装 |
|------|------|
| 認証方式 | JWT (HS256) |
| トークン有効期限 | 24時間 |
| パスワードハッシュ | bcrypt |
| データ分離 | 全テーブル `user_id` によるマルチテナント |
| JWT自動注入 | HttpClient がリクエストに自動付与 |
| AI tool user_id注入 | AgentRunner が自動注入 (LLMに依存しない) |
| トレース伝搬 | HttpClient が W3C `traceparent` ヘッダーを自動生成 |

---

## データベース設計

### テーブル構成概要

```mermaid
graph TB
    subgraph "認証・設定"
        users["users"]
        user_settings["user_settings"]
    end

    subgraph "タスク管理"
        goals["goals"]
        milestones["milestones"]
        tags["tags"]
        tasks["tasks"]
        task_tags["task_tags"]
    end

    subgraph "時間管理"
        time_blocks["time_blocks<br/>(予定)"]
        time_entries["time_entries<br/>(実績)"]
        running_timers["running_timers"]
        routine_tasks["routine_tasks"]
    end

    subgraph "記録"
        note_types["note_types<br/>(データ駆動)"]
        notes["notes"]
        note_tags["note_tags"]
        memos["memos"]
    end

    subgraph "AI"
        ai_contexts["ai_contexts<br/>(プロンプト設定)"]
        ai_interactions["ai_interactions"]
        ai_review_reports["ai_review_reports"]
        user_memory["user_memory"]
        user_facts["user_facts"]
        note_content_chunks["note_content_chunks<br/>(ベクトル検索)"]
    end

    users --> user_settings
    users --> goals
    goals --> milestones
    milestones --> tasks
    tasks --> task_tags
    tags --> task_tags
    users --> time_blocks
    users --> time_entries
    users --> notes
    note_types --> notes
    notes --> note_tags
    users --> ai_interactions
    users --> user_memory
    users --> user_facts
```

### 主要な設計原則

| 原則 | 詳細 |
|------|------|
| **UUID主キー** | PostgreSQL uuid-ossp拡張 |
| **マルチテナント** | 全テーブルに`user_id`でデータ完全分離 |
| **UTC保存** | TIMESTAMPTZ型、フロントで変換 |
| **非正規化** | TimeBlock/TimeEntry/NoteにGoal名・色を複製 (JOIN回避) |
| **同期トリガー** | Goal/Milestone/Task名変更時に非正規化フィールドを自動同期 |
| **タイムスタンプ自動更新** | `updated_at`トリガーによる自動更新 |
| **データ駆動タイプ** | note_typesテーブルでノートタイプを管理 (ハードコード不要) |
| **ベクトル検索** | pgvectorによるセマンティック検索 |

---

## フロントエンドアーキテクチャ

### レイヤー構成

```mermaid
graph TB
    subgraph "UIレイヤー"
        Pages["ページ<br/>DailyPage, T01_TaskManagement, ..."]
        Layout["レイアウト<br/>Header, Sidebar"]
        Domain["ドメインコンポーネント<br/>TaskCard, TimeBlockTimeline, ..."]
        UI["UIプリミティブ (shadcn/ui)<br/>Button, Card, Dialog, ..."]
    end

    subgraph "状態管理レイヤー"
        Auth["useAuthStore<br/>認証・トークン"]
        Settings["useSettingsStore<br/>タイムゾーン・テーマ"]
        Stores["ドメインストア<br/>useGoalStore, useTaskStore,<br/>useTimeBlockStore, useNoteStore, ..."]
    end

    subgraph "APIレイヤー"
        Services["APIサービス<br/>tasksApi, timeblocksApi, notesApi, ..."]
        Client["HttpClient<br/>JWT自動付与・エラーハンドリング"]
    end

    Pages --> Domain
    Pages --> Stores
    Domain --> UI
    Layout --> UI
    Stores --> Services
    Services --> Client
    Auth --> Client

    style Pages fill:#dbeafe
    style Stores fill:#dcfce7
    style Client fill:#fef3c7
```

15のZustandストアが各ドメインの状態を管理。`createCrudStore` ファクトリで標準CRUDパターンを統一化。認証(useAuthStore)・設定(useSettingsStore)のみlocalStorage永続化。

> 詳細: [frontend/src/ARCHITECTURE.md](frontend/src/ARCHITECTURE.md)

---

## バックエンドアーキテクチャ

### レイヤードアーキテクチャ (全サービス共通)

```mermaid
graph TB
    subgraph "HTTPレイヤー"
        MW["ミドルウェアチェーン<br/>RequestID → OTelTrace → Logger → CORS → Auth"]
        Handler["Handler<br/>リクエスト解析・レスポンス整形"]
    end

    subgraph "ビジネスロジックレイヤー"
        Service["Service<br/>ドメインバリデーション・ルール"]
    end

    subgraph "データアクセスレイヤー"
        Repo["Repository<br/>SQLクエリ・行スキャン"]
    end

    subgraph "共通基盤 (shared/)"
        Bootstrap["Bootstrap<br/>サービス初期化"]
        Config["Config<br/>環境変数"]
        AuthPkg["Auth<br/>JWT管理"]
        MWPkg["Middleware<br/>共通ミドルウェア"]
        Errors["Errors<br/>エラーパッケージ"]
        Telemetry["Telemetry<br/>OpenTelemetry"]
    end

    MW --> Handler
    Handler --> Service
    Service --> Repo
    Repo --> DB[("PostgreSQL")]

    Bootstrap --> MW
    Bootstrap --> Config
    Bootstrap --> AuthPkg
    Bootstrap --> MWPkg
    Bootstrap --> Telemetry
    Service --> Errors
```

全サービスが `Handler → Service → Repository` の3層を厳守。共通基盤 `shared/` パッケージで初期化・認証・ミドルウェア・テレメトリを一元管理。

> 詳細: [backend/ARCHITECTURE.md](backend/ARCHITECTURE.md)

---

## AIサービスアーキテクチャ

### コア概念

kensan-aiは**エージェントベース**のアーキテクチャで、LLM APIのDirect Tools (Function Calling) を使用してユーザーデータに直接アクセスします。

```mermaid
graph TB
    subgraph "エージェント"
        Chat["チャットエージェント<br/>汎用会話 + タスク管理"]
        Review["週次レビューエージェント<br/>構造化振り返り"]
        Planning["計画提案エージェント<br/>JSON構造化出力"]
    end

    subgraph "コンテキスト管理"
        Detect["状況検出<br/>(briefing/evening/weekly/chat)"]
        Resolve["コンテキスト解決<br/>(DBからプロンプト読込)"]
        Replace["変数置換<br/>({user_memory}, {today_schedule}, ...)"]
        AB["A/Bテスト<br/>(トラフィック分割)"]
    end

    subgraph "Direct Tools (39個)"
        DBTools["DB操作<br/>get_tasks, create_time_block, ..."]
        MemTools["メモリ<br/>get_user_memory, get_user_facts, ..."]
        SearchTools["検索<br/>semantic_search, keyword_search, ..."]
        WebTools["外部<br/>web_search, web_fetch (Tavily)"]
    end

    subgraph "メモリシステム"
        Logger["InteractionLogger<br/>会話記録"]
        Extractor["FactExtractor<br/>ファクト自動抽出"]
        Summarizer["ProfileSummarizer<br/>プロフィール要約"]
    end

    Chat --> Detect
    Detect --> Resolve
    Resolve --> Replace
    Resolve --> AB
    Chat --> DBTools
    Chat --> MemTools
    Chat --> SearchTools
    Chat --> WebTools
    Review --> DBTools

    Logger --> Extractor
    Extractor --> Summarizer
```

### 動的ツール選択

全ツールを毎回送信するとトークンコストが増大するため、メッセージの意図に基づき必要なツールのみを選択:

```mermaid
flowchart LR
    Msg["ユーザーメッセージ"] --> Analyze["意図分析<br/>Read/Write判定<br/>+ キーワードマッチ"]

    Analyze --> Core["core<br/>(常に含む)"]
    Analyze -->|書込キーワードあり| Write["Writeグループ<br/>planning, task, notes_write..."]
    Analyze -->|目標・分析の話題| Read["Readグループ<br/>goals_read, analytics..."]
    Analyze -->|検索の話題| Search["search グループ"]
    Analyze -->|Web検索の話題| Web["web グループ"]

    Core --> Select["選択されたツールのみ送信<br/>例: 7/39ツール"]
    Write --> Select
    Read --> Select
    Search --> Select
    Web --> Select
```

### メモリ構築パイプライン

```mermaid
flowchart LR
    subgraph "リアルタイム"
        Chat["チャット"]
        Log["記録"]
        Extract["ファクト抽出<br/>(非同期)"]
    end

    subgraph "ストレージ"
        Interactions[("ai_interactions")]
        Facts[("user_facts")]
        Memory[("user_memory")]
    end

    subgraph "次回チャット"
        Prompt["{user_memory}<br/>変数としてプロンプトに注入"]
    end

    Chat --> Log --> Interactions
    Log -.->|async| Extract --> Facts
    Facts -.->|新factあれば| Summarize["プロフィール要約"] --> Memory
    Memory --> Prompt --> Chat
```

> 詳細: [kensan-ai/ARCHITECTURE.md](kensan-ai/ARCHITECTURE.md)

---

## Observability

### OpenTelemetry統合

Go/Python両方のサービスがOpenTelemetryに対応 (`OTEL_ENABLED=true`で有効化):

```mermaid
graph LR
    subgraph "Goサービス群"
        GoTrace["HTTPスパン + DBスパン"]
        GoMetrics["REDメトリクス"]
    end

    subgraph "kensan-ai"
        PyTrace["FastAPIスパン + DBスパン"]
        PyMetrics["GenAIメトリクス<br/>(token.usage, operation.duration)"]
    end

    subgraph "フロントエンド"
        FETrace["traceparent<br/>ヘッダー伝搬"]
    end

    Collector["OTel Collector<br/>:4318 (OTLP HTTP)"]
    Tempo["Tempo<br/>(トレース)"]
    Prom["Prometheus<br/>(メトリクス)"]
    Loki["Loki<br/>(ログ)"]
    Grafana["Grafana<br/>(可視化)"]

    GoTrace --> Collector
    GoMetrics --> Collector
    PyTrace --> Collector
    PyMetrics --> Collector
    FETrace --> GoTrace
    FETrace --> PyTrace

    Collector --> Tempo
    Collector --> Prom
    Collector --> Loki

    Tempo --> Grafana
    Prom --> Grafana
    Loki --> Grafana
```

Traces↔Logs が双方向リンクされ、トレースIDをキーにドリルダウン可能。

> 詳細: [observability/ARCHITECTURE.md](observability/ARCHITECTURE.md)

---

## 詳細ドキュメント

各コンポーネントの詳細なアーキテクチャドキュメント:

| ドキュメント | 内容 |
|------------|------|
| [backend/ARCHITECTURE.md](backend/ARCHITECTURE.md) | Goマイクロサービス: 共通パッケージ、レイヤー設計、DBスキーマ、API仕様 |
| [frontend/src/ARCHITECTURE.md](frontend/src/ARCHITECTURE.md) | フロントエンド: コンポーネント階層、Zustandストア、APIクライアント、タイムゾーン変換 |
| [kensan-ai/ARCHITECTURE.md](kensan-ai/ARCHITECTURE.md) | AIサービス: Direct Tools、エージェント、コンテキスト管理、メモリシステム |
| [observability/ARCHITECTURE.md](observability/ARCHITECTURE.md) | Observability: OTel, Grafana, Tempo, Loki, Prometheus |

各サービスの個別ドキュメント:

| ドキュメント | 内容 |
|------------|------|
| [backend/services/user/ARCHITECTURE.md](backend/services/user/ARCHITECTURE.md) | user-service |
| [backend/services/task/ARCHITECTURE.md](backend/services/task/ARCHITECTURE.md) | task-service |
| [backend/services/timeblock/ARCHITECTURE.md](backend/services/timeblock/ARCHITECTURE.md) | timeblock-service |
| [backend/services/routine/ARCHITECTURE.md](backend/services/routine/ARCHITECTURE.md) | routine-service |
| [backend/services/analytics/ARCHITECTURE.md](backend/services/analytics/ARCHITECTURE.md) | analytics-service |
| [backend/services/memo/ARCHITECTURE.md](backend/services/memo/ARCHITECTURE.md) | memo-service |
| [backend/services/note/ARCHITECTURE.md](backend/services/note/ARCHITECTURE.md) | note-service |
