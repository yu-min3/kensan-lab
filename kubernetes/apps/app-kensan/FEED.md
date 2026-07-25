# Personal Daily Briefing 運用

`feed` CronJobは毎朝07:15（Asia/Tokyo）に1回だけ起動し、Google Driveの
Markdown importと全ProjectのGitHub Stars更新を同じprocessで実行する。
kensan DeploymentにはGoogle/GitHub credentialを渡さない。

## 初回Secret作成

Google Service Account JSONとDrive output folder IDはGitにcommitせず、
clusterへ直接登録する。

```bash
kubectl -n app-kensan create secret generic feed-google-drive \
  --from-file=credentials.json=/path/to/service-account.json \
  --from-literal=output-folder-id='<drive-folder-id>'
```

対象SecretはこのCronJobだけがmountする。Service AccountにはDrive output
folderのViewer権限だけを付与する。

## 手動実行

通常の再実行:

```bash
kubectl -n app-kensan create job --from=cronjob/feed feed-manual-YYYYMMDD-HHMM
kubectl -n app-kensan logs -f job/feed-manual-YYYYMMDD-HHMM
```

対象日を指定する場合:

```bash
kubectl -n app-kensan create job --from=cronjob/feed feed-retry-YYYYMMDD \
  --dry-run=client -o yaml > /tmp/feed-retry.yaml
# /tmp/feed-retry.yaml の args を ["run", "--date", "YYYY-MM-DD"] に変更
kubectl apply -f /tmp/feed-retry.yaml
```

Jobが失敗しても既存の`feeds/`と`projects/*/metrics.ndjson`は残る。
確認後、不要な手動Jobは個別に削除する。
