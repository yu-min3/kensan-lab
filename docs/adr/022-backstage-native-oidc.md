# ADR-022: Backstage uses native OIDC

## Status

**Accepted** (2026-08-11). Supersedes the Backstage-specific proxy-provider
decision in `docs/auth/backstage-sso-design.md`. ADR-010 remains valid for apps
without useful native authentication, such as Prometheus, Hubble and Longhorn.

## Context

Backstage was first integrated through the shared Gateway oauth2-proxy. That
established a valid Catalog identity, but required three credentials and two
authorization planes on every API request: an oauth2-proxy cookie, a Keycloak
access token in `X-Auth-Request-Access-Token`, and a Backstage token in
`Authorization`.

The implementation exposed avoidable coupling between Istio ext_authz,
oauth2-proxy response headers and Backstage's own token verification. Backstage,
unlike a header-only UI, already provides an OIDC module, identity resolvers and
application tokens. Argo CD and Grafana successfully use the same app-native
pattern in this cluster.

## Decision

Backstage authenticates directly with the Keycloak `kensan` realm using a
dedicated confidential client named `backstage`.

1. Gateway treats both Backstage hosts as Category 1 app-native auth and does
   not call oauth2-proxy for them.
2. Backstage's official `oidc` auth provider performs Authorization Code flow
   with Keycloak.
3. `emailMatchingUserEntityProfileEmail` maps the verified email claim to the
   existing `user:default/yu` Catalog entity. Unregistered users fail closed.
4. Backstage issues and verifies its own token for Catalog, Search and other
   plugin APIs.
5. The Keycloak client secret and Backstage auth session secret live at Vault
   KV `secret/backstage/oidc` and reach the Pod through External Secrets.
6. Local development keeps the guest provider; production does not configure
   a guest endpoint.

```text
Browser -> Backstage -> Keycloak OIDC
                    <- verified OIDC identity
        -> Catalog email resolver -> user:default/yu
        <- Backstage token -> plugin APIs
```

## Alternatives considered

| Decision | Alternative | Reason |
|---|---|---|
| **Adopted** | Backstage native OIDC | Backstage owns identity and application tokens; matches Argo CD/Grafana and removes trusted identity headers |
| **Rejected** | Keep oauth2-proxy provider | Working, but couples application auth to Gateway headers and leaves two token planes in one request path |
| **Rejected** | Run both Gateway oauth2-proxy and native OIDC | Adds two browser auth layers without adding a useful boundary for this single-user deployment |
| **Deferred** | Keycloak groups to Backstage RBAC | Permission policy and group synchronization are separate from authentication |

## Consequences

- Backstage is not affected by an oauth2-proxy outage.
- A dedicated Keycloak client, callback URI and secret must be managed.
- Keycloak SSO still avoids a second credential prompt after signing into
  Argo CD or Grafana, although each app maintains its own application session.
- Gateway no longer enforces `platform-admin` / `platform-dev` for Backstage.
  The Catalog resolver is the current allowlist; Permission Framework RBAC is
  the next authorization layer if more users are added.
- The external host is registered as an allowed callback, while the current
  Backstage deployment keeps the LAN hostname as its canonical `baseUrl`.

## Rollback

Revert the application image/config and the two Gateway policy edits together.
The shared oauth2-proxy client remains available and no stateful Backstage data
is changed by this migration.
