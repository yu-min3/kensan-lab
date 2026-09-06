# Vault Database Secret Engine

The platform for issuing **dynamic users — short-lived credentials with a TTL —**
against a Postgres instance. One chart renders **six resources for a single
instance: the Vault side (config + role + auth role) and the app side (SA +
ExternalSecret + VaultDynamicSecret)**. The application pod reads the short-lived
credential from a Kubernetes Secret, and ESO refreshes that Secret before the TTL
expires.

## Layout

```
kubernetes/secrets/vault-database-engine/
├── chart/                                # Helm chart (PE-owned, does not move)
│   ├── Chart.yaml
│   ├── values.yaml                       # PE defaults (TTLs, Vault path convention, ESO provider, ...)
│   └── templates/
│       ├── _helpers.tpl                  # the smart-default / override derivation logic
│       ├── connection.yaml               # DatabaseSecretEngineConfig    (Vault side, vault ns)
│       ├── role.yaml                     # DatabaseSecretEngineRole      (Vault side, vault ns)
│       ├── vault-auth-role.yaml          # KubernetesAuthEngineRole      (Vault side, vault ns)
│       ├── eso-sa.yaml                   # ServiceAccount vault-db-<basename>  (app ns)
│       ├── eso-vault-dynamic-secret.yaml # VaultDynamicSecret (generator)      (app ns)
│       └── eso-external-secret.yaml      # ExternalSecret                       (app ns)
├── shared/                               # capability bootstrap, applied once
│   ├── mount.yaml                        # SecretEngineMount (database/)
│   ├── policy-eso-read.yaml              # Vault policy for ESO (read on database/creds/*)
│   ├── ccnp-postgres-ingress.yaml        # CCNP: managed-ns Postgres ← vault, TCP/5432 (cluster-wide)
│   └── cnp-vault-egress.yaml             # CNP: vault → managed-ns Postgres, TCP/5432 (vault ns)
└── platform-values/
    └── vault-database/                   # capability convention dir — one file per instance
        └── (no instances at present)      # e.g. kensan-dagster.yaml, removed with kensan-legacy in #403; keycloak reverted in ADR-019
```

On the Argo CD side:

- `applications/secrets/vault-database-engine/app-shared.yaml` — a single Application syncing `shared/` (the mount and policy, once)
- `applications/secrets/vault-database-engine/applicationset-instances.yaml` — an ApplicationSet that discovers values files by recursive glob (`**/platform-values/vault-database/*.yaml`) and generates one Argo CD app per instance

## Where this applies

This chart syncs the dynamic credential into a Kubernetes Secret, which the
consumer pod reads through `env` or `secretKeyRef`. Getting an updated Secret
into a running process needs either a rolling restart via Reloader or an explicit
credential reload in the application.

For that reason it is not used for long-lived services such as Keycloak,
Backstage, or Polaris, for services that own a schema, or for any service where a
restart directly costs user sessions or catalog availability. The only current
target is the Dagster system database.

## The core idea: smart defaults plus overrides

PE fills in **convention-based defaults** in the chart's `values.yaml`, and an app
developer writes **only the required fields**, overriding only where something
differs from the convention.

### What an app developer writes (when it differs from the default)

| Key | Required? | Default | Notes |
|---|---|---|---|
| `ns` | ✅ required | none | The Kubernetes namespace to deploy into. Required unless `host` is given directly |
| `rootOwner` | optional | the filename basename | Expected to match Bitnami's `auth.username`. Override only when it does not |
| `dbName` | optional | reuses `rootOwner` | Consistent with Bitnami's `auth.database` (which defaults to `auth.username`) |
| `host` | optional | `<releaseName>.<ns>.svc.cluster.local` | To give an FQDN directly |
| `releaseName` | optional | `postgresql` | When the Bitnami release name is non-standard |
| `name` | optional | `postgres-<filename-basename>`, injected by the AppSet | Normally left unset |
| `ttl` / `maxTtl` | optional | `24h` / `72h` | Only for instances that need a shorter life |
| `targetSecretName` | optional | `<name>-cred` | Only to match a Secret name the app pod's existing env already reads |
| `keyMapping.user` / `.password` | optional | `POSTGRES_USER` / `POSTGRES_PASSWORD` | When the app pod's env expects different key names |
| `esoRefreshInterval` | optional | `12h` | Half of the 24 h TTL. Only for instances that need it shorter |

### The PE-side conventions (contained in the chart)

| Item | Value |
|---|---|
| Vault role naming | `postgres-<filename-basename>`, injected by the AppSet from the filename |
| Vault KV admin path | `secret/data/db-admin/<name>` — fully automatic, invisible to the app developer |
| KV key names | `username` / `password`, fixed |
| Postgres host pattern | `<releaseName>.<ns>.svc.cluster.local` |
| ESO Vault provider | Kubernetes auth as the `external-secrets` SA (the operator's central SA) |
| Default generated Secret name | `<name>-cred` (e.g. `postgres-backstage-cred`) |
| Default Secret key names | `POSTGRES_USER` / `POSTGRES_PASSWORD`, the Bitnami standard |

## Adding one instance (from the app developer's side)

Two steps: (1) write one values file, (2) add one label to the target namespace.

```yaml
# (1) <owner-dir>/platform-values/vault-database/<instance>.yaml
ns: my-app
```

```yaml
# (2) the app's namespace.yaml (under the app's own repo once per-app namespaces land)
apiVersion: v1
kind: Namespace
metadata:
  name: my-app
  labels:
    kensan-lab.platform/vault-managed-postgres: "true"  # permits Vault → Postgres on TCP/5432
```

The namespace label is required, because the cluster-wide CCNP and CNP in
`shared/` pick namespaces up on an opt-in basis. Without it VCO cannot reach
Postgres and dynamic user issuance fails.

That alone establishes:

- Vault role name `postgres-<instance>`, taken from the filename
- Postgres host `postgresql.my-app.svc.cluster.local`
- Database name and owner `<instance>` (the filename basename)
- Vault KV admin path `secret/data/db-admin/postgres-<instance>` (the PE convention)
- A Kubernetes Secret `postgres-<instance>-cred` in the `my-app` namespace, with the keys `POSTGRES_USER` and `POSTGRES_PASSWORD`
- ESO refreshing the dynamic user every 12 h

Write more only to override an unusual case:

```yaml
ns: my-app
releaseName: my-postgres-release  # something other than Bitnami
rootOwner: my_app_db_user         # auth.username differs from the default
dbName: my_app_db                 # auth.database differs
host: my-postgres.example.com     # an FQDN given directly
keyMapping:                       # the app pod's env expects different key names
  user: MY_APP_DB_USER
  password: MY_APP_DB_PASSWORD
targetSecretName: my-app-postgres-cred  # to match the old static Secret name
```

### Switching the app pod's Secret (a separate PR, Phase 5c)

The new Secret (`<name>-cred`) and the old static Secret are **different objects**.
A PR is needed to point the app pod's `envFrom.secretRef.name` — or its individual
key references — at the new Secret. The key names default to the Bitnami standard
(`POSTGRES_USER` / `POSTGRES_PASSWORD`), so pod-side env names usually stay as
they are. Where they do not — Dagster, for instance, expects `DAGSTER_PG_USER` and
`DAGSTER_PG_PASSWORD` — override `keyMapping` in the values file; the pod's code
still does not change.

## Relocating (after the move to per-app namespaces)

Just `git mv` the values file:

```bash
git mv kubernetes/secrets/vault-database-engine/platform-values/vault-database/<name>.yaml \
       apps/<app>/platform-values/vault-database/<name>.yaml
```

The AppSet's `**/platform-values/vault-database/*.yaml` glob follows automatically.

## Design notes

### The root user reuses the existing Bitnami app user

With the Bitnami PostgreSQL chart's default `auth.enablePostgresUser: false`, the
`POSTGRES_USER` (the app user) receives every privilege including Superuser and
CREATEROLE, and no `postgres` role exists at all. That user can therefore serve as
the root user with no extra setup.

Once applications have finished moving to dynamic users, there is room to redesign
this — stripping SUPERUSER from those app users, so applications connect as
short-lived restricted users and only human administrators are superusers.

### The convention path for the Vault KV admin credential

The convention path is `secret/data/db-admin/<name>`, with the keys `username`
and `password`. Before this work merged, a one-off migration copied the admin
credential from the old path — the static admin credential loaded in Stage 3/3.5 —
into it; the script was deleted after running and is in the git history. The old
path is left in place because existing ExternalSecret consumers still read it, and
is removed once the pod env switch (Phase 5c) is complete.

### The consumer-side ESO auth model (a per-instance SA)

`VaultDynamicSecret` is a **namespace-scoped CR**, and its
`serviceAccountRef.namespace` field is ignored — the CRD spec says "Ignored if
referent is not cluster-scoped". The operator's central SA
(`external-secrets/external-secrets`) therefore cannot be borrowed from the app
namespace.

Instead, an SA and a Vault auth role are created per instance:

| Resource | Where | Name | Role |
|---|---|---|---|
| ServiceAccount | app ns | `vault-db-<basename>` | Authentication subject for VaultDynamicSecret |
| KubernetesAuthEngineRole | vault ns | `vault-db-<basename>` | Binds that SA and grants the `eso-read` policy |

The policy itself is the shared `eso-read` (`shared/policy-eso-read.yaml` in this
chart). It grants read across all of `database/creds/*`, so there is room to narrow
it to a per-instance policy later if needed.

The chart renders all of this, so the app developer's values file does not change:
adding an instance generates the SA and Vault role under the same naming
convention.

### Where the `eso-read` policy is managed

The bootstrap chain (Terraform) needs only two policies, admin and vco-admin.
Everything else — `eso-read` among them — is created as a CR once VCO is running.
`shared/policy-eso-read.yaml` in this chart is the source of truth; the
`vault_policy "eso_read"` resource has been removed from `bootstrap/vault/policies.tf`.

### The shared convention: `<owner-dir>/platform-values/<capability>/<instance>.yaml`

The convention established by this capability (vault-database) is meant to carry
over to future platform capabilities — vault-pki, monitoring rules, network
policy, and so on. Each capability gets one ApplicationSet, discovering instances
through the glob `**/platform-values/<capability>/*.yaml`.

### Supporting separate repositories (future)

To handle apps in other repositories, add an `scmProvider` generator to the
ApplicationSet with `merge`:

```yaml
generators:
  - merge:
      mergeKeys: [name]
      generators:
        - git: { ... }                                          # today
        - scmProvider:
            github: { organization: yu-min3 }
            filters:
              - paths: { include: ["platform-values/vault-database/*.yaml"] }
              - repositoryMatch: "^app-.*"
```

## How to verify it works

```bash
# 1. every Argo CD app is Healthy
kubectl get app -n argocd | grep vault-db

# 2. the mount, connection, and role are present in Vault
kubectl exec -n vault vault-0 -c vault -- vault secrets list  # database/ is listed
kubectl exec -n vault vault-0 -c vault -- vault list database/config
kubectl exec -n vault vault-0 -c vault -- vault list database/roles

# 3. issue a dynamic credential directly from Vault (<basename> is the instance name)
kubectl exec -n vault vault-0 -c vault -- vault read database/creds/postgres-<basename>
# → returns a username and password
# \du in Postgres shows the temporary user; it is dropped when the TTL expires

# 4. the Kubernetes Secret has been created through ESO
kubectl get secret -n <app-ns> postgres-<basename>-cred

# 5. ExternalSecret status (SecretSynced=True in each app namespace)
kubectl get externalsecret -A | grep postgres-

# 6. confirm the synced user is a Vault dynamic user
kubectl describe secret -n <app-ns> postgres-<basename>-cred
# → the key names are listed; the issued username has the form v-kubernet-postgres-...
```
