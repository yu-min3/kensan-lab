# ADR-016: LAN-Frictionless Sessions + External-Gate Model (gate decision deferred)

## Status

**Accepted in part.**

- **Decided & shipping (PR #390):** the inner identity session is extended to 30 days so the LAN
  is frictionless (log in once per device, then stay signed in).
- **Deferred:** the *external-gate model* — whether internet traffic is controlled by Cloudflare
  Access or simply passes through to the internal Gateway's Keycloak auth. See "Deferred decision".
- **Interim state (safe):** Cloudflare Access is already configured with **OTP** in front of
  `*.yu-mins.com`. That remains a genuine second factor, so external access is protected while the
  model decision is open.

> Builds on [ADR-010](010-istio-native-oauth2-absent.md) (oauth2-proxy as the Gateway-level
> `ext_authz` OIDC front). This ADR does not change *how* identity is established inside the
> cluster — it changes *how often* a user must re-authenticate, and frames how external traffic is
> gated.

## Date

2026-06-28

## Context

Platform UIs are reachable by two ingress paths that land on the same Istio Gateway
(see `kubernetes/network/README.md`):

| Path | Host | Entry |
|---|---|---|
| **LAN** | `*.platform.yu-min3.com` | Cilium L2 LoadBalancer (`.242`) |
| **External** | `*.yu-mins.com` | Cloudflare Tunnel (`cloudflared`, outbound-only) |

Today both paths are gated identically by oauth2-proxy `ext_authz`, and the oauth2-proxy session is
pinned to **12h** (`cookie_expire = "12h"`, deliberately matched to Keycloak `SSO Session Max = 12h`).

Two problems with treating both paths the same:

1. **LAN is needlessly high-friction.** On a trusted home network, a forced login every 12h adds
   friction with little benefit — the LAN threat model is "a device already inside the house".
2. **External is not meaningfully stricter than LAN**, even though internet exposure warrants a
   harder gate.

### The core tension

"No auth on LAN" and "still know who the user is" are contradictory — identity *requires*
authentication. The real lever is **session lifetime + step-up**, not auth on/off: LAN
authenticates once then runs a long session; external gets a separate, shorter challenge.

### Constraint discovered in the existing config

`oauth2-proxy/values.yaml` already serves all three cookie domains from a **single instance**
(`cookie_domains = [".platform.yu-min3.com", ".app.yu-min3.com", ".yu-mins.com"]`), so no second
proxy is needed. But `cookie_expire` is **global** — one instance cannot give the LAN domain a
longer cookie than the external domain. And oauth2-proxy can only keep a session alive as long as
**Keycloak's refresh token / `SSO Session Max`** allows; the current 12h is a Keycloak-side cap,
not merely a cookie setting.

## Decision

### Decided now (shipping)

1. **Inner identity session → 30 days.** Set `cookie_expire = "720h"` with `cookie_refresh` keeping
   the access token fresh. LAN becomes frictionless: one login per device, ~30 days signed in.
   30 days (not 90) bounds the blast radius of a stolen, already-unlocked LAN device.

2. **Keycloak session must be raised to match (follow-up).** `cookie_expire` alone does not deliver
   30 days — Keycloak caps it. The `kensan` realm's `SSO Session Idle` / `SSO Session Max` (and the
   oauth2-proxy client's refresh-token lifetime) must be raised to ~30 days. **Until then the 720h
   cookie simply degrades to the effective Keycloak max (12h) — no regression**, just the LAN-
   frictionless benefit isn't active yet. This is configured in the Keycloak admin console, **not in
   Git** (the realm is not yet managed as code) — a drift surface, tracked as a follow-up.

3. **Inner authz unchanged.** oauth2-proxy stays the single source of in-cluster identity
   (`Authorization: Bearer <id_token>` + `X-Auth-Request-*`), so existing `AuthorizationPolicy`
   `when claims[groups]` rules and app header-trust are untouched.

### Deferred decision — the external-gate model

How internet traffic (`*.yu-mins.com`) is gated is **left open**. The honest framing:
**DDoS absorption and TLS termination come from the Cloudflare Tunnel and are identical in all
options below** — they are not a differentiator. The options:

| Option | External auth | CF Access role | Trade-off |
|---|---|---|---|
| **A. Keep CF Access OTP** *(current)* | CF Access OTP **+** Keycloak at gateway | Independent **2nd factor** (different IdP) | Real 2FA; **double prompt** (UX cost) |
| **B. CF Access → Keycloak OIDC** | Keycloak (once), silent 2nd hop | **Edge gate, not a 2nd factor** | Single prompt; value = attack-surface defense-in-depth + a separate external 24h re-auth cadence |
| **C. Pass-through (drop CF Access)** | Keycloak at gateway only | none | Simplest; **functionally equivalent auth** + same DDoS/TLS; loses defense-in-depth and the separate external cadence; exposes the gateway auth layer to anonymous internet traffic |

**What actually separates B from C** (both authenticate via the same Keycloak):

- **Attack surface / defense-in-depth.** With Access, unauthenticated requests die at Cloudflare's
  edge and never enter the Tunnel; with pass-through they reach the in-cluster Gateway / Envoy /
  oauth2-proxy before being bounced. App backends are not anonymously reachable either way (assuming
  `ext_authz` is correctly applied to every route), and Keycloak's login page is exposed in both
  (it is the IdP). So the protected delta is narrow: **a bug/0-day/misconfig in *our own* Istio /
  oauth2-proxy / AuthorizationPolicy does not become exposure when Access fronts it.**
- **Decoupled external re-auth cadence.** Access can force a 24h external re-challenge independent of
  the 30-day inner cookie. A single global `cookie_expire` cannot express "external 24h, internal
  30d"; Access is the mechanism that delivers it.

The decision therefore hinges on two questions, to be answered later:
**(a)** is a separate external 24h re-auth cadence a requirement? **(b)** how much do we want defense-
in-depth against our own gateway-auth config? If both are "no", **C (pass-through) is the honest,
simplest choice**. If either is "yes", keep Access (B for single-prompt UX, or A to retain an
independent second factor).

Optional refinement for whichever keeps Access: enabling **MFA in Keycloak** gives a strong single
login (with MFA) under option B, recovering a second factor without the OTP double prompt.

## Consequences

### Positive

- LAN: one login per device, then ~30 days frictionless — while every request still carries a
  Keycloak identity (groups claim intact for authz).
- The 720h change is safe to ship independently: it degrades to the current 12h until Keycloak is
  raised, with no regression and no dependency on the deferred gate decision.

### Trade-offs / watch-outs

- A stolen, unlocked device on the LAN stays signed in up to 30 days (acceptable for a homelab; the
  lever is `cookie_expire` + Keycloak session max).
- **Keycloak realm session settings are imperative (admin console), not GitOps** — they will drift
  unless the realm is managed as code. Tracked as a follow-up.
- The external double-prompt persists while option **A (OTP)** stays in place; that is the accepted
  interim cost of leaving the gate decision open.

## Follow-ups

- [ ] **Keycloak** `kensan` realm: raise `SSO Session Idle` / `SSO Session Max` + oauth2-proxy
  client refresh-token lifetime to ~30 days (admin console — activates the 720h cookie).
- [ ] **Decide the external-gate model** (A / B / C above). Until decided, CF Access OTP stays
  (already configured, safe).
- [ ] (Stretch) Manage the Keycloak realm as code to remove the session-setting drift surface.

> Note: creating a CF Access application is **not** a follow-up — Access is already configured (OTP).

## Alternatives considered (rejected during design)

- **Second oauth2-proxy instance** with a short external cookie, selected by host — achieves per-path
  session length without Cloudflare, but adds a proxy + ext_authz provider + policy rule and still
  lacks an edge gate. Rejected in favour of a single inner proxy + (optional) Access as the gate.
- **Bypass auth entirely on LAN** (source-IP / hostname allow) — genuinely "no auth", but LAN
  requests would carry no identity, breaking groups-based authz. Rejected.

## References

- [ADR-010](010-istio-native-oauth2-absent.md) — oauth2-proxy as the Gateway `ext_authz` OIDC front
- [ADR-002](002-authentication-authorization-architecture.md) — original authn/authz architecture
- `kubernetes/auth/oauth2-proxy/values.yaml` — `cookie_expire` / `cookie_refresh` (inner session)
- `kubernetes/network/istio/authorizationpolicy-gateway-platform-oauth2.yaml` — CUSTOM ext_authz hosts
- `.claude/rules/network-ingress.md` — edge architecture (two ingress paths) & Gateway-level auth
