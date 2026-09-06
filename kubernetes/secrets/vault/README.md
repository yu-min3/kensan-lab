# vault

The Vault HA server — three replicas, AWS KMS auto-unseal, Raft storage. It is
the core of secret management, hosting the KV, database, Transit, and OIDC secret
engines.

## Layout

- `values.yaml` — overrides on HashiCorp's official chart (HA, KMS auto-unseal, Raft `retry_join`, a pinned image)
- `resources/`
  - `namespace.yaml` — the `vault` namespace
  - `aws-kms-credentials-sealed.yaml` — the IAM credential for KMS, as a SealedSecret. It cannot come from Vault itself, because Vault needs it to start
  - `httproute.yaml` — UI and API (`vault.platform.yu-min3.com`)

## Things to watch

- **`spec.syncOptions: Prune=false`** — stops the PV holding the Raft data from vanishing if the Application is deleted. The resources finalizer is removed as well
- **The image tag is pinned explicitly**, never `latest`. The reasoning is in ADR-011
- **TLS is disabled** — in-cluster traffic is covered by Istio mTLS. To be revisited in Phase 2 and beyond

## Related

- The four methods and how they are operated: [`docs/secret-management/index.md`](../../../docs/secret-management/index.md)
- Bootstrap (first unseal, the root token): [`docs/bootstrapping/vault-stage1.md`](../../../docs/bootstrapping/vault-stage1.md)
