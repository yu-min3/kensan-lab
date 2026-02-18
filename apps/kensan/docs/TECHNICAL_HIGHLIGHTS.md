# Kensan - Technical Highlights

> エンジニア向けパーソナル生産性アプリ。タスク・時間管理、学習記録、AI アシスタント、週次レビュー自動生成を統合。

## System Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Frontend (React/TS + Zustand + Tailwind)                                │
│   Chat UI ←──SSE──→ AI Agent  │  CRUD Pages ←──REST──→ Go Services     │
└────────────┬─────────────────────────────────────┬───────────────────────┘
             │                                     │
      ┌──────▼──────┐                    ┌─────────▼─────────┐
      │  kensan-ai   │                    │ Go Microservices  │
      │  (FastAPI)   │───────────────────▶│  ×6 (chi/pgx)    │
      │  Agent Loop  │  DB shared         │  user/task/time/  │
      │  Tool Exec   │◀──────────────────▶│  analytics/memo/  │
      └──────┬───────┘                    │  note             │
             │                            └─────────┬─────────┘
             │ write(Bronze)                        │
      ┌──────▼──────┐                    ┌─────────▼─────────┐
      │  Lakehouse   │◀───batch ETL──────│   PostgreSQL 16   │
      │  (Iceberg +  │                   │  + pgvector        │
      │   Polaris)   │───read(Gold)─────▶│                    │
      └──────────────┘                   └───────────────────┘
             │
      ┌──────▼──────┐
      │ Observability│  Grafana / Loki / Tempo / Prometheus
      │ (OTel)       │  + AI Interaction Explorer
      └──────────────┘
```

---

## 1. Agentic Loop

### Read/Write 分離による承認フロー付き ReAct Loop

AI エージェントは Anthropic API の Tool Use を活用した ReAct スタイルのループで動作する。最大の特徴は **read ツールの即時実行と write ツールの承認フロー分離**。

```
User Message
  │
  ▼
┌─────────────────────────────────────┐
│  Turn N                              │
│  1. Claude API (streaming SSE)       │
│  2. Response に tool_use あり?        │
│     ├─ readonly tool → 並列実行       │  ← asyncio.gather で並列化
│     └─ write tool   → pending に蓄積  │  ← 実行せず提案として返す
│  3. readonly 結果を history に追加     │
│  4. 次の turn へ (max_turns まで)      │
└─────────────────────────────────────┘
  │
  ▼
action_proposal SSE event (write tools をまとめて送信)
  │
  ▼
Frontend: チェックボックス付き承認 UI
  │
  ├─ approve → 選択されたツールのみ実行
  └─ reject  → 実行せずに会話継続
```

**ポイント:**
- `is_readonly_tool()` でツールを分類。read は即座に `asyncio.gather` で並列実行し、write は承認待ちとしてクライアントに返す
- SSE ストリーミング中にkeepalive を非同期タスクで送信し、API 待機中のクライアントタイムアウトを防止
- 各 turn で OpenTelemetry span を生成し、トークン使用量・ツール実行時間を計測

### Intent-Aware Dynamic Tool Selection + Deferred Write Injection

全 30+ ツールを毎回渡すのではなく、ユーザーの発話意図から必要なツールグループだけを選択する。

```python
TOOL_GROUPS = {
    "core":     ["get_tasks", "get_time_blocks", ...],     # 常に含む
    "planning": ["create_time_block", "update_time_block"], # 予定操作
    "task":     ["create_task", "update_task", ...],        # タスク操作
    "search":   ["semantic_search", "hybrid_search", ...],  # 検索
    "review":   ["generate_review", "get_reviews", ...],    # レビュー
    ...
}
```

#### Deferred Write Tool Injection

従来はキーワードベースで write 意図を判定していたが、「予定よろしく」「スケジュールお願い」のような自然な表現では write ツールが利用不可になる問題があった。

**解決策**: `select_tools()` は read ツールのみ返し、write ツールは LLM が read ツールを呼んだ後に動的追加する。read ツール呼び出し自体が write 意図のシグナルとなる。

```
Turn 1: select_tools("予定よろしく")
        → core + goals_read (read ツールのみ)
        → deferred: [create_time_block, update_time_block, ...]

        LLM が get_time_blocks を呼ぶ
        → deferred unlock: write ツールを allowed_tools に追加

Turn 2: LLM が create_time_block × N を提案
        → action_proposal として返す
```

追加 API コストはゼロ。read クエリは元々 2 ターン（Turn 1: ツール呼び出し → Turn 2: 結果で回答）なので、Turn 2 で write ツールを追加しても余分な API コールは発生しない。

```python
READ_TOOL_TO_WRITE_GROUPS = {
    "get_tasks":               ["task"],       # → create/update/delete_task
    "get_time_blocks":         ["planning"],   # → create/update/delete_time_block
    "get_goals_and_milestones": ["goals_write"], # → create/update/delete_goal, ...
    "get_notes":               ["notes_write"], # → create/update_note, create_memo
    ...
}
```

Runner 実装（Anthropic / Gemini / ADK 共通）:
- `__init__` に `deferred_tools` パラメータを追加
- 各ターン先頭で `_get_tools_schema()` を再計算
- readonly ツール実行後に `_unlock_deferred_tools()` で write ツールをマージ（1 回限り）
- ADK はループを内部管理するため、`__init__` 時点で全マージ

#### 語幹ベースの書き込み意図検出（ユーティリティ）

`_has_write_intent()` は分析・ログ用のユーティリティとして残存。日本語の活用形を網羅的にカバー:

```python
# サ変動詞: 名詞部分でマッチ → 「作成して/作成したい/作成しよう」全てOK
_SURU_VERB_STEMS = ["作成", "追加", "登録", "変更", ...]

# 五段動詞: 語幹＋活用行の正規表現
_GODAN_VERB_PATTERNS = [
    re.compile(r"作[らりるれろっ]"),  # 作る → 作って/作りたい/作ろう
    re.compile(r"組[まみむめもん]"),  # 組む → 組んで/組みたい
]
```

#### 変数注入によるツール除外

**システムプロンプトに変数注入済みのデータと重複するツールを自動除外**:

```python
VARIABLE_EXCLUDES_TOOLS = {
    "pending_tasks": ["get_tasks"],         # 変数で注入済み → ツール不要
    "goal_progress": ["get_goals_and_milestones"],
    "weekly_summary": ["get_analytics_summary"],
}
```

### Tool Nudge

エージェントがツールを呼ばずにテキストだけで返答した場合、自動的にリトライを促す:

- **Turn 0**: 「ツールを直接呼び出してください」（read ツール呼び忘れ対策）
- **Turn 1+**: 「具体的なアクションをツール呼び出しで提案してください」（write ツール呼び忘れ対策）

### Multi-Provider Support

`create_agent_runner()` ファクトリで Anthropic / Google Gemini / Google ADK を切り替え可能。`AgentRunner` (Anthropic)、`GeminiAgentRunner`、`AdkAgentRunner` が共通インターフェースを実装。

---

## 2. Context & Prompt Management

### Situation-Aware Context Resolution

```
ユーザーメッセージ
  │
  ▼
detect_situation()     ← キーワードベースで状況判定
  │                       (chat / review / daily_advice)
  ▼
ContextResolver.get_context(situation, user_id)
  │
  ├─ Experiment あり? → ABSelector で variant 選択
  │                      (SHA256 consistent hashing)
  │
  └─ ai_contexts テーブルから system_prompt + 設定を取得
  │
  ▼
VariableReplacer.replace()
  │  15 種の動的変数を解決:
  │  {current_datetime}, {user_memory}, {user_patterns},
  │  {goal_progress}, {pending_tasks}, {weekly_summary},
  │  {emotion_summary}, {interest_profile}, {user_traits}, ...
  │
  ▼
完成した system_prompt + allowed_tools → AgentRunner
```

### A/B Testing with Consistent Hashing

```python
class ABSelector:
    @staticmethod
    def compute_bucket(user_id, experiment_id) -> int:
        hash_input = f"{user_id}:{experiment_id}".encode("utf-8")
        return int.from_bytes(
            hashlib.sha256(hash_input).digest()[:4], "big"
        ) % 100

    @staticmethod
    def select_from_weights(bucket, contexts) -> ExperimentContext:
        # traffic_weight に基づく累積分布でバケットを振り分け
```

同一ユーザーは同一 experiment 内で常に同じ variant を受け取る。フロントエンドの Prompt Editor から experiment を作成し、A/B 投票 UI でラウンドごとに比較評価できる。

### Automated Prompt Optimization Pipeline

```
Weekly Schedule (Mon 3:10 AM)
  │
  ▼
ExperimentManager.run_optimization()
  ├─ 各 situation の active context を取得
  ├─ LLM で評価 (弱点 + 改善提案を抽出)
  ├─ PromptOptimizer.generate_improved_prompt()
  │    ├─ テンプレート変数の保持を検証
  │    └─ 長さ制約 (±20%) を検証
  └─ candidate version として DB 保存 (本番未適用)
       └─ Prompt Editor UI で人間がレビュー → 採用/棄却
```

---

## 3. Go Microservices - Clean Architecture

### Bootstrap Pattern

全 6 サービスが共通の初期化パターンで統一:

```go
func main() {
    svc, _ := bootstrap.New("task-service") // DB, JWT, OTel, Router 一括初期化
    defer svc.Close()

    repo := repository.NewPostgresRepository(svc.Pool)
    service := service.NewService(repo)
    handler := handler.NewHandler(service)

    svc.RegisterRoutes(handler.RegisterRoutes)  // Auth middleware 自動適用
    svc.Run()                                   // Graceful shutdown 付き
}
```

`bootstrap.New()` が提供するもの:
- pgxpool (OpenTelemetry instrumented)
- JWT Manager (HS256)
- chi Router + Middleware Chain (RequestID → OTelTrace → Metrics → Logger → CORS → Auth)
- Health endpoint (`/health`)
- Graceful shutdown (SIGTERM/SIGINT, 30s timeout)

### Strict Layer Separation

```
Handler                    Service                   Repository
─────────────────         ─────────────────         ─────────────────
HTTP parse/validate  →    Domain logic         →    Parameterized SQL
middleware.GetUserID →    Input validation     →    sqlbuilder (safe)
middleware.JSON      ←    Domain errors        ←    pgx scan
middleware.Error     ←    (ErrNotFound, etc.)  ←
```

**Interface Segregation** — 各ドメインを細分化したインターフェースで定義:

```go
type GoalService interface { ListGoals, GetGoal, CreateGoal, ... }
type TaskService interface { ListTasks, GetTask, CreateTask, ... }
type FullService interface { GoalService; TaskService; MilestoneService; ... }
```

### Safe Dynamic SQL Builder

```go
w := sqlbuilder.NewWhereBuilder(userID)         // user_id = $1 (常に)
sqlbuilder.AddFilter(w, "status", filter.Status) // status = $2 (nilならスキップ)
w.AddInClause("tag_id", tagIDs)                  // tag_id IN ($3, $4, ...)
// → プレースホルダ番号を自動管理、文字列結合ゼロ
```

### Denormalization with Auto-Sync Triggers

JOIN を排除するため、子テーブルに親の `goal_name`, `goal_color` 等を複製。PostgreSQL トリガーが `IS DISTINCT FROM` で差分検知し自動同期:

```sql
CREATE FUNCTION sync_goal_denormalized_fields() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.name IS DISTINCT FROM OLD.name OR NEW.color IS DISTINCT FROM OLD.color THEN
        UPDATE time_blocks SET goal_name=NEW.name, goal_color=NEW.color WHERE goal_id=NEW.id;
        UPDATE time_entries SET goal_name=NEW.name, goal_color=NEW.color WHERE goal_id=NEW.id;
        UPDATE notes SET goal_name=NEW.name, goal_color=NEW.color WHERE goal_id=NEW.id;
    END IF;
    RETURN NEW;
END; $$ LANGUAGE plpgsql;
```

---

## 4. Lakehouse - Medallion Architecture

Apache Iceberg + Polaris REST Catalog + Dagster で構築したデータレイクハウス。

```
          ┌─────────┐     ┌──────────┐     ┌─────────┐
PG ──────▶│ Bronze   │────▶│ Silver   │────▶│  Gold   │
Loki ────▶│ (生データ)│    │ (整形)    │    │ (集計)   │
          └─────────┘     └──────────┘     └─────────┘
           11 tables       12 tables        7 tables
```

### データフロー 3 系統

| 系統 | ソース | 方式 | 頻度 |
|------|--------|------|------|
| PG → Bronze | PostgreSQL テーブル | バッチ差分取り込み | 日次 |
| Loki → Bronze | AI エージェントログ | Loki API バッチ | 5 分 |
| kensan-ai → Bronze | 外部ツール実行結果 | LakehouseWriter 直接 append | リアルタイム |

### Gold → AI Agent フィードバックループ

Lakehouse の集計結果を AI エージェントのシステムプロンプトに動的注入:

```
Gold Tables                    VariableReplacer Variables
──────────────                 ──────────────────────────
emotion_weekly          →      {emotion_summary}
user_interest_profile   →      {interest_profile}
user_trait_profile      →      {user_traits}, {communication_style}
```

LakehouseReader が PyIceberg で Gold/Silver テーブルを読み取り、ContextResolver が VariableReplacer 経由でシステムプロンプトに埋め込む。ユーザーの行動データが AI の応答品質を継続的に改善するフィードバックループを形成。

### AI Interaction Explorer

エージェントのログを Loki から Bronze → Silver に ETL し、フロントエンドの Explorer UI で可視化:

```
Agent Log (Loki) → bronze.ai_explorer_events_raw
                     → silver.ai_explorer_events (user_id伝播, event_order付与)
                     → silver.ai_explorer_interactions (trace_idでグルーピング)
                        → GET /explorer/interactions
                           → InteractionTable + ConversationFlow UI
```

---

## 5. Observability

### End-to-End OpenTelemetry

Frontend → Backend → AI Service の全レイヤーでトレースを伝播:

| Layer | Implementation |
|-------|---------------|
| Frontend | `@opentelemetry/sdk-trace-web`, `traceparent` ヘッダ自動付与 |
| Go Backend | `otelhttp` middleware + `otelpgx` DB instrumentation |
| kensan-ai | 各 turn/tool_call に span、GenAI semantic conventions 準拠 |

### Grafana Dashboards (2 枚)

**Service Overview**: RED メトリクス (Rate/Error/Duration)、サービスマップ、Error Budget ゲージ
**Request Explorer**: レイテンシヒートマップ、エンドポイント別テーブル、Slow DB Queries (TraceQL)、ログ相関

### AI Agent Metrics

```python
token_counter.add(tokens, {"gen_ai.token.type": "input"})
duration_hist.record(duration, {"gen_ai.request.model": model})
op_counter.add(1, {"gen_ai.response.outcome": outcome})
```

---

## 6. Frontend Architecture

### Store Factory Pattern

`createCrudStore` + `createApiService` のファクトリパターンで 18 ストアを統一:

```typescript
// API ファクトリ: エンベロープ自動アンラップ + 型変換
const goalsApi = createApiService({
  baseUrl: API_CONFIG.baseUrls.task,
  resourcePath: '/goals',
  transform: transformGoal,
})

// ストアファクトリ: CRUD state + actions + 拡張
const useGoalStore = createCrudStore(
  { api: goalsApi, getId: (g) => g.id },
  (set, get) => ({ reorderGoals: async (ids) => { ... } })
)
```

### SSE Streaming + Approval UI

`useChatStream` hook が SSE イベントを async generator で処理:

```
SSE Events: text → tool_call → tool_result → text → action_proposal → done
                                                      │
Frontend:   ChatMessage accumulator                   ActionProposal component
            (text を逐次追記)                          (チェックボックス付きリスト)
```

---

## Tech Stack

| Layer | Technologies |
|-------|-------------|
| Frontend | React 18, TypeScript, Zustand, Tailwind CSS 4, shadcn/ui, TipTap, Recharts |
| Backend | Go, chi, pgx, OpenTelemetry SDK |
| AI Service | Python, FastAPI, Anthropic API, Google Gemini API |
| Database | PostgreSQL 16, pgvector |
| Lakehouse | Apache Iceberg, Polaris REST Catalog, Dagster, PyIceberg, DuckDB |
| Observability | Grafana, Loki, Tempo, Prometheus, OpenTelemetry Collector |
| Infra | Docker Compose, nginx, GCE |
