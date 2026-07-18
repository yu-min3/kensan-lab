# oauth2-proxy

The implementation of Path A, adopted in ADR-010. The Istio Gateway (`gateway-platform`) delegates protected hosts under `*.platform.yu-min3.com` to oauth2-proxy via `envoy_ext_authz_http`; oauth2-proxy handles the entire OIDC redirect / cookie / refresh flow.

See [ADR-010](../adr/010-istio-native-oauth2-absent.md) for the full rationale and the comparison against Path B/C.

## Components

| Item | Value | Notes |
|---|---|---|
| Chart | `oauth2-proxy/oauth2-proxy 10.4.3` (appVersion 7.15.2) | |
| Namespace | `auth-system` | |
| Replicas | 2 | High availability while `failOpen=false` |
| nodeSelector | `kubernetes.io/arch: amd64` | Pinned to m4neo. The Pi 5 fleet isn't a good fit for load-balancing OIDC redirects |
| Provider | `keycloak-oidc` | realm `kensan` |
| Cookie domain | `.platform.yu-min3.com` | One session shared across every platform host (the core of SSO) |

## Architecture

```
[browser]
    ↓ HTTPS
[Istio gateway-platform]
    ↓ ext_authz (HTTP)
[oauth2-proxy] ←→ [Keycloak realm "kensan"]
    ↓ allow (200) or deny (302 to login)
[Istio backend route]
```

- Registered as `envoy_ext_authz_http` via Istio's `meshConfig.extensionProviders[oauth2-proxy]` (`kubernetes/network/istio/istiod/values.yaml`)
- On successful auth, oauth2-proxy must return a plain 200 OK and stop there, so `upstreams = ["static://200"]` is set. Istio's ext_authz only treats an HTTP 200 as "allow" (a 202 counts as deny)

## Failure behavior

Because Istio is configured with `failOpen: false`, protected hosts (Backstage / Hubble / Prometheus / Longhorn, etc.) return 503 if oauth2-proxy goes down. Grafana bypasses this entirely (app-native auth) and is outside the CUSTOM ext_authz's scope, so it's unaffected by an oauth2-proxy outage (this bypass exists specifically to avoid Grafana misreading oauth2-proxy's injected Bearer token as its own API key and returning 403 — it's passed straight through via Category 1 in `authorizationpolicy-gateway-platform-allow.yaml`).

- Mitigated by 2 replicas + a PodDisruptionBudget (`minAvailable: 1`)
- Deliberately avoiding scheduling onto Pi 5 nodes: redirect / cookie validation benefits from AMD64 performance, and being exposed to the Pi 5 fleet's network instability ([cilium-wifi-stability](../runbooks/cilium-wifi-stability.md)) would take down authentication for the whole platform

## Scopes and claims

`scope = "openid email profile"`. `groups` is a claim name, not a scope, and is deliberately excluded from scope (Keycloak rejects it with "Invalid scopes: groups").

The groups claim is auto-attached to the id_token by an `oidc-group-membership-mapper` on the `istio-gateway-platform` client, with `id.token.claim=true`.

## CLI coexistence (skip_jwt_bearer_tokens)

Requests that already carry a Bearer token (e.g. an API call made after `vault login -method=oidc` or `argocd login --sso`) pass straight through via `skip_jwt_bearer_tokens = true`. This avoids layering an oauth2-proxy cookie session on top of an already-authenticated request.

`extra_jwt_issuers = ["https://auth.platform.yu-min3.com/realms/kensan=istio-gateway-platform"]` allowlists the issuer.

## Chart gotchas (implementation notes)

- The chart prepends `image.registry` (default `quay.io`) to the repository. Writing `quay.io/...` into `image.repository` double-prefixes it. Just `oauth2-proxy/oauth2-proxy` is correct.
- The chart's top-level `securityContext:` maps to the container level (per the chart's values schema). `fsGroup` is pod-level only, so it needs to go under `podSecurityContext`.
- Passing `configFile` causes the `emailDomains` chart value to be ignored (`configFile` takes precedence). Both need to be set.

## Related

- [ADR-010](../adr/010-istio-native-oauth2-absent.md): discovering the absence of Istio-native OAuth2 and adopting Path A
- [Gateway OIDC](./gateway-oidc.md): the overall operations guide
