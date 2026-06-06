# kensan-next

ファイルベースのナレッジ & ゴール管理アプリ（統合版）。Markdown ファイルを Single Source of Truth として、Go 単一サービス + React UI（Whetstone design system）で操作する。

旧 kensan（`apps/kensan/`）と kensan-app-v2 のいいとこ取り統合。計画: kensan-workspace `projects/kensan-workspace/unification-plan.md`

## 設計原則

- **インデックスを持たない**: リクエスト毎に WalkDir + stat、パース結果は mtime/size キーでメモ化。正しさは常に stat に立脚し、キャッシュは速度だけを担う（fsnotify・再スキャンループ・起動時リビルドなし）
- **寛容なパース**: frontmatter が壊れていてもエラーにせず「未分類」として返す
- **conventions.md が契約**: Claude Code / VSCode / app が同じ規約で同じファイルを触る

## 構成

```
backend/   Go 単一サービス（REST API + kensan CLI）
frontend/  React SPA（別途。design system は packages/design-tokens）
```

## 開発

```bash
cd backend
go run ./cmd/kensan          # serve がデフォルト。KENSAN_DATA_DIR=~/kensan-workspace
go test ./...
```

環境変数:

| 変数 | 既定値 | 説明 |
|------|--------|------|
| `KENSAN_DATA_DIR` | `~/kensan-workspace` | workspace のパス |
| `KENSAN_ADDR` | `:8080` | リッスンアドレス |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | （未設定 = 無効） | 設定すると OTLP trace export が有効化 |

## API

| Method | Path | 説明 |
|--------|------|------|
| GET | `/api/v1/files?type=&tag=&status=&q=` | ドキュメント一覧（frontmatter フィルタ） |
| GET | `/api/v1/files/{path...}` | ファイル詳細（メタ + 本文） |
| GET | `/api/v1/daily?date=YYYY-MM-DD` / `?limit=N` | daily 取得 / 直近一覧 |
| GET | `/api/v1/tasks` | タスクボード（today / stock / someday / milestones、project 紐付き） |
| GET | `/api/v1/tags` | タグ集計 |
| GET | `/api/v1/stats` | 統計（type 別件数・タスク数・今月の daily 数） |
| GET | `/api/v1/search?q=&type=` | 全文検索 |
| GET | `/healthz` | ヘルスチェック |
