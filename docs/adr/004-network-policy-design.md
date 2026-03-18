# ADR-004: NetworkPolicy Design

## Status

Accepted

## Date

2026-02-25

## Context

The kensan-lab cluster uses Cilium as its CNI and has NetworkPolicy enforcement capabilities available. However, no NetworkPolicy has been defined so far, leaving unrestricted communication permitted between all namespaces.

The gitops-evaluation-report.md flagged "P1 (Critical): No NetworkPolicy exists," and this is being introduced as part of security hardening.

## Decision

### 1. Default-Deny + Allow Pattern

Place default-deny (both ingress and egress) in each target namespace, explicitly allowing only necessary communication.

### 2. Trusted Infrastructure Layer (NetworkPolicy Exempt)

The following 4 namespaces are exempt from NetworkPolicy:

| Namespace | Reason |
|-----------|------|
| **kube-system** | Cilium (CNI) itself runs here. Applying default-deny could break the NetworkPolicy enforcement mechanism itself |
| **istio-system** | istiod pushes xDS config to all sidecar-injected namespaces. Policy updates would be needed every time a target namespace is added, causing high operational costs |
| **monitoring** | Prometheus scrapes all namespaces and OTel Collector receives data from all namespaces. Restricting monitoring egress would require policy updates with every new namespace |
| **argocd** | Argo CD communicates with the Kubernetes API server, GitHub, and Helm registries. In a Cilium kube-proxy replacement environment, egress to `kubernetes.default.svc` cannot be controlled with namespaceSelector, and NetworkPolicy would block API server communication |

These are protected at another layer with PSS: `privileged` (kube-system) or `baseline` (istio-system, monitoring, argocd).

### 3. Use Standard Kubernetes NetworkPolicy

Use standard `networking.k8s.io/v1` NetworkPolicy rather than CiliumNetworkPolicy (L7 filtering, DNS-based policy, etc.).

Rationale:
- Portability (works with CNIs other than Cilium)
- Native Kubernetes resource with lower learning curve
- L3/L4 level control is sufficient for current needs

Consider migrating to CiliumNetworkPolicy in the future if L7 level control (HTTP path-based, DNS-based) becomes necessary.

### 4. Common Policy Patterns

The following common policies are placed in all target namespaces:

| Policy Name | Type | Content |
|-----------|------|------|
| `default-deny-all` | Ingress + Egress | Deny all traffic (baseline) |
| `allow-dns` | Egress | Allow kube-system:53/UDP,TCP |
| `allow-intra-namespace` | Ingress + Egress | Allow Pod-to-Pod communication within the same namespace |
| `allow-prometheus-scrape` | Ingress | Allow scraping from the monitoring namespace |

For Istio sidecar-injected namespaces, additionally:

| Policy Name | Type | Content |
|-----------|------|------|
| `allow-istio` | Ingress + Egress | Allow bidirectional communication with istio-system (xDS + Gateway routing) |

### 5. File Placement

| Namespace | Location | Management Method |
|-----------|--------|---------|
| kensan-prod | `infrastructure/environments/kensan-prod/network-policy.yaml` | ApplicationSet (Git directory) |
| kensan-dev | `infrastructure/environments/kensan-dev/network-policy.yaml` | ApplicationSet (Git directory) |
| kensan-data | `infrastructure/environments/kensan-data/network-policy.yaml` | ApplicationSet (Git directory) |
| backstage | `backstage/manifests/base/network-policy.yaml` | Kustomize |
| platform-auth-* | `infrastructure/security/keycloak/base/network-policy.yaml` | Kustomize (shared in base) |
| cert-manager | `infrastructure/security/cert-manager/resources/network-policy.yaml` | Argo CD resources source |

## Consequences

### Positive

- Unauthorized communication between namespaces is blocked
- Communication patterns are declaratively managed as code
- Communication requirements are forced to be made explicit when adding new namespaces/services

### Negative

- NetworkPolicy updates are required when adding new communication patterns (intentional trade-off)
- Identifying blocked communication as the root cause during debugging may take time

### Risks

- Overlooked existing communication patterns could cause service outages immediately after deployment
- Mitigation: Verify each Application's sync status and Pod communication after merging
