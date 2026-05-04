# ADR-005: Phase 1 Authentication via Istio Native oauth2 + Keycloak

## Status

**Re-evaluation Required (2026-05-05)** — superseded in part by [ADR-010](010-istio-native-oauth2-absent.md).

This ADR's *premise* — that "Istio 1.27 has shipped a stable Gateway API + native `oauth2` extension provider" — is incorrect. The `MeshConfig.ExtensionProvider` proto in Istio 1.27 (and master) does not include any OAuth2/OIDC provider type; the available `oneof` cases are `envoy_ext_authz_http`, `envoy_ext_authz_grpc`, telemetry providers, log providers, and `sds`. See ADR-010 for the verification, the option matrix (Path A: oauth2-proxy via ext_authz / Path B: EnvoyFilter wrapping `envoy.filters.http.oauth2` / Path C: per-service OIDC), and the recommended path.

The phased approach (Phase 1-3), the two-stage authorization architecture, and the Phase 3 multi-layer authorization plan in this ADR all remain valid; only the **mechanism** for the Gateway-level OIDC handshake needs re-selection.

Originally accepted on 2026-05-03; re-evaluation triggered on 2026-05-05 during implementation prep.

## Date

2026-05-03

## Context

ADR-002 designed Phase 2 Gateway-level authentication on the assumption that OAuth2 Proxy would be the ExtAuthz provider, but the manifests have not yet been created (the implementation remains at Phase 1 = no auth).

Subsequently, Istio 1.27 has shipped a stable Gateway API + native `oauth2` extension provider. OIDC + JWT cookie handling can now be completed by Istio alone, without operating OAuth2 Proxy as a separate Pod.

### Environment Verification (2026-05-03)

- **Istio**: 1.27.3 (Argo CD Application `targetRevision: 1.27.3`)
- **Gateway API mode**: enabled (`PILOT_ENABLE_GATEWAY_API: "true"`)
- **Existing Gateways**: `gateway-platform` / `gateway-prod` / `gateway-dev` already defined as Gateway API resources
- => Confirmed that the native `oauth2` ext provider can be used reliably

### Requirements

1. **Minimize additional components**: Single-operator homelab; do not add operational surface
2. **Keycloak as the sole IdP**: Unified SSO experience across Vault / Argo CD / Backstage / Grafana
3. **Two-stage authorization**: Coarse authz at the Gateway (can the user reach this app?) + fine authz inside the app (what can the user do?)
4. **Fail-secure**: Missing authz rules result in "no access" by structure (default-deny base)
5. **JWT forwarding to apps**: Apps can read claims directly

### Patterns Considered

#### Pattern A: OAuth2 Proxy ExtAuthz (Original ADR-002 Assumption)

Place an OAuth2 Proxy Pod in an `auth-system` namespace and route every request through ExtAuthz from Istio.

**Pros:**
- Many production examples; widely known as the AWS ALB + Cognito-style pattern
- Each service can simply read the headers it injects

**Cons:**
- The OAuth2 Proxy Pod itself is a SPOF; replicas + PodDisruptionBudget needed for redundancy
- Adds an `auth-system` namespace and OAuth2 Proxy Helm chart to the operational scope
- Auth no longer completes within Istio + Keycloak; one extra hop is introduced

#### Pattern B: Istio Native `oauth2` Extension Provider (Adopted)

Define a native `oauth2` provider under `meshConfig.extensionProviders` and bind it to a Gateway via `AuthorizationPolicy` with `action: CUSTOM`. Istio handles the OIDC code flow / JWT cookie / token refresh directly.

**Pros:**
- No extra Pod; auth completes within Istio
- The OIDC client secret only needs to live in an Istio-side Secret
- Clean narrative of "authentication completed by Istio + Keycloak"
- Attaches directly to existing Gateway resources (Gateway API `Gateway`)

**Cons:**
- The feature is newer; community know-how and troubleshooting examples are fewer than for OAuth2 Proxy
- Stronger coupling to the Istio version; downgrades require careful behavioral checks

#### Pattern C: Per-Service OIDC

Provision dedicated OIDC clients for Vault / Argo CD / Grafana / Backstage / every app individually.

**Pros:**
- Each service's native authorization features (Argo CD RBAC, Grafana role mapping, etc.) can be fully utilized

**Cons:**
- Per-app OIDC config burden grows linearly
- Application teams must implement OIDC every time, expanding the AD (Application Developer) responsibility surface

## Decision

**Adopt Pattern B (Istio native `oauth2` extension provider).**

In addition, fix the following.

### 1. Two-Stage Authorization Architecture

- **Gateway-level (coarse authz)**: `AuthorizationPolicy` in `istio-system` evaluates host x group claim. Default-deny as the base, enumerate explicit ALLOW only
- **Inside the app (fine authz)**: Forward JWT to the app with `forwardOriginalToken: true`; the app's OIDC library decodes claims to implement detailed authorization (admin button toggling, etc.)

### 2. JWT Forwarding Configuration

```yaml
apiVersion: security.istio.io/v1
kind: RequestAuthentication
metadata:
  name: keycloak-jwt
  namespace: istio-system
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway-prod
  jwtRules:
  - issuer: "https://auth.platform.yu-min3.com/realms/kensan"
    jwksUri: "https://auth.platform.yu-min3.com/realms/kensan/protocol/openid-connect/certs"
    forwardOriginalToken: true
```

### 3. default-deny + Explicit ALLOW Gateway-level AuthorizationPolicy

```yaml
# default-deny base (no rules = deny all)
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-prod-deny-all
  namespace: istio-system
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway-prod
---
# Explicit ALLOW per host x groups
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-prod-allow
  namespace: istio-system
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: gateway-prod
  action: ALLOW
  rules:
  - to:
    - operation:
        hosts: ["streamlit.app.yu-min3.com"]
    when:
    - key: request.auth.claims[groups]
      values: ["app-team-a", "platform-admin"]
```

### 4. Workload-level AuthorizationPolicy is Phase 2 or Later

Per-app namespace `AuthorizationPolicy` is not required in Phase 1. Add it only for high-security apps that need defense-in-depth in Phase 2 or later.

### 5. Relationship to ADR-002

- **The "Phase 2: Gateway-level authentication via OAuth2 Proxy" portion of ADR-002 is replaced by this ADR.** OAuth2 Proxy Helm chart deployment and the `auth-system` namespace will not be created
- ADR-002's Phase 3 (per-service fine-grained authorization) remains valid. Because Istio native `oauth2` forwards JWT via `forwardOriginalToken: true`, the Phase 3 per-service authz for Backstage / Argo CD / Grafana works on top unchanged
- ADR-002 as a whole is **not Superseded** (only the Phase 2 implementation method changes; the phased approach and Phase 3 multi-layer authz are retained)

## Consequences

### Positive

- No need to operate a separate Pod (OAuth2 Proxy). Auth completes with Istio config + Keycloak realm config alone
- Default-deny base + explicit ALLOW makes missing authz rules result in "no access" so the gap is noticed (fail-secure)
- Apps only need to receive `Authorization: Bearer <JWT>`; no per-app OIDC implementation required
- Vault / Argo CD / Backstage / Grafana / general apps - 5 systems unified under Keycloak-based SSO

### Trade-offs

- Istio native `oauth2` provider has fewer community examples than OAuth2 Proxy; investigation cost during incidents may rise
- When upgrading Istio, extension provider config compatibility must be verified (OAuth2 Proxy could be decoupled from Istio)
- A full Keycloak outage makes all 5 systems inaccessible. Break-glass paths (Vault userpass / Argo CD built-in admin / Grafana local admin / Backstage local user) must be maintained separately

## References

- ADR-002: Phased Implementation of Authentication and Authorization Architecture (Phase 2 implementation method replaced by this ADR)
- ADR-006: Application Namespace Naming Convention (the `kensan-lab.platform/team` label is referenced from AuthorizationPolicy)
- ADR-008: Keycloak DB Credentials Are Not Moved to Vault (the boring-keeping policy for Keycloak itself)
- Design source: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § Application authentication/authorization flow (Istio + Keycloak)
- [Istio: External Authorization](https://istio.io/latest/docs/tasks/security/authorization/authz-custom/)
- [Istio 1.27 Release Notes](https://istio.io/latest/news/releases/1.27.x/)
