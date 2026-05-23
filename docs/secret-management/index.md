# Secret Management

このプラットフォームでは 4 つの方式で Secret / 暗号鍵を管理する。用途に応じて使い分ける。

## 方式と使い分け

| 方式 | 用途 | ローテ | Source of Truth |
|------|------|--------|-----------------|
| **Vault dynamic (VDBE)** | Postgres app 接続 cred | TTL 24h / 12h rotation | Vault Database engine が動的生成 |
| **Vault static (ESO)** | DB root cred / API key / 管理者 pw / 各種トークン | Vault に新値を書けば最大 1h で配布 | Vault KV v2 |
| **Vault Transit** | DB に保存する PII の暗号鍵 (kensan `users.name`) | 手動 rotate (`vault write -f transit/keys/.../rotate`)、新規 encrypt は最新版で実行 | Vault Transit engine (鍵自体が Vault 内、ciphertext のみ DB) |
| **SealedSecret** | Vault 自身の起動依存 / 内部 PKI / image pull token | 手動 (低頻度) | Git repo |

### 方針
- 漏洩時の blast radius を最小化したい **Postgres app 接続 cred** → Vault dynamic
- ローテが必要だが頻度低く、Vault が前提として動いていれば取得できる secret → Vault static (ESO)
- **DB カラムに保存する PII (氏名など)** → Vault Transit (鍵をアプリ Pod / DB に降ろさない envelope 暗号化、Stage 6)
- Vault 自身 / cert-manager / Cilium PKI など **Vault に依存できない低レイヤ** → SealedSecret

## VCO カバレッジと例外

Vault 側の declarative 管理は **Vault Config Operator (VCO、redhat-cop/vault-config-operator)** が担う。CR を kensan-lab repo に置いて GitOps、controller が reconcile して Vault に sync。
ただし Vault API の全てに CR が用意されているわけではなく、**少数の例外は手動 (setup script) で運用** している。

### VCO で declarative にできる操作

| Vault 操作 | CR | 使用箇所 |
|---|---|---|
| Secret engine mount | `SecretEngineMount` | `vault-database-engine/shared/mount.yaml`、`vault-transit-engine/shared/mount.yaml` |
| Policy (HCL) | `Policy` | `vault-database-engine/shared/policy-eso-read.yaml`、`vault-transit-engine/chart/templates/policy.yaml` |
| K8s auth role (SA bind + policy 付与) | `KubernetesAuthEngineRole` | 両 engine の `chart/templates/vault-auth-role.yaml` |
| OIDC auth role | `JWTOIDCAuthEngineRole` | Keycloak SSO 統合 (Stage 4) |
| Database engine config (root cred + connectionURL) | `DatabaseSecretEngineConfig` | `vault-database-engine/chart/templates/connection.yaml` (root cred は Vault KV から参照 = 完全 declarative) |
| Database engine role (CREATE/REVOKE SQL) | `DatabaseSecretEngineRole` | `vault-database-engine/chart/templates/role.yaml` |
| Identity Group / Alias | `Group` / `GroupAlias` | OIDC ログイン UX (Stage 4-5) |

Stage 2〜5 と Stage 6 の policy/auth role は **ほぼ 100% VCO で完結** している。

### 例外 (setup script で運用)

| Vault 操作 | 状況 | 手当て |
|---|---|---|
| **Transit key 作成 / rotate** | VCO に `TransitSecretEngine` 系 CR が存在しない | `kubernetes/secrets/vault-transit-engine/temp/setup-transit-keys.sh` を新 key 名で 1 度きり実行。rotate は `vault write -f transit/keys/<name>/rotate` を手動 |
| **OIDC config (`auth/oidc/config`)** | VCO の patch が未対応で、provider URL 等の full-write が必要 | `temp/post-bootstrap-oidc-ux.sh` (Stage 4) で full-write |

撤去せずハイブリッド継続を選んだ理由 (撤去判断のトリガーライン含む) は **ADR-012 (起票予定)** で扱う。

## Inventory

### Vault dynamic (Postgres app cred)

| ns | Secret | アプリ | 状態 |
|---|---|---|---|
| kensan | postgres-kensan-dagster-cred | dagster (3 deployment) | ✅ 使用中 |
| platform-auth-prod | keycloak-postgres-cred | keycloak | ✅ 使用中 |
| kensan | postgres-kensan-app-cred | (なし) | ⏸ 生成のみ。kensan-app は static cred で稼働中、dynamic cutover は別タスク |
| backstage | postgres-backstage-cred | (なし) | ⏸ 生成のみ。backstage は plugin DB GRANT 課題で revert (下記 *据置* 参照) |

### Vault static (ESO 経由、Vault KV v2 → K8s Secret、refreshInterval 1h)

| ns | Secret | 中身 | 役割 |
|---|---|---|---|
| backstage | backstage-secret | GITHUB_TOKEN | Backstage GitHub 連携 |
| backstage | postgresql-secret | POSTGRES_USER/PASSWORD/DB | Postgres StatefulSet boot + VCO root cred + backstage app (revert で参照中) |
| cloudflare-tunnel | cloudflare-tunnel | tunnel token | Cloudflare Tunnel |
| kensan | kensan-db-credentials | POSTGRES_USER/PASSWORD | kensan Postgres boot + VCO root |
| kensan | kensan-lakehouse-credentials | AWS_*, POLARIS_BOOTSTRAP_CREDENTIALS | S3 / Polaris (DAGSTER_PG_* は dynamic 移行で削除済) |
| kensan | kensan-minio-credentials | MinIO root | MinIO StatefulSet |
| kensan | kensan-app-credentials | JWT_SECRET + DB_USER/PASSWORD | kensan microservices (dynamic cutover は別タスク) |
| kensan | kensan-ai-credentials | API key 等 | kensan-ai |
| kensan | kensan-minio-app-credentials | MinIO app cred | kensan microservices から MinIO アクセス |
| monitoring | alertmanager-slack | Slack webhook | Alertmanager 通知 |
| monitoring | grafana-admin | Grafana admin | Grafana UI |
| platform-auth-{prod,dev} | keycloak-secret | KEYCLOAK_ADMIN_PASSWORD | Keycloak admin (KC_DB_* は dynamic 移行で削除済) |
| platform-auth-{prod,dev} | postgresql-secret | POSTGRES_USER/PASSWORD/DB | Keycloak Postgres boot + VCO root |
| {app-prod, backstage, kensan} | ghcr-pull-secret | `.dockerconfigjson` (GHCR image pull token) | 各 ns の imagePullSecrets。Vault path `secret/ghcr/pull-token` を全 ns で共有 |

### Vault Transit (Stage 6, アプリ側で encrypt/decrypt API を直叩き)

K8s Secret として配布する形式ではなく、アプリ Pod が Vault に **K8s SA auth で login → Transit API 直叩き** で encrypt / decrypt する。鍵自体は Vault 内に常駐し、Pod / DB のどこにも降りない。

| ns | 鍵名 / mount | 用途 | アプリ |
|---|---|---|---|
| vault | `transit/keys/users-name` (aes256-gcm96) | kensan `users.name` カラム encryption + HMAC | kensan user-service (kensan ns) |

設計詳細: [`kubernetes/secrets/vault-transit-engine/README.md`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-transit-engine)

| 項目 | 値 |
|---|---|
| Auth role | `transit-kensan-users` (kubernetes auth method、vault-database-engine の命名 `postgres-<base>` と対称) |
| bind 先 SA | `kensan/transit-kensan-users` (chart が自動生成、user-service Pod の SA) |
| Policy | `transit-kensan-users` (transit/{encrypt,decrypt,hmac,rewrap}/users-name のみ) |
| ConfigMap | `kensan/transit-kensan-users-config` (chart 生成、`VAULT_ADDR` / `VAULT_AUTH_ROLE` / `VAULT_TRANSIT_KEY`、Reloader match=true) |
| Token TTL | 30 min (アプリ側 renew loop で延長、max 1h) |
| Key 作成 | `kubernetes/secrets/vault-transit-engine/temp/setup-transit-keys.sh` を 1 度だけ手動実行 |
| Rotation | `vault write -f transit/keys/users-name/rotate` → アプリ側で `transit/rewrap` 経由で旧 ciphertext を更新 |

Vault 側 (mount/policy/auth role) と consumer ns 側 (SA/ConfigMap) は chart 化済 (`vault-transit-engine/chart/` + `platform-values/vault-transit/<consumer>.yaml`)。consumer Deployment は `serviceAccountName` + `envFrom: configMapRef` で参照、Reloader で rotation 連動。key 作成だけ手動 (VCO 限界、上の「VCO カバレッジと例外」参照)。

### SealedSecret (Vault に依存しない静的、ローテ頻度低)

| ns | Secret | 役割 | なぜ Vault じゃない |
|---|---|---|---|
| vault | vault-aws-kms-credentials | Vault auto-unseal KMS cred | Vault 自身の起動に必要 (循環依存回避) |
| cert-manager | route53-credentials | Let's Encrypt DNS-01 用 IAM | cert-manager は低レイヤ、ローテ低頻度 |
| kube-system | cilium-ca / hubble-relay-client-certs / hubble-server-certs | Cilium / Hubble 内部 PKI | 内部発行、ローテ低頻度 |
| longhorn-system | longhorn-r2-backup | R2 backup token | Longhorn 自己完結 |

### 据置 (dynamic 化が望ましいが構造的事情で未移行)

| アプリ | 課題 | 後日方針 |
|--------|------|----------|
| kensan-app | Phase 3a-3 で ns 統合済み (cross-ns 課題は解消)。dynamic 化の cutover はまだ実施していない | static (kensan-app-credentials) を ESO 経由で参照中。kensan-app 系の env を `postgres-kensan-app-cred` 参照に切り替えれば cutover 可能 |
| backstage | Backstage は内部で 12 DB (`backstage` + `backstage_plugin_*` x11) に分散接続。VDBE の `creationStatements` は `dBName` のみ GRANT するため plugin DB 11 個で `permission denied` | Bitnami `initdbScripts` で plugin DB pre-create + group role 戦略 (dynamic role を `backstage` group の member に) で再 cutover |

## Operations

### SealedSecret 作成

```bash
# 1. raw secret を作成 (dry-run)
kubectl create secret generic <name> --namespace <ns> \
  --from-literal=<key>=<value> --dry-run=client -o yaml > temp/<name>-raw.yaml

# 2. kubeseal で暗号化
kubeseal --format yaml < temp/<name>-raw.yaml > kubernetes/<cat>/<comp>/resources/<name>-sealed.yaml

# 3. raw を破棄
rm temp/<name>-raw.yaml

# 4. commit + push → Sealed Secrets controller が cluster 内で復号 → Secret 作成
```

`temp/*-raw.yaml` は `.gitignore` 済 (絶対 commit しない)。

### Vault static (ESO) 追加

```bash
# 1. Vault KV v2 に書く
vault kv put secret/<path> KEY1=value1 KEY2=value2

# 2. ExternalSecret manifest を作成 (例)
cat <<EOF > kubernetes/<cat>/<comp>/resources/<name>-external-secret.yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: <name>
  namespace: <ns>
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: <k8s-secret-name>
    creationPolicy: Owner
  data:
    - secretKey: KEY1
      remoteRef: { key: <path>, property: KEY1 }
EOF

# 3. commit + push → ESO が refresh interval 内に同期
```

### Vault dynamic (VDBE) 追加

1. `kubernetes/secrets/vault-database-engine/platform-values/vault-database/<name>.yaml` を作成 (`ns`, `rootOwner`, `keyMapping` 等を override)
2. ApplicationSet が拾って `DatabaseSecretEngineRole` (Vault) + `ExternalSecret` (K8s) を自動生成
3. 詳細: [`kubernetes/secrets/vault-database-engine/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/secrets/vault-database-engine/README.md)

## 関連ドキュメント

- Sealed Secrets controller: [`kubernetes/secrets/sealed-secrets/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/sealed-secrets)
- External Secrets Operator: [`kubernetes/secrets/external-secrets/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/external-secrets)
- Vault: [`kubernetes/secrets/vault/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault)
- Vault Config Operator: [`kubernetes/secrets/vault-config-operator/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-config-operator)
- Vault Database engine + ESO 統合: [`kubernetes/secrets/vault-database-engine/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-database-engine)
- Vault Transit engine (Stage 6): [`kubernetes/secrets/vault-transit-engine/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-transit-engine)
