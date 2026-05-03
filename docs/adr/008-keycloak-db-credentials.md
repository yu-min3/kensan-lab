# ADR-008: Keycloak DB Credentials Are Not Moved to Vault

## Status

Accepted

## Date

2026-05-03

## Context

From Phase 2 onward, dynamic PG credentials via the Vault Database engine become an option, but whether to include Keycloak's DB credentials in that scope is a separate judgment. Keycloak is the "sole human IdP" of this platform, and 5 systems (Vault / Argo CD / Grafana / Backstage / general apps) all look at Keycloak's OIDC (ADR-005).

### Dependency Chain

```
[Human] -- OIDC --> [Vault]
                      | (uses for human auth)
                      v
                  [Keycloak]
                      | (needs DB)
                      v
                  [Postgres]
```

If, under this structure, Keycloak's DB credentials are issued via Vault dynamic credentials, a Vault outage directly translates into a Keycloak outage, taking down login for all 5 systems.

### Existing Implementation

`infrastructure/security/keycloak/` review:

- A dedicated Postgres StatefulSet (Keycloak-only, not shared)
- Both Keycloak and Postgres credentials managed by Sealed Secrets
- prod / dev separated as overlays

=> The existing implementation already matches this ADR's conclusion. **No change is required; this ADR's purpose is to make the decision explicit.**

### Requirements

1. **Keep Keycloak as boring as possible at the infrastructure tier**: Avoid bringing in extra dependencies
2. **Avoid cascade failures**: A Vault outage must not propagate to Keycloak
3. **Keep DR recovery paths simple**: Restore via Postgres snapshot + static pw

### Patterns Considered

#### Pattern A: Static Management via Sealed Secrets (Adopted; matches existing)

Manage Keycloak DB credentials with a dedicated Postgres + Sealed Secrets.

**Pros:**
- Keycloak runs even when Vault is down (independent failure domain)
- During recovery from a full outage, Keycloak -> Vault startup order is possible (one-way dependency)
- Matches the existing implementation; zero added operational surface

**Cons:**
- DB pw rotation is manual (re-run kubeseal + Pod restart)
- Audit logs stay at the level of Sealed Secrets controller K8s events

#### Pattern B: Dynamic Credentials via Vault Database Engine

Vault dynamically issues PG users with a TTL such as 1h, injected into the Keycloak Pod via ESO.

**Pros:**
- Pw rotation is fully automated
- Each Keycloak Pod can connect with its own user; audit can be taken per user

**Cons:**
- **A full Vault outage stops Keycloak (cred refresh impossible)** -> SSO is wiped out
- The startup order during full-outage recovery becomes circular (Keycloak needs Vault, Vault's OIDC backend needs Keycloak)
- During DR, Keycloak cannot run without Vault

#### Pattern C: Phased Migration (Phase 1 static, switch to dynamic in Phase 3)

Use Sealed Secrets in Phase 1, switch to Vault dynamic in Phase 3.

**Pros:**
- Allows migration after a learning period

**Cons:**
- The destination (Pattern B) still has the cascade-failure problem
- Likely to be reverted in Phase 3

#### Pattern D: Static Management via Vault KV

Move the static-management role of Sealed Secrets into Vault KV.

**Pros:**
- Audit logs are centralized into Vault
- Aligns with the "everything on Vault" headline

**Cons:**
- Keycloak Pod restarts during a Vault outage still cannot fetch credentials
- The added Vault dependency yields only audit-log benefits

## Decision

**Adopt Pattern A (static management with Sealed Secrets + dedicated Postgres). Make this an explicit permanent exception.**

| Item | Decision |
|---|---|
| Keycloak DB credentials | Static management via Sealed Secrets (permanent) |
| Postgres placement | Keycloak-dedicated Postgres (not shared) |
| Future Vault migration | Do not perform (keep Keycloak boring at the infra tier) |
| Vault Database engine demo | Demonstrate via the kensan app's Postgres (Phase 3 / Stage 5) |

### 1. Position as the Keycloak-Side Counterpart of "Do Not Move Everything to Vault"

As a counterpart to ADR-007 (do not adopt Vault PKI), centralization in Vault is not a goal in itself. When increasing Vault dependency, judge by **whether the cascade-failure blast radius is bounded**.

### 2. Mark as a "Permanent Exception" in the Secret Registry

In the secret-location registry of `secrets-phase1-design.md`, record Keycloak DB credentials as "static management via Sealed Secrets (permanent)". They are **explicitly excluded** from the Phase 3 "reduce Sealed Secret YAML in the repo" migration scope.

### 3. The Demo Value of Vault Database Engine Is Covered by Other Apps

The need to "showcase Vault dynamic credentials" is satisfied by placing the kensan apps' (Streamlit / Jupyter, etc.) Postgres under the Vault Database engine. There is no need to sacrifice Keycloak.

### 4. DR Scenarios

| Failure | Response |
|---|---|
| Full Vault outage | Keycloak unaffected (DB cred is static). Humans recover Vault via break-glass userpass |
| Full Keycloak outage | Humans tide over via Vault userpass / Argo CD built-in admin / Grafana local admin |
| Full Keycloak Postgres outage | Restore from snapshot, reconnect via static pw |
| Full outage | Sequential startup: K8s -> Sealed Secrets -> cert-manager -> Istio -> **Keycloak Postgres -> Keycloak -> Vault** |

## Consequences

### Positive

- A Vault outage does not propagate to Keycloak (independent failure domain)
- DR startup order does not cycle; Keycloak -> Vault recovery is possible
- No change to the existing implementation (only making it explicit completes the work)
- Making Postgres Keycloak-dedicated bounds the blast radius to Keycloak

### Trade-offs

- DB pw rotation becomes a manual operation (executed periodically, e.g., once per year)
- Audit logs are not centralized in Vault (stays at Sealed Secrets controller K8s event level)
- The "everything on Vault" headline is unavailable; documentation cost arises to explain the intentional exception

## References

- ADR-005: Phase 1 Authentication via Istio Native oauth2 + Keycloak (premise that Keycloak is the sole IdP)
- ADR-007: Do Not Adopt Vault PKI (the same "do not move everything to Vault" principle)
- Design source: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § #4 The Keycloak DB chicken-and-egg problem and decision
- [Keycloak: Database Configuration](https://www.keycloak.org/server/db)
- [HashiCorp Vault Database Secrets Engine](https://developer.hashicorp.com/vault/docs/secrets/databases)
