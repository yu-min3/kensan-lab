# ADR-019: Keycloak DB Credentials Reverted to Vault Static (Supersedes ADR-013)

## Status

**Accepted — Supersedes [ADR-013](013-keycloak-db-credentials-vault-dynamic.md)**, returning to the
position of [ADR-008](008-keycloak-db-credentials.md) (static credentials), now delivered via Vault
static + ESO rather than Sealed Secrets.

> **Note on timing**: Like ADR-013, this is a *retroactive record*. The revert was implemented on
> 2026-07-02 (commit `cc2f3fe`, "DB dynamic secret の対象を整理"), which removed the VDBE instance
> `platform-values/vault-database/keycloak-prod.yaml` along with the unconsumed `backstage` and
> `kensan-app` instances — but the decision reversing ADR-013 was not written down at the time.
> Discovered during the 2026-07-11 documentation review.

## Date

2026-07-11 (decision implemented 2026-07-02)

## Context

ADR-013 moved Keycloak's Postgres credentials to Vault dynamic (VDBE, 24h lease, ESO refresh +
Reloader restart), superseding ADR-008's "stay static permanently". Operating it surfaced the cost
that ADR-008 had originally predicted:

- **Dynamic credentials presuppose restart-on-rotation.** VDBE delivery is `env` injection + Reloader
  rollout. For Keycloak, every credential rotation restarts the IdP and destroys active SSO sessions —
  a platform-wide, user-visible interruption on a ~12h cadence, for a homelab with exactly one
  operator-facing IdP.
- **The blast-radius argument cuts the other way for an IdP.** Keycloak sits at the core of the
  SSO/Vault ecosystem (Vault's own OIDC auth depends on it). Coupling its availability to Vault's
  Database engine health re-creates the cascade ADR-008 warned about, and the marginal benefit of
  short-lived credentials is small for a single-instance Postgres reachable only inside the cluster.
- The same sweep concluded the unconsumed `backstage` (plugin-DB GRANT / schema-owner problem) and
  `kensan-app` (no runtime credential reload) instances were not worth cutting over — "generated but
  unconsumed" secrets were themselves a source of confusion.

## Decision

Keycloak DB credentials are **Vault static, delivered by ESO**: `keycloak-secret`
(`KEYCLOAK_ADMIN_PASSWORD` / `KC_DB_PASSWORD`, the latter synced from
`secret/platform-auth/prod/postgresql`). The VDBE instance is removed.

The general rule extracted from this arc: **Vault dynamic only where a restart / credential reload is
acceptable.** IdPs, catalog metastores, and long-lived services owning DB schemas stay on Vault
static (the 据置 list in [`docs/secret-management/index.md`](../secret-management/index.md)).

## Consequences

- ADR-008's cascade-risk analysis is re-affirmed; ADR-013's mitigations are retired with the instance.
- The VDBE rails (chart, shared resources, ApplicationSet) **remain deployed** for future consumers
  that tolerate restart-on-rotation. After the kensan-legacy retirement removed the last consumer
  (dagster, PR #403), there are currently **zero active dynamic instances** — the capability is live,
  the inventory is empty.
- Documentation that presented Keycloak as the worked example of dynamic credentials
  (`kubernetes/secrets/README.md`, `.claude/rules/security-secrets.md`) is corrected alongside this
  ADR: the Keycloak arc (static → dynamic → static) is now the worked example of *re-evaluating* the
  right-sizing framework as operational evidence accumulates.
