# oauth2-proxy

The implementation of Path A adopted in ADR-010. The Istio Gateway (`gateway-platform`) delegates the protected hosts under `*.platform.yu-min3.com` to oauth2-proxy via `envoy_ext_authz_http`; oauth2-proxy handles the entire OIDC redirect / cookie / refresh lifecycle.

For the full adoption story and the Path B/C evaluation, see [ADR-010](../adr/010-istio-native-oauth2-absent.md).

## Configuration

| Item | Value | Notes |
|---|---|---|
| Chart | `oauth2-proxy/oauth2-proxy 10.4.3` (appVersion 7.15.2) | |
| Namespace | `auth-system` | |
| Replicas | 2 | availability under failOpen=false |
| nodeSelector | `kubernetes.io/arch: amd64` | pinned to the amd64 node; the Pi5s are a poor fit for OIDC redirect load |
| Provider | `keycloak-oidc` | realm `kensan` |
| Cookie domain | `.platform.yu-min3.com` | one session shared across all platform hosts (the core of SSO) |

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

- Registered as `envoy_ext_authz_http` via Istio `meshConfig.extensionProviders[oauth2-proxy]` (`kubernetes/network/istio/istiod/values.yaml`)
- On successful auth, oauth2-proxy must terminate the check with a plain 200 OK, hence `upstreams = ["static://200"]`. Istio ext_authz treats only HTTP 200 as "allow" (a 202 counts as deny)

## Failure behavior

With `failOpen: false` (set on the Istio side), an oauth2-proxy outage makes the protected hosts (Backstage / Hubble / Prometheus / Longhorn etc.) return 503. Grafana is a bypass (app-native auth) outside the CUSTOM ext_authz, so it is unaffected by an oauth2-proxy outage (it is passed through in Category 1 of `authorizationpolicy-gateway-platform-allow.yaml` to avoid Grafana misreading oauth2-proxy's Bearer header as an API key and returning 403).

- Mitigated with replicas 2 + PodDisruptionBudget (`minAvailable: 1`)
- Spreading onto the Pi 5 nodes is deliberately avoided: redirect / cookie validation benefits from AMD64 performance, and inheriting Pi 5 network instability ([cilium-wifi-stability](../runbooks/cilium-wifi-stability.md)) would take down authentication as a whole

## Scopes and claims

`scope = "openid email profile"`. `groups` is a claim name, not a scope — do not add it to the scope list (Keycloak rejects it with `Invalid scopes: groups`).

The groups claim is added to the id_token automatically by the `oidc-group-membership-mapper` attached directly to the `istio-gateway-platform` client, with `id.token.claim=true`.

## CLI coexistence (skip_jwt_bearer_tokens)

Requests that arrive with their own Bearer token (`vault login -method=oidc`, API calls after `argocd login --sso`, etc.) pass straight through thanks to `skip_jwt_bearer_tokens = true` — no second cookie session is layered on top.

`extra_jwt_issuers = ["https://auth.platform.yu-min3.com/realms/kensan=istio-gateway-platform"]` allowlists the issuer.

## Chart pitfalls (implementation notes)

- The chart prepends `image.registry` (default `quay.io`) to the repository. Writing `quay.io/...` in `image.repository` produces a double prefix — write only `oauth2-proxy/oauth2-proxy`
- The chart's top-level `securityContext:` maps to the *container* (chart values quirk). `fsGroup` is pod-level only, so it must go in `podSecurityContext`
- When `configFile` is provided, the `emailDomains` chart value is ignored (configFile wins). Both must be written

## Related

- [ADR-010](../adr/010-istio-native-oauth2-absent.md): discovering the absence of Istio-native OAuth2 and adopting Path A
- [Gateway OIDC](./gateway-oidc.md): the end-to-end operations guide
