# Namespace Label Design

This document defines a unified namespace labeling strategy across the entire platform.

## Design Principles

1. **Consistency**: Use the same label key and value patterns across all namespaces
2. **Discoverability**: Easily filterable with `kubectl get ns -l <label>=<value>`
3. **Security**: Leverage label selectors in NetworkPolicy, RBAC, etc.
4. **Standards Compliance**: Follow Kubernetes recommended labels (`app.kubernetes.io/*`)

## Label Schema

### Required Labels (Applied to All Namespaces)

| Label Key | Value | Description |
|-----------|-------|------|
| `app.kubernetes.io/managed-by` | `argocd` | Identifies the GitOps tool (Kubernetes recommended label) |
| `kensan-lab.platform/environment` | `production` \| `development` \| `infrastructure` | Environment type |
| `kensan-lab.platform/tier` | `platform` \| `application` | Responsibility layer (PE-managed vs AD-managed) |

### Optional Labels (Applied as Needed)

| Label Key | Example Values | Description | Target |
|-----------|---------------|------|----------|
| `kensan-lab.platform/component` | `keycloak`, `backstage`, `monitoring`, `service-mesh`, `core` | Component identification | Platform tier only |
| `istio-injection` | `enabled` | Istio automatic sidecar injection | Namespaces targeted by service mesh |

## Label Value Definitions

### `kensan-lab.platform/environment`

| Value | Description | Example Namespaces |
|-------|------|-----------------|
| `infrastructure` | Core cluster infrastructure | `kube-system`, `istio-system`, `monitoring`, `argocd` |
| `production` | Production environment | `backstage`, `platform-auth-prod`, `app-prod` |
| `development` | Development environment | `platform-auth-dev`, `app-dev` |

### `kensan-lab.platform/tier`

| Value | Description | Manager | Example Namespaces |
|-------|------|--------|-----------------|
| `platform` | Platform infrastructure tier | Platform Engineer (PE) | `istio-system`, `backstage`, `platform-auth-prod` |
| `application` | Application tier | Application Developer (AD) | `app-prod`, `app-dev`, `app-prod-<name>` |

### `kensan-lab.platform/component`

| Value | Description | Target Namespace |
|-------|------|--------------|
| `core` | Core Kubernetes system | `kube-system` |
| `service-mesh` | Service mesh control plane | `istio-system` |
| `gitops` | GitOps tooling | `argocd` |
| `observability` | Monitoring and log management | `monitoring` |
| `keycloak` | Authentication platform | `platform-auth-prod`, `platform-auth-dev` |
| `developer-portal` | Developer portal | `backstage` |

## Namespace Definition Templates

### Platform Infrastructure Tier (Environment-Independent)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <namespace-name>
  labels:
    app.kubernetes.io/managed-by: argocd
    kensan-lab.platform/environment: infrastructure
    kensan-lab.platform/tier: platform
    kensan-lab.platform/component: <component-name>
    # Optional: when using Istio
    # istio-injection: enabled
```

### Platform Tier (Environment-Specific)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <namespace-name>
  labels:
    app.kubernetes.io/managed-by: argocd
    kensan-lab.platform/environment: production|development
    kensan-lab.platform/tier: platform
    kensan-lab.platform/component: <component-name>
    istio-injection: enabled
```


### Application Tier

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <namespace-name>
  labels:
    app.kubernetes.io/managed-by: argocd
    kensan-lab.platform/environment: production|development
    kensan-lab.platform/tier: application
    istio-injection: enabled
```

## Label Usage Examples

### 2. NetworkPolicy Usage

```yaml
# Allow communication between platform tier namespaces
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-platform-tier
  namespace: backstage
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kensan-lab.platform/tier: platform
```

```yaml
# Allow production applications to access Keycloak
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-prod-apps-to-keycloak
  namespace: platform-auth-prod
spec:
  podSelector:
    matchLabels:
      app: keycloak
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kensan-lab.platform/environment: production
          kensan-lab.platform/tier: application
```

### 3. RBAC Usage

```yaml
# Grant Application Developers access to development environment namespaces
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: app-developer
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
  # Access limited to application tier in the development environment
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-developers-binding
  namespace: app-dev
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: app-developer
subjects:
- kind: Group
  name: application-developers
  apiGroup: rbac.authorization.k8s.io
```

### 4. Prometheus ServiceMonitor Selectors

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: platform-services
  namespace: monitoring
spec:
  namespaceSelector:
    matchLabels:
      kensan-lab.platform/tier: platform
  selector:
    matchLabels:
      monitoring: enabled
```

### 5. Istio Sidecar Auto-Injection Verification

```bash
# Check namespaces with Istio enabled
kubectl get ns -l istio-injection=enabled

# Enable Istio auto-injection on a new namespace
kubectl label namespace my-new-namespace istio-injection=enabled
```

## Migration Guide

Steps for migrating existing namespace labels to the unified design:

### Phase 1: Apply to New Namespaces First (Priority: High)

All namespaces created going forward must conform to this label design.

### Phase 2: Update Application Tier (Priority: Medium)

```bash
# app-dev/namespace.yaml
# Before:
labels:
  environment: development

# After:
labels:
  app.kubernetes.io/managed-by: argocd
  kensan-lab.platform/environment: development
  kensan-lab.platform/tier: application
  istio-injection: enabled
```

### Phase 3: Update Platform Tier (Priority: Low)

Gradually update existing namespaces. Proceed while confirming no impact on cluster operations.

**Notes:**
- Label changes do not affect the namespace itself, but check existing resources that use label selectors (NetworkPolicy, ServiceMonitor, etc.) beforehand
- Update via Argo CD (maintain GitOps principles)

## Summary

### Label Matrix

| Namespace | environment | tier | component | istio-injection |
|-----------|-------------|------|-----------|-----------------|
| kube-system | infrastructure | platform | core | No |
| istio-system | infrastructure | platform | service-mesh | No |
| argocd | infrastructure | platform | gitops | No |
| monitoring | infrastructure | platform | observability | No |
| backstage | production | platform | developer-portal | Yes |
| platform-auth-prod | production | platform | keycloak | Yes |
| platform-auth-dev | development | platform | keycloak | Yes |
| app-prod | production | application | - | Yes |
| app-dev | development | application | - | Yes |

### Best Practices

1. **Always set required labels**: `managed-by`, `environment`, `tier`
2. **Component label for platform tier only**: Not needed for the application tier
3. **Set istio-injection explicitly**: Only for namespaces targeted by the service mesh
4. **Minimize custom labels**: Use the `kensan-lab.platform/` prefix when necessary
5. **Check NetworkPolicies when changing labels**: Selectors may be affected

This label design improves consistency, discoverability, and security across the entire platform.
