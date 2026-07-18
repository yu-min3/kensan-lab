# Storage roadmap: local-path → Longhorn

> **✅ Done** — the Longhorn migration completed in 2026-05, and `local-path` was fully retired in 2026-07 (both the provisioner and the StorageClass were removed). `kubernetes/storage/README.md` is the source of truth for the current storage setup, StorageClasses, and R2 backups. This page is an archived record of the roadmap as originally planned.
>
> Where the implementation diverged: the `longhorn-single` StorageClass envisioned at the time was never adopted. It shipped instead as two classes, **`longhorn`** (the default, replicated block) **and `longhorn-workspace`**. Read any mention of "`longhorn-single`" below as `longhorn` / `longhorn-workspace` in the actual implementation.

What follows is the historical record as written at planning time (see above for how it actually turned out).

Every PV currently runs on `local-path-provisioner` (node-local). HA stateful workloads (Vault raft, Keycloak PostgreSQL) are **HA in name only** — since the PVC is pinned to the node it was created on, we're accepting the risk of data loss on node failure.

Adopting Longhorn will switch this to distributed storage.

## Current constraints

- **Node-local**: a PVC is pinned to whichever node it first bound to. Moving a pod to a different node means recreating the PVC — i.e. data loss
- **HA in name only**: `vault-0/1/2` are spread across separate nodes via anti-affinity, so "quorum survives a single-node failure out of 3" does hold. However, the Vault raft peer data on the failed node is lost; recovery relies on the raft rejoin process (which auto-syncs peer data)

## Affected components

| Component | PVC | Current storageClass | Migrating to |
|---|---|---|---|
| Vault raft | `data-vault-{0,1,2}` (10Gi × 3) | `local-path` | `longhorn-single` (replica 1 — Vault raft itself already provides 3x redundancy) |
| Vault audit | `audit-vault-{0,1,2}` (10Gi × 3) | `local-path` | `longhorn-single` |
| Keycloak PostgreSQL | TBD | `local-path` | `longhorn-single` (primary redundancy comes from Keycloak HA replicas) |
| Prometheus / Loki / Tempo | (not yet surveyed) | `local-path` | TBD |

Why `longhorn-single`: Vault raft and Keycloak already replicate at the app layer, so replicating again at the storage layer would multiply to 9x total copies — too heavy for homelab scale. `longhorn-single` (replica=1) buys just the one property we actually want: "a PVC recovers on a different node after a node failure."

## Phases

1. Adopt Longhorn (separate ADR / separate PR)
2. Switch each component's `storageClass` from `local-path` to `longhorn-single`
3. Migrate PVC data onto Longhorn volumes
4. Remove local-path-provisioner (`kubernetes/storage/`)

## Known prerequisites

- Longhorn requires the `iscsi_tcp` kernel module and open-iscsi → verify on every Pi 5 and the Bosgame M4 Neo node beforehand
- The Pi 5's microSD has weak I/O performance → prefer AMD64 nodes when placing Longhorn replicas

## Related

- [Stage 1 Bootstrap](../bootstrapping/12-vault-stage1.md): the current state where Vault runs on `local-path`
- [Longhorn restore test](../runbooks/longhorn-restore-test.md): the existing Longhorn restore runbook
