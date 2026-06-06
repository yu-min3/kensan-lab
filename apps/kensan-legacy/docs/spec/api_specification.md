# Kensan API 仕様書

このドキュメントは、Kensanバックエンドの各マイクロサービスが提供するREST APIの仕様を定義します。

**最終更新: 2026-02-03**

## 目次

1. [アーキテクチャ概要](#アーキテクチャ概要)
2. [共通仕様](#共通仕様)
3. [User Service](#user-service-8081)
4. [Task Service](#task-service-8082)
5. [TimeBlock Service](#timeblock-service-8084)
6. [Analytics Service](#analytics-service-8088)
7. [Memo Service](#memo-service-8090)
8. [Note Service](#note-service-8091)
9. [AI Service (kensan-ai)](#ai-service-8089)

---

## アーキテクチャ概要

### マイクロサービス構成

```
ローカル開発 (make up)             本番 GCE (make prod-up)
──────────────────────             ──────────────────────
Browser                            Browser
  ├── :5173  Frontend                └── :443 nginx (HTTPS)
  ├── :8081  user-service                  ├── /         → Frontend
  ├── :8082  task-service                  ├── /api/v1/* → Go ×6
  ├── :8084  timeblock-service             └── /api/v1/agent/ → kensan-ai
  ├── :8088  analytics-service
  ├── :8089  kensan-ai             内部のみ: PostgreSQL, MinIO,
  ├── :8090  memo-service                   Grafana, Polaris, Dagster
  ├── :8091  note-service
  └── :3000  Grafana
```

全サービスは PostgreSQL 16 + pgvector に接続。kensan-ai は Gemini 2.0 Flash (ADK) を使用。

### サービス一覧

| サービス | ポート | 言語 | 役割 |
|----------|--------|------|------|
| **user-service** | 8081 | Go | 認証、ユーザー設定、AI同意管理 |
| **task-service** | 8082 | Go | 目標・マイルストーン・タスク・タグ・Todo・EntityMemo |
| **timeblock-service** | 8084 | Go | タイムブロック（計画）、時間記録（実績）、タイマー |
| **analytics-service** | 8088 | Go | 週次/月次サマリー、トレンド分析 |
| **memo-service** | 8090 | Go | クイックメモ |
| **note-service** | 8091 | Go | 統合ノート（日記・学習記録）、ファイルストレージ |
| **kensan-ai** | 8089 | Python | AIエージェント（ストリーミング対話、週次レビュー） |

### Observability スタック

| サービス | ポート | 役割 |
|----------|--------|------|
| OpenTelemetry Collector | 4317/4318 | テレメトリ収集 |
| Tempo | 3200 | 分散トレーシング |
| Loki | 3100 | ログ集約 |
| Prometheus | 9090 | メトリクス |
| Grafana | 3000 | ダッシュボード |

---

## 共通仕様

### ベースURL

全サービス共通: `/api/v1`

### 認証

JWT Bearer トークンを使用。公開エンドポイント（auth/register, auth/login）以外は認証必須。

```http
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

### レスポンス形式

#### 成功

```json
{
  "data": { ... },
  "meta": {
    "requestId": "uuid",
    "timestamp": "2026-01-21T10:30:00Z"
  }
}
```

#### リスト（ページネーション付き）

```json
{
  "data": [ ... ],
  "meta": { "requestId": "...", "timestamp": "..." },
  "pagination": {
    "page": 1,
    "perPage": 20,
    "total": 100,
    "totalPages": 5
  }
}
```

#### エラー

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "入力値が不正です",
    "details": [{ "field": "name", "message": "名前は必須です" }]
  },
  "meta": { "requestId": "...", "timestamp": "..." }
}
```

### エラーコード

| HTTP | コード | 説明 |
|------|--------|------|
| 400 | `VALIDATION_ERROR` | フィールド単位のバリデーションエラー（details 配列あり） |
| 400 | `INVALID_INPUT` | 入力値が不正（汎用） |
| 400 | `INVALID_REQUEST` | リクエスト形式エラー（必須URLパラメータ欠落等） |
| 401 | `UNAUTHORIZED` | 認証エラー |
| 403 | `FORBIDDEN` | 権限エラー |
| 404 | `NOT_FOUND` | リソース未発見 |
| 409 | `CONFLICT` | リソース競合 |
| 500 | `INTERNAL_ERROR` | サーバー内部エラー |

### 日付・時刻形式

| 項目 | 形式 | 例 |
|------|------|-----|
| 日付 | `YYYY-MM-DD` | `2026-01-21` |
| タイムスタンプ | ISO 8601 (UTC) | `2026-01-21T10:30:00Z` |

---

## User Service (8081)

ユーザー認証・設定管理。

### 公開エンドポイント

#### POST /auth/register

ユーザー登録

```json
// Request
{ "email": "user@example.com", "password": "password123", "name": "Yu" }

// Response 201
{ "data": { "token": "jwt...", "user": { "id": "uuid", "email": "...", "name": "..." } } }
```

#### POST /auth/login

ログイン

```json
// Request
{ "email": "user@example.com", "password": "password123" }

// Response 200
{ "data": { "token": "jwt...", "user": { "id": "uuid", "email": "...", "name": "..." } } }
```

### 認証済みエンドポイント

#### GET /users/me

現在のユーザー情報を取得

#### PUT /users/me

プロフィール更新

```json
// Request
{ "name": "Updated Name", "email": "new@example.com" }
```

#### GET /users/me/settings

ユーザー設定を取得

```json
// Response 200
{
  "data": {
    "timezone": "Asia/Tokyo",
    "theme": "dark",
    "aiEnabled": true,
    "aiConsentGiven": true
  }
}
```

#### PUT /users/me/settings

ユーザー設定を更新

```json
// Request
{ "timezone": "Asia/Tokyo", "theme": "dark", "aiEnabled": true }
```

#### POST /users/me/ai-consent

AI機能の利用同意を記録

```json
// Request
{ "consent": true }
```

---

## Task Service (8082)

目標・マイルストーン・タスク・タグ・Todo・EntityMemoを管理する中心的なサービス。

### Goal（目標）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/goals` | 目標一覧（`?status=` フィルタ可） |
| POST | `/goals` | 目標作成 |
| POST | `/goals/reorder` | 目標並び替え |
| GET | `/goals/{goalId}` | 目標詳細 |
| PUT | `/goals/{goalId}` | 目標更新 |
| DELETE | `/goals/{goalId}` | 目標削除 |

```json
// POST /goals Request
{ "name": "Kensanリリース", "description": "...", "color": "#FFD700" }

// PUT /goals/{goalId} Request
{ "name": "...", "description": "...", "color": "...", "status": "active|completed|archived" }
```

### Milestone（マイルストーン）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/milestones` | 一覧（`?goal_id=`, `?status=`） |
| POST | `/milestones` | 作成 |
| GET | `/milestones/{milestoneId}` | 詳細 |
| PUT | `/milestones/{milestoneId}` | 更新 |
| DELETE | `/milestones/{milestoneId}` | 削除 |

```json
// POST /milestones Request
{
  "goalId": "uuid",
  "name": "Dockerで利用できるようにする",
  "description": "...",
  "startDate": "2026-01-01",
  "targetDate": "2026-03-31"
}
```

### Tag（タグ）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/tags` | タスク用タグ一覧 |
| POST | `/tags` | タグ作成 |
| GET | `/tags/{tagId}` | タグ詳細 |
| PUT | `/tags/{tagId}` | タグ更新 |
| DELETE | `/tags/{tagId}` | タグ削除 |
| GET | `/note-tags` | ノート用タグ一覧 |
| POST | `/note-tags` | ノート用タグ作成 |

```json
// POST /tags Request
{ "name": "開発", "color": "#3B82F6", "pinned": true }
```

### Task（タスク）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/tasks` | 一覧（`?milestone_id=`, `?completed=`, `?parent_id=`） |
| POST | `/tasks` | 作成 |
| POST | `/tasks/reorder` | 並び替え |
| POST | `/tasks/bulk-delete` | 一括削除 |
| POST | `/tasks/bulk-complete` | 一括完了/未完了 |
| GET | `/tasks/{taskId}` | 詳細 |
| PUT | `/tasks/{taskId}` | 更新 |
| PATCH | `/tasks/{taskId}/complete` | 完了トグル |
| DELETE | `/tasks/{taskId}` | 削除 |

```json
// POST /tasks Request
{
  "milestoneId": "uuid",
  "parentTaskId": null,
  "name": "第1章 クラスタアーキテクチャ",
  "tagIds": ["uuid"],
  "estimatedMinutes": 120,
  "completed": false,
  "dueDate": "2026-02-15",
  "frequency": "daily",
  "daysOfWeek": [1, 3, 5]
}

// POST /tasks/bulk-delete Request
{ "taskIds": ["uuid1", "uuid2"] }

// POST /tasks/bulk-complete Request
{ "taskIds": ["uuid1", "uuid2"], "completed": true }
```

### Todo（デイリーTodo）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/todos` | 一覧（`?date=`, `?enabled=`, `?is_recurring=`） |
| POST | `/todos` | 作成 |
| GET | `/todos/{todoId}` | 詳細 |
| PUT | `/todos/{todoId}` | 更新 |
| DELETE | `/todos/{todoId}` | 削除 |
| PATCH | `/todos/{todoId}/complete` | 完了トグル（`?date=YYYY-MM-DD`） |

```json
// GET /todos?date=2026-01-21 Response (TodoWithStatus)
{
  "data": [{
    "id": "uuid",
    "name": "技術ニュースチェック",
    "frequency": "daily",
    "completedToday": true,
    "completedAt": "2026-01-21T07:30:00Z",
    "isOverdue": false
  }]
}
```

### EntityMemo（エンティティ固有メモ）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/entity-memos` | 一覧（`?entity_type=`, `?entity_id=`, `?pinned=`） |
| POST | `/entity-memos` | 作成 |
| GET | `/entity-memos/{memoId}` | 詳細 |
| PUT | `/entity-memos/{memoId}` | 更新 |
| DELETE | `/entity-memos/{memoId}` | 削除 |

```json
// POST /entity-memos Request
{
  "entityType": "goal|milestone|task",
  "entityId": "uuid",
  "content": "メモ内容",
  "pinned": false
}
```

---

## TimeBlock Service (8084)

タイムブロック（計画）、時間記録（実績）、タイマーを管理。

### TimeBlock（計画）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/timeblocks` | 一覧（`?start_datetime=`, `?end_datetime=` UTC ISO 8601） |
| POST | `/timeblocks` | 作成 |
| POST | `/timeblocks/generate-from-routines` | ルーチンから生成 |
| PUT | `/timeblocks/{timeBlockId}` | 更新 |
| DELETE | `/timeblocks/{timeBlockId}` | 削除 |

```json
// POST /timeblocks Request
{
  "taskName": "CKA学習",
  "startDatetime": "2026-01-21T00:00:00Z",
  "endDatetime": "2026-01-21T01:30:00Z"
}

// POST /timeblocks/generate-from-routines Request
{ "date": "2026-01-21" }
```

### TimeEntry（実績）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/time-entries` | 一覧（`?start_datetime=`, `?end_datetime=` UTC ISO 8601） |
| POST | `/time-entries` | 作成 |
| PUT | `/time-entries/{entryId}` | 更新 |
| DELETE | `/time-entries/{entryId}` | 削除 |

```json
// POST /time-entries Request
{
  "taskName": "CKA学習",
  "startDatetime": "2026-01-21T00:15:00Z",
  "endDatetime": "2026-01-21T01:45:00Z"
}
```

### Timer（タイマー）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/timer/current` | 実行中タイマー取得 |
| POST | `/timer/start` | タイマー開始 |
| POST | `/timer/stop` | タイマー停止 |

```json
// POST /timer/start Request
{ "taskName": "CKA学習" }
```

---

## Analytics Service (8088)

週次・月次の時間分析。

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/analytics/summary` | 期間サマリー（`?start_date=`, `?end_date=`, `?timezone=`） |
| GET | `/analytics/summary/weekly` | 週次サマリー（`?week_start=`, `?timezone=`） |
| GET | `/analytics/summary/monthly` | 月次サマリー（`?year=`, `?month=`, `?timezone=`） |
| GET | `/analytics/trends` | トレンド（`?period=week|month|quarter`, `?count=`） |
| GET | `/analytics/daily-study-hours` | 日別学習時間（`?start_date=`, `?end_date=`, `?days=`, `?timezone=`） |

```json
// GET /analytics/summary/weekly?week_start=2026-01-20&timezone=Asia/Tokyo Response
{
  "data": {
    "weekStart": "2026-01-20",
    "weekEnd": "2026-01-26",
    "totalMinutes": 2400,
    "dailyBreakdown": [
      { "date": "2026-01-20", "minutes": 360 }
    ]
  }
}
```

---

## Memo Service (8090)

クイックメモ（スクラッチパッド）。

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/memos` | 一覧（`?archived=`, `?include_all=`, `?date=`, `?limit=`） |
| POST | `/memos` | 作成 |
| GET | `/memos/{memoId}` | 詳細 |
| PATCH | `/memos/{memoId}` | 更新 |
| POST | `/memos/{memoId}/archive` | アーカイブ |
| DELETE | `/memos/{memoId}` | 削除 |

```json
// POST /memos Request
{ "content": "メモ内容" }
```

---

## Note Service (8091)

統合ノート（日記・学習記録）を管理。Markdown/Drawio対応、MinIOファイルストレージ連携。

### Note Type

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/note-types` | 利用可能なノートタイプ一覧 |

### Note

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/notes` | 一覧（フィルタ多数） |
| POST | `/notes` | 作成 |
| GET | `/notes/search` | 検索（`?q=`必須, `?types=`, `?archived=`, `?limit=`） |
| GET | `/notes/{noteId}` | 詳細 |
| PUT | `/notes/{noteId}` | 更新 |
| DELETE | `/notes/{noteId}` | 削除 |
| POST | `/notes/{noteId}/archive` | アーカイブ切り替え |

**GET /notes クエリパラメータ:**

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `types` | string | ノートタイプフィルタ |
| `goal_id` | string | 目標でフィルタ |
| `milestone_id` | string | マイルストーンでフィルタ |
| `task_id` | string | タスクでフィルタ |
| `format` | string | `markdown` / `drawio` |
| `date_from` | string | 日付範囲（開始） |
| `date_to` | string | 日付範囲（終了） |
| `archived` | boolean | アーカイブ済みフィルタ |
| `q` | string | 全文検索 |
| `tag_ids` | string | タグでフィルタ |

```json
// POST /notes Request
{
  "type": "learning_record",
  "title": "Kubernetes Schedulerの仕組み",
  "content": "# Kubernetes Scheduler\n\n...",
  "format": "markdown",
  "date": "2026-01-21",
  "metadata": {},
  "tagIds": ["uuid"]
}

// POST /notes/{noteId}/archive Request
{ "archived": true }
```

### NoteContent（ブロックコンテンツ）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/notes/{noteId}/contents` | コンテンツ一覧 |
| POST | `/notes/{noteId}/contents` | コンテンツ追加 |
| GET | `/notes/{noteId}/contents/{contentId}` | コンテンツ詳細 |
| PUT | `/notes/{noteId}/contents/{contentId}` | コンテンツ更新 |
| DELETE | `/notes/{noteId}/contents/{contentId}` | コンテンツ削除 |
| PATCH | `/notes/{noteId}/contents/reorder` | 並び替え |

```json
// POST /notes/{noteId}/contents Request
{ "contentType": "markdown", "data": "..." }

// PATCH /notes/{noteId}/contents/reorder Request
{ "contentIds": ["uuid1", "uuid2"] }
```

### Storage（ファイル管理）

| メソッド | パス | 説明 |
|---------|------|------|
| POST | `/notes/{noteId}/contents/upload-url` | 署名付きアップロードURL取得 |
| GET | `/notes/{noteId}/contents/{contentId}/download-url` | ダウンロードURL取得 |

```json
// POST /notes/{noteId}/contents/upload-url Request
{ "fileName": "diagram.drawio", "mimeType": "application/xml", "fileSize": 12345 }
```

---

## AI Service (8089)

Python (FastAPI) で実装されたAIエージェントサービス。Claude API または Gemini API を使用し（`AI_PROVIDER`環境変数で切替: `anthropic` or `google`）、DBに直接接続（asyncpg）してデータ取得・操作を行う。39個の Direct Tools で DB 操作、メモリ管理、検索、レビュー、Web検索などを実行する。

### Health

| メソッド | パス | 認証 | 説明 |
|---------|------|------|------|
| GET | `/health` | 不要 | ヘルスチェック |

### Agent（ストリーミング対話）

| メソッド | パス | 説明 |
|---------|------|------|
| POST | `/agent/stream` | エージェントストリーミング（SSE） |
| POST | `/agent/approve` | アクション承認・実行 |

```json
// POST /agent/stream Request
{
  "message": "明日の予定を作って",
  "situation": "...",
  "context": "...",
  "conversation_id": "uuid"
}

// SSE Response Events:
// event: text         - テキスト応答
// event: action_proposal - 書き込み操作の提案（承認待ち）
// event: tool_call    - ツール呼び出し
// event: tool_result  - ツール結果
// event: done         - 完了

// POST /agent/approve Request
{
  "conversation_id": "uuid",
  "action_ids": ["uuid1", "uuid2"]
}
```

### Conversation（会話履歴）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/conversations` | 会話一覧（`?limit=`, `?offset=`） |
| GET | `/conversations/{conversation_id}` | 会話詳細 |

### Feedback

| メソッド | パス | 説明 |
|---------|------|------|
| POST | `/interactions/{interaction_id}/feedback` | フィードバック送信 |

```json
// Request
{ "rating": 5, "feedback": "参考になりました" }
```

---

## 変更履歴

| バージョン | 日付 | 変更内容 |
|-----------|------|----------|
| 1.0.0 | 2025-01-05 | 初版作成（旧: Record/Diary/Sync/gRPC含む） |
| 2.0.0 | 2026-02-01 | 実装に合わせて全面改訂。旧サービス削除、Task Service拡張（Goal/Milestone/Tag/Todo/EntityMemo）、AI Service更新（エージェント/ストリーミング）、Note Service追加、Timer追加 |
