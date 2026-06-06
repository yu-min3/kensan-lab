---
description: React/TypeScript frontend conventions
globs: frontend/src/**/*.{ts,tsx}
---

# Frontend Conventions

## Architecture

```
Component → Zustand Store → API Service → HttpClient → Backend
```

## Component Hierarchy

1. **UI** (`components/ui/`): shadcn/ui primitives。ビジネスロジックなし。
2. **Common** (`components/common/`): ドメイン知識を持つ再利用コンポーネント。
3. **Page** (`pages/`): ルートに対応するページコンポーネント。

## Page Naming Convention

| Prefix | Domain | Example |
|--------|--------|---------|
| S | Settings/System | S01_Settings |
| D | Daily | DailyPage |
| W | Weekly | W01_WeeklyPlanning |
| N | Notes | N01_NoteList, N02_NoteEdit |
| T | Task | T01_TaskManagement |
| A | Analytics/AI | A01_AnalyticsReport, A03_PromptEditor |
| O | Observability | O01_InteractionExplorer |

## State Management (Zustand)

- ストアファクトリ: `createCrudStore` で標準 CRUD パターン
- 認証: `useAuthStore` (localStorage persist, key: 'kensan-auth')
- 設定: `useSettingsStore` (localStorage persist, key: 'kensan-settings')
- ISP ベースの分離: Goal, Milestone, Tag, Task は個別ストア

## API Client

- `httpClient` シングルトン。自動 JWT ヘッダー、traceparent 生成。
- レスポンスエンベロープ (`{data, meta, pagination}`) を自動アンラップ。
- `createApiService` ファクトリで標準 CRUD API を生成。

## Timezone (重要)

- DB は UTC 保存。フロントエンドでローカル変換。
- `localDateToUtcRange()` でクエリ範囲取得
- `getLocalDate()` / `getLocalTime()` で表示変換
- `localToUtcDatetime()` で作成/更新時の UTC 変換

## Styling

- Tailwind CSS 4.x + `cn()` ヘルパー (`clsx` + `twMerge`)
- セマンティックカラー: CSS 変数ベース (`--background`, `--primary`, `--brand`)
- パスエイリアス: `@/*` → `./src/*`
