# Storage roadmap: local-path → Longhorn

現状 PV は全て `local-path-provisioner` (node-local) で運用している。HA stateful workload (Vault raft, Keycloak PostgreSQL) は **形式上 HA** だが、PVC が作成ノードに張り付くため node 障害時の data 喪失リスクを許容している状態。

Longhorn 導入で分散ストレージに切り替える計画。

## 現状の制約

- **node-local**: PVC は最初に bind されたノードに張り付く。Pod を別ノードへ移動するには PVC 再作成 = data 喪失
- **HA は形式上**: vault-0/1/2 は anti-affinity で別ノードに分散しているので「3 中 1 ノード障害なら quorum 維持」は成立する。ただし障害ノード上の Vault raft 1 peer 分のデータは喪失するため、復旧は raft 再 join (peer データ自動 sync) で対応

## 影響を受けているコンポーネント

| Component | PVC | 現 storageClass | 切替先 |
|---|---|---|---|
| Vault raft | `data-vault-{0,1,2}` (10Gi × 3) | `local-path` | `longhorn-single` (replica 1、Vault raft 自体が 3 重化済み) |
| Vault audit | `audit-vault-{0,1,2}` (10Gi × 3) | `local-path` | `longhorn-single` |
| Keycloak PostgreSQL | TBD | `local-path` | `longhorn-single` (主は Keycloak HA replica) |
| Prometheus / Loki / Tempo | (未調査) | `local-path` | TBD |

`longhorn-single` を選ぶ理由: Vault raft / Keycloak は app 層で多重化済みなので、storage 層でも replicate すると合計 9 倍になり homelab スケールには重い。`longhorn-single` (replica=1) で「ノード障害でも PVC が他ノードで復旧する」だけ得たい。

## Phase

1. Longhorn 導入 (別 ADR / 別 PR)
2. 各 component の `storageClass` を `local-path` → `longhorn-single` に切替
3. PVC migration (data の Longhorn volume への移行)
4. local-path-provisioner を削除 (`infrastructure/storage/`)

## 既知の前提

- Longhorn は kernel module (iscsi_tcp) と open-iscsi が必要 → Pi 5 と Bosgame M4 Neo 全ノードで事前確認
- Pi 5 の microSD は IO 性能が低い → Longhorn replica 配置は AMD64 ノード優先で検討

## 関連

- [Stage 1 Bootstrap](../bootstrapping/12-vault-stage1.md): Vault が `local-path` を使っている現状
- [Longhorn restore test](../runbooks/longhorn-restore-test.md): 既存の Longhorn restore runbook
