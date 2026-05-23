# vault-transit-engine

Vault Transit secret engine の cluster-wide capability。
Postgres カラム等の **アプリ層暗号化** (Vault が鍵を保持、Pod / DB に鍵を降ろさない envelope encryption) を、consumer ごとに最小権限で払い出す。

## 構成

```
kubernetes/secrets/vault-transit-engine/
├── README.md                                # 本ファイル
├── chart/                                   # Helm chart (PE 管理、不動)
│   ├── Chart.yaml
│   ├── values.yaml                          # PE 規約のデフォルト値 (TTL / 命名規約)
│   └── templates/
│       ├── _helpers.tpl                     # name / basename / vcoAuth helper
│       ├── policy.yaml                      # Policy (key 単位、encrypt/decrypt/hmac/rewrap)
│       └── vault-auth-role.yaml             # KubernetesAuthEngineRole
├── shared/                                  # capability bootstrap (1 度だけ)
│   └── mount.yaml                           # SecretEngineMount (transit/)
├── platform-values/
│   └── vault-transit/                       # capability convention dir
│       └── kensan-users.yaml                # 1 consumer ぶん、AD が触る
└── temp/
    └── setup-transit-keys.sh                # 1 度きり: transit/keys/<name> 作成 (VCO 未対応)
```

ArgoCD 側:
- `applications/secrets/vault-transit-engine/app-shared.yaml` — single Application、`shared/` を sync (mount だけ)
- `applications/secrets/vault-transit-engine/applicationset-instances.yaml` — ApplicationSet、`**/platform-values/vault-transit/*.yaml` を glob で discover、per-consumer ArgoCD app を auto 生成

## 設計の核: smart default + override (vault-database-engine と同パターン)

PE が `chart/values.yaml` で convention based デフォルトを埋め、AD は **必須項目のみ** 書く。

### AD が書く項目

| キー | 必須? | デフォルト | 補足 |
|---|---|---|---|
| `ns` | ✅ 必須 | なし | consumer SA がある K8s namespace |
| `serviceAccount` | ✅ 必須 | なし | consumer SA 名 (chart は SA 自体は作らない、app 側 manifest に置く) |
| `keyName` | ✅ 必須 | なし | Transit key 名 (encrypt/decrypt 対象) |
| `extraKeyNames` | 任意 | `[]` | 同一 consumer に複数 key を許可したい場合 |
| `tokenTTL` / `tokenMaxTTL` | 任意 | `1800` / `3600` | Vault k8s auth token lease (秒) |
| `name` | 任意 | AppSet inject `transit-<filename>` | 通常書かない |

### PE 側の convention (chart 内に閉じる)

| 項目 | 値 |
|---|---|
| Vault role / policy 命名 | `transit-<filename-basename>` (vault-database-engine の `postgres-<base>` と対称) |
| Token TTL / maxTTL | 30 min / 1 h (default 12h+ 防止、renew loop 強制) |
| 許可 endpoint | `transit/{encrypt,decrypt,hmac,rewrap}/<keyName>` + `keys/<keyName>` read + `auth/token/{renew,lookup}-self` |

## 1 consumer 追加方法

(1) values file を 1 個書く、で済む。

```yaml
# <owner-dir>/platform-values/vault-transit/<consumer>.yaml
ns: my-app
serviceAccount: my-service
keyName: my-pii-column
```

これで以下が成立:
- Vault role / policy 名 = `transit-<consumer>` (filename から)
- `my-app` ns の `my-service` SA が Vault に bind される
- `transit/{encrypt,decrypt,hmac,rewrap}/my-pii-column` のみ最小権限で許可

key (`transit/keys/my-pii-column`) は **1 度きり手動作成**。`temp/setup-transit-keys.sh` を参考に新 key 名で実行する (VCO 未対応のため。後述「設計判断」参照)。

## 設計判断: なぜ key 作成だけ手動か

redhat-cop/vault-config-operator は **TransitSecretEngine 系 CR を持たない** (2026-05 時点)。
利用可能な CRD は `SecretEngineMount` / `Policy` / `KubernetesAuthEngineRole` のみ。

選択肢:

| 案 | pros | cons | 採否 |
|---|---|---|---|
| 1. mount + policy + auth role を GitOps、key は手動 | 単純、追加 operator 不要 | key 作成が手作業 (ただし 1 度きり) | ✅ **採用** |
| 2. 別 operator (hashicorp/vault-secrets-operator 等) を導入 | key も declarative | operator 増、Transit のためだけは過剰 | ✕ |
| 3. VCO を fork して TransitSecretEngine CR を追加 | declarative + 純正 VCO 1 本 | fork 維持コスト | ✕ |
| 4. CronJob で `vault write -f transit/keys/<name>` を冪等実行 | GitOps 寄り | "delete されないこと" だけ保証、操作は依然非 declarative | ✕ |

→ key 作成は 1 度きりかつ手動 rotate 運用なので、案 1 でコスト最小。VCO 全体のカバレッジと例外整理は [`docs/secret-management/index.md`](../../../docs/secret-management/index.md) の「VCO カバレッジと例外」を参照。

## Key Rotation 運用

```bash
# 旧 ciphertext は復号可、新規 encrypt は最新 version
kubectl exec -n vault vault-0 -c vault -- vault write -f transit/keys/<keyName>/rotate

# ※ 全行 rewrap したい場合: app 側 (Repository 層) で transit/rewrap/<keyName> を呼ぶ
```

## Audit

Vault audit device は bootstrap TF で enable 済み (`/vault/audit/audit.log`)。
`transit/encrypt/<keyName>` / `transit/decrypt/<keyName>` が per-call ログされる。

## 関連

- 全体方針: [docs/secret-management/index.md](../../../docs/secret-management/index.md)
- 同パターンの先行例: [vault-database-engine/README.md](../vault-database-engine/README.md)
- 実装サンプル (Go shared/vault): `apps/kensan/backend/shared/vault/`
