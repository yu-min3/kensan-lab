# Backstage SSO implementation design

> **Superseded (2026-08-11):** This document records the original oauth2-proxy
> implementation. [ADR-022](../adr/022-backstage-native-oidc.md) replaces it
> with Backstage native OIDC after production validation exposed unnecessary
> coupling between Gateway identity headers and Backstage application tokens.

## Decision

No Backstage-specific OIDC client is created. The existing **Istio Gateway +
oauth2-proxy + Keycloak** path is reused as the authentication entry point, and
the verified identity headers it produces are handed to Backstage's
`oauth2Proxy` auth provider.

The goal of this implementation is that Backstage identifies its user as
`user:default/yu` rather than `guest`, and that the dangerous auth bypass in
production is removed. Per-operation RBAC through the Permission Framework is a
later phase; it is deliberately not enabled at the same time.

| Verdict | Approach | Reasoning |
|---|---|---|
| **Adopted** | oauth2-proxy header trust + Backstage proxy provider | Reuses the existing SSO session and needs no new client secret. Consistent with ADR-002 / ADR-010 and the current Gateway design |
| **Rejected** | A dedicated Keycloak OIDC client for Backstage | Adds a second OAuth callback, session, and secret to operate, and reverses the decision to centralise authentication at the Gateway |
| **Rejected** | Keep `guest` and rely on Gateway authentication alone | Controls reachability, but leaves Backstage with no user identity, no audit trail, and no basis for future authorisation |
| **Deferred** | Permission Framework + group-based RBAC | Introducing identity and changing authorisation together makes an incident hard to bisect. It follows once SSO is stable |

## Current state and problems

The Gateway already enforces oauth2-proxy on both the LAN and external Backstage
hosts, admitting only Keycloak's `platform-admin` or `platform-dev` groups. On
success it passes `X-Auth-Request-User`, `X-Auth-Request-Email`, and
`X-Auth-Request-Groups` upstream.

Inside Backstage, however, the following interim state applies.

| Severity | Problem | Evidence | Impact |
|---|---|---|---|
| 🔴 **Critical** | The guest provider is permitted in production | `backstage/app/app-config.kubernetes.yaml` | Every user shares one identity, so nothing can be audited or authorised individually |
| 🔴 **Critical** | The default backend auth policy is disabled globally | `dangerouslyDisableDefaultAuthPolicy` in the same file | Gateway reachability control and the Backstage plugin API's own auth boundary are not separated |
| 🟠 **High** | The frontend is pinned to the guest provider | `backstage/app/packages/app/src/App.tsx` | The oauth2-proxy identity cannot be converted into a Backstage session |
| 🟠 **High** | No real user exists in the catalog | `backstage/app/catalog/organizations/teams.yaml` | The email resolver cannot resolve `user:default/yu` |
| 🟡 **Medium** | The Permission Framework is disabled | Production config | Even after identity lands, every user has the same rights. Stated here as an explicit non-goal |

## Target architecture

```text
Browser
  │  shared SSO cookie
  ▼
Istio gateway-platform
  │  ext_authz check
  ▼
oauth2-proxy ───────────────► Keycloak realm: kensan
  │  X-Auth-Request-Email: ymisaki00@gmail.com
  │  X-Auth-Request-Groups: platform-admin
  ▼
Backstage oauth2Proxy provider
  │  emailMatchingUserEntityProfileEmail
  ▼
Catalog User: user:default/yu
  │
  ▼
Backstage token / plugin API identity
```

The trust boundary is split in two.

1. The Gateway decides, from the Keycloak group, whether this user may reach
   Backstage at all.
2. Backstage resolves the verified email header to a catalog user, and thereby
   expresses who is performing an operation.

`X-Auth-Request-*` headers are not signed credentials. The design therefore
assumes the Backstage Service is never exposed directly and that arbitrary
headers cannot be injected from anywhere but the Gateway. The existing
ClusterIP, Gateway route, NetworkPolicy, and Istio sidecar are kept as that
boundary, and acceptance testing confirms there is no direct path in.

## Identity mapping

The initial implementation uses email as the stable key.

| Source | Value | In Backstage |
|---|---|---|
| Keycloak `email` claim | `ymisaki00@gmail.com` | Matched against `spec.profile.email` |
| Keycloak username | `yu` | Display and diagnostics only; never the resolution key |
| Keycloak group | `platform-admin` / `platform-dev` | Used for Gateway admission. Not synchronised into Backstage groups by this change |
| Catalog user | `user:default/yu` | The subject of Backstage identity |

The `emailMatchingUserEntityProfileEmail` resolver is used, with a real user
entity carrying a matching email added statically to the catalog. Tidying up the
demo users can be a separate change; it is not a prerequisite for the SSO
cutover.

Resolvers that mint a fixed user name straight from a header, or that allow
sign-in without a matching catalog entity, are not used. The catalog is the
source of truth for the identity inventory, so a wrong email or an unregistered
user fails closed at sign-in.

## Change design

### Backstage application

| Target | Change |
|---|---|
| `packages/backend/package.json` | Add `@backstage/plugin-auth-backend-module-oauth2-proxy-provider` on the same Backstage release line. The guest module stays, scoped to local development |
| `packages/backend/src/index.ts` | Register the oauth2-proxy provider and add a profile transform that reads Istio ext_authz's `X-Auth-Request-*` headers |
| `packages/app/src/App.tsx` | Replace automatic `guest` sign-in with `oauth2Proxy`. A user already signed in at the Gateway gets a Backstage session with no extra UI |
| `app-config.kubernetes.yaml` | Configure `auth.providers.oauth2Proxy` and the email resolver; remove the production guest provider |
| `app-config.kubernetes.yaml` | Remove `dangerouslyDisableDefaultAuthPolicy` and restore Backstage's default plugin auth policy |
| `catalog/organizations/teams.yaml` | Add `user:default/yu` with the real email and `platform-engineering` membership |

Local development has no Gateway headers, so the guest provider is configured
only in `app-config.development.yaml`. The production image does not load that
config, and the frontend uses `ProxiedSignInPage` on production hosts. The guest
module's code may therefore ship in the bundle without a guest provider endpoint
existing in production.

When oauth2-proxy is used as Istio's ext_authz at `/oauth2/auth`, the
authentication result comes back in **response** headers such as
`X-Auth-Request-Email`. Backstage's official provider ships a default profile
transform that reads the reverse-proxy style `X-Forwarded-Email`, so kensan-lab
reuses the official authenticator but substitutes a profile transform for the
`X-Auth-Request-*` shape.

### Platform manifests

After sign-in, the Backstage plugin API uses a bearer token Backstage issued
itself. If the shared ext_authz provider sets a Keycloak token in
`Authorization`, it overwrites that token. The Keycloak token is therefore
verified at the Gateway through a dedicated header, and `Authorization` keeps
the original application token on every host.

| Target | Decision |
|---|---|
| oauth2-proxy Keycloak client / secret | **Unchanged.** The existing `istio-gateway-platform` client is shared |
| Istio `headersToUpstreamOnAllow` | **Identity headers only.** `Authorization` keeps the original application token |
| Istio `includeRequestHeadersInCheck` | **Session verified by cookie.** The application `Authorization` header is not passed to oauth2-proxy |
| Gateway JWT verification | `X-Auth-Request-Access-Token` is verified against JWKS, preserving the existing group allowlist |
| Backstage users | Allowlisted twice: by the Gateway's admin/dev rule, and again by the catalog user resolver |
| Workload `RequestAuthentication` | **Removed.** The Keycloak token is verified at the Gateway through its dedicated header, and the workload forwards the Backstage token to the backend |
| Backstage ExternalSecret | **Unchanged.** No dedicated client secret is needed |
| Backstage image | Retagged after the application build. `latest` is not used |

## Request flow

1. The browser opens Backstage.
2. Istio performs an ext_authz check against oauth2-proxy's `/oauth2/auth`.
3. With no session, oauth2-proxy redirects to Keycloak and sets the shared
   cookie after authentication.
4. The shared ext_authz provider verifies the session and overwrites the
   identity headers.
5. The Gateway verifies the groups in the access-token header, and leaves the
   browser's `Authorization` header untouched.
6. The Backstage oauth2Proxy provider matches the email header to a catalog user
   and issues a Backstage token.
7. Frontend and backend plugins share the user identity through that token.

## Staged rollout

| Phase | Change | Gate | Rollback |
|---|---|---|---|
| 0 | Add the real user entity to the catalog | Entity and email visible through the catalog API | Revert the entity |
| 1 | Add the proxy provider, replace the production guest provider | `/api/auth/oauth2Proxy/refresh` returns an identity | Revert to the previous image tag and guest config |
| 2 | Restore the default backend auth policy | Catalog, Search, Scaffolder, and TechDocs APIs succeed | Temporarily restore the bypass — never as a steady state |
| 3 | Remove the old guest dependency after E2E and operational checks | 24 hours of normal use with no auth errors | Return to the Phase 1 image |

Because the platform is GitOps-managed, every runtime change goes through a Git
commit and an Argo CD sync. Application names do not change, and PostgreSQL and
its PVC are not touched.

## Acceptance criteria

### Functional

- A user with a Keycloak session opens Backstage with no additional login form.
- User Settings and the Backstage identity API return `user:default/yu`.
- `ownershipEntityRefs` contains `group:default/platform-engineering`.
- Representative operations in Catalog, Search, Scaffolder, TechDocs, and
  Notifications succeed.
- The LAN host and the Cloudflare Tunnel host resolve to the same identity.

### Security

- An unauthenticated browser is redirected to Keycloak.
- An email not registered as a catalog user is rejected by the Backstage sign-in
  resolver.
- Sign-in through a production guest endpoint is not possible.
- A missing email header, or one with no catalog match, fails sign-in closed.
- Spoofing `X-Auth-Request-Email` from outside is overwritten or rejected by the
  Gateway and oauth2-proxy; it cannot produce a different user.
- `dangerouslyDisableDefaultAuthPolicy` is absent from the production config.
- No token, client secret, or cookie secret has been added to Git.

### Availability and regression

- An oauth2-proxy outage still fails closed with a 503, as it does today.
- Backstage health probes and internal plugin-to-plugin calls still succeed once
  the default auth policy is restored.
- The frontend's Backstage bearer token is not overwritten at the Gateway and
  reaches the plugin API.
- The workload sidecar does not reject the Backstage token as an unknown issuer.
- A Backstage session can be re-established after a restart and after an
  oauth2-proxy cookie refresh.

## Observability

Authentication failures are isolated at these boundaries.

| Symptom | Boundary | What to look at |
|---|---|---|
| 302 loop / 503 | Gateway → oauth2-proxy | oauth2-proxy logs, ext_authz metrics, cookie domain |
| 403 at the Gateway | Gateway AuthorizationPolicy | The CUSTOM/ALLOW policy, host category |
| Sign-in resolver error | Backstage auth backend | Presence of the email header, catalog user email |
| Plugin API 401 | Backstage backend auth | Backstage token, service-to-service auth, presence of a workload JWT policy |

Authentication header values and token bodies are kept out of normal logs. Even
when diagnosing, email is logged minimally, and access tokens, cookies, and
authorization headers are never recorded.

## Non-goals

- Automatic synchronisation of Keycloak groups into Backstage group entities
- Implementing Permission Framework RBAC policy
- Creating a dedicated Backstage OIDC client or secret
- Changing the Keycloak realm session policy
- Changing the authentication method of oauth2-proxy or the Gateway as a whole

## Open questions

Nothing here blocks implementation. The initial identity can proceed on the
assumption that the existing Keycloak user `yu` and the email
`ymisaki00@gmail.com` map to `user:default/yu`.

When RBAC is taken up later, a choice is needed for how `platform-admin` and
`platform-dev` reach Backstage catalog groups: static management, the Keycloak
catalog provider, or a bespoke synchronisation.

## References

- [Backstage: OAuth2 Proxy provider](https://backstage.io/docs/auth/oauth2-proxy/provider/)
- [Backstage: Sign-in identities and resolvers](https://backstage.io/docs/auth/identity-resolver/)
- [Backstage: Default auth policy](https://backstage.io/docs/auth/service-to-service-auth/)
- [ADR-002: Authentication and Authorization Architecture](../adr/002-authentication-authorization-architecture.md)
- [ADR-010: oauth2-proxy ext_authz](../adr/010-istio-native-oauth2-absent.md)
- [Gateway OIDC operation guide](gateway-oidc.md)
