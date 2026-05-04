# Storage

kensan-lab のストレージ層。**ノード固定の local-path** と **分散ブロックストレージ Longhorn** を併用し、ワークロード特性に応じて使い分ける。

## レイアウト

```
storage/
├── local-path-provisioner/
│   └── local-path-provisioner.yaml          # raw manifest (Pattern B)
└── longhorn/
    ├── values.yaml                          # Helm chart values (Pattern A)
    └── resources/
        ├── namespace.yaml                   # PSA: privileged
        ├── httproute.yaml                   # UI を gateway-platform 経由で公開
        └── r2-credential-sealed.yaml        # Cloudflare R2 backup target credential
```

## 提供する StorageClass

| StorageClass | Provisioner | Replica | Default | 用途 |
|---|---|---|---|---|
| `local-path` | `rancher.io/local-path` | n/a (ノード固定) | ✅ | ephemeral / 再生成可能なキャッシュ / Backstage や ArgoCD のメタデータ |
| `longhorn` | `driver.longhorn.io` | 1 (Phase 1) → 3 (Phase 3) | ❌ | 永続データ (Postgres, Iceberg, Prometheus 等)、Phase 3 で本格採用 |

## Longhorn デプロイ範囲 (Phase 1)

- **対象ノード**: `hardware-class=high-performance` を持つノードのみ (現状 m4neo 1 台)
- **理由**: Pi5 群 (master, worker1, worker2) は microSD / USB ストレージのみで、Longhorn の I/O・寿命要件を満たさない
- **データパス**: `/opt/longhorn` (root 同居、Phase 3 で `/mnt/longhorn` 専用 SSD に切替予定)
- **replica=1**: Phase 1 は HA 諦め。Phase 3 で worker Pi5 ×2 + Bosgame + Desktop の 4 ノード分散・replica=3 へ

```yaml
# values.yaml (抜粋)
longhornManager:
  nodeSelector:
    hardware-class: high-performance
defaultSettings:
  systemManagedComponentsNodeSelector: "hardware-class:high-performance"
  defaultReplicaCount: 1
  defaultDataPath: /opt/longhorn
```

## Backup: Cloudflare R2

Longhorn の backup target に **R2** を S3 互換 API 経由で接続。**egress 無料**がリストア検証用途で効く。

```yaml
# values.yaml (抜粋)
defaultSettings:
  backupTarget: s3://kensan-lab-longhorn-backup@auto/
  backupTargetCredentialSecret: longhorn-r2-backup
```

R2 credential は SealedSecret で投入 (`resources/r2-credential-sealed.yaml`)。Phase 3+α で Vault KV + ESO 経由に移行予定。

| | Cloudflare R2 |
|---|---|
| ストレージ単価 | $0.015/GB/月 |
| **Egress** | **$0**（無料）|
| 想定月額 (Phase 3 本番、180GB) | 約 **$2.25** |
| Region | `auto`（jurisdiction-specific endpoint 不使用）|

リストア検証手順は [`docs/runbooks/longhorn-restore-test.md`](../../docs/runbooks/longhorn-restore-test.md)。

## UI へのアクセス

- 通常: `https://longhorn.platform.yu-min3.com/` (gateway-platform 経由、LAN-only DNS)
- Auth: 現状 Gateway レベル auth なし (ADR-005 の Istio native oauth2 実装後に保護される設計)
- 緊急時 / Gateway 経路死亡時: `kubectl -n longhorn-system port-forward svc/longhorn-frontend 8080:80`

## 実装フェーズ

| Phase | スコープ | 状態 |
|---|---|---|
| 1 | m4neo 単独、replica=1、`/opt/longhorn`、R2 backup 動線確認 | ✅ 完遂 (2026-05-04) |
| 2 | Verbatim Vi3000 256GB ×4 購入、Pi5 群有線化、Desktop 追加 | 🔲 ハードウェア発注待ち |
| 3 | 5 ノード replica=3、Tier StorageClass 階層、`/mnt/longhorn`、RecurringJob、Prometheus アラート | 🔲 Phase 2 後 |

詳細な設計判断・トレードオフ・実装ログは Yu の作業 repo (kensan-workspace) `projects/kensan-lab/storage-plan.md` 参照（kensan-lab repo には含まれない）。

## 関連 ADR

- (起票予定) Longhorn 採用 — Ceph / TrueNAS / OpenEBS との比較、規模適合性、思想整合性
