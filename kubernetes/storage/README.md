# storage

PersistentVolume プロビジョナ。Longhorn (レプリケート block storage) に一本化済み（`local-path` は 2026-07 全廃）。

## 構成

| dir | 役割 | StorageClass |
|---|---|---|
| `longhorn/` | Longhorn (replicated block, default)。R2 への定期 backup + snapshot RecurringJob 同梱 | `longhorn` (default) |

## 使い分け

- **新規ワークロードは `longhorn`**。複数ノードに block レベルで複製されるので、ノード障害でデータ消失しない

Longhorn 移行は commit 45a6a61 で完了済み。Postgres 系 4 個は commit cb3cdb6 で Longhorn 移行済み。

## Longhorn 注意点

- **`Prune=false`**: Application 削除で Longhorn ns + PV データが蒸発するのを防ぐ
- **disaster recovery runbook**: [`docs/runbooks/longhorn-restore-test.md`](../../docs/runbooks/longhorn-restore-test.md)
- バックアップターゲットは Cloudflare R2 (credential は SealedSecret)

## 関連

- ストレージ詳細 + R2 構成: [`docs/architecture/infrastructure.md`](../../docs/architecture/infrastructure.md) (`Storage` 章)
- node-local 制約とスケジューリング: [`.claude/rules/kubernetes-cluster.md`](../../.claude/rules/kubernetes-cluster.md)
