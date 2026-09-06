# vault-transit-engine

The Vault Transit engine as a cluster-wide capability: envelope encryption for
things like Postgres columns, where Vault holds the key and it never reaches the
pod or the database. One values file per consumer, scoped to least privilege.

Where this sits among the four secret-delivery methods is in
[`docs/secret-management/index.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/secret-management/index.md).

## Layout

- `chart/` — the PE-owned Helm chart. It renders both sides: the Vault policy and auth role, and the SA plus ConfigMap in the consumer namespace. `values.yaml` carries the conventions and is annotated knob by knob
- `shared/` — applied once: the `transit/` mount
- `platform-values/vault-transit/` — one file per consumer. Currently empty (the `kensan-users` consumer went with `kensan-legacy` in #404)
- `bootstrap/setup-transit-keys.sh` — creates `transit/keys/<name>`, run once by hand

Argo CD wires it as `app-shared.yaml` (a single Application for `shared/`) plus
`applicationset-instances.yaml`, discovering consumers by the glob
`**/platform-values/vault-transit/*.yaml`.

## Adding a consumer

Write the values file:

```yaml
# <owner-dir>/platform-values/vault-transit/<consumer>.yaml
ns: my-app
keyName: my-pii-column
```

then point the deployment at what the chart created —
`serviceAccountName: transit-<consumer>` and
`envFrom: configMapRef: transit-<consumer>-config`, which carries `VAULT_ADDR`,
`VAULT_AUTH_ROLE`, and `VAULT_TRANSIT_KEY`. Add
`reloader.stakater.com/auto: "true"` so a rotation reaches the running process.

Naming is uniform: the Vault role, policy, SA, and ConfigMap are all
`transit-<filename-basename>`, mirroring vault-database-engine's `postgres-<base>`.

## Things to watch

- **Key creation is manual, by design.** vault-config-operator has no Transit CRs — only `SecretEngineMount`, `Policy`, and `KubernetesAuthEngineRole`. Adding a second operator, forking VCO, or faking it with an idempotent CronJob all cost more than one manual step that happens once per key. VCO's coverage and its exceptions are catalogued in [`docs/secret-management/index.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/secret-management/index.md)
- **Token TTLs are 30 min / 1 h**, deliberately short of the 12 h+ default, which forces a renew loop rather than a long-lived token
- **The policy is per key**, permitting only `transit/{encrypt,decrypt,hmac,rewrap}/<keyName>` plus a read on the key and token self-renewal
- **Rotation keeps old ciphertext readable.** `vault write -f transit/keys/<keyName>/rotate` makes new encryptions use the latest version; rewrapping existing rows is an application-side call to `transit/rewrap/<keyName>`
- **Every call is audited** — the audit device enabled by the bootstrap Terraform logs each encrypt and decrypt

## Related

- The four methods and their inventory: [`docs/secret-management/index.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/secret-management/index.md)
- The same smart-default pattern, for dynamic database users: [`vault-database-engine/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/secrets/vault-database-engine/README.md)
- Bootstrap and the Vault root of trust: [`bootstrap/vault/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/bootstrap/vault/README.md)
