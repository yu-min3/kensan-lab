# Personal Daily Briefing 運用

`feed` CronJobは毎朝07:15（Asia/Tokyo）に1回だけ起動し、Google Driveの
Markdown importと全ProjectのGitHub Stars更新を同じprocessで実行する。
kensan DeploymentにはGoogle/GitHub credentialを渡さない。

## 初回Secret登録

Google Service Account JSONはVault KV v2へ登録し、External Secrets Operatorで
`app-kensan/feed-google-drive`からKubernetes Secretへ同期する。
Kubernetes Secretを手動作成しない。

```bash
vault kv put secret/app-kensan/feed-google-drive \
  credentials.json=@/path/to/service-account.json \
  output-folder-id='<drive-folder-id>'
```

`feed-google-drive-external-secret.yaml`をArgo CDで同期すると、ESOが
`feed-google-drive` Secretを作成する。対象SecretはCronJobだけがread-only
mountする。Service AccountにはDrive output folderのViewer権限だけを付与する。

Drive output folder ID自体はcredentialではないが、公開repoへ識別子を露出させず
一つの受け渡し契約にまとめるため、同じVault pathで管理する。

同期確認:

```bash
kubectl -n app-kensan get externalsecret feed-google-drive
kubectl -n app-kensan get secret feed-google-drive
```

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
