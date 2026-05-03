# bootstrap/vault — Vault Bootstrap (Pattern A')

このディレクトリは **Vault HA cluster の信頼ルート設定**を Terraform で行う。
secrets-phase1-design.md の **Pattern A'（TF 使い捨て）**の実体。

## 位置付け

| 役割 | 担当 |
|---|---|
| **一回限り**: auth methods enable, OIDC config, root admin policy, K8s auth roles, KV mount, audit device enable | **このディレクトリ (Terraform)** |
| **永続的**: per-app policy, per-app KV path, DB role 等 | vault-config-operator (CRD) via ArgoCD |

apply 完了後 **state は破棄する**。再実行は新規 cluster 構築 / DR 復旧時のみ。

## なぜ Bootstrap TF が必要か (鶏卵問題の解)

ArgoCD + Helm だけで Vault を運用したいところだが、**鶏卵問題が 2 つ**ある。

### 鶏卵その 1: VCO bootstrap (これが本命)

`vault-config-operator (VCO)` は Vault 設定を CRD で管理する operator。
だが VCO 自身が Vault に認証するためには K8s auth method + role が要る。
そして K8s auth method の enable と role 作成こそが「Vault 設定を変える」操作。

```
VCO が Vault 設定を変える → "auth method を有効化したい"
   ↓
VCO は Vault に認証要求 → "kubernetes auth method 使う"
   ↓
"kubernetes auth method はまだ有効化されてない"  ← 詰む
```

→ **誰かが先に外から** auth method を enable + VCO role を作る必要がある。
   それが Bootstrap TF。一回これをやれば、以降は VCO が永続管理を引き継ぐ。

### 鶏卵その 2: Keycloak ↔ Vault (#4 で別解決済み)

- Vault は Keycloak で人間 OIDC 認証する (Vault → Keycloak 依存)
- もし Keycloak の DB 認証を Vault dynamic creds にすると逆方向の依存も発生 = 循環

→ secrets-phase1-design.md #4 で、**Keycloak の DB 認証は Sealed Secrets で永続静的管理** することに確定。
   これで Keycloak は Vault に依存しない (片道のみ)、循環解消。

### なぜ ArgoCD + Helm だけで bootstrap できないか

ArgoCD/Helm が触れる範囲は **K8s manifest (Pod, Service, PVC 等) だけ**。
Vault の auth methods / policies / secret engines / OIDC config は **Vault 内部の状態**で、
Vault API (HTTP / CLI) でしか操作できない。ArgoCD は Vault API call の能力なし。

```
┌─ ArgoCD/Helm が触れる範囲 ─┐
│  K8s manifest                │ ← ここまではフル GitOps
│  - Vault server pod          │
│  - VCO pod, ESO pod          │
└──────────────────────────────┘
              ↓
┌─ ここから Vault の "中身" ────────┐
│  auth methods, policies,        │ ← K8s manifest じゃない、
│  secret engines, OIDC config    │   Vault API でしか触れない
└──────────────────────────────────┘
              ↑
        ┌─────┴─────┐
        │           │
   [Bootstrap TF]  [VCO (CRD via ArgoCD)]
   一回限り        永続管理
```

### Pattern A vs A' (なぜ TF を選んだか)

| | Pattern A (純 GitOps) | Pattern A' (採用) |
|---|---|---|
| Bootstrap | Helm chart の post-install Job で bash + vault CLI | Terraform 1 回 apply、state 破棄 |
| GitOps 純度 | ◎ | ◯ (TF は外で 1 回だけ) |
| 読みやすさ | △ bash は脆い、root token 扱いが汚い | ◎ HCL で宣言的 |
| 冪等性 | 自前 | TF が面倒見る |
| 再現性 | scripts/ 配下に置けば OK | このディレクトリそのもの |

Pattern A' は「bash の脆さを TF で代替、ただし state は持たない」が要旨。
state を持たないので **永続的な Vault 設定**は VCO + ArgoCD に任せられる (思想を保てる)。

詳細は `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` 参照。

## 前提条件 (順番に確認)

1. **Vault HA cluster up**: `infrastructure/security/vault/` が ArgoCD で sync 済み、3 pod が `Running`
2. **Vault initialized**: `kubectl exec -n vault vault-0 -- vault operator init` 実行済み
   - Recovery Keys (Shamir 5/3) を **1Password に保存**
   - Initial root token を控える (このあと TF で使う)
3. **AWS KMS auto-unseal が動作**: `vault status` で `Sealed: false` 確認
4. **Keycloak realm `kensan` 構築済み**:
   - groups: `platform-admin` (Yu in it), `platform-dev`
   - OIDC client `vault` 作成、Valid Redirect URIs 設定:
     - `https://vault.platform.yu-min3.com/ui/vault/auth/oidc/oidc/callback`
     - `http://localhost:8250/oidc/callback` (CLI 用)
   - Client Authentication: Client secret 取得
5. **Vault に到達できる経路を確保**:
   - 一番楽: `kubectl port-forward -n vault svc/vault 8200:8200` でローカル `localhost:8200` に出す
   - or `https://vault.platform.yu-min3.com` 経由 (Keycloak SSO 通る前なので少し面倒)

## 実行手順

### 1. terraform.tfvars を作成

```bash
cp /dev/null terraform.tfvars
$EDITOR terraform.tfvars
```

中身:
```hcl
vault_address               = "http://localhost:8200"  # port-forward 経由
vault_token                 = "<initial root token>"
keycloak_realm_url          = "https://auth.platform.yu-min3.com/realms/kensan"
keycloak_oidc_client_id     = "vault"
keycloak_oidc_client_secret = "<from Keycloak admin UI>"
emergency_admin_password    = "<generated, store in 1Password>"
```

### 2. apply

```bash
terraform init
terraform plan
terraform apply
```

### 3. 動作確認

```bash
# OIDC ログイン (Keycloak 経由)
vault login -method=oidc role=platform-admin

# K8s auth role 確認
vault read auth/kubernetes/role/vault-config-operator
vault read auth/kubernetes/role/external-secrets

# KV mount 確認
vault secrets list

# audit device 確認
vault audit list

# Pod 側からの認証確認 (vault-config-operator namespace で)
kubectl run -it --rm test --image=curlimages/curl --restart=Never \
  --serviceaccount=default --namespace=vault-config-operator -- \
  curl -X POST http://vault.vault.svc:8200/v1/auth/kubernetes/login \
    -d "{\"role\":\"vault-config-operator\",\"jwt\":\"$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\"}"
```

### 4. クリーンアップ (Pattern A' の核)

```bash
# state を破棄 (絶対 commit しない)
rm -rf .terraform/ .terraform.lock.hcl terraform.tfstate*

# tfvars も破棄 (機密含むので gitignore でも履歴に入らないよう注意)
rm terraform.tfvars

# Vault root token を revoke (root token は使い捨て)
vault token revoke <initial root token>
```

これ以降の Vault 操作は **OIDC ログイン (人間)** か **K8s auth (operator)** で行う。

## DR シナリオ (Vault 全損)

新規 cluster で同じ手順をやり直す:

1. Vault HA を再 deploy
2. `vault operator init` で新規 root token + Recovery Keys
3. (※ 旧 Recovery Keys + snapshot から restore する経路もあるが、その場合は `vault operator raft snapshot restore` で別ルート)
4. このディレクトリで `terraform apply` 再実行
5. 終わったら state 破棄

state 破棄前提なので、実行のたびに「全部新規作成」される。Vault 側に既に同名 resource があるとエラーになるので、その時は `terraform import` で取り込むか、Vault 側を手動で reset する。

## Files

| File | 内容 |
|---|---|
| `versions.tf` | Terraform 1.6+, hashicorp/vault provider ~> 5.0 |
| `variables.tf` | 入力変数 (vault_token, keycloak_*, emergency_admin_password) |
| `main.tf` | Provider 設定 |
| `auth.tf` | auth methods enable + OIDC + K8s roles + userpass |
| `policies.tf` | admin / vco-admin / eso-read / platform-dev policy |
| `engines.tf` | KV v2 mount (`secret/`) + audit devices x2 |
| `.gitignore` | state / tfvars / lock 全部除外 |

## 関連ドキュメント

- `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` — 全体設計
- `kensan-workspace/projects/kensan-lab/secrets-plan.md` — Phase 計画
- `docs/bootstrapping/12-vault-stage1.md` (Stage 1 完了後に作成予定)
