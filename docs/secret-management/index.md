# Secret Management

このプラットフォームでは 3 つの方式で Secret を管理する。用途に応じて使い分ける。

## 方式と使い分け

| 方式 | 用途 | ローテ | Source of Truth |
|------|------|--------|-----------------|
| **Vault dynamic (VDBE)** | Postgres app 接続 cred | TTL 24h / 12h rotation | Vault Database engine が動的生成 |
| **Vault static (ESO)** | DB root cred / API key / 管理者 pw / 各種トークン | Vault に新値を書けば最大 1h で配布 | Vault KV v2 |
| **SealedSecret** | Vault 自身の起動依存 / 内部 PKI / image pull token | 手動 (低頻度) | Git repo |

### 方針
- 漏洩時の blast radius を最小化したい **Postgres app 接続 cred** → Vault dynamic
- ローテが必要だが頻度低く、Vault が前提として動いていれば取得できる secret → Vault static (ESO)
- Vault 自身 / cert-manager / Cilium PKI など **Vault に依存できない低レイヤ** → SealedSecret

## Inventory

### Vault dynamic (Postgres app cred)

| ns | Secret | アプリ | 状態 |
|---|---|---|---|
| kensan-data | postgres-kensan-dagster-cred | dagster (3 deployment) | ✅ 使用中 |
| platform-auth-prod | keycloak-postgres-cred | keycloak prod | ✅ 使用中 |
| platform-auth-dev | keycloak-postgres-cred | keycloak dev | ✅ 使用中 |
| kensan-data | postgres-kensan-app-cred | (なし) | ⏸ 生成のみ。kensan-app は cross-ns 課題で revert (下記 *据置* 参照) |
| backstage | postgres-backstage-cred | (なし) | ⏸ 生成のみ。backstage は plugin DB GRANT 課題で revert (下記 *据置* 参照) |

### Vault static (ESO 経由、Vault KV v2 → K8s Secret、refreshInterval 1h)

| ns | Secret | 中身 | 役割 |
|---|---|---|---|
| backstage | backstage-secret | GITHUB_TOKEN | Backstage GitHub 連携 |
| backstage | postgresql-secret | POSTGRES_USER/PASSWORD/DB | Postgres StatefulSet boot + VCO root cred + backstage app (revert で参照中) |
| cloudflare-tunnel | cloudflare-tunnel | tunnel token | Cloudflare Tunnel |
| kensan-data | kensan-db-credentials | POSTGRES_USER/PASSWORD | kensan Postgres boot + VCO root |
| kensan-data | kensan-lakehouse-credentials | AWS_*, POLARIS_BOOTSTRAP_CREDENTIALS | S3 / Polaris (DAGSTER_PG_* は dynamic 移行で削除済) |
| kensan-data | kensan-minio-credentials | MinIO root | MinIO StatefulSet |
| kensan-{prod,dev} | kensan-app-credentials | JWT_SECRET + DB_USER/PASSWORD (revert) | kensan app (cross-ns 課題で dynamic 据置) |
| kensan-{prod,dev} | kensan-ai-credentials | API key 等 | kensan-ai |
| kensan-{prod,dev} | kensan-minio-app-credentials | MinIO app cred | kensan app から MinIO アクセス |
| monitoring | alertmanager-slack | Slack webhook | Alertmanager 通知 |
| monitoring | grafana-admin | Grafana admin | Grafana UI |
| platform-auth-{prod,dev} | keycloak-secret | KEYCLOAK_ADMIN_PASSWORD | Keycloak admin (KC_DB_* は dynamic 移行で削除済) |
| platform-auth-{prod,dev} | postgresql-secret | POSTGRES_USER/PASSWORD/DB | Keycloak Postgres boot + VCO root |

### SealedSecret (Vault に依存しない静的、ローテ頻度低)

| ns | Secret | 役割 | なぜ Vault じゃない |
|---|---|---|---|
| vault | vault-aws-kms-credentials | Vault auto-unseal KMS cred | Vault 自身の起動に必要 (循環依存回避) |
| cert-manager | route53-credentials | Let's Encrypt DNS-01 用 IAM | cert-manager は低レイヤ、ローテ低頻度 |
| kube-system | cilium-ca / hubble-relay-client-certs / hubble-server-certs | Cilium / Hubble 内部 PKI | 内部発行、ローテ低頻度 |
| longhorn-system | longhorn-r2-backup | R2 backup token | Longhorn 自己完結 |
| {app-dev, app-prod, backstage, kensan-{prod,dev,data}} | ghcr-pull-secret | GHCR image pull token | 静的、各 ns に配布のみ |

### 据置 (dynamic 化が望ましいが構造的事情で未移行)

| アプリ | 課題 | 後日方針 |
|--------|------|----------|
| kensan-app | Pod は `kensan-{prod,dev}` ns、Postgres は `kensan-data` ns。K8s Secret の cross-ns 参照不可 | app 側で ns 構造を統合してから再 cutover |
| backstage | Backstage は内部で 12 DB (`backstage` + `backstage_plugin_*` x11) に分散接続。VDBE の `creationStatements` は `dBName` のみ GRANT するため plugin DB 11 個で `permission denied` | Bitnami `initdbScripts` で plugin DB pre-create + group role 戦略 (dynamic role を `backstage` group の member に) で再 cutover |

## Operations

### SealedSecret 作成

```bash
# 1. raw secret を作成 (dry-run)
kubectl create secret generic <name> --namespace <ns> \
  --from-literal=<key>=<value> --dry-run=client -o yaml > temp/<name>-raw.yaml

# 2. kubeseal で暗号化
kubeseal --format yaml < temp/<name>-raw.yaml > infrastructure/<cat>/<comp>/resources/<name>-sealed.yaml

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
cat <<EOF > infrastructure/<cat>/<comp>/resources/<name>-external-secret.yaml
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

1. `infrastructure/security/vault-database-engine/platform-values/vault-database/<name>.yaml` を作成 (`ns`, `rootOwner`, `keyMapping` 等を override)
2. ApplicationSet が拾って `DatabaseSecretEngineRole` (Vault) + `ExternalSecret` (K8s) を自動生成
3. 詳細: [`infrastructure/security/vault-database-engine/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/infrastructure/security/vault-database-engine/README.md)

## 関連ドキュメント

- Sealed Secrets controller: [`infrastructure/security/sealed-secrets/`](https://github.com/yu-min3/kensan-lab/tree/main/infrastructure/security/sealed-secrets)
- External Secrets Operator: [`infrastructure/security/external-secrets/`](https://github.com/yu-min3/kensan-lab/tree/main/infrastructure/security/external-secrets)
- Vault: [`infrastructure/security/vault/`](https://github.com/yu-min3/kensan-lab/tree/main/infrastructure/security/vault)
- Vault Config Operator: [`infrastructure/security/vault-config-operator/`](https://github.com/yu-min3/kensan-lab/tree/main/infrastructure/security/vault-config-operator)
- Vault Database engine + ESO 統合: [`infrastructure/security/vault-database-engine/`](https://github.com/yu-min3/kensan-lab/tree/main/infrastructure/security/vault-database-engine)
