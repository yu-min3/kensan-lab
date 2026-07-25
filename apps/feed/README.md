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
| `FEED_TIMEZONE` | no | `Asia/Tokyo` |

Google Service Accountには、Google DriveのBriefing output folderだけをViewerとして共有する。

## Current scope

- Drive上の日付別Markdownを検索・download
- schemaとMarkdown安全性の検証
- workspace frontmatterへの変換
- `feeds/YYYY/MM/DD.md`へのatomic write
- `feeds/state/import.json`への同期状態保存

GitHub Stars更新とKubernetes CronJobは後続phaseで追加する。
