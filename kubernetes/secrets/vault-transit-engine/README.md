# vault-transit-engine

The Vault Transit secret engine as a cluster-wide capability. It hands out
**application-layer encryption** for things like Postgres columns — envelope
encryption where Vault holds the key and it is never delivered to the pod or the
database — scoped to least privilege per consumer.

## Layout

```
kubernetes/secrets/vault-transit-engine/
├── README.md                                # this file
├── chart/                                   # Helm chart (PE-owned, does not move)
│   ├── Chart.yaml
│   ├── values.yaml                          # PE defaults (TTLs, naming convention, imagePullSecrets)
│   └── templates/
│       ├── _helpers.tpl                     # name / basename / vcoAuth helpers
│       ├── policy.yaml                      # Vault policy, per key (encrypt/decrypt/hmac/rewrap)
│       ├── vault-auth-role.yaml             # Vault KubernetesAuthEngineRole
│       ├── serviceaccount.yaml              # SA in the consumer namespace (the pod's SA is the Vault auth subject)
│       └── configmap.yaml                   # ConfigMap in the consumer namespace (VAULT_ADDR / VAULT_AUTH_ROLE / VAULT_TRANSIT_KEY)
├── shared/                                  # capability bootstrap, applied once
│   └── mount.yaml                           # SecretEngineMount (transit/)
├── platform-values/
│   └── vault-transit/                       # capability convention dir — one file per consumer
│       └── (no consumers at present)         # e.g. kensan-users.yaml, removed with kensan-legacy in #404
└── bootstrap/
    └── setup-transit-keys.sh                # one-off: creates transit/keys/<name> (not covered by VCO)
```

On the Argo CD side:

- `applications/secrets/vault-transit-engine/app-shared.yaml` — a single Application syncing `shared/` (the mount only)
- `applications/secrets/vault-transit-engine/applicationset-instances.yaml` — an ApplicationSet that discovers `**/platform-values/vault-transit/*.yaml` by glob and generates one Argo CD app per consumer

## The core idea: smart defaults plus overrides (the same pattern as vault-database-engine)

PE fills in convention-based defaults in `chart/values.yaml`, and an app
developer writes **only the required fields**. The chart renders **both the Vault
side (policy + KubernetesAuthEngineRole) and the consumer-namespace side (SA +
ConfigMap)**. The consumer deployment picks up the chart's output through
`serviceAccountName` and `envFrom: configMapRef`, with Reloader tying rotation to
a rollout.

### What an app developer writes

| Key | Required? | Default | Notes |
|---|---|---|---|
| `ns` | ✅ required | none | The Kubernetes namespace the consumer pod runs in |
| `keyName` | ✅ required | none | The Transit key name to encrypt and decrypt with |
| `extraKeyNames` | optional | `[]` | For a consumer that needs more than one key |
| `tokenTTL` / `tokenMaxTTL` | optional | `1800` / `3600` | Vault Kubernetes-auth token lease, in seconds |
| `imagePullSecrets` | optional | `[{name: ghcr-pull-secret}]` | Image pull secret attached to the consumer SA |
| `vaultProvider.server` | optional | `http://vault.vault.svc.cluster.local:8200` | The Vault address the consumer pod uses (rendered into the ConfigMap) |
| `name` | optional | injected by the AppSet as `transit-<filename>` | Normally left unset |

### The PE-side conventions (contained in the chart)

| Item | Value |
|---|---|
| Naming of the Vault role, policy, consumer SA, and ConfigMap | All `transit-<filename-basename>`, mirroring vault-database-engine's `postgres-<base>` |
| Token TTL / max TTL | 30 min / 1 h, to avoid the 12 h+ default and force a renew loop |
| Permitted endpoints | `transit/{encrypt,decrypt,hmac,rewrap}/<keyName>`, read on `keys/<keyName>`, and `auth/token/{renew,lookup}-self` |
| ConfigMap key names | `VAULT_ADDR` / `VAULT_AUTH_ROLE` / `VAULT_TRANSIT_KEY`, read by the consumer pod through `envFrom` |

## Adding one consumer

Two steps: (1) write one values file, (2) point the consumer deployment at the SA
and ConfigMap.

### (1) The values file

```yaml
# <owner-dir>/platform-values/vault-transit/<consumer>.yaml
ns: my-app
keyName: my-pii-column
```

That produces:

- A Vault role and policy named `transit-<consumer>`, taken from the filename
- An SA `transit-<consumer>` and a ConfigMap `transit-<consumer>-config` in the `my-app` namespace
- Least-privilege access to `transit/{encrypt,decrypt,hmac,rewrap}/my-pii-column` and nothing else

### (2) The consumer deployment

```yaml
metadata:
  annotations:
    reloader.stakater.com/auto: "true"  # roll out automatically when the chart's ConfigMap or Secret changes
spec:
  template:
    spec:
      serviceAccountName: transit-<consumer>  # the SA the chart creates in the same namespace
      containers:
      - name: app
        envFrom:
        - configMapRef:
            name: transit-<consumer>-config   # VAULT_ADDR / VAULT_AUTH_ROLE / VAULT_TRANSIT_KEY
```

The key itself (`transit/keys/my-pii-column`) is **created by hand, once**. Use
`bootstrap/setup-transit-keys.sh` as the model and run it with the new key name.
Why it is manual is explained below.

## Design decision: why key creation alone is manual

redhat-cop/vault-config-operator **has no Transit secret engine CRs** (as of
2026-05). The CRDs available are `SecretEngineMount`, `Policy`, and
`KubernetesAuthEngineRole`.

| Option | Pros | Cons | Verdict |
|---|---|---|---|
| 1. GitOps the mount, policy, and auth role; create the key by hand | Simple, no extra operator | Key creation is manual — though only once | ✅ **Adopted** |
| 2. Add a second operator (hashicorp/vault-secrets-operator or similar) | The key becomes declarative too | Another operator to run, excessive for Transit alone | ✕ |
| 3. Fork VCO and add a TransitSecretEngine CR | Declarative, and still one operator | The cost of maintaining a fork | ✕ |
| 4. Run `vault write -f transit/keys/<name>` idempotently from a CronJob | Closer to GitOps | Only guarantees "not deleted"; the operation is still not declarative | ✕ |

Key creation happens once and rotation is a manual operation anyway, so option 1
costs the least. VCO's overall coverage and its exceptions are catalogued under
"VCO coverage and exceptions" in
[`docs/secret-management/index.md`](../../../docs/secret-management/index.md).

## Key rotation

```bash
# old ciphertext stays decryptable; new encryptions use the latest version
kubectl exec -n vault vault-0 -c vault -- vault write -f transit/keys/<keyName>/rotate

# to rewrap every row, call transit/rewrap/<keyName> from the app's repository layer
```

## Audit

The Vault audit device is enabled by the bootstrap Terraform
(`/vault/audit/audit.log`). Every `transit/encrypt/<keyName>` and
`transit/decrypt/<keyName>` call is logged.

## Related

- Overall approach: [docs/secret-management/index.md](../../../docs/secret-management/index.md)
- The earlier example of the same pattern: [vault-database-engine/README.md](../vault-database-engine/README.md)
- Reference implementation (Go, shared/vault): `apps/kensan-legacy/backend/shared/vault/` — removed; see the tag `kensan-legacy-final`
