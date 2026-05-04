# Gateway OIDC Foundation (DRAFT — not yet active)

## What this directory is

Reference YAML for Gateway-level OIDC at `gateway-platform`, prepared for **whichever Path is chosen in ADR-010**. Files here are **NOT** under any Argo CD `Application` source — they are scaffolding that must be copied/moved into a synced location once Yu confirms the Path.

## Status

⚠️ **Inert.** Nothing in this directory is applied to the cluster.

The files use `targetRef: gateway-platform` (the real Gateway) — copying them under `infrastructure/network/istio/` (the istio-resources Application source) and committing **WILL** activate them. Do not do that until you are sure of the Path and have prepared the secret material.

## File index

| File | Purpose | Path-dependent? |
|---|---|---|
| `requestauthentication.yaml` | JWT verification on gateway-platform, `forwardOriginalToken: true` | No (works for A, B; for C this is unused) |
| `authorizationpolicy-deny-all.yaml` | Default-deny base, no rules | No |
| `authorizationpolicy-allow.yaml` | Explicit ALLOW per host × group claim | No |
| `path-a-meshconfig-snippet.yaml` | Snippet to add to `infrastructure/network/istio/istiod/values.yaml` | Path A only |
| `path-a-oauth2-proxy-values.yaml` | oauth2-proxy Helm chart values draft | Path A only |
| `path-a-externalsecret.yaml` | ExternalSecret in `auth-system` ns, pulls client_secret + cookie_secret | Path A only |
| `path-a-authorizationpolicy-custom.yaml` | `action: CUSTOM` policy that invokes ext_authz provider | Path A only |
| `path-b-envoyfilter.yaml` | EnvoyFilter inserting envoy.filters.http.oauth2 | Path B only |
| `path-b-externalsecret.yaml` | ExternalSecret in `istio-system` ns | Path B only |
| `activation-runbook.md` | Step-by-step activation procedure per Path | All |

## Activation overview

The activation order is described in detail in `activation-runbook.md`. Conceptually:

```
1. Pick Path (A / B / C) — see ADR-010
2. Add OIDC client to Keycloak (uncomment the relevant block in
   bootstrap/keycloak/setup.sh and re-run; idempotent)
3. Put client_secret / cookie_secret into Vault
4. Move path-specific YAML into a synced Application source path
5. Apply ext_authz provider OR EnvoyFilter (path-specific)
6. Apply RequestAuthentication (gateway-platform)
7. Apply AuthorizationPolicy in the order:
     a. ALLOW rules first (claim-based, host-by-host)
     b. THEN default-deny — only after step 6 ALLOW rules cover all hosts
        currently in use, otherwise lockout
8. Verify per-host: anonymous → 302 to Keycloak; authenticated user with
   group → reaches app; authenticated user without group → 403
```

## Why "deny-all" must come *after* the ALLOW rules

Istio AuthorizationPolicy semantics: when a workload (or Gateway) has any
`ALLOW` policy targeting it, traffic not matching any ALLOW rule is denied.
A separate "deny-all" policy is therefore not strictly required to achieve
default-deny — it is a *belt-and-suspenders* clarity measure.

But applying the deny-all policy *before* the ALLOW policy creates a
window (seconds to minutes, depending on Istio config push) during which
all gateway-platform traffic is denied including traffic Yu is currently
using to apply the ALLOW policy. For homelab single-operator use this
window can be a real lockout.

The runbook orders ALLOW first, deny-all after.

## Rollback

`kubectl delete authorizationpolicy -n istio-system <name>` is sufficient
to revert. Do this in *reverse* of the activation order:

```
1. Remove default-deny first (so traffic is no longer constrained)
2. Remove ALLOW policy
3. Remove RequestAuthentication
4. Remove ext_authz provider entry from istiod values (Path A) OR
   remove EnvoyFilter (Path B)
5. Disable oauth2-proxy Application (Path A only)
```

## Per-host coverage matrix (Path A reference)

When Phase 1 is on for `gateway-platform`, the ALLOW rules below must
cover every host currently routed to gateway-platform, otherwise that
host returns 403:

| Host | Group required | Rationale |
|---|---|---|
| `argocd.platform.yu-min3.com` | `platform-admin` | Argo CD admin only |
| `grafana.platform.yu-min3.com` | `platform-admin`, `platform-dev` | Read access for devs |
| `prometheus.platform.yu-min3.com` | `platform-admin` | Internal use only |
| `backstage.platform.yu-min3.com` | `platform-admin`, `platform-dev` | Developer portal |
| `vault.platform.yu-min3.com` | `platform-admin` | break-glass via Vault userpass remains |
| `hubble.platform.yu-min3.com` | `platform-admin` | Network observability |
| `longhorn.platform.yu-min3.com` | `platform-admin` | Storage admin only |
| `auth.platform.yu-min3.com` | **(no policy)** | Keycloak itself; OIDC redirect target — **must bypass** |

⚠️ The `auth.platform.yu-min3.com` host is the OIDC IdP. If the OIDC flow
is itself gated by OIDC, login becomes impossible. The deny-all policy
must explicitly exclude this host (see `authorizationpolicy-deny-all.yaml`
comments).

## Cross-reference

- ADR-010 (this PR) — the Path selection ADR
- ADR-005 — the original "native oauth2" decision, status: Re-evaluation Required
- ADR-002 — phased authn/authz approach (still valid)
- `projects/kensan-lab/secrets-phase1-design.md` § アプリ認証/認可フロー (kensan-workspace)
- `notes/security/break-glass-runbook.md` (kensan-workspace)
