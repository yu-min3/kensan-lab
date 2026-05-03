# ADR-002: Phased Implementation of Authentication and Authorization Architecture

## Status

Accepted - The Phase 2 implementation method is partially replaced by ADR-005

## Date

2025-11-08

## Change History

- **2026-05-03**: The implementation method for Phase 2 ("Gateway-level authentication via OAuth2 Proxy") is replaced by [ADR-005: Phase 1 Authentication via Istio Native oauth2 + Keycloak](005-istio-native-oauth2.md). Istio 1.27's native `oauth2` extension provider became available, eliminating the need to operate OAuth2 Proxy as a separate Pod. The phased approach (Phase 1-3) and Phase 3 multi-layer authorization in this ADR remain valid.

## Context

Access control needs to be implemented for platform infrastructure services (Backstage, Argo CD, Grafana, Prometheus, Hubble UI, Keycloak, etc.).

### Requirements

1. **Phased adoption**: Start with no authentication in early development, gradually strengthen auth
2. **Unified authentication platform**: Unified authentication experience centered on Keycloak
3. **Service-specific handling**: Authentication methods suited to each service's characteristics (SPA, API, UI-only)
4. **Future extensibility**: Support for fine-grained permission control (RBAC)
5. **Operability**: Easy authentication setup when adding new services

### Platform Service Inventory

| Service | URL | Service Characteristics | Authorization Needs |
|---------|-----|------------|------------|
| Keycloak (Prod) | keycloak.platform.your-org.com | Auth server | None (self-authenticating) |
| Keycloak (Dev) | keycloak-dev.platform.your-org.com | Auth server | None (self-authenticating) |
| Backstage | backstage.platform.your-org.com | SPA + API | High (Permission Framework) |
| Argo CD | argocd.platform.your-org.com | UI + API | High (RBAC) |
| Grafana | grafana.platform.your-org.com | UI + API | Medium (Role Mapping) |
| Prometheus | prometheus.platform.your-org.com | UI + API | Low (same permissions for everyone) |
| Hubble | hubble.platform.your-org.com | UI | Low (same permissions for everyone) |

### Authentication Patterns Considered

#### Pattern A: Per-Service Authentication

Each service implements OIDC/OAuth2 independently

```
Browser -> Backstage (OIDC) -> Keycloak
Browser -> Argo CD (OIDC) -> Keycloak
Browser -> Grafana (OAuth2) -> Keycloak
Browser -> Prometheus (no auth)
```

**Pros:**
- Maximizes each service's native capabilities
- Failure isolation per service

**Cons:**
- OIDC configuration needed for each service (operational burden)
- Cannot protect non-OIDC services like Prometheus/Hubble
- Configuration required for each new service

#### Pattern B: Gateway-Level Authentication (OAuth2 Proxy)

Unified authentication at the Istio Gateway using OAuth2 Proxy (similar to AWS ALB + Cognito)

```
Browser -> Istio Gateway -> OAuth2 Proxy -> Keycloak
                              | Authenticated
                         All services
```

**Pros:**
- Authentication managed in one place (easy operations)
- All services automatically protected
- No configuration needed when adding new services
- Non-OIDC services like Prometheus can also be protected
- Single login provides access to all services (cookie sharing)

**Cons:**
- Cannot use each service's native auth features
- Fine-grained authorization must be implemented at the app level
- OAuth2 Proxy is a single point of failure (redundancy needed)

#### Pattern C: Hybrid (Gateway + Per-Service)

Gateway authentication as the base, with additional auth/authorization only for services that need it

```
Browser -> Istio Gateway -> OAuth2 Proxy (Auth Layer 1)
                              | JWT issued
                         +----+----+
                    Backstage    Prometheus
                         |            |
                    OIDC verification  Trusted as-is
                  Permission           (Authenticated)
                  Framework
                  (Auth Layer 2)
```

**Pros:**
- All benefits of Pattern B
- Fine-grained authorization can be added only where needed
- Easy phased adoption (Phase 2 -> Phase 3)
- OAuth2 Proxy forwards JWT, allowing downstream services to reuse it

**Cons:**
- Most complex (but can be built incrementally)

## Decision

**Adopt Pattern C: Hybrid Authentication (Gateway + Per-Service)**

### Phased Implementation

#### Phase 1: No Authentication (Current State)

```yaml
State: All services accessible without authentication
Duration: Early development
Purpose: Platform construction and verification
```

#### Phase 2: Gateway-Level Authentication (OAuth2 Proxy)

```
Browser -> Istio Gateway (gateway-platform)
              |
         OAuth2 Proxy (ExtAuthz) <-> Keycloak
              | Authenticated (JWT issued)
         +----+----+----------+---------+
    Backstage  Prometheus  Argo CD  Grafana
         |          |         |        |
    Header      Header     Header   Header
    read        read       read     read
```

**Implementation:**
- Deploy OAuth2 Proxy (auth-system namespace)
- Configure Istio ExtAuthz (EnvoyFilter)
- Create Keycloak Realm `platform`
- Create OAuth2 Proxy Client
- Basic Role/Group setup (platform-admin)

**Benefits:**
- All services automatically protected
- Single login provides access to all services
- Minimal operational burden

**Per-Service Handling:**
- Receives `Authorization: Bearer <JWT>` header from OAuth2 Proxy
- Trusts `X-Auth-Request-User`, `X-Auth-Request-Email` headers
- No authorization performed (everyone has full access)

#### Phase 3: Multi-Layer Authentication (Gateway + Per-Service Authorization)

```
Browser -> Istio Gateway -> OAuth2 Proxy (Auth Layer 1)
                              | JWT issued
         +--------------------+---------------------+
    Backstage                            Prometheus
         |                                    |
    Proxy Provider                       No change
    (JWT verification)                  (Remains at Phase 2)
         |
    Permission Framework
    (Auth Layer 2)
         |
    Fine-grained permissions
    - Catalog read: Everyone
    - Template execution: Developers
    - Catalog deletion: Platform Engineers
```

**Implementation (per service, incrementally):**

1. **Backstage**:
   ```typescript
   // Add auth-backend-module-proxy-provider
   // Enable Permission Framework
   // RBAC based on Keycloak groups claim
   ```

2. **Argo CD**:
   ```yaml
   # Add OIDC configuration
   # JWT verification
   # Role mapping via policy.csv
   ```

3. **Grafana**:
   ```yaml
   # Generic OAuth configuration
   # Role mapping via role_attribute_path
   ```

4. **Prometheus/Hubble**:
   - No changes (OAuth2 Proxy from Phase 2 is sufficient)

**Benefits:**
- No changes needed to OAuth2 Proxy configuration
- Fine-grained authorization added only to services that need it
- JWT verification occurs twice (defense in depth)
- Can migrate service by service incrementally

### Authentication Method by Service

| Service | Phase 2 | Phase 3 | Rationale |
|---------|---------|---------|------|
| **Backstage** | OAuth2 Proxy | OAuth2 Proxy + Proxy Provider + Permission Framework | SPA, needs fine-grained permissions |
| **Argo CD** | OAuth2 Proxy | OAuth2 Proxy + OIDC + RBAC | Good native RBAC support |
| **Grafana** | OAuth2 Proxy | OAuth2 Proxy + Generic OAuth + Role Mapping | Role mapping needed |
| **Prometheus** | OAuth2 Proxy | **No change** | No OIDC support, same permissions for everyone |
| **Hubble** | OAuth2 Proxy | **No change** | No OIDC support, same permissions for everyone |

## Rationale

### Why the Hybrid Approach Was Chosen

#### 1. Easy Phased Adoption

**Phase 1 -> Phase 2:**
```bash
# Just deploy OAuth2 Proxy
kubectl apply -f infrastructure/oauth2-proxy/
kubectl apply -f infrastructure/network/istio/oauth2-proxy-extauthz.yaml

# All services are automatically protected
# No configuration changes needed per service
```

**Phase 2 -> Phase 3:**
```bash
# Migrate service by service
# 1. Backstage first
kubectl apply -f backstage/manifests/
# OAuth2 Proxy unchanged

# 2. Then Argo CD
kubectl apply -f infrastructure/gitops/argocd/
# OAuth2 Proxy unchanged

# Prometheus/Hubble need no migration (stays at Phase 2)
```

#### 2. Minimal Operational Burden

- **Phase 2**: Authentication managed in one place (OAuth2 Proxy)
- **Phase 3**: Additional configuration only for services that need it
- New services are automatically protected by OAuth2 Proxy

#### 3. Defense in Depth

```
Layer 1 (Gateway): OAuth2 Proxy
  - Rejects invalid JWTs early
  - Basic protection for all services

Layer 2 (Application): Per-service authorization
  - Re-verifies JWT
  - Fine-grained resource-level permission control
  - Backstage Permission Framework
  - Argo CD RBAC
```

#### 4. JWT Reuse

OAuth2 Proxy forwards JWT to downstream services with these settings:

```yaml
args:
- --pass-access-token=true
- --pass-authorization-header=true
- --set-authorization-header=true
```

Each service receives the `Authorization: Bearer <JWT>` header:
- Phase 2: Just reads the header (no verification)
- Phase 3: Verifies JWT + authorizes based on groups claim

**OAuth2 Proxy stays out of the way** -- it just passes the JWT through

#### 5. Handling Service-Specific Characteristics

| Service Characteristic | Approach |
|------------|----------|
| **OIDC-capable (Backstage/Argo CD/Grafana)** | Individual OIDC implementation in Phase 3 |
| **Non-OIDC (Prometheus/Hubble)** | OAuth2 Proxy only from Phase 2 |
| **Fine-grained authorization needed (Backstage/Argo CD)** | Permission Framework/RBAC in Phase 3 |
| **No authorization needed (Prometheus/Hubble)** | Stays at Phase 2 |

### Why Pattern A (Per-Service Authentication) Was Not Chosen

1. **Cannot protect Prometheus/Hubble**
   - No OIDC implementation
   - Basic auth is insufficient

2. **High operational burden**
   - OIDC configuration needed for each service
   - Configuration required for each new service

3. **Poor user experience**
   - Potential redirects per service
   - Cookies/Sessions not shared

### Why Pattern B (Gateway Authentication Only) Was Not Chosen

Pattern B is adopted for Phase 2, but has the following limitations long-term:

1. **Cannot do fine-grained authorization**
   - Cannot control "only developers can execute templates" in Backstage
   - Cannot control "only Platform Engineers can deploy to production" in Argo CD

2. **Cannot leverage service-specific features**
   - Cannot use Backstage Permission Framework
   - Cannot use Argo CD RBAC

-> **Resolved by hybridizing in Phase 3**

## Implementation Details

### Keycloak Realm Design

```yaml
Realm: platform

Clients:
  - oauth2-proxy-client (Confidential, Standard Flow)
    # For OAuth2 Proxy
    # Redirect URI: https://auth.platform.your-org.com/oauth2/callback

  - backstage-client (Confidential, Standard Flow) # Used in Phase 3
  - argocd-client (Confidential, Standard Flow)    # Used in Phase 3
  - grafana-client (Confidential, Standard Flow)   # Used in Phase 3

Roles:
  - platform-admin     # Full access to all services
  - platform-developer # Limited access
  - platform-viewer    # Read-only

Groups:
  - platform-engineers
    roles: [platform-admin]

  - app-developers
    roles: [platform-developer]

  - viewers
    roles: [platform-viewer]

Users:
  - admin@example.com
    groups: [platform-engineers]
```

### OAuth2 Proxy Configuration (Phase 2)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oauth2-proxy
  namespace: auth-system
spec:
  containers:
  - name: oauth2-proxy
    image: quay.io/oauth2-proxy/oauth2-proxy:v7.5.1
    args:
    # OIDC Provider
    - --provider=oidc
    - --oidc-issuer-url=https://keycloak.platform.your-org.com/realms/platform
    - --client-id=oauth2-proxy-client
    - --client-secret=$(CLIENT_SECRET)

    # Cookie settings (shared across all subdomains)
    - --cookie-name=_oauth2_proxy
    - --cookie-secure=true
    - --cookie-domain=.platform.your-org.com

    # JWT forwarding (used in Phase 3)
    - --pass-access-token=true
    - --pass-authorization-header=true
    - --set-authorization-header=true

    # Authentication settings
    - --email-domain=*
    - --skip-provider-button=true
```

### Istio ExtAuthz Configuration (Phase 2)

```yaml
# Istio ConfigMap
extensionProviders:
- name: oauth2-proxy
  envoyExtAuthzHttp:
    service: oauth2-proxy.auth-system.svc.cluster.local
    port: 4180
    pathPrefix: /oauth2/auth
    headersToUpstreamOnAllow:
    - authorization
    - x-auth-request-user
    - x-auth-request-email
    - x-auth-request-access-token

---
# AuthorizationPolicy (authenticate all requests)
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-platform-oauth2
  namespace: istio-system
spec:
  selector:
    matchLabels:
      gateway.networking.k8s.io/gateway-name: gateway-platform
  action: CUSTOM
  provider:
    name: oauth2-proxy
  rules:
  - {}  # All requests

---
# Exception paths (skip authentication)
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: gateway-platform-skip-auth
  namespace: istio-system
spec:
  action: ALLOW
  rules:
  # OAuth2 Proxy callback
  - to:
    - operation:
        hosts: ["auth.platform.your-org.com"]
        paths: ["/oauth2/*"]

  # Keycloak (the auth server itself)
  - to:
    - operation:
        hosts:
        - "keycloak.platform.your-org.com"
        - "keycloak-dev.platform.your-org.com"
```

### Backstage Configuration (Phase 3)

```yaml
# app-config.kubernetes.yaml
auth:
  environment: production
  providers:
    # Trust headers from OAuth2 Proxy
    proxy:
      signIn:
        resolvers:
        - resolver: forwardedUserMatchingUserEntityEmail

permission:
  enabled: true  # Enable in Phase 3
```

```typescript
// packages/backend/src/plugins/permission.ts
export class PlatformPermissionPolicy implements PermissionPolicy {
  async handle(request, user): Promise<PolicyDecision> {
    const groups = user?.identity.ownershipEntityRefs || [];

    // Platform Engineers: Allow everything
    if (groups.includes('group:default/platform-engineers')) {
      return { result: AuthorizeResult.ALLOW };
    }

    // App Developers: Limited access
    if (groups.includes('group:default/app-developers')) {
      if (
        request.permission === catalogEntityReadPermission ||
        request.permission.id === 'scaffolder.template.execute'
      ) {
        return { result: AuthorizeResult.ALLOW };
      }
      return { result: AuthorizeResult.DENY };
    }

    return { result: AuthorizeResult.DENY };
  }
}
```

## Consequences

### Authentication Flow (Phase 2)

```
1. User -> https://backstage.platform.your-org.com

2. Istio Gateway -> Check authentication with OAuth2 Proxy
   GET /oauth2/auth
   Headers: Cookie, X-Forwarded-*

3-A. Valid cookie (authenticated):
   OAuth2 Proxy -> 200 OK
   Headers:
     Authorization: Bearer <JWT>
     X-Auth-Request-User: admin@example.com
     X-Auth-Request-Email: admin@example.com

   -> Forward request to Backstage
   -> Backstage: Read headers (no verification)
   -> Return response

3-B. Invalid cookie (unauthenticated):
   OAuth2 Proxy -> 302 Redirect
   Location: https://keycloak.platform.your-org.com/realms/platform/...

   -> Login at Keycloak
   -> OAuth2 Proxy obtains token
   -> Set cookie
   -> Redirect to original URL
   -> Proceed to flow 3-A
```

### Authentication Flow (Phase 3 - Backstage Example)

```
1. User -> https://backstage.platform.your-org.com

2. OAuth2 Proxy (Layer 1):
   - Cookie verification
   - JWT issued: Authorization: Bearer <JWT>

3. Backstage (Layer 2):
   - Receives Authorization header
   - JWT decoded: { email: "admin@example.com", groups: ["platform-engineers"] }
   - JWT verified with Keycloak public key
   - Permission Framework: Authorization decision based on groups

   Example: catalogEntityDeletePermission
   -> groups.includes('platform-engineers') -> ALLOW
   -> groups.includes('app-developers') -> DENY
```

### Benefits

1. **Phased Adoption**
   - Phase 1: No authentication (development)
   - Phase 2: Gateway authentication (blanket protection)
   - Phase 3: Multi-layer authentication (fine-grained permissions)

2. **Minimal Operational Burden**
   - Phase 2: Authentication managed in one place
   - Phase 3: Additional configuration only for services that need it
   - No changes to OAuth2 Proxy configuration

3. **Flexibility**
   - Authentication methods suited to service characteristics
   - Prometheus/Hubble stay at Phase 2
   - Backstage/Argo CD get fine-grained authorization in Phase 3

4. **Defense in Depth**
   - Gateway: Early rejection of invalid JWTs
   - Application: Resource-level authorization

5. **User Experience**
   - Single login provides access to all services
   - Cookie sharing (.platform.your-org.com)

### Drawbacks and Mitigations

1. **Increased Complexity**
   - Mitigation: Build incrementally (Phase 1 -> 2 -> 3)
   - Very simple up through Phase 2

2. **OAuth2 Proxy as Single Point of Failure**
   - Mitigation: 2 replicas for redundancy
   - PodDisruptionBudget for availability

3. **Performance Overhead**
   - ExtAuthz call latency
   - Mitigation: Place OAuth2 Proxy in istio-system namespace (proximity)
   - Mitigation: Cookie caching makes authenticated requests fast

4. **Debugging Difficulty**
   - Mitigation: Log output at each layer
   - OAuth2 Proxy: `--auth-logging=true`
   - Backstage: Permission Framework logs

### Trade-offs

**Complexity vs Flexibility**

- Adopted: Start simple with Phase 2, add flexibility in Phase 3
- Not adopted: OIDC implementation for all services from the start (high operational burden)
- Not adopted: Gateway authentication only (cannot do fine-grained authorization)

**Defense in Depth vs Performance**

- Adopted: Two rounds of JWT verification (Gateway + Application)
- Not adopted: Single verification only (reduced security)

The hybrid approach is appropriate for this environment because:
- Platform services are used by administrators and developers (security matters)
- Phased adoption reduces burden during early development
- Supports fine-grained permission control in the future

## Implementation Roadmap

### Phase 1: Maintain Current State (Complete)
- Everything without authentication
- Focus on platform construction

### Phase 2: Gateway Authentication (2 weeks out)

**Week 1:**
1. Create Keycloak Realm `platform`
2. Create OAuth2 Proxy Client
3. Create admin user/group
4. Obtain Client Secrets

**Week 2:**
1. Create OAuth2 Proxy Deployment/Service
2. Manage secrets with Sealed Secrets
3. Create HTTPRoute (auth.platform.your-org.com)
4. Configure Istio ExtAuthz
5. Verify functionality

**Outcome:**
- All services protected
- Single login provides access to all services

### Phase 3: Multi-Layer Authentication (1 month out, incrementally)

**Month 1: Backstage**
1. Add Proxy Provider module
2. Enable Permission Framework
3. Implement custom policy
4. Groups claim-based RBAC

**Month 2: Argo CD**
1. Add OIDC configuration
2. Role mapping via policy.csv
3. Verify functionality

**Month 3: Grafana**
1. Generic OAuth configuration
2. Role mapping via role_attribute_path
3. Verify functionality

**Prometheus/Hubble:**
- No changes (stays at Phase 2)

## References

- [OAuth2 Proxy Documentation](https://oauth2-proxy.github.io/oauth2-proxy/)
- [Istio External Authorization](https://istio.io/latest/docs/tasks/security/authorization/authz-custom/)
- [Backstage Auth Proxy Provider](https://backstage.io/docs/auth/proxy/)
- [Backstage Permission Framework](https://backstage.io/docs/permissions/overview)
- [Argo CD OIDC Configuration](https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/#oidc)
- [Keycloak Authorization Services](https://www.keycloak.org/docs/latest/authorization_services/)
- [AWS ALB + Cognito Pattern](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/listener-authenticate-users.html)
