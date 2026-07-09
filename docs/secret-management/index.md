# Secret Management

The platform manages secrets and encryption keys through four methods, chosen per use case.

## Methods and when to use each

| Method | Use for | Rotation | Source of Truth |
|------|------|--------|-----------------|
| **Vault dynamic (VDBE)** | Postgres app connection creds where a restart / credential reload is acceptable | TTL 24h / 12h rotation | Generated on demand by the Vault Database engine |
| **Vault static (ESO)** | DB root creds / API keys / admin passwords / misc tokens | Write a new value to Vault → delivered within 1h | Vault KV v2 |
| **Vault Transit** | Encryption keys for PII stored in DB columns (kensan `users.name`) | Manual rotate (`vault write -f transit/keys/.../rotate`); new encryptions use the latest key version | Vault Transit engine (the key itself stays inside Vault; only ciphertext reaches the DB) |
| **SealedSecret** | Vault's own startup dependencies / internal PKI / image pull tokens | Manual (low frequency) | Git repo |

### Policy

- **Postgres app connection creds**, where we want the smallest possible blast radius on leak → Vault dynamic. Because delivery relies on `env` injection + a Reloader restart, it is *not* used for long-lived services that own an IdP, a catalog metastore, or a schema (see the "kept static" list below)
- Secrets that need rotation but change rarely, and can assume Vault is up → Vault static (ESO)
- **PII stored in DB columns** (names etc.) → Vault Transit (envelope encryption that never hands the key to the app pod or the DB — Stage 6)
- **Low layers that cannot depend on Vault** — Vault itself, cert-manager, Cilium PKI → SealedSecret

## VCO coverage and exceptions

Declarative management on the Vault side is handled by the **Vault Config Operator (VCO, redhat-cop/vault-config-operator)**: CRs live in the kensan-lab repo (GitOps), and the controller reconciles them into Vault.
Not every Vault API has a corresponding CR, however — **a small number of exceptions are operated by setup scripts**.

### Operations VCO makes declarative

| Vault operation | CR | Used in |
|---|---|---|
| Secret engine mount | `SecretEngineMount` | `vault-database-engine/shared/mount.yaml`, `vault-transit-engine/shared/mount.yaml` |
| Policy (HCL) | `Policy` | `vault-database-engine/shared/policy-eso-read.yaml`, `vault-transit-engine/chart/templates/policy.yaml` |
| K8s auth role (SA binding + policy grant) | `KubernetesAuthEngineRole` | `chart/templates/vault-auth-role.yaml` in both engines |
| OIDC auth role | `JWTOIDCAuthEngineRole` | Keycloak SSO integration (Stage 4) |
| Database engine config (root cred + connectionURL) | `DatabaseSecretEngineConfig` | `vault-database-engine/chart/templates/connection.yaml` (root cred is referenced from Vault KV = fully declarative) |
| Database engine role (CREATE/REVOKE SQL) | `DatabaseSecretEngineRole` | `vault-database-engine/chart/templates/role.yaml` |
| Identity Group / Alias | `Group` / `GroupAlias` | OIDC login UX (Stage 4–5) |

The policies and auth roles of Stages 2–5 and Stage 6 are **almost 100% covered by VCO**.

### Exceptions (operated via setup scripts)

| Vault operation | Situation | Handling |
|---|---|---|
| **Transit key creation / rotation** | VCO has no `TransitSecretEngine`-style CR | Run `kubernetes/secrets/vault-transit-engine/bootstrap/setup-transit-keys.sh` once per new key name; rotate manually with `vault write -f transit/keys/<name>/rotate` |
| **OIDC config (`auth/oidc/config`)** | VCO's patch support can't do the full-write needed for provider URL etc. | Full-write via `bootstrap/vault/post-bootstrap-oidc-ux.sh` (Stage 4) |

The rationale for keeping this hybrid rather than eliminating the scripts (including the trigger line for retiring them) is covered in [ADR-015](../adr/015-vco-setup-script-hybrid.md).

## Inventory

### Vault dynamic (Postgres app creds)

| ns | Secret | App | Status |
|---|---|---|---|
| kensan | postgres-kensan-dagster-cred | dagster (3 deployments) | ✅ in use |

### Vault static (via ESO — Vault KV v2 → K8s Secret, refreshInterval 1h)

| ns | Secret | Contents | Role |
|---|---|---|---|
| argocd | argocd-oidc-secret | client-id / client-secret | Argo CD's Keycloak OIDC client cred (`security/argocd/oidc`) |
| backstage | backstage-secret | GITHUB_TOKEN | Backstage GitHub integration |
| backstage | postgresql-secret | POSTGRES_USER/PASSWORD/DB | Postgres StatefulSet boot + VCO root cred + backstage app |
| cloudflare-tunnel | cloudflare-tunnel | tunnel token | Cloudflare Tunnel |
| kensan | kensan-db-credentials | POSTGRES_USER/PASSWORD | kensan Postgres boot + VCO root |
| kensan | kensan-lakehouse-credentials | AWS_*, POLARIS_BOOTSTRAP_CREDENTIALS, POLARIS_PG_* | S3 / Polaris |
| kensan | kensan-minio-credentials | MinIO root | MinIO StatefulSet |
| kensan | kensan-app-credentials | JWT_SECRET + DB_USER/PASSWORD | kensan microservices |
| kensan | kensan-ai-credentials | API keys etc. | kensan-ai |
| kensan | kensan-minio-app-credentials | MinIO app cred | MinIO access from kensan microservices |
| monitoring | alertmanager-slack | Slack webhook | Alertmanager notifications |
| monitoring | grafana-admin | Grafana admin | Grafana UI |
| monitoring | grafana-oidc-secret | GF_AUTH_GENERIC_OAUTH_CLIENT_ID/SECRET | Grafana's Keycloak OIDC client cred (`observability/grafana/oidc`) |
| monitoring | grafana-cloud-remote-write | username / password | Prometheus → Grafana Cloud remote-write auth (`monitoring/grafana-cloud/remote-write`) |
| platform-auth-prod | keycloak-secret | KEYCLOAK_ADMIN_PASSWORD / KC_DB_PASSWORD | Keycloak admin + DB password (`KC_DB_PASSWORD` is synced from `platform-auth/prod/postgresql.POSTGRES_PASSWORD`) |
| platform-auth-{prod,dev} | postgresql-secret | POSTGRES_USER/PASSWORD/DB | Keycloak Postgres boot + VCO root |
| {app-prod, backstage, kensan, app-kensan} | ghcr-pull-secret | `.dockerconfigjson` (GHCR image pull token) | `imagePullSecrets` in each ns; Vault path `secret/ghcr/pull-token` is shared across all of them (app-kensan receives it via the app-base chart's `externalsecret-ghcr.yaml`) |

### Vault Transit (Stage 6 — apps call the encrypt/decrypt API directly)

Not delivered as a K8s Secret. The app pod **logs into Vault with K8s SA auth and calls the Transit API directly** for encrypt / decrypt. The key itself stays resident inside Vault and never lands in a pod or a DB.

| ns | Key / mount | Purpose | App |
|---|---|---|---|
| vault | `transit/keys/users-name` (aes256-gcm96) | kensan `users.name` column encryption + HMAC | kensan user-service (kensan ns) |

Design details: [`kubernetes/secrets/vault-transit-engine/README.md`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-transit-engine)

| Item | Value |
|---|---|
| Auth role | `transit-kensan-users` (kubernetes auth method; named symmetrically with vault-database-engine's `postgres-<base>`) |
| Bound SA | `kensan/transit-kensan-users` (auto-generated by the chart; the user-service pod's SA) |
| Policy | `transit-kensan-users` (only transit/{encrypt,decrypt,hmac,rewrap}/users-name) |
| ConfigMap | `kensan/transit-kensan-users-config` (chart-generated; `VAULT_ADDR` / `VAULT_AUTH_ROLE` / `VAULT_TRANSIT_KEY`, Reloader match=true) |
| Token TTL | 30 min (renewed by the app's renew loop, max 1h) |
| Key creation | Run `kubernetes/secrets/vault-transit-engine/bootstrap/setup-transit-keys.sh` once, manually |
| Rotation | `vault write -f transit/keys/users-name/rotate` → the app refreshes old ciphertext via `transit/rewrap` |

The Vault side (mount/policy/auth role) and the consumer-ns side (SA/ConfigMap) are chart-managed (`vault-transit-engine/chart/` + `platform-values/vault-transit/<consumer>.yaml`). Consumer Deployments reference them via `serviceAccountName` + `envFrom: configMapRef`, with Reloader wiring rotation through. Only key creation is manual (a VCO limitation — see "VCO coverage and exceptions" above).

### SealedSecret (static, independent of Vault, low rotation frequency)

| ns | Secret | Role | Why not Vault |
|---|---|---|---|
| vault | vault-aws-kms-credentials | Vault auto-unseal KMS cred | Needed for Vault's own startup (avoids the circular dependency) |
| cert-manager | route53-credentials | IAM cred for Let's Encrypt DNS-01 | cert-manager is a low layer; rotation is rare |
| kube-system | cilium-ca / hubble-relay-client-certs / hubble-server-certs | Cilium / Hubble internal PKI | Internally issued; rotation is rare |
| longhorn-system | longhorn-r2-backup | R2 backup token | Longhorn is self-contained |

### Kept static (deliberately not dynamic)

| App | Reason | Policy |
|--------|------|----------|
| keycloak | DB credential rotation restarts the pod, which destroys SSO sessions. The IdP is the core of Vault / SSO and carries a large circular dependency | Pinned to Vault static (ESO) `keycloak-secret` |
| backstage | Long-lived service where a 12h restart has little value; also has plugin DB / schema-owner issues | Pinned to Vault static (ESO) `postgresql-secret` |
| polaris | Catalog metastore that owns its schema; bootstrap fails unless the dynamic user owns the existing tables | Pinned to Vault static (ESO) `kensan-lakehouse-credentials` |
| kensan-app | In-house app, but no runtime credential reload support today; dynamic via Reloader restarts isn't worth it yet | Pinned to Vault static (ESO) `kensan-app-credentials` |

## Operations

### Creating a SealedSecret

```bash
# 1. Create the raw secret (dry-run)
kubectl create secret generic <name> --namespace <ns> \
  --from-literal=<key>=<value> --dry-run=client -o yaml > temp/<name>-raw.yaml

# 2. Encrypt with kubeseal
kubeseal --format yaml < temp/<name>-raw.yaml > kubernetes/<cat>/<comp>/resources/<name>-sealed.yaml

# 3. Discard the raw file
rm temp/<name>-raw.yaml

# 4. commit + push → the Sealed Secrets controller decrypts in-cluster → Secret created
```

`temp/*-raw.yaml` is gitignored (never commit it).

### Adding a Vault static (ESO) secret

```bash
# 1. Write to Vault KV v2
vault kv put secret/<path> KEY1=value1 KEY2=value2

# 2. Create the ExternalSecret manifest (example)
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

# 3. commit + push → ESO syncs within the refresh interval
```

### Adding a Vault dynamic (VDBE) credential

1. Create `kubernetes/secrets/vault-database-engine/platform-values/vault-database/<name>.yaml` (override `ns`, `rootOwner`, `keyMapping`, etc.)
2. The ApplicationSet picks it up and auto-generates the `DatabaseSecretEngineRole` (Vault) + `ExternalSecret` (K8s)
3. Details: [`kubernetes/secrets/vault-database-engine/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/secrets/vault-database-engine/README.md)

## Related

- Sealed Secrets controller: [`kubernetes/secrets/sealed-secrets/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/sealed-secrets)
- External Secrets Operator: [`kubernetes/secrets/external-secrets/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/external-secrets)
- Vault: [`kubernetes/secrets/vault/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault)
- Vault Config Operator: [`kubernetes/secrets/vault-config-operator/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-config-operator)
- Vault Database engine + ESO integration: [`kubernetes/secrets/vault-database-engine/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-database-engine)
- Vault Transit engine (Stage 6): [`kubernetes/secrets/vault-transit-engine/`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/secrets/vault-transit-engine)
