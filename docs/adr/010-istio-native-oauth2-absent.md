# ADR-010: Istio Native OAuth2 Provider Does Not Exist — Path Re-selection Required

## Status

**Proposed** — pending Yu's decision among Paths A / B / C below.

This ADR supersedes the **architectural premise** of ADR-005, which assumed Istio 1.27 had a "native `oauth2` extension provider". That premise turns out to be incorrect. The phased approach and two-stage authorization model in ADR-005 remain valid; only the implementation mechanism for Gateway-level OIDC needs re-selection.

## Date

2026-05-05

## Context

ADR-005 (2026-05-03) decided to use "Istio's native `oauth2` extension provider" to perform Gateway-level OIDC authentication, rejecting the OAuth2 Proxy approach from ADR-002 because:

> "Istio 1.27 has shipped a stable Gateway API + native `oauth2` extension provider. OIDC + JWT cookie handling can now be completed by Istio alone, without operating OAuth2 Proxy as a separate Pod."

### Verification (2026-05-05)

While preparing the implementation, the proto definition of `MeshConfig.ExtensionProvider` was inspected directly from the Istio API repository:

```bash
curl -s https://raw.githubusercontent.com/istio/api/release-1.27/mesh/v1alpha1/config.proto \
  | grep -E "Provider [a-z]+ = [0-9]+;"
```

The available `oneof provider` types in `release-1.27` are:

| Field | Purpose |
|---|---|
| `envoy_ext_authz_http` | External authorization via HTTP service (e.g. oauth2-proxy) |
| `envoy_ext_authz_grpc` | External authorization via gRPC service |
| `zipkin` / `datadog` / `stackdriver` / `skywalking` / `opentelemetry` | Tracing |
| `prometheus` | Metrics |
| `envoy_file_access_log` / `envoy_http_als` / `envoy_tcp_als` | Access logging |
| `sds` | Secret Discovery Service |

**There is no `oauth2` / `oidc` / `envoy_oauth2` provider type.** The same is true for `master`, `release-1.28`, and `release-1.26` branches.

The Istio reference docs (`istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/`) confirm the same set.

### What Was Likely Meant by "Native OAuth2"

There are two plausible features that were probably conflated:

1. **Envoy's `envoy.filters.http.oauth2` HTTP filter** — exists in Envoy proper, can be inserted into Istio via the `EnvoyFilter` CRD. NOT a `MeshConfig.ExtensionProvider`. Currently flagged by Envoy docs as "currently under active development" and lacks OIDC discovery URL support (token/auth endpoints must be configured by hand).

2. **`RequestAuthentication` + `AuthorizationPolicy`** — these are GA and stable, but they only do JWT *verification* and authorization. They cannot drive an OIDC redirect/cookie flow. A user without a JWT cannot be redirected to Keycloak by these resources alone.

Neither of these is a "drop-in" replacement for OAuth2 Proxy.

## Requirements (unchanged from ADR-005 §Context)

1. Minimize additional components
2. Keycloak as the sole IdP for unified SSO
3. Two-stage authorization: coarse at Gateway, fine in app
4. Fail-secure: missing rules → no access by default
5. JWT forwarding to apps (apps can read claims)

## Paths Considered

### Path A: oauth2-proxy via `envoy_ext_authz_http` (Recommended)

Place oauth2-proxy in `auth-system` namespace, register it as a `MeshConfig.ExtensionProvider` of type `envoy_ext_authz_http`, bind via `AuthorizationPolicy` with `action: CUSTOM`. oauth2-proxy handles the OIDC redirect, cookie, and refresh; Istio enforces the gate.

**Pros**
- The documented Istio pattern (Istio docs explicitly cover ext_authz with oauth2-proxy)
- oauth2-proxy is the K8s ecosystem standard (homelab references like khuedoan / ricsanfre / onedr0p use it widely)
- Native OIDC discovery URL support, mature refresh token handling, CSRF protection
- `envoy_ext_authz_http` is GA in Istio (stable proto)
- The "Enterprise Platform Engineering reference" framing benefits from a production-realistic pattern (AWS ALB + Cognito K8s analog)

**Cons**
- Adds one Pod in `auth-system` namespace (mitigated by replicas + PDB)
- Adds an oauth2-proxy Helm chart to operational scope
- Auth no longer "completes within Istio"; one extra hop (Gateway → ext_authz → Keycloak)

### Path B: `EnvoyFilter` Wrapping `envoy.filters.http.oauth2`

Use Istio's `EnvoyFilter` escape hatch to inject Envoy's native `oauth2` HTTP filter into the gateway-platform listener. Envoy handles the redirect/cookie directly.

**Pros**
- No additional Pod; closer to ADR-005's "auth completes within Istio" intent
- Refresh token + single logout supported in the Envoy filter

**Cons**
- The Envoy `oauth2` filter is flagged "currently under active development" — not a stability guarantee suitable for production-aligned homelab
- No OIDC discovery URL support — `authorization_endpoint` and `token_endpoint` must be hardcoded (drift risk vs Keycloak)
- `EnvoyFilter` is Istio's brittlest API surface — known to break across Istio minor upgrades, especially when filter chain ordering changes
- CSRF protection requires a separate Envoy filter; HTTPS-only (acceptable for our case)
- Few production references; debugging requires Envoy-level expertise

### Path C: Per-Service OIDC, Skip Gateway-level OIDC

Keep gateway-level Authorization at host/IP level (or none). Each service does its own OIDC: Argo CD, Grafana, Backstage, Vault all have native OIDC. Hubble/Prometheus/Longhorn UI guarded by NetworkPolicy + intra-LAN only or basic auth.

**Pros**
- Zero implementation cost; effectively "stay at ADR-002 Phase 1"
- Each service uses its native authz best
- No extra components

**Cons**
- SSO experience is fragmented (re-login per service)
- No fail-secure default-deny at the edge
- Services without OIDC support (Hubble / Prometheus / Longhorn UI) need ad-hoc protection
- The "Istio + Keycloak で SSO 完結" homelab differentiator is lost

## Decision (Pending)

**Recommended: Path A.** Rationale:
- Path B's Envoy filter stability disclaimer ("under active development") is a real concern for any production-aligned setup, even a homelab
- The "extra Pod" cost in Path A is one Helm chart and one namespace — small in absolute terms vs the differentiation Path A buys
- Path C abandons the unified SSO story that ADR-002/005 explicitly chose
- ADR-005's "no extra Pod" constraint was framed as a *consequence* (positive), not a requirement; the underlying requirement (unified SSO without per-service OIDC plumbing) is fully satisfied by Path A

**Yu's decision is required before any of the path-specific YAML in this PR is bound to a live Gateway.**

## Implementation in This PR

This PR commits only **path-independent** scaffolding:

1. Keycloak `setup.sh` extended with a parameterized OIDC client function. The `istio-gateway-platform` client is added but with `redirectUris` left as a placeholder (`__TBD_BY_PATH__`) so the client cannot complete a redirect until Yu picks a path and amends.
2. `RequestAuthentication` and `AuthorizationPolicy` template YAML in `infrastructure/network/istio/oidc-foundation/` — but **NOT included in any ArgoCD `Application` source path**. They are reference scaffolding that must be moved into a synced location (or pointed to by a new Application CR) once the path is chosen.
3. ADR-010 (this file) and ADR-005 status update.

Nothing in this PR causes runtime behavior change.

## Consequences

**If Path A is chosen** (recommended):
- One follow-up PR adds `infrastructure/security/oauth2-proxy/` Helm multi-source app
- Existing `meshConfig.extensionProviders` entry needs to be added (this PR includes a draft commented-out version in `infrastructure/network/istio/istiod/values.yaml`)
- The OIDC foundation YAML moves into a synced path (e.g., `infrastructure/network/istio/resources/`) and `targetRef` is bound to `gateway-platform`
- `istio-gateway-platform` Keycloak client gets `redirectUris=[https://oauth2-proxy.platform.yu-min3.com/oauth2/callback]` (or the chosen oauth2-proxy hostname)

**If Path B is chosen**:
- One follow-up PR adds an `EnvoyFilter` resource and the OIDC client `redirectUris` is `[https://<each-host>.platform.yu-min3.com/oauth2/callback]` per protected host
- `meshConfig.extensionProviders` is not used

**If Path C is chosen**:
- This PR's foundation YAML is removed (or kept as documentation)
- Per-service OIDC is configured separately for ArgoCD / Grafana / Backstage / Vault
- Hubble / Prometheus / Longhorn UI gain ad-hoc protection (basic auth or LAN-only NetworkPolicy)
- ADR-005 is marked **Superseded** by ADR-010, with this ADR documenting the actual decision

## References

- ADR-002: Phased Implementation of Authentication and Authorization Architecture
- ADR-005: Phase 1 Authentication via Istio Native oauth2 + Keycloak (premise revision required)
- Istio API repo (release-1.27 branch): `mesh/v1alpha1/config.proto` `ExtensionProvider` oneof
- Envoy docs: [HTTP OAuth2 filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/oauth2_filter)
- Istio docs: [External Authorization task](https://istio.io/latest/docs/tasks/security/authorization/authz-custom/)
- oauth2-proxy: [official Istio integration guide](https://oauth2-proxy.github.io/oauth2-proxy/configuration/integration#configuring-for-use-with-the-istio-ingress-gateway)
