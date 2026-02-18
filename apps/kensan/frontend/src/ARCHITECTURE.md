# フロントエンドアーキテクチャ

Kensanパーソナル生産性アプリケーションのReact + TypeScript SPA。

---

## 目次

1. [概要](#概要)
2. [ディレクトリ構成](#ディレクトリ構成)
3. [コンポーネント階層](#コンポーネント階層)
4. [状態管理](#状態管理)
5. [APIクライアント層](#apiクライアント層)
6. [ルーティング](#ルーティング)
7. [主要パターン](#主要パターン)

---

## 概要

### アーキテクチャスタイル

- **React 18 SPA** + TypeScript strictモード
- **Zustand** によるグローバル状態管理
- **レイヤードアーキテクチャ**: Components → Stores → API Services → Backend
- **タイムゾーン対応**: 全ての日時操作はローカルとUTC間で変換

### 技術スタック

| コンポーネント | 技術 | バージョン |
|--------------|------|----------|
| フレームワーク | React | 18.3 |
| 言語 | TypeScript | 5.6 |
| ビルドツール | Vite | 6.x |
| 状態管理 | Zustand | 5.x |
| ルーティング | React Router | 7.x |
| スタイリング | Tailwind CSS | 4.x |
| UIコンポーネント | shadcn/ui (Radix UIベース) | - |
| アイコン | Lucide React | 0.562 |
| エディタ | TipTap | 3.16 |
| チャート | Recharts | 3.6 |

---

## ディレクトリ構成

```
src/
├── api/                          # HTTPクライアントとAPIサービス
│   ├── client.ts                 # HttpClientシングルトン
│   ├── telemetry.ts              # OTel SDK 初期化 + トレースヘルパー
│   ├── config.ts                 # サービスURL設定
│   ├── createApiService.ts       # 汎用CRUDファクトリ
│   └── services/                 # ドメイン別API（12ファイル）
├── components/
│   ├── ui/                       # shadcn/uiプリミティブ
│   ├── layout/                   # Header, Sidebar, Layout
│   ├── common/                   # ドメインコンポーネント
│   ├── editor/                   # Markdown, Drawio, Mindmapエディタ
│   ├── task/                     # タスク関連UI
│   ├── daily/                    # デイリーページセクション
│   ├── note/                     # ノートエディタ
│   ├── agent/                    # AIチャットUI (ChatPanel, ActionProposal, ProposalTimeline, ConversationRating)
│   ├── analytics/                # 分析レポート (AIReviewSection, AIReviewContent, AnalyticsPeriodSelector)
│   ├── weekly/                   # ウィークリーページ（DnD対応）
│   ├── interactions/             # AI Interaction Explorer
│   ├── guide/                   # ページガイドシステム (PageGuide, SpotlightTour, SpotlightOverlay)
│   └── prompt/                  # プロンプト管理 (PromptSidebar, PromptEditor, VersionHistory, VersionDetail, ABTestPanel, VersionDiffDialog, ChallengeChatPanel, ChallengePromptDiff, ToolSelector)
├── pages/                        # ページコンポーネント（10ファイル）
├── stores/                       # Zustandストア（18ストア）
├── hooks/                        # カスタムReactフック (usePanelResize, useChatStream, useVersionSeen)
├── lib/                          # ユーティリティ（timezone, taskUtils等）
├── mocks/                        # MSWハンドラとモックデータ
├── types/                        # TypeScript型定義
└── App.tsx                       # ルートルーター
```

---

## コンポーネント階層

### 全体図

```mermaid
graph TB
    subgraph "アプリシェル"
        Router["React Router"]
        Layout["Layout"]
    end

    subgraph "レイアウト"
        Header["Header<br/>(TimerWidget, テーマ切替)"]
        Sidebar["Sidebar<br/>(ナビゲーション)"]
        Main["メインコンテンツエリア"]
        FAB["FloatingMemoButton"]
    end

    subgraph "ページ"
        Daily["DailyPage<br/>タイムライン+タスク"]
        Weekly["W01_WeeklyPlanning<br/>週間カレンダー"]
        Tasks["T01_TaskManagement<br/>目標・タスク管理"]
        Notes["N01_NoteList / N02_NoteEdit"]
        Analytics["A01 / A03<br/>分析・プロンプト管理"]
        Interactions["O01_InteractionExplorer"]
    end

    subgraph "ドメインコンポーネント"
        Timeline["TimeBlockTimeline<br/>ドラッグ/リサイズ対応"]
        TaskCard["TaskCard<br/>チェック+バッジ"]
        DetailPanel["TaskDetailPanel<br/>Sheet右スライド"]
        TBDialog["TimeBlockDialog<br/>予定/実績共通"]
        ChatPanel["ChatPanel<br/>SSEストリーミング"]
        BriefingCards["BriefingCardGrid<br/>8種のカード"]
    end

    subgraph "UIプリミティブ (shadcn/ui)"
        Primitives["Button, Card, Dialog,<br/>Sheet, Input, Select,<br/>Checkbox, Tabs, ..."]
    end

    Router --> Layout
    Layout --> Header
    Layout --> Sidebar
    Layout --> Main
    Layout --> FAB

    Main --> Daily
    Main --> Weekly
    Main --> Tasks
    Main --> Notes
    Main --> Analytics

    Daily --> Timeline
    Daily --> TaskCard
    Daily --> DetailPanel
    Tasks --> TaskCard
    Tasks --> DetailPanel
    Tasks --> TBDialog
    Timeline --> TBDialog
    Timeline --> Primitives
    TaskCard --> Primitives
    DetailPanel --> Primitives
    TBDialog --> Primitives
```

### 3層構造

| 層 | ディレクトリ | 役割 |
|----|------------|------|
| **UIプリミティブ** | `components/ui/` | shadcn/ui (Radix UI)。ビジネスロジックなし |
| **ドメインコンポーネント** | `components/common/`, `components/task/` 等 | ドメイン知識を持つ再利用コンポーネント |
| **ページ** | `pages/` | ルートに対応。ストアとコンポーネントを組み合わせ |

### 主要ドメインコンポーネント

| コンポーネント | 目的 |
|--------------|------|
| `TimeBlockTimeline` | インタラクティブタイムライン（ドラッグ移動/エッジリサイズ、15分スナップ） |
| `TimeBlockDialog` | 予定(plan)/実績(entry)モード切替の共通ダイアログ |
| `TaskCard` | チェックボックス、ゴールバッジ付きタスク表示 |
| `TaskDetailPanel` | 右からスライドインする詳細パネル（Sheet） |
| `TimerWidget` | ヘッダー内アクティブタイマー表示 |
| `ChatPanel` | AIチャットUI（`useChatStream`フック利用） |
| `ActionProposal` | AI提案UI（タイムブロック+その他アクション、日別タブ切替、`readOnly`モード対応） |
| `ProposalTimeline` | 提案タイムライン（TimelineCoreリードオンリー、既存ブロック半透明表示） |
| `ConversationRating` | 会話評価UI（4段階ボタン: イマイチ/ふつう/いい/とてもいい） |
| `ChallengeChatPanel` | バージョンベースA/B比較用パネル（`useChatStream(contextId, versionNumber)` + `ChatMessage` + `ActionProposal readOnly`で構成） |
| `PageGuide` | ページ初回訪問時のウェルカムカード（✕で閉じるとlocalStorage記憶） |
| `SpotlightTour` | ステップ式スポットライトツアー（clip-pathくり抜きオーバーレイ） |

### ページガイドシステム (`components/guide/`)

初回ユーザー向けのオンボーディングシステム。削除時は `components/guide/` フォルダごと削除可能。

| ファイル | 役割 |
|---------|------|
| `useGuideStore.ts` | Zustand store (localStorage `kensan-guide` に永続化) |
| `guideContent.ts` | 全9ページのウェルカムカード内容定義 |
| `tourSteps.ts` | 3ページ (Daily, Task, Weekly) のツアーステップ定義 |
| `PageGuide.tsx` | ウェルカムカード表示（Tips 2列グリッド + ツアー開始ボタン） |
| `SpotlightTour.tsx` | ツアーエンジン（ステップ管理 + createPortal でオーバーレイ表示） |
| `SpotlightOverlay.tsx` | `clip-path` でターゲット要素をくり抜くオーバーレイ描画 |

**ツアー対象ページ**: `data-guide="..."` 属性を使って要素を特定。

### TimeBlockTimelineアーキテクチャ

```mermaid
graph TB
    subgraph "timeline/"
        Core["TimelineCore.tsx<br/>Daily/Weekly共通<br/>ドラッグ/リサイズ, ズーム, DnD"]
        Grid["TimeBlockTimelineGrid.tsx<br/>時間線と背景グリッド"]
        Content["TimelineItemContent.tsx<br/>ブロック/エントリの表示"]
        Context["TimelineColumnContext<br/>設定・コールバックをContext経由で渡す"]
    end

    subgraph "ラッパー"
        Daily["TimeBlockTimeline.tsx<br/>Daily用（実績+タイマー表示）"]
        Weekly["WeeklyCalendarGrid<br/>Weekly用（複数カラム）"]
    end

    Daily --> Core
    Weekly --> Core
    Core --> Grid
    Core --> Content
    Core --> Context
```

**安定性のためのパターン:**
- `previewTime` は state + ref の二重管理（再レンダリング + mouseupリスナーの最新値参照）
- `mousemove`/`mouseup` リスナーはドラッグ開始時に1回登録、終了時に1回除去
- `offsetFromStart` はドラッグ開始時に1回計算して保存（ドリフト防止）

---

## 状態管理

### Zustandアーキテクチャ

```mermaid
flowchart TB
    subgraph "Reactレイヤー"
        Component["コンポーネント"]
    end

    subgraph "Zustandストア"
        State["状態 + アクション + ゲッター"]
    end

    subgraph "APIレイヤー"
        APIService["APIサービス"]
        HttpClient["HttpClient<br/>(JWT自動付与)"]
    end

    subgraph "バックエンド"
        Service["Goサービス"]
    end

    Component -->|"subscribe + call"| State
    State -->|"call"| APIService
    APIService -->|"request"| HttpClient
    HttpClient -->|"HTTP + JWT"| Service
    Service -->|"JSON {data}"| HttpClient
    HttpClient -->|".data抽出"| APIService
    APIService -->|"transform"| State
    State -->|"notify"| Component
```

### ストア一覧

| ストア | ドメイン | 永続化 | 備考 |
|-------|--------|--------|------|
| `useAuthStore` | 認証 (token, user) | localStorage | onRehydrateでHTTPクライアントにtoken設定 |
| `useSettingsStore` | 設定 (timezone, theme) | localStorage | 全ストアのタイムゾーン源泉 |
| `useGoalStore` | 目標 | - | createCrudStoreファクトリ + reorder拡張 |
| `useMilestoneStore` | マイルストーン | - | createCrudStoreファクトリ |
| `useTagStore` | タスクタグ | - | createCrudStoreファクトリ |
| `useNoteTagStore` | ノートタグ | - | noteTagsApi (list/create)。タスクタグと完全分離 |
| `useTaskStore` | タスク | - | 独自実装（toggle, reorder, bulk操作） |
| `useTimeBlockStore` | 予定・実績 | - | タイムゾーン対応フェッチ |
| `useTimerStore` | タイマー | - | start/stop/fetch |
| `useNoteTypeStore` | ノートタイプ設定 | - | APIから取得、isLoadedでキャッシュ |
| `useNoteStore` | ノート | - | noteCache (Map) でフルコンテンツキャッシュ |
| `useMemoStore` | メモ | - | createCrudStoreファクトリ |
| `useAnalyticsStore` | 分析データ | - | 週次/月次サマリー |
| `useChatStore` | AIチャット | - | パネルUI状態（開閉、履歴、評価、プリフィル）。SSEストリーミングは`useChatStream`フックに移行 |
| `usePromptStore` | プロンプト管理 | - | AIコンテキスト一覧・バージョン管理 |
| `useTaskManagerStore` | タスク管理統合 | - | Goal/Milestone/Tag/Taskストアの統合フック |

### カスタムフック

| フック | 用途 | 備考 |
|--------|------|------|
| `useChatStream` | SSEストリーミングチャット | contextId/versionNumber対応、ChatPanel/ChallengeChatPanelで使用 |
| `usePanelResize` | ドラッグリサイズ | min/max/default幅指定 |
| `useVersionSeen` | 未確認バージョン追跡 | localStorage (`kensan-version-seen`)、useSyncExternalStore パターン。initializeIfNeededで初回ロード時のfalse positive防止 |

### ストア初期化フロー

```mermaid
flowchart TB
    Auth["useAuthStore<br/>isAuthenticated?"]
    Init["useInitializeData"]
    Settings["fetchSettings<br/>(timezone取得)"]
    Parallel["並列フェッチ"]

    Auth -->|true| Init
    Init --> Settings
    Settings --> Parallel

    Parallel --> Goals["fetchGoals"]
    Parallel --> MS["fetchMilestones"]
    Parallel --> Tags["fetchTags"]
    Parallel --> Tasks["fetchTasks"]
    Parallel --> TB["fetchTimeBlocks<br/>(today, timezone)"]
    Parallel --> TE["fetchTimeEntries<br/>(today, timezone)"]
    Parallel --> NoteTypes["fetchNoteTypes"]
```

### ストア連携図

```mermaid
graph TB
    subgraph "認証と初期化"
        AuthStore["useAuthStore"]
        InitHook["useInitializeData"]
    end

    subgraph "設定"
        SettingsStore["useSettingsStore"]
    end

    subgraph "ドメインストア"
        GoalStore["useGoalStore"]
        TaskStore["useTaskOnlyStore"]
        TimeBlockStore["useTimeBlockStore"]
        TimerStore["useTimerStore"]
        NoteStore["useNoteStore"]
    end

    subgraph "APIクライアント"
        HttpClient["httpClient"]
    end

    AuthStore -->|setAuthToken| HttpClient
    AuthStore -->|isAuthenticated| InitHook

    InitHook -->|fetchAll| SettingsStore
    InitHook -->|fetchAll| GoalStore
    InitHook -->|fetchAll| TaskStore
    InitHook -->|fetchAll| TimeBlockStore

    SettingsStore -->|timezone| TimeBlockStore
    SettingsStore -->|timezone| TimerStore

    HttpClient -->|401 Unauthorized| AuthStore
```

---

## APIクライアント層

### 全体構造

```mermaid
graph TB
    subgraph "APIサービス層"
        Factory["createApiService<br/>汎用CRUDファクトリ"]
        Extend["extendApiService<br/>カスタム操作追加"]
        Custom["カスタムAPIサービス<br/>(agent, analytics, observability)"]
    end

    subgraph "HTTPクライアント"
        Client["HttpClient シングルトン"]
    end

    subgraph "自動処理"
        JWT["JWT Bearer 自動付与"]
        Trace["OTel スパン + traceparent 注入"]
        Unwrap["レスポンスエンベロープ<br/>{data} → data"]
        Auth401["401 → 自動ログアウト"]
    end

    Factory --> Client
    Extend --> Client
    Custom --> Client
    Client --> JWT
    Client --> Trace
    Client --> Unwrap
    Client --> Auth401
```

### APIサービス一覧

| ファイル | 対象サービス | 方式 |
|---------|------------|------|
| `auth.ts` | user-service | カスタム |
| `user.ts` | user-service | カスタム |
| `tasks.ts` | task-service | ファクトリ + extend（Goals, Milestones, Tags, Tasks） |
| `timeblocks.ts` | timeblock-service | カスタム（タイムゾーン変換付き） |
| `timer.ts` | timeblock-service | カスタム |
| `notes.ts` | note-service | ファクトリ + extend |
| `memos.ts` | memo-service | ファクトリ |
| `agent.ts` | kensan-ai | カスタム（SSE対応） |
| `analytics.ts` | analytics-service | カスタム |
| `prompts.ts` | kensan-ai | カスタム（AIコンテキスト・バージョン管理） |
| `observability.ts` | kensan-ai | カスタム（AI Interaction Explorer用、Lakehouse Silver経由） |

### タイムゾーン変換（timeblocks.ts）

```mermaid
sequenceDiagram
    participant Store as useTimeBlockStore
    participant API as timeblocksApi
    participant BE as timeblock-service

    Note over Store,API: フェッチ: ローカル日付 → UTC範囲
    Store->>API: listByLocalDate("2026-01-27", "Asia/Tokyo")
    API->>API: localDateToUtcRange() → start/endUtc
    API->>BE: GET /timeblocks?start_timestamp=...&end_timestamp=...

    Note over Store,API: 作成: ローカル日時 → UTC ISO
    Store->>API: createFromLocal("2026-01-27", "09:00", "10:00", data, "Asia/Tokyo")
    API->>API: localToUtcDatetime() → "2026-01-27T00:00:00.000Z"
    API->>BE: POST /timeblocks {startDatetime, endDatetime} (UTC)
```

フェッチ・作成・更新すべてでローカル⇔UTC変換を API サービス層が吸収。変換ユーティリティは `lib/timezone.ts`。

---

## ルーティング

### ルート構成

```mermaid
graph TB
    subgraph "公開ルート"
        Login["/login → LoginPage"]
    end

    subgraph "保護ルート (RequireAuth、Layout なし)"
        SettingsInitial["/settings/initial → S01_Settings<br/>(初期設定)"]
    end

    subgraph "保護ルート (RequireAuth + RequireConfigured + Layout)"
        SettingsRoute["/settings → S01_Settings"]
        Daily["/ → DailyPage<br/>(?date=YYYY-MM-DD)"]
        Weekly["/weekly → W01_WeeklyPlanning"]
        TasksRoute["/tasks → T01_TaskManagement"]
        NotesRoute["/notes → N01_NoteList"]
        NoteNew["/notes/new → N02_NoteEdit"]
        NoteEdit["/notes/:id → N02_NoteEdit"]
        AnalyticsRoute["/analytics → A01_AnalyticsReport"]
        InteractionsRoute["/interactions → O01_InteractionExplorer"]
        PromptsRoute["/prompts → A03_PromptEditor"]
    end
```

### ルート保護

```mermaid
flowchart TB
    Request["ルートアクセス"] --> AuthCheck{"認証済み?"}
    AuthCheck -->|No| LoginRedirect["/login にリダイレクト"]
    AuthCheck -->|Yes| ConfigCheck{"設定完了?"}
    ConfigCheck -->|No| SettingsRedirect["/settings にリダイレクト"]
    ConfigCheck -->|Yes| Layout["Layout + ページ表示"]
```

### ページ命名規則

| プレフィックス | ドメイン | 例 |
|--------------|---------|-----|
| S | 設定/システム | S01_Settings |
| W | ウィークリー | W01_WeeklyPlanning |
| D | デイリー | DailyPage |
| N | ノート | N01_NoteList, N02_NoteEdit |
| T | タスク | T01_TaskManagement |
| A | 分析/AI | A01_AnalyticsReport, A03_PromptEditor |
| O | Observability | O01_InteractionExplorer |

---

## 主要パターン

### タイムゾーン変換

```mermaid
flowchart TB
    subgraph "保存時"
        LocalInput["ローカル日時入力"] --> ToUTC["localToUtcDatetime()"]
        ToUTC --> UTC["UTC ISO 8601 でAPI送信"]
    end

    subgraph "表示時"
        UTCResp["UTC ISO 8601 (APIレスポンス)"] --> ToLocal["getLocalDate() / getLocalTime()"]
        ToLocal --> LocalDisplay["ローカル日時で表示"]
    end

    subgraph "クエリ時"
        LocalDate["ローカル日付"] --> ToRange["localDateToUtcRange()"]
        ToRange --> UTCRange["UTC範囲でクエリ"]
    end
```

全変換ユーティリティは `lib/timezone.ts` に集約。`useSettingsStore` の `timezone` を源泉として使用。

### 目標ベースの時間集計

DailySummaryの達成率計算は **goal_idあり** のデータのみを対象:

```mermaid
flowchart LR
    All["全TimeBlock/Entry"] --> Filter{"goal_id あり?"}
    Filter -->|Yes| GoalBased["目標ベース<br/>→ 達成率計算"]
    Filter -->|No| Other["その他の作業<br/>→ 別枠表示（達成率に含まず）"]
```

タスク名直接入力（目標未設定）は「その他」として扱い、目標達成の進捗追跡を正確に保つ。

### スタイリング

- **Tailwind CSS 4.x** + `cn()` ヘルパー（`clsx` + `twMerge`）
- **セマンティックカラー**: CSS変数ベース（`--background`, `--primary`, `--brand`）
- **ダークモード**: `.dark` クラスでCSS変数を切替
- **パスエイリアス**: `@/*` → `./src/*`

### A03_PromptEditor (プロンプト最適化管理)

AIコンテキストのプロンプトをバージョン中心で管理するページ。2タブ構成:

- **プロンプト編集タブ**: コンテキスト一覧 + エディタ
- **最適化タブ**: AI最適化 + バージョン履歴 + バージョン詳細/A/Bテスト

```mermaid
flowchart TB
    subgraph "最適化タブ 左パネル"
        Optimize["AI最適化ボタン"]
        CtxSelect["コンテキスト選択"]
        VH["VersionHistory<br/>source/candidateバッジ付き"]
    end

    subgraph "最適化タブ 右パネル"
        VD["VersionDetail<br/>メタデータ + eval_summary + diff"]
        AB["ABTestPanel<br/>エフェメラルA/Bテスト"]
    end

    VH -->|バージョン選択| VD
    VH -->|A/Bテスト開始| AB
    VD -->|採用/却下| Optimize
```

**バージョン中心モデル:**

| 概念 | 説明 |
|------|------|
| `active_version` | ai_contextsの現在有効なバージョン番号 |
| `candidate_status` | NULL(通常) / pending(AI候補) / adopted(採用) / rejected(却下) |
| `source` | manual(手動) / ai(AI生成) / rollback(ロールバック) |
| `eval_summary` | AI評価データ (interaction_count, avg_rating, strengths, weaknesses) |

**コンポーネント構成:**

| コンポーネント | 役割 |
|--------------|------|
| `A03_PromptEditor` (ページ) | タブ管理、状態統合、adopt/reject/rollback/A/Bテスト操作 |
| `PromptSidebar` | コンテキスト一覧 + pending候補バッジ |
| `VersionHistory` | バージョン一覧 (source/candidateバッジ、選択、A/Bテスト開始) |
| `VersionDetail` | バージョン詳細表示 + eval_summary + diff + アクションボタン |
| `ABTestPanel` | エフェメラルA/Bテスト (ChallengeChatPanel x2、投票、採用) |

**ストア連携:**

| ストア | 用途 |
|--------|------|
| `usePromptStore` | AIコンテキスト一覧・バージョン管理・ロールバック |

### AI Interaction Explorer

Lakehouse Silver テーブルから kensan-ai API 経由で AI インタラクションを可視化（5分バッチ更新）:

```mermaid
flowchart LR
    Loki["Loki<br/>(構造化ログ)"] --> Dagster["Dagster<br/>(5分バッチ)"]
    Dagster --> Bronze["Bronze<br/>ai_explorer_events_raw"]
    Bronze --> Silver["Silver<br/>ai_explorer_interactions<br/>ai_explorer_events"]
    Silver --> API["kensan-ai API<br/>/explorer/interactions"]
    API --> TS["observability.ts<br/>fetchInteractions()"]
    TS --> Table["InteractionTable<br/>一覧表示"]
    Table -->|選択| Flow["ConversationFlow<br/>詳細タイムライン"]
```

| コンポーネント | 表示内容 |
|--------------|---------|
| `InteractionTable` | 時刻、モデル、トークン数、ツール数、outcome |
| `ConversationFlow` | システムプロンプト分析、ツール定義サマリー、各ターン詳細、ツール呼び出しI/O |
