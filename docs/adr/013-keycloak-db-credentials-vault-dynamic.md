# ADR-013: Keycloak DB Credentials Moved to Vault Dynamic (Supersedes ADR-008)

## Status

**Superseded by [ADR-019](019-keycloak-db-credentials-revert-to-static.md)** (2026-07-02 revert to
Vault static) — was: Accepted, supersedes [ADR-008](008-keycloak-db-credentials.md)

> **Note on timing**: This ADR is a *retroactive record*. The migration itself was implemented at Stage 5c
> (`keycloak-postgres-cred`, defined by the VDBE instance
> `kubernetes/secrets/vault-database-engine/platform-values/vault-database/keycloak-prod.yaml`), but the decision
> that reversed ADR-008 was never written down at the time. This document reconstructs the rationale so that
> the ADR trail matches reality. Discovered during the 2026-06-07 design review.

## Date

2026-06-07 (decision implemented earlier at Stage 5c)

## Context

ADR-008 decided that Keycloak DB credentials stay on static Sealed Secrets **permanently** ("Future Vault
migration — Do not perform"), to avoid a cascade failure where a Vault outage takes down Keycloak and with it
all SSO. Despite that, the implementation has since moved Keycloak to Vault dynamic credentials:

- `kubernetes/secrets/vault-database-engine/platform-values/vault-database/keycloak-prod.yaml` defines a
  VDBE instance (`rootOwner: keycloak`, `targetSecretName: keycloak-postgres-cred`, keyMapping to
  `KC_DB_USERNAME` / `KC_DB_PASSWORD`)
- `kubernetes/auth/keycloak/keycloak-deployment.yaml` consumes `keycloak-postgres-cred` via `envFrom`
  with a Stakater Reloader annotation (12h ESO refresh, 24h lease TTL)
- `docs/secret-management/index.md` inventory lists it as ✅ in use

### What changed since ADR-008

1. **The vault-database-engine platform capability matured** (self-made chart + ApplicationSet +
   platform-values). Adding one instance file is the entire cost of dynamic credentials; ADR-008's
   "zero added operational surface" argument for static no longer differentiates the two patterns.
2. **ADR-008's circular-dependency concern does not hold for the machine path.** The cycle ADR-008 feared
   (Keycloak needs Vault, Vault's OIDC needs Keycloak) only exists for *human* OIDC login. The credential
   machinery uses **Kubernetes auth** (VCO / ESO service accounts), and Vault itself unseals via **AWS KMS
   auto-unseal** with no human or Keycloak involvement. Full-outage recovery is therefore linear:
   `K8s → Vault (auto-unseal) → VDBE/ESO → keycloak-postgres-cred → Keycloak`.
3. **The blast radius of a Vault outage is bounded by the lease, not zero.** The last-synced K8s Secret
   survives ESO refresh failures, and the dynamic PG user stays valid until lease expiry (TTL 24h /
   max 72h). A Vault outage shorter than the remaining lease does not interrupt Keycloak at all.

## Decision

**Move Keycloak DB credentials to Vault dynamic (VDBE), accepting the residual cascade risk with the
following mitigations.** This supersedes ADR-008's Decision table in full.

| Item | Decision |
|---|---|
| Keycloak DB credentials | Vault dynamic via vault-database-engine (`keycloak-postgres-cred`) |
| Postgres placement | Unchanged — Keycloak-dedicated Postgres StatefulSet |
| Static `postgresql-secret` | **Kept** — Postgres server bootstrap + VDBE root credential + break-glass |
| Rotation | Automated: lease TTL 24h, ESO refresh 12h, Reloader rolling restart |

### Accepted risk and mitigations

| ADR-008 concern | How it is addressed now |
|---|---|
| Vault outage → Keycloak outage → all SSO down | Outage must exceed the remaining lease (≤24h) *and* coincide with a Keycloak credential refresh to bite. Vault runs Raft HA (3 replicas) + KMS auto-unseal + Longhorn (Retain) storage |
| Circular DR startup order | No longer circular — see Context. Vault recovery is human-independent (KMS auto-unseal); humans use break-glass userpass only if OIDC is down |
| DR restore path complexity | Static root cred (`postgresql-secret`) still restores Postgres from snapshot directly; Keycloak can be pointed back at it manually in a worst-case DR |

### DR startup order (replaces ADR-008 §4)

```
K8s → Sealed Secrets → cert-manager → Istio
    → Keycloak Postgres (boots with static postgresql-secret)
    → Vault (KMS auto-unseal, no human needed)
    → VDBE issues dynamic user → ESO syncs keycloak-postgres-cred
    → Keycloak
```

## Consequences

### Positive

- DB password rotation is fully automated (was: manual kubeseal + restart, "executed once per year")
- Per-lease PG users give per-connection auditability
- Keycloak follows the same platform pattern as dagster / (eventually) kensan-app — fewer exceptions to document

### Trade-offs

- A Vault outage longer than the lease TTL now *does* stop Keycloak logins (accepted; bounded and mitigated)
- One more moving part (VDBE instance + ESO + Reloader) in the Keycloak path
- ADR-008's "keep Keycloak boring" principle is partially traded for platform consistency

### Cautionary note (from the 2026-06-07 review)

Dynamic migration is **not** mechanically safe for every app: Backstage's attempted cutover failed because it
connects to 12 databases while VDBE `creationStatements` only GRANTs on `dbName` (see the 据置 section of
`docs/secret-management/index.md`, and PR #377 → #379). Check the secret-management inventory before
migrating any further app.

## References

- [ADR-008](008-keycloak-db-credentials.md) — superseded by this ADR; its reasoning was correct for the
  platform state of 2026-05-03 and is preserved unchanged as the historical record
- `kubernetes/secrets/vault-database-engine/` — the platform capability used
- `docs/secret-management/index.md` — credential inventory (single source of truth for what uses what)
