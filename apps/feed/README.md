# feed

Personal Daily BriefingとProject Metricsを外部サービスからworkspaceへ取り込む短命なbatch application。

## Commands

```bash
go run ./cmd/feed validate /path/to/YYYY-MM-DD.md
go run ./cmd/feed run
go run ./cmd/feed run --date 2026-07-25
```

## Environment

| Variable | Required | Default |
|---|---:|---|
| `KENSAN_DATA_DIR` | yes | - |
| `GOOGLE_DRIVE_OUTPUT_FOLDER_ID` | yes | - |
| `GOOGLE_APPLICATION_CREDENTIALS` | yes | - |
| `GITHUB_TOKEN` | no | - |
| `FEED_TIMEZONE` | no | `Asia/Tokyo` |

Google Service Accountには、Google DriveのBriefing output folderだけをViewerとして共有する。

## Current scope

- Drive上の日付別Markdownを検索・download
- schemaとMarkdown安全性の検証
- workspace frontmatterへの変換
- `feeds/YYYY/MM/DD.md`へのatomic write
- `feeds/state/import.json`への同期状態保存
- `projects/*/metrics.yaml`の`github-stars`を収集
- 同日同値の`metrics.ndjson` observationを重複させない

Briefingと一部Projectの取得に失敗しても、独立した残りの処理は継続する。
最終的に1件でも失敗があればprocessは非zeroで終了する。

KubernetesのCronJobと手動再実行手順は
`kubernetes/apps/app-kensan/FEED.md`を参照。
