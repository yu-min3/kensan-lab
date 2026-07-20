# ADR-021: gateway-prod authorization flips from opt-in host enumeration to default-protected

## Status

**Accepted** (2026-07-19). Decided while onboarding konro (the second app-base consumer)
surfaced how much per-app platform editing the opt-in design required.

## Context

gateway-prod's SSO enforcement was **opt-in**: the CUSTOM (ext_authz) policy listed
protected hosts explicitly, and the ALLOW policy carried both a `hosts:` list (platform-admin
required) and a catch-all `notHosts:` rule that let every unlisted host through with **no
authentication at all**. This shape was inherited from the legacy-kensan era, when apps on
gateway-prod were expected to bring their own auth — it was never a deliberate policy choice.

Onboarding konro made the cost concrete. Adding one app host required synchronized edits in
three shared places (CUSTOM `hosts`, ALLOW `hosts`, ALLOW `notHosts`) plus a Keycloak
redirect-URI registration, and the failure mode of forgetting any of the authz entries was
an **unauthenticated exposure** — the dangerous direction. The kensan Phase 7 cutover
(#394→#396) had already demonstrated the redirect-URI variant of this failure.

## Decision

1. **Default-protected (opt-out) on gateway-prod.** The CUSTOM policy sends every request on
   the gateway to oauth2-proxy ext_authz (no host enumeration), and the ALLOW policy requires
   `groups: platform-admin` for everything. The catch-all pass-through rule is deleted.
   Exemptions stay path-scoped and host-agnostic: `/oauth2/*` (OIDC flow itself) and
   `/manifest.webmanifest` + `/favicon.svg` (PWA assets that browsers must fetch without
   credentials; CORS blocks the Keycloak redirect otherwise).
2. **A new app host needs zero authz edits.** Publishing an HTTPRoute on gateway-prod places
   the host behind SSO automatically. Forgetting something now fails closed: an unregistered
   redirect URI shows a visible Keycloak error instead of silently serving unauthenticated
   traffic.
3. **Public hosts become explicit opt-outs.** If a host must ever be served without
   authentication, it gets an explicit ALLOW rule (and CUSTOM exclusion) plus an ADR entry.
   No opt-outs exist at the time of writing; legacy test hosts (`test-app-*`, `shop`,
   `kensan-preview`) simply fall behind SSO, which is acceptable and reversible.
4. **gateway-platform keeps its current opt-in shape for now.** Platform UIs are added
   rarely and several use per-app OIDC bypasses (Grafana Path B, ArgoCD); flipping that
   gateway is a separate decision with a different blast radius.

## Alternatives considered

- **Keycloak host-wildcard redirect URI** (`https://*.app.yu-min3.com/oauth2/callback`) to
  also eliminate the per-host `GATEWAY_PROTECTED_HOSTS` entry in `bootstrap/keycloak/setup.sh`:
  **rejected — Keycloak does not support wildcards in the hostname**, only trailing path
  wildcards, and upstream treats this as a deliberate spec-alignment choice
  (keycloak/keycloak#14113, #8929). The per-host list in setup.sh therefore remains the one
  per-app platform touch, but its failure mode is a visible login error, not exposure.
- **Static oauth2-proxy `redirect-url`** (single canonical callback host + `rd` bounce):
  would reduce Keycloak registration to one URI, but reconfigures the shared oauth2-proxy
  used by both gateways — a much larger blast radius than the problem justifies today.
  Recorded as a future option if per-host registration ever becomes painful.

## Consequences

- App onboarding on gateway-prod is now: app manifests + Application CR + two one-line
  platform touches — a setup.sh host entry (redirect URI) and a `from:` entry in the
  `allow-app-oauth2-routes` ReferenceGrant (the per-app `/oauth2` HTTPRoute lives in the app
  namespace because gateway-prod's allowedRoutes selector rejects auth-system). No shared
  AuthorizationPolicy edits. Konro onboarding also surfaced and removed a third touch:
  AppProject destinations now use an `app-*` glob (#452). A future follow-up could remove
  the ReferenceGrant touch by serving a wildcard `*.app.yu-min3.com` oauth2 route from
  auth-system, but that requires widening the gateway-prod allowedRoutes selector — a
  separate decision.
- Every host on gateway-prod requires platform-admin. Per-host authorization tiers (e.g. a
  family-members group for konro) will need a host-scoped ALLOW rule when the need arises —
  that reintroduces enumeration for those hosts only, which is the correct trade.
- Live legacy hosts changed behavior from open to SSO-gated at rollout (verified 2026-07-19:
  kensan unaffected, `test-app-9` now redirects to Keycloak).
