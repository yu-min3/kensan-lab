# Vault storage migration: local-path → Longhorn

> **Status: 手順書ドラフト（未実行）。** リファクタリング計画 Phase 5 / W15。
> 実行は 1 セッションを確保し、各 Step の確認を通してから次へ進むこと。

## 背景

PR #324（災害復旧）で `kubernetes/secrets/vault/values.yaml` の storageClass は `longhorn` に
変更されたが、**live の StatefulSet は `local-path` のまま**動き続けている。
`volumeClaimTemplates` は immutable のため Argo CD の server-side diff が恒久的に失敗し、
`vault` Application は **SYNC STATUS: Unknown**（ComparisonError）を表示し続けている。

```
StatefulSet.apps "vault" is invalid: spec: Forbidden: updates to statefulset spec for fields
other than 'replicas', 'ordinals', 'template', ... are forbidden
```

さらに `local-path` は node-local（Pod を別ノードに移せない）かつ **reclaim=Delete**
（PVC 削除 = 物理データ即消滅）であり、Vault raft データの置き場として不適切。

## 現状（2026-07-06 実機確認）

| 項目 | 値 |
|---|---|
| StatefulSet | `vault` ns、replicas=3、live VCT: `local-path` 10Gi / git: `longhorn` |
| PVC | `data-vault-{0,1,2}` + `audit-vault-{0,1,2}`（worker2 / m4neo / worker1） |
| Unseal | AWS KMS auto-unseal（`alias/vault-unseal-kensan`）— **手動 unseal 不要** |
| Raft join | `retry_join` ×3 設定済み（`docs/runbooks/vault-raft-join.md`）— **手動 join 不要** |

この 2 点（auto-unseal + retry_join）により、**データを空にした member は Pod 再作成だけで
自動的に unseal → raft 再 join → leader からスナップショット同期**される。ノード単位の
「剥がして再 join」戦略が成立する。

## 戦略

- **Phase A**: STS を `--cascade=orphan` で削除 → Argo CD が longhorn VCT の STS を再作成
  （Pod は無停止）。→ ComparisonError 解消、`vault` app が Synced に戻る
- **Phase B**: member を 1 台ずつ（quorum 2/3 を常に維持）: raft remove-peer → PVC/Pod 削除 →
  新 Pod が longhorn PVC で起動し自動 re-join
- **Phase C**: 検証と後片付け

## 事前チェック（全て pass してから開始）

```bash
# 1. raft が 3 peers で健全（leader 1 + follower 2）
kubectl exec -n vault vault-0 -c vault -- sh -c 'VAULT_TOKEN=$(cat /tmp/token 2>/dev/null || echo $VAULT_TOKEN) vault operator raft list-peers'
# ※ 要ログイン。OIDC admin か root token で vault login してから実行

# 2. raft スナップショットを取得して手元に退避（最後の保険）
kubectl exec -n vault vault-0 -c vault -- vault operator raft snapshot save /tmp/pre-migration.snap
kubectl cp vault/vault-0:/tmp/pre-migration.snap ./temp/vault-pre-migration-$(date +%Y%m%d).snap -c vault

# 3. KMS auto-unseal が機能している（直近の Pod 再起動で Sealed:false になった実績 or）
kubectl exec -n vault vault-0 -c vault -- vault status | grep -E "Sealed|Total Shares"   # Sealed: false / Recovery Seed

# 4. Longhorn が健全（全 volume healthy、空き容量 >30Gi）
kubectl get volumes.longhorn.io -n longhorn-system --no-headers | awk '$3!="attached" && $2!="detached"' | head

# 5. ESO / dynamic secret が正常（移行中に切れると気づけるようベースライン確認)
kubectl get externalsecret -A --no-headers | grep -cv SecretSynced   # → 0 ならOK
```

## Phase A: StatefulSet の再作成（Pod 無停止・ComparisonError 解消）

```bash
# 1. STS だけ削除（Pod と PVC は残る）
kubectl delete sts vault -n vault --cascade=orphan

# 2. Argo CD が git（longhorn VCT）から STS を再作成するのを待つ
kubectl get application vault -n argocd -w   # → Synced / Healthy になること
kubectl get sts vault -n vault -o jsonpath='{.spec.volumeClaimTemplates[0].spec.storageClassName}'  # → longhorn

# 3. Pod が採用され再起動していないこと（AGE が維持されている）
kubectl get pods -n vault
```

**確認**: `vault` app の SYNC STATUS が Unknown → **Synced** に変わる。
既存 Pod は古い local-path PVC のまま動き続ける（VCT は新規 Pod にのみ効く）。

## Phase B: member を 1 台ずつ移行（vault-2 → vault-1 → vault-0）

leader を最後にする。`vault status | grep "HA Mode"` で active ノードを確認し、
active が vault-2/1 だったら先に `vault operator step-down` で leader を移す。

各 member について（例: vault-2）:

```bash
# 1. peer を raft から除名（クリーンな離脱）
kubectl exec -n vault vault-0 -c vault -- vault operator raft remove-peer vault-2

# 2. PVC を削除予約（Pod が使用中なので Terminating で待機する）
kubectl delete pvc data-vault-2 audit-vault-2 -n vault --wait=false

# 3. Pod を削除 → PVC 削除が完了し、STS が新 Pod + longhorn PVC を作る
kubectl delete pod vault-2 -n vault

# 4. 新 Pod の起動と自動復帰を確認（KMS auto-unseal + retry_join で全自動）
kubectl get pvc -n vault | grep vault-2          # → longhorn になっている
kubectl get pod vault-2 -n vault -w              # → 2/2 Running
kubectl exec -n vault vault-2 -c vault -- vault status | grep Sealed   # → false

# 5. raft に 3 peers 揃ったこと・同期済みを確認してから次の member へ
kubectl exec -n vault vault-0 -c vault -- vault operator raft list-peers
kubectl exec -n vault vault-0 -c vault -- vault operator raft autopilot state | grep -A5 Servers  # healthy: true
```

**やってはいけない**: 2 台同時に作業（quorum 喪失 = 全停止）。
1 台の再同期が完全に終わる（autopilot healthy）まで次に進まない。

vault-0 の番では、active leader なら先に step-down:

```bash
kubectl exec -n vault vault-0 -c vault -- vault operator step-down
# active が vault-1/2 に移ったのを確認してから vault-0 に同じ手順を適用
# remove-peer は新 leader に対して実行する:
kubectl exec -n vault vault-1 -c vault -- vault operator raft remove-peer vault-0
```

## Phase C: 検証・後片付け

```bash
# 全 PVC が longhorn
kubectl get pvc -n vault    # 6 本すべて STORAGECLASS=longhorn

# raft 3 peers / autopilot healthy / app Synced
kubectl exec -n vault vault-0 -c vault -- vault operator raft list-peers
kubectl get application vault -n argocd

# 消費者が無事（ESO 同期・Keycloak dynamic cred・cert-manager）
kubectl get externalsecret -A --no-headers | grep -cv SecretSynced   # → 0

# Longhorn 側: vault volume 6 本が healthy + RecurringJob (R2 backup) の対象になっているか
kubectl get volumes.longhorn.io -n longhorn-system | grep -c healthy
```

- 事前退避した raft snapshot は 1 週間程度残してから破棄
- local-path の PV はreclaim=Delete のため PVC 削除時に自動消滅（ノード上の残骸は
  `/opt/local-path-provisioner/` を目視確認）

## ロールバック

- **Phase A で失敗**（STS 再作成が変な状態）: `kubectl delete sts vault --cascade=orphan` →
  git revert 無しでも旧 VCT の STS を手で apply し直せる（ただし基本は前進あるのみ。
  Pod は生きているので落ち着いて対処できる）
- **Phase B で member が復帰しない**: 残り 2 peers で quorum は生きている。
  `vault-raft-join.md` の手動 join 手順 → それでもダメなら PVC/Pod をもう一度作り直す。
  **最悪時**は事前スナップショットから `vault operator raft snapshot restore`
- **絶対にやらないこと**: 復帰失敗の状態で次の member に手を付ける

## 残作業（このrunbookのスコープ外）

- `monitoring` ns の local-path PVC 5 本（prometheus / loki / tempo / grafana / alertmanager）
  の移行 — retention データなので「STS 再作成 + データ捨て」の割り切りも選択肢
- 全 local-path PVC 消滅後: `local-path-provisioner` と
  `applications/namespaces/local-path-storage` の撤去（W15 完了条件）
