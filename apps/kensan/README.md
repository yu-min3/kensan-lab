# kensan

ファイルベースのナレッジ & ゴール管理アプリ（統合版）。Markdown ファイルを Single Source of Truth として、Go 単一サービス + React UI（Whetstone design system）で操作する。

旧 kensan（`apps/kensan-legacy/`）と kensan-app-v2 のいいとこ取り統合。計画: kensan-workspace `projects/kensan-workspace/unification-plan.md`

## 設計原則

- **インデックスを持たない**: リクエスト毎に WalkDir + stat、パース結果は mtime/size キーでメモ化。正しさは常に stat に立脚し、キャッシュは速度だけを担う（fsnotify・再スキャンループ・起動時リビルドなし）
- **寛容なパース**: frontmatter が壊れていてもエラーにせず「未分類」として返す
- **conventions.md が契約**: Claude Code / VSCode / app が同じ規約で同じファイルを触る

## 構成

```
backend/   Go 単一サービス（REST API + ビルド済み SPA 配信 + kensan CLI）
frontend/  React SPA（design system は packages/design-tokens の Whetstone tokens）
Dockerfile 単一 image（frontend を build → dist を Go バイナリに同梱）
```

本番は **単一コンテナ image**。`frontend/dist` をビルドして `/srv/dist` に置き、Go の静的配信（`KENSAN_STATIC_DIR`）が `/api/*`・`/healthz` 以外を SPA fallback で返す（`backend/internal/api/static.go`）。ローカル開発時のみ Vite dev server（`localhost:5173`）を別に立て、API は CORS 許可で受ける。

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

ルート定義は `backend/internal/api/server.go`（`Handler()`）が単一の真実。

| Method | Path | 説明 |
|--------|------|------|
| GET | `/api/v1/files?type=&tag=&status=&q=` | ドキュメント一覧（frontmatter フィルタ） |
| POST | `/api/v1/files` | ファイル新規作成 |
| GET | `/api/v1/files/{path...}` | ファイル詳細（メタ + 本文） |
| PUT | `/api/v1/files/{path...}` | ファイル更新 |
| DELETE | `/api/v1/files/{path...}` | ファイル削除 |
| GET | `/api/v1/daily?date=YYYY-MM-DD` / `?limit=N` | daily 取得 / 直近一覧 |
| GET | `/api/v1/tasks` | タスクボード（today / stock / someday / milestones、project 紐付き） |
| POST | `/api/v1/tasks/move` | タスク行をファイル間で移動（かんばんの DnD） |
| PATCH | `/api/v1/tasks` | タスクのチェック状態変更 |
| GET | `/api/v1/tags` | タグ集計 |
| GET | `/api/v1/reviews` | レビュー成果物一覧（weekly/daily/monthly の HTML） |
| GET | `/api/v1/reviews/{path...}` | レビュー HTML の配信 |
| GET | `/api/v1/stats` | 統計（type 別件数・タスク数・今月の daily 数） |
| GET | `/api/v1/search?q=&type=` | 全文検索 |
| GET | `/healthz` | ヘルスチェック |

## 運用アーキテクチャ

デプロイ定義は `kubernetes/apps/app-kensan/`（`charts/app-base` chart の最初の消費者）。`app-kensan` 専用 namespace で動く。

- **画面構成** — ダッシュボード / daily / notes / memo に加えて、
  - **タスクかんばん**（`frontend/src/pages/TasksPage.tsx`）— ストック（`projects/<p>/README.md` の `## タスク`）と今日やる（`todo.md` の `## Now`）を DnD で行移動。`/morning` が Claude 側で行う操作と対称。
  - **レビュービューワ**（`frontend/src/pages/ReviewsPage.tsx`）— `/weekly-review`・`/reflection` が生成した `reviews/` 配下の HTML をそのまま表示（可視化の進化は Claude の HTML 生成側が担う契約）。
- **workspace volume** — Markdown の実体は Longhorn 上の `kensan-workspace` PVC（Retain）。app pod に `/data`（`KENSAN_DATA_DIR`）でマウント。
- **Syncthing 同期** — Mac のローカル workspace ⇄ クラスタ volume を Syncthing で双方向同期する（`resources/syncthing.yaml`）。
  - **LAN-only** — 同期ポート 22000 を Cilium L2 の **VIP 192.168.0.245** で LAN に出し、Mac が direct 接続。global discovery / relay / NAT traversal は無効。
  - **config-guard initContainer** — Syncthing の default では global discovery/relay/NAT が有効なため、pod 起動のたびに `config.xml` を書き換えて LAN-only 不変条件を Git 側から強制する（config PVC が空でも drift しても効く）。
  - GUI（8384）は非公開。device 鍵を持つ config PVC は `Prune=false`。git 操作は Mac のみ（cluster 側は読むだけ）。
