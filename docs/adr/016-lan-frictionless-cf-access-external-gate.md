# ADR-016: LAN-Frictionless Sessions + Cloudflare Access as the External Gate

## Status

Accepted

> Builds on [ADR-010](010-istio-native-oauth2-absent.md) (oauth2-proxy as the Gateway-level
> `ext_authz` OIDC front). This ADR does not change *how* identity is established inside the
> cluster — it changes *how often* a user must re-authenticate, and adds an outer gate for
> traffic that originates outside the LAN.

## Date

2026-06-28

## Context

Platform UIs are reachable by two ingress paths that land on the same Istio Gateway
(see `kubernetes/network/README.md`):

| Path | Host | Entry |
|---|---|---|
| **LAN** | `*.platform.yu-min3.com` | Cilium L2 LoadBalancer (`.242`) |
| **External** | `*.yu-mins.com` | Cloudflare Tunnel (`cloudflared`, outbound-only) |

Today both paths are gated identically: the `gateway-platform` `AuthorizationPolicy` (CUSTOM)
forces every host through oauth2-proxy `ext_authz`, and the oauth2-proxy session is pinned to
**12h** (`cookie_expire = "12h"`, deliberately matched to Keycloak's `SSO Session Max = 12h`).

Two problems with treating both paths the same:

1. **LAN is needlessly high-friction.** On a trusted home network, being bounced to a login
   every 12h adds friction with little security benefit — the threat model on the LAN is "a
   device already inside the house", not "an anonymous internet client".
2. **External is *not* stricter than LAN**, even though it should be. Internet exposure wants a
   harder gate (re-auth cadence, MFA, edge bot/DDoS filtering) that a 12h cookie does not provide.

### The core tension

The naive framing is "no auth on LAN, auth on external". But **"no auth" and "know who the user
is" are contradictory** — identity *requires* authentication. The real lever is therefore not
auth on/off but **session lifetime + step-up**:

- LAN → authenticate **once per device**, then a long-lived (effectively permanent) session →
  frictionless *and* identified.
- External → a separate, shorter outer gate that re-challenges independently of the inner session.

### Why the double-prompt happens today (and how to avoid it)

External users currently meet **two separate identity providers**: Cloudflare Access (OTP) and
then oauth2-proxy (Keycloak). Two IdPs ⇒ two logins. The fix is to **unify the IdP on Keycloak**:
point Cloudflare Access's login method at Keycloak via OIDC instead of OTP. Then the user enters
credentials once (at the CF Access → Keycloak hop); when oauth2-proxy subsequently redirects to
Keycloak, the Keycloak SSO session already exists and the second hop is a **silent redirect**.

### Constraint discovered in the existing config

`oauth2-proxy/values.yaml` already serves all three cookie domains from a **single instance**
(`cookie_domains = [".platform.yu-min3.com", ".app.yu-min3.com", ".yu-mins.com"]`), so no second
proxy is needed. But `cookie_expire` is **global** — one instance cannot give the LAN domain a
longer cookie than the external domain. And oauth2-proxy can only keep a session alive as long as
**Keycloak's refresh token / `SSO Session Max`** allows; the current 12h is a Keycloak-side cap,
not just a cookie setting.

## Decision

Adopt a **two-layer model**: a long-lived inner identity session (frictionless on LAN) plus an
external-only outer gate (Cloudflare Access). Concretely:

1. **Inner identity layer — oauth2-proxy session extended to 30 days.**
   Set `cookie_expire = "720h"` with `cookie_refresh` keeping the access token fresh. This makes
   LAN frictionless: log in once per device, stay signed in for ~30 days. A single global value is
   acceptable because external strictness is supplied by the outer gate, not by this cookie
   (see point 3). 30 days (not 90) is chosen as a conventional "remember-me" ceiling that still
   bounds the blast radius of a stolen, already-unlocked LAN device.

2. **Keycloak session raised to match.** `cookie_expire` alone does not deliver 30 days — Keycloak
   caps it. The `kensan` realm's `SSO Session Idle` / `SSO Session Max` (and refresh-token lifetime
   for the oauth2-proxy client) must be raised to ~30 days, otherwise refresh fails and the user is
   bounced regardless of the cookie. **This is configured in the Keycloak admin console, not in
   Git** (the realm is not yet managed as code) — a drift surface, tracked as a follow-up.

3. **External outer gate — Cloudflare Access in front of `*.yu-mins.com`.**
   Because external traffic *only* exists on the Cloudflare Tunnel path, putting an Access
   application on `*.yu-mins.com` automatically makes "external" the stricter path with **no
   per-source logic in Istio**. The LAN path never traverses Cloudflare. Access session TTL is set
   to **24h** and may enforce MFA. The 30-day inner cookie does not weaken external posture: CF
   Access re-challenges every 24h independently, so external re-auth cadence = 24h.

4. **Single IdP — Cloudflare Access logs in via Keycloak (OIDC), not OTP.** This eliminates the
   double prompt (the second hop becomes a silent SSO redirect) and keeps **one user store** in
   Keycloak. User management — who may access externally — is expressed as Keycloak users/groups,
   surfaced to CF Access through the OIDC claims.

5. **Inner authz unchanged.** oauth2-proxy remains the single source of in-cluster identity
   (`Authorization: Bearer <id_token>` + `X-Auth-Request-*`), so existing `AuthorizationPolicy`
   `when claims[groups]` rules and app-side header trust are untouched. We deliberately **keep**
   oauth2-proxy on the external (`*.yu-mins.com`) hosts rather than dropping it in favour of
   verifying the CF Access JWT — that alternative would introduce a second identity-token shape and
   force per-host authz/plumbing changes (see Alternatives).

## Consequences

### Positive

- LAN: one login per device, then ~30 days frictionless — while every request still carries a
  Keycloak identity (groups claim intact for authz).
- External: a genuinely stronger gate (24h re-auth + optional MFA + Cloudflare edge filtering)
  layered *in front of* the same identity, cleanly orthogonal to the inner session.
- No second oauth2-proxy and no Istio source-IP logic: external strictness falls out of topology
  (CF only sits on the tunnel path).
- One user store (Keycloak) for both the outer gate and inner identity; no split allowlists.

### Trade-offs / watch-outs

- **A stolen, unlocked device on the LAN stays signed in for up to 30 days.** Acceptable for a
  homelab; revisit if the threat model changes (the lever is `cookie_expire` + Keycloak session max).
- **Keycloak realm session settings are imperative (admin console), not GitOps.** They will drift
  unless the realm is managed as code. Tracked as a follow-up; until then, the 30-day values live
  only in the running Keycloak.
- **Cloudflare Access config (application, session TTL, Keycloak OIDC IdP) is dashboard-managed**,
  outside Git — another non-GitOps surface to document and remember.
- Two redirect hops remain on the external path (CF Access → Keycloak, then oauth2-proxy →
  Keycloak), but only **one** credential entry; the second hop is a silent SSO redirect.

## Alternatives considered

### B. Drop oauth2-proxy on the external path; verify the CF Access JWT in Istio

Have Istio `RequestAuthentication` validate the `Cf-Access-Jwt-Assertion` against Cloudflare's
JWKS and bypass oauth2-proxy for `*.yu-mins.com`. Fewer hops, but the cluster would then carry
**two identity-token shapes** (CF JWT externally, Keycloak `id_token` internally), forcing
per-host `AuthorizationPolicy` and app header changes. Rejected: complexity not worth it for a
homelab whose authz is uniformly Keycloak-groups-based.

### C. Second oauth2-proxy instance with a shorter external cookie

Run a dedicated external oauth2-proxy with a short `cookie_expire`, selected by host. Achieves
per-path session length without Cloudflare, but adds a proxy + ext_authz provider + policy rule and
still lacks an edge gate (MFA, bot/DDoS filtering). Rejected in favour of CF Access doing the outer
gate with a single inner proxy.

### D. Bypass auth entirely on LAN (source-IP / hostname allow)

Genuinely "no auth" on LAN — but then LAN requests carry **no identity**, breaking the "still know
who the user is" requirement and the groups-based authz. Rejected.

## References

- [ADR-010](010-istio-native-oauth2-absent.md) — oauth2-proxy as the Gateway `ext_authz` OIDC front
- [ADR-002](002-authentication-authorization-architecture.md) — original authn/authz architecture
- `kubernetes/auth/oauth2-proxy/values.yaml` — `cookie_expire` / `cookie_refresh` (inner session)
- `kubernetes/network/istio/authorizationpolicy-gateway-platform-oauth2.yaml` — CUSTOM ext_authz hosts
- `.claude/rules/network-ingress.md` — edge architecture (two ingress paths) & Gateway-level auth
