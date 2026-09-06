# Operating the personal daily briefing

The `feed` CronJob fires once each morning at 07:15 (Asia/Tokyo) and, in the same
process, imports Markdown from Google Drive and refreshes GitHub stars for every
project. The kensan Deployment is never given Google or GitHub credentials.

## Registering the secret the first time

The Google service account JSON goes into Vault KV v2 and is synced into a
Kubernetes Secret by External Secrets Operator from
`app-kensan/feed-google-drive`. Do not create the Kubernetes Secret by hand.

```bash
vault kv put secret/app-kensan/feed-google-drive \
  credentials.json=@/path/to/service-account.json \
  output-folder-id='<drive-folder-id>'
```

Once Argo CD syncs `feed-google-drive-external-secret.yaml`, ESO creates the
`feed-google-drive` Secret. Only the CronJob mounts it, read-only. Grant the
service account nothing beyond Viewer on the Drive output folder.

The Drive output folder ID is not itself a credential, but it is kept in the same
Vault path so that no identifier is exposed in a public repository and the whole
handover is one contract.

Check the sync:

```bash
kubectl -n app-kensan get externalsecret feed-google-drive
kubectl -n app-kensan get secret feed-google-drive
```

## Running it by hand

An ordinary re-run:

```bash
kubectl -n app-kensan create job --from=cronjob/feed feed-manual-YYYYMMDD-HHMM
kubectl -n app-kensan logs -f job/feed-manual-YYYYMMDD-HHMM
```

To target a specific date:

```bash
kubectl -n app-kensan create job --from=cronjob/feed feed-retry-YYYYMMDD \
  --dry-run=client -o yaml > /tmp/feed-retry.yaml
# edit args in /tmp/feed-retry.yaml to ["run", "--date", "YYYY-MM-DD"]
kubectl apply -f /tmp/feed-retry.yaml
```

A failed job leaves the existing `feeds/` and `projects/*/metrics.ndjson` intact.
Delete manual jobs individually once you have checked the result.
