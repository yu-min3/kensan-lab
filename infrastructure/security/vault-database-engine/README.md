## Vault Database Secret Engine

Postgres instance に対する **動的 user (TTL 付き短命 cred)** 払い出し基盤。
1 instance ぶんで **Vault 側 (config + role + auth role) と app 側 (SA + ExternalSecret + VaultDynamicSecret) の 6 リソース** を 1 chart で render する。
app Pod は K8s Secret 経由で短命 cred を読み、ESO が TTL 切れ前に refresh して Secret を更新する。

## 構成

```
infrastructure/security/vault-database-engine/
├── chart/                                # Helm chart (PE 管理、不動)
│   ├── Chart.yaml
│   ├── values.yaml                       # PE 規約のデフォルト値 (TTL / Vault path 規約 / ESO provider 等)
│   └── templates/
│       ├── _helpers.tpl                  # smart default + override の derive logic
│       ├── connection.yaml               # DatabaseSecretEngineConfig    (Vault-side, vault ns)
│       ├── role.yaml                     # DatabaseSecretEngineRole      (Vault-side, vault ns)
│       ├── vault-auth-role.yaml          # KubernetesAuthEngineRole      (Vault-side, vault ns)
│       ├── eso-sa.yaml                   # ServiceAccount vault-db-<basename>  (app ns)
│       ├── eso-vault-dynamic-secret.yaml # VaultDynamicSecret (generator)      (app ns)
│       └── eso-external-secret.yaml      # ExternalSecret                       (app ns)
├── shared/                               # capability bootstrap (1 度だけ)
│   ├── mount.yaml                        # SecretEngineMount (database/)
│   └── policy-eso-read.yaml              # ESO 用 Vault policy (database/creds/* read)
└── platform-values/
    └── vault-database/                   # capability convention dir
        ├── backstage.yaml                # 1 instance、AD が触る (override 必要分のみ書く)
        ├── kensan-app.yaml
        ├── kensan-dagster.yaml
        ├── keycloak-prod.yaml
        └── keycloak-dev.yaml
```

ArgoCD 側:
- `applications/security/vault-database-engine/app-shared.yaml` — single Application、`shared/` を sync (mount + policy 1 度だけ)
- `applications/security/vault-database-engine/applicationset-instances.yaml` — ApplicationSet、values file を recursive glob (`**/platform-values/vault-database/*.yaml`) で discover、per-instance ArgoCD app を auto 生成

## 設計の核: smart default + override

PE が chart の `values.yaml` で **convention based デフォルト** を埋め、AD は **必須項目のみ** 書く。
追加要素が convention と異なる場合だけ override する。

### AD が書く項目 (デフォルトと違う場合)

| キー | 必須? | デフォルト | 補足 |
|---|---|---|---|
| `ns` | ✅ 必須 | なし | deploy 先 K8s namespace。`host` を直接指定しない場合は必須 |
| `rootOwner` | 任意 | filename basename | Bitnami `auth.username` と一致する想定。違う場合のみ override |
| `dbName` | 任意 | `rootOwner` 流用 | Bitnami `auth.database` (default = auth.username) と整合 |
| `host` | 任意 | `<releaseName>.<ns>.svc.cluster.local` | FQDN を直接指定したい場合 |
| `releaseName` | 任意 | `postgresql` | Bitnami release 名が標準と異なる場合 |
| `name` | 任意 | `postgres-<filename-basename>` (AppSet 自動 inject) | 通常書かない |
| `ttl` / `maxTtl` | 任意 | `24h` / `72h` | 短命化したい instance のみ |
| `targetSecretName` | 任意 | `<name>-cred` | app Pod 既存 env が読んでる Secret 名と揃えたい時のみ |
| `keyMapping.user` / `.password` | 任意 | `POSTGRES_USER` / `POSTGRES_PASSWORD` | app Pod env が別 key 名を期待する場合 |
| `esoRefreshInterval` | 任意 | `12h` | TTL 24h の半分。短くしたい instance のみ |

### PE 側の convention (chart 内に閉じる)

| 項目 | 値 |
|---|---|
| Vault role 命名 | `postgres-<filename-basename>` (filename → AppSet が inject) |
| Vault KV admin path | `secret/data/db-admin/<name>` (完全自動、AD には見えない) |
| KV キー名 | `username` / `password` (固定) |
| Postgres host pattern | `<releaseName>.<ns>.svc.cluster.local` |
| ESO Vault provider | `external-secrets` SA (operator central) で kubernetes auth |
| 生成 Secret default 名 | `<name>-cred` (e.g., `postgres-backstage-cred`) |
| Secret default key 名 | `POSTGRES_USER` / `POSTGRES_PASSWORD` (Bitnami 標準) |

## 1 instance 追加方法 (AD 視点)

values file を 1 個書くだけ。最小構成:

```yaml
# 配置例: <owner-dir>/platform-values/vault-database/<instance>.yaml
ns: my-app
```

これだけで以下が自動的に成立:
- Vault role 名 = `postgres-<instance>` (filename から)
- Postgres host = `postgresql.my-app.svc.cluster.local`
- DB 名 / owner = `<instance>` (filename basename)
- Vault KV admin path = `secret/data/db-admin/postgres-<instance>` (PE convention)
- `my-app` ns に K8s Secret `postgres-<instance>-cred` (key: `POSTGRES_USER` / `POSTGRES_PASSWORD`) が生成される
- ESO が 12h ごとに動的 user を更新

異常系を override する場合のみ追加で書く:
```yaml
ns: my-app
releaseName: my-postgres-release  # Bitnami 以外
rootOwner: my_app_db_user         # auth.username が default と異なる
dbName: my_app_db                 # auth.database が異なる
host: my-postgres.example.com     # FQDN 直接指定
keyMapping:                       # app Pod env が別 key 名を期待する場合
  user: MY_APP_DB_USER
  password: MY_APP_DB_PASSWORD
targetSecretName: my-app-postgres-cred  # 旧 static Secret 名と揃えたい場合
```

### app Pod の Secret 切り替え (Phase 5c で別 PR)

新しい Secret (`<name>-cred`) と旧 static Secret は **別物**。app Pod の `envFrom.secretRef.name` (もしくは Pod env の各 key 参照) を新 Secret 名に向ける PR が必要。
key 名は default で Bitnami 標準 (`POSTGRES_USER` / `POSTGRES_PASSWORD`) なので、Pod 側 env 名はそのままで OK。ただし dagster のように既存 env 名が `DAGSTER_PG_USER` / `DAGSTER_PG_PASSWORD` の場合は values で `keyMapping` を override しておく (この場合も Pod 側コードは不変)。

## relocate (per-app ns 移行後)

values file を `git mv` するだけ:
```bash
git mv infrastructure/security/vault-database-engine/platform-values/vault-database/backstage.yaml \
       backstage/platform-values/vault-database/postgresql.yaml
```
AppSet の `**/platform-values/vault-database/*.yaml` glob が auto 追従。

## 設計メモ

### root user は既存 Bitnami app user を流用

Bitnami PostgreSQL chart `auth.enablePostgresUser: false` (default) では `POSTGRES_USER` (= app user) に Superuser + CREATEROLE 等の全特権が付く。`postgres` role 自体は存在しない。
追加セットアップ不要で既存 user をそのまま root として使える。

将来 app 側を動的 user に切替終わった段階で、これら app user の SUPERUSER 権限剥奪 (= app は短命の制限付き user で接続、人間 admin のみ super) に再設計する余地あり。

### Vault KV admin cred の convention path

新 convention path: `secret/data/db-admin/<name>` (keys: `username` / `password`)。
本 PR の merge 前に、旧 path (Stage 3/3.5 で投入された static admin cred) からこの path に admin cred を複製する一度きりの migration を実施済み (script は実行後に削除、git 履歴参照)。
旧 path は app の既存 ExternalSecret consumer がまだ読んでいるため残置。Pod env 切り替え (Phase 5c) 完了後に旧 path も削除する。

### ESO consumer-side の auth model (per-instance SA)

VaultDynamicSecret は **namespace-scoped CR** で `serviceAccountRef.namespace` フィールドが無視される (CRD spec: "Ignored if referent is not cluster-scoped")。
そのため ESO operator central SA (`external-secrets/external-secrets`) は app ns から流用できない。

代わりに per-instance で SA + Vault auth role を作る:

| リソース | 場所 | 名前 | 役割 |
|---|---|---|---|
| ServiceAccount | app ns | `vault-db-<basename>` | VaultDynamicSecret の auth |
| KubernetesAuthEngineRole | vault ns | `vault-db-<basename>` | 上記 SA を bind、policy `eso-read` を付与 |

policy 自体は共有 `eso-read` (本 chart `shared/policy-eso-read.yaml`) を流用。`database/creds/*` 全体に read 権があるので、必要なら将来 per-instance policy に絞る余地あり。

これらは chart が自動 render するので、AD values file は変更不要 (1 instance 増やすたびに SA/Vault role が同名規約で生成される)。

### `eso-read` policy の管理場所

bootstrap chain (TF) で必要な policy は admin / vco-admin の 2 つだけ。それ以外 (eso-read 等) は VCO 起動後に CR で作る方針に統一。本 chart の `shared/policy-eso-read.yaml` が SoT。`bootstrap/vault/policies.tf` からは `vault_policy "eso_read"` resource を削除済み。

### 共通 convention: `<owner-dir>/platform-values/<capability>/<instance>.yaml`

本 capability (vault-database) で確立した convention は、将来の他 platform 機能 (vault-pki / monitoring rules / network-policy 等) でも同様に使い回す前提。各 capability ごとに ApplicationSet を 1 個立て、glob `**/platform-values/<capability>/*.yaml` で discover する。

### 別 repo 対応 (将来)

別 repo の app を扱いたくなったら ApplicationSet generator に `scmProvider` を `merge` で追加:
```yaml
generators:
  - merge:
      mergeKeys: [name]
      generators:
        - git: { ... }                                          # 今
        - scmProvider:
            github: { organization: yu-min3 }
            filters:
              - paths: { include: ["platform-values/vault-database/*.yaml"] }
              - repositoryMatch: "^app-.*"
```

## 動作確認方法

```bash
# 1. 全 ArgoCD app が Healthy
kubectl get app -n argocd | grep vault-db

# 2. Vault に mount + 5 connection + 5 role が入っている
kubectl exec -n vault vault-0 -c vault -- vault secrets list  # database/ あり
kubectl exec -n vault vault-0 -c vault -- vault list database/config  # 5 個
kubectl exec -n vault vault-0 -c vault -- vault list database/roles   # 5 個

# 3. 動的 cred 払い出しテスト (Vault 直)
kubectl exec -n vault vault-0 -c vault -- vault read database/creds/postgres-backstage
# → username + password が払い出される
# Postgres 側で \du すると一時 user が見える、TTL 切れで DROP USER される

# 4. ESO 経由で K8s Secret が生成されている (5 ns 分)
kubectl get secret -n backstage          postgres-backstage-cred
kubectl get secret -n kensan-data        postgres-kensan-app-cred
kubectl get secret -n kensan-data        postgres-kensan-dagster-cred
kubectl get secret -n platform-auth-prod postgres-keycloak-prod-cred
kubectl get secret -n platform-auth-dev  postgres-keycloak-dev-cred

# 5. ExternalSecret status (各 app ns で SecretSynced=True)
kubectl get externalsecret -A | grep postgres-

# 6. 中身確認 (user/password が動的 user 名になっている)
kubectl get secret -n backstage postgres-backstage-cred -o jsonpath='{.data.POSTGRES_USER}' | base64 -d
# → v-kubernet-postgres-... のような Vault 動的 user 名
```
