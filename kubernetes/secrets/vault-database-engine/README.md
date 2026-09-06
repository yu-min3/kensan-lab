# vault-database-engine

Dynamic Postgres users — short-lived credentials with a TTL — as a cluster-wide
capability. One values file per instance renders six resources: the Vault side
(config, role, auth role) and the app side (SA, VaultDynamicSecret,
ExternalSecret). The pod reads the credential from a Kubernetes Secret that ESO
refreshes before the TTL expires.

Where this sits among the four secret-delivery methods is in
[`docs/secret-management/index.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/secret-management/index.md).

## Layout

- `chart/` — the PE-owned Helm chart. `values.yaml` carries the conventions and is annotated knob by knob; read it rather than duplicating it here
- `shared/` — applied once: the `database/` mount, the `eso-read` policy, and the CCNP/CNP pair that lets Vault reach Postgres on TCP/5432
- `platform-values/vault-database/` — one file per instance. Currently empty (the Dagster instance went with `kensan-legacy` in #403; Keycloak was reverted in ADR-019)

Argo CD wires it as `app-shared.yaml` (a single Application for `shared/`) plus
`applicationset-instances.yaml`, which discovers instances by the glob
`**/platform-values/vault-database/*.yaml`.

## Adding an instance

Two steps. Write the values file:

```yaml
# <owner-dir>/platform-values/vault-database/<instance>.yaml
ns: my-app
```

and label the target namespace `kensan-lab.platform/vault-managed-postgres: "true"`.

Everything else is derived: the Vault role becomes `postgres-<instance>` from the
filename, the host becomes `<releaseName>.<ns>.svc.cluster.local`, the KV admin
path becomes `secret/data/db-admin/postgres-<instance>`, and the Secret
`postgres-<instance>-cred` appears in the app namespace with the Bitnami-standard
keys. Override only what differs — `rootOwner`, `dbName`, `host`, `keyMapping`,
`targetSecretName`, `ttl` — each documented in `chart/values.yaml`.

## Things to watch

- **The namespace label is not optional.** The cluster-wide network policies in `shared/` are opt-in, so without it VCO cannot reach Postgres and issuance fails with no other symptom
- **This delivers credentials through a Kubernetes Secret**, which means a running process needs a Reloader restart or an explicit reload to pick up a rotation. That rules it out for Keycloak, Backstage, Polaris, anything owning a schema, and anything where a restart costs sessions
- **`VaultDynamicSecret` ignores `serviceAccountRef.namespace`** (the CRD is namespace-scoped), so the operator's central SA cannot be borrowed. The chart creates an SA and a Vault auth role per instance instead — nothing for the app developer to write
- **The `eso-read` policy lives here, not in the bootstrap Terraform.** Only admin and vco-admin are bootstrapped; everything else is a CR created once VCO is running
- **`<owner-dir>/platform-values/<capability>/<instance>.yaml` is a shared convention**, meant to be reused by future capabilities with one ApplicationSet each

## Related

- The four methods and their inventory: [`docs/secret-management/index.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/secret-management/index.md)
- The same pattern for application-layer encryption: [`vault-transit-engine/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/secrets/vault-transit-engine/README.md)
- Why Keycloak's credentials are not delivered this way: [ADR-019](https://github.com/yu-min3/kensan-lab/blob/main/docs/adr/019-keycloak-db-credentials-revert-to-static.md)
- Bootstrap and the Vault root of trust: [`bootstrap/vault/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/bootstrap/vault/README.md)
