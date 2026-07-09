# Incident: Vault OIDC login broken by credential drift — 3 weeks latent

## Summary

| Item | Detail |
|------|------|
| Occurred | 2026-05-12 (the drift was created) |
| Detected | 2026-06-06 (first use of the affected path in ~3 weeks) |
| Impact | `vault login -method=oidc` failed with `unauthorized_client`; Keycloak admin console partially broken. No impact on day-to-day paths (ESO, browser SSO) — which is exactly why nobody noticed |
| Root cause | Disaster recovery on 5/12 rebuilt the Keycloak realm and **regenerated credentials, but the new values never propagated** to the password manager or to Vault's OIDC client config |

The interesting part of this incident is not the fix but the failure mode: **a credential chain silently desynchronized by a disaster recovery, surviving unnoticed because the broken path had no regular traffic and no monitoring.**

## What actually broke (3 stacked failures)

Running one rare operation surfaced three independent problems at once:

| # | Symptom | Cause | State |
|---|------|------|------|
| 1 | Keycloak admin console: "Network response was not OK" | DB creds expired — the 8-day node outage ([m4neo #2](2026-06-24-m4neo-silent-freeze.md)) outlived the Vault dynamic credential lease. Self-healed via ESO + Reloader | recovered on its own |
| 2 | Same error persisting after recovery | The RequestAuthentication `gateway-platform-keycloak-jwt` had no issuer entry for realm `master`, so the admin console's token got a 401 ("Jwt issuer is not configured") at the Istio layer. **Latent for 32 days** since the RA landed — the admin console simply hadn't been used | fixed in PR #358 |
| 3 | `vault login -method=oidc` → `unauthorized_client` | client-secret drift — see the timeline | fixed by re-syncing the secret from Keycloak into Vault's `auth/oidc/config` |

## Timeline (failure 3 — the credential drift)

| When | Event |
|------|--------|
| early March | Initial bootstrap. User password P1 / client secret S1 generated → stored in the password manager; Vault OIDC config set to S1. **Verified working** (the "I tested this before" memory dates here) |
| 05-12 | ApplicationSet takeover → cascade delete → local-path PV physically destroyed → **total Keycloak DB loss** (PR #324) |
| 05-12 23:21 | Realm rebuilt by re-running setup.sh, which **regenerated the user / client with new random values P2 / S2** |
| 05-12 → 13 | By design, setup.sh **skips existing password-manager entries** (displays new secrets on screen only), so the manager kept P1 / S1. The PR #324 checklist item "terraform plan/apply (OIDC parts)" was **merged unchecked** → Vault also kept S1 |
| 05-13 → 06-06 | The mismatch sat latent — daily paths (ESO = kubernetes auth; browser SSO = the gateway client) never touch the `vault` client, so nothing failed |
| 06-06 | First `vault login -method=oidc` in a month → discovery. The stored user password (P1) was also stale |

## Root cause analysis

1. **A by-design trap in setup.sh**: on re-run, it doesn't update existing password-manager items ("new secret shown on screen only") and leaves existing users' passwords for manual reset. In a "rebuild everything" disaster-recovery scenario, this makes the credential SoT **silently stale**
2. **An unfinished recovery checklist**: PR #324's verification checkboxes were merged all-unchecked, and nobody tracked them afterwards
3. **Rarely-used paths can break invisibly**: there was no active monitoring of the Vault OIDC login path

## Response

| Measure | Detail |
|------|------|
| Immediate recovery | Fetched the current client secret from the Keycloak admin API and updated Vault's `auth/oidc/config` using the break-glass admin |
| RA fix | PR #358: added the master-realm issuer (verified 401 → 200; tested via temporary apply, letting selfHeal roll it back) |
| Password manager | Updated the stale user / client entries to current values |
| setup.sh fix | Changed re-runs to update password-manager items in place |

## Lessons

- **"I tested it" is a snapshot of bootstrap time.** When disaster recovery rebuilds the foundation, every credential chain that depends on it (IdP → password manager / Vault config) must be re-synchronized. If a PR carries a checklist, track it to completion
- **Break-glass access paid for itself.** An emergency non-OIDC admin (userpass) was the only way in — full Keycloak dependence would have been checkmate. Provision the break-glass before you need it
- **GitOps selfHeal doubles as a try-and-auto-revert test rig**: the RA fix was validated with a temporary apply that selfHeal rolled back on its own
- **Monitor the rarely-used paths too**: a synthetic probe of the Vault OIDC discovery + token endpoints joined the candidates for layer ② of [cluster health monitoring](../architecture/cluster-health-monitoring.md)

## Related

- [m4neo silent-freeze series](2026-06-24-m4neo-silent-freeze.md) — the outage whose recovery surfaced this
- PR #324 (the 5/12 disaster recovery) / PR #358 (RA fix)
- The prune-cascade that destroyed the Keycloak DB is why stateful resources now carry per-resource `Prune=false` and run on Longhorn with `Retain` — see [storage architecture](../architecture/storage.md)
