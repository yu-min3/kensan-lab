## Vault Transit Engine (Stage 6)

kensan の users.name (個人氏名) を **Vault Transit API で暗号化** する capability。
Postgres には ciphertext + HMAC のみ保存し、平文の鍵管理を Vault に集約する。

## なぜ Transit か (vs pgcrypto / TDE)

| 方式 | 鍵の所在 | 鍵 rotation | 監査 |
|---|---|---|---|
| Postgres TDE | DB ファイルに紐付く KEK | DB 再起動級 | DB 内のみ |
| pgcrypto | Postgres の関数引数で都度渡す | アプリ実装次第 | DB 内のみ |
| **Vault Transit** | **Vault** に常駐、ciphertext が DB | `vault write -f transit/keys/<k>/rotate` | Vault audit log で API 単位に追える |

Vault Transit は **鍵を一切アプリ Pod / DB に出さない** envelope 暗号化のショーケース。
Stage 5 (動的 DB cred) で「DB cred の所在を Vault に集約」したのと同じ思想。

## 構成

```
infrastructure/security/vault-transit-engine/
├── README.md                                  # 本ファイル
├── shared/                                    # GitOps 管理 (Application: vault-transit-engine-shared)
│   ├── mount.yaml                             # SecretEngineMount (transit/)
│   ├── policy-kensan-users-transit.yaml       # Policy (kensan-users-transit)
│   └── vault-auth-role-kensan-user-service.yaml
│                                              # KubernetesAuthEngineRole
│                                              # (kensan-{prod,dev}/user-service SA を bind)
└── temp/
    └── setup-transit-keys.sh                  # 1 度きり: transit/keys/users-name 作成 (手動)
```

ArgoCD 側:
- `infrastructure/gitops/argocd/applications/security/vault-transit-engine/app-shared.yaml`

## 設計判断: なぜ key 作成だけ手動か

redhat-cop/vault-config-operator は **TransitSecretEngine 系 CR を持たない** (2026-05 時点)。
利用可能な CRD は `SecretEngineMount` (mount だけ) と `Policy` / `KubernetesAuthEngineRole` のみ。

選択肢:

| 案 | pros | cons | 採否 |
|---|---|---|---|
| 1. mount のみ GitOps、key は手動 | 単純、追加 operator 不要 | key 作成が手作業 (ただし 1 度きり) | ✅ **採用** |
| 2. 別 operator (ex: hashicorp/vault-secrets-operator) を導入 | key も declarative | operator 増、Stage 6 のためだけは過剰 | ✕ |
| 3. VCO を fork して TransitSecretEngine CR を追加 | declarative + 純正 VCO 1 本 | fork 維持コスト | ✕ |
| 4. CronJob で `vault write -f transit/keys/users-name` を冪等実行 | GitOps 寄り | "delete されないこと" だけ保証、操作は依然非 declarative | ✕ |

→ key 作成は 1 度きりかつ手動 rotate 運用なので、案 1 でコスト最小。

## デプロイ手順

### 1) GitOps (本 Application)

PR `vault-stage6-transit/vco-config` を merge → ArgoCD が自動 sync:

- `transit/` mount が enable される
- Policy `kensan-users-transit` が作成される
- KubernetesAuthEngineRole `kensan-users-transit` が作成される (kensan-{prod,dev}/user-service SA を bind)

### 2) Key 作成 (1 度きり、手動)

```bash
# vault Pod に入って実行 (もしくは macOS から VAULT_TOKEN export して実行)
kubectl exec -n vault vault-0 -c vault -i -- /bin/sh \
  < infrastructure/security/vault-transit-engine/temp/setup-transit-keys.sh
```

確認:
```bash
kubectl exec -n vault vault-0 -c vault -- vault list transit/keys
# → users-name が見える
```

### 3) user-service 側 (Stage 6 PR #2 / #3 で実装)

- `apps/kensan/manifests/base/app/user-service.yaml` の `spec.serviceAccountName: user-service`
- `infrastructure/kensan/kensan-{prod,dev}/serviceaccount-user-service.yaml` (本 PR で同梱)
- user-service Pod が起動時に Vault k8s auth method で login → token 取得 → `transit/encrypt/users-name` 等を呼ぶ

## Key Rotation 運用

```bash
# 旧 ciphertext は復号可、新規 encrypt は最新 version
kubectl exec -n vault vault-0 -c vault -- vault write -f transit/keys/users-name/rotate

# ※ 全行 rewrap したい場合 (Stage 6 後続タスク):
#   user-service の rewrap loop が transit/rewrap/users-name で更新する
```

## Audit (Vault audit log で encrypt/decrypt を追跡)

Vault audit device は bootstrap TF (Step e) で enable 済み (`/vault/audit/audit.log`)。
`transit/encrypt/users-name` / `transit/decrypt/users-name` 各 1 リクエストが per-call ログされる。
