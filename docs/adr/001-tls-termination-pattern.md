# ADR-001: TLS Termination Pattern Selection

## Status

Accepted

## Date

2025-11-07

## Context

When exposing platform infrastructure services (Argo CD, Grafana, Prometheus, Keycloak, Hubble UI, etc.) externally, we need to decide where TLS termination occurs.

### Three TLS Termination Patterns

#### 1. Edge Termination

```
[Browser] --HTTPS--> [Gateway] --HTTP--> [Backend]
                      ^ TLS termination
```

- **Characteristics**: Gateway manages TLS certificates and forwards HTTP to the backend
- **Pros**:
  - Centralized certificate management at a single point (Gateway)
  - No TLS configuration needed on backends
  - Certificates can be shared across multiple services
  - Good performance (encryption/decryption happens only once)
- **Cons**:
  - Traffic between Gateway and Backend is plaintext (within the cluster)

#### 2. Passthrough Termination

```
[Browser] --HTTPS--> [Gateway] --HTTPS--> [Backend]
                      ^ Forwards encrypted   ^ TLS termination
```

- **Characteristics**: Gateway forwards encrypted traffic as-is; backend terminates TLS
- **Pros**:
  - End-to-end encryption
  - Backend has full control over certificates
- **Cons**:
  - Each backend needs its own certificate
  - No L7 routing at the Gateway (traffic is encrypted)
  - Cannot perform HTTP header manipulation or authentication

#### 3. Re-encryption

```
[Browser] --HTTPS--> [Gateway] --HTTPS--> [Backend]
                      ^ TLS termination  ^ TLS termination
                      v TLS re-encryption
```

- **Characteristics**: Gateway decrypts then re-encrypts before forwarding to the backend
- **Pros**:
  - End-to-end encryption
  - L7 processing possible at the Gateway (header injection, authentication, etc.)
  - Gateway-to-Backend communication is also encrypted
- **Cons**:
  - Most complex approach
  - Performance overhead (two rounds of encryption/decryption)
  - Certificates needed on both Gateway and Backend
  - **Istio Sidecar required as a Service Mesh** (heavyweight)

## Decision

**Adopt Edge Termination**

Use the Edge Termination pattern with Istio Gateway for all platform infrastructure services.

### Services in Scope

- Argo CD
- Grafana
- Prometheus
- Keycloak (Prod/Dev)
- Hubble UI
- Future platform services

### Implementation Approach

1. **Gateway side**:
   - Use Let's Encrypt certificates on Istio Gateway (auto-managed by cert-manager)
   - Wildcard certificate `*.platform.your-org.com`
   - TLS termination (`tls.mode: Terminate`)

2. **Backend side**:
   - Services operate in HTTP mode (insecure mode)
   - Example: Argo CD `server.insecure: "true"`
   - Exposed via ClusterIP Service

3. **HTTPRoute**:
   - Targets the backend service's HTTP port (typically port 80)

## Rationale

### Why Edge Termination Was Chosen

1. **Simplified Certificate Management**
   - Manage Let's Encrypt certificates in one place (cert-manager + Gateway)
   - No need to distribute certificates to each backend service
   - Certificate renewal is automated with limited blast radius

2. **Performance**
   - Encryption/decryption happens only once (at the Gateway)
   - Resource-efficient for bare-metal Raspberry Pi environment

3. **Operational Simplicity**
   - No TLS configuration needed on backend services
   - Helm charts and existing manifests can be used as-is
   - Easier troubleshooting

4. **Intra-Cluster Trust**
   - Kubernetes intra-cluster network is treated as trusted
   - NetworkPolicy can control inter-namespace communication
   - Physically isolated bare-metal environment

### Why Re-encryption Was Not Chosen

1. **Istio Sidecar Injection Required**
   - Service Mesh (mTLS) must be enabled
   - Envoy Sidecar injected into each Pod
   - **Significant resource overhead** (heavyweight for Raspberry Pi environment)

2. **Increased Complexity**
   - Certificates managed in two places (Gateway + Backend)
   - Difficult to debug
   - Performance overhead (two rounds of encryption/decryption)

3. **Low Necessity for This Environment**
   - Physically isolated bare-metal environment
   - NetworkPolicy provides sufficient control
   - Platform infrastructure services are accessed only by administrators

### Why Passthrough Was Not Chosen

1. **No L7 Routing**
   - HTTP header-based routing is not possible
   - Future Keycloak JWT authentication integration would be difficult

2. **Complex Certificate Management**
   - Each backend needs its own certificate
   - Let's Encrypt certificates must be managed individually

## Consequences

### Implementation Notes

1. **Backend Service Configuration**

   Each service must explicitly indicate that "TLS termination is handled by the proxy":

   - **Argo CD**: `server.insecure: "true"` in ConfigMap
   - **Grafana**: HTTP by default (no additional configuration needed)
   - **Prometheus**: HTTP by default (no additional configuration needed)
   - **Keycloak**: `KC_PROXY=edge` environment variable

2. **Avoiding Redirect Loops**

   If a backend service enforces HTTPS redirects, an infinite loop will occur.
   Always enable insecure/edge mode.

3. **Certificate Scope**

   - `*.platform.your-org.com`: For platform infrastructure
   - `*.app.your-org.com`: For applications (future)

   Let's Encrypt wildcard certificates cover only one level of subdomain,
   so certificates should be separated according to hierarchical subdomain structure.

### Trade-offs

**Security vs Performance/Operability**

- Adopted: Plaintext within the cluster, easy operations, good performance
- Not adopted: End-to-end encryption, complex, heavyweight

Edge Termination is appropriate for this environment for the following reasons:
- Physically isolated bare-metal environment
- Raspberry Pi resource constraints
- Platform infrastructure services accessed only by administrators
- Inter-namespace communication control via NetworkPolicy

### Conditions for Future Reconsideration

Consider migrating to the Re-encryption pattern in the following cases:

1. **Compliance Requirements**
   - When regulations mandate end-to-end encryption

2. **Multi-Tenant Environment**
   - When hosting applications from multiple organizations on the same cluster

3. **Zero-Trust Networking**
   - When adopting a security policy that requires encryption for intra-cluster communication

4. **Hardware Upgrade**
   - When migrating to higher-performance nodes where Istio Sidecar overhead becomes acceptable

## References

- [Istio Gateway API - TLS Configuration](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/#configuring-tls)
- [Argo CD - Running Argo CD behind a proxy](https://argo-cd.readthedocs.io/en/stable/operator-manual/ingress/)
- [Keycloak - Using a reverse proxy](https://www.keycloak.org/server/reverseproxy)
- [Let's Encrypt - Wildcard Certificates](https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates)
