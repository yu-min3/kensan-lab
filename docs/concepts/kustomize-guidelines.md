# Kustomize Usage Guidelines

> **DEPRECATED (2026-05):** Kustomize has been removed from `infrastructure/` as part of the dev/prod consolidation work. All components now use ArgoCD `directory` source on flat YAML directories. The Backstage scaffolder template still emits Kustomize for new apps; that's the only remaining usage. This doc will be rewritten or removed in a follow-up.

This document defines guidelines for when to use and when not to use Kustomize within the `infrastructure/` directory.

## Guiding Principles

Platform components are classified into three tiers based on **update frequency** and **need for environment separation**.

### Tier 1: Kustomize Required (Frequent Updates + Environment Separation)

**Target Components:**
- Keycloak (`platform-auth-dev` / `platform-auth-prod`)
- Custom applications (user-created apps)

**Rationale:**
- Frequent image tag updates (`newTag` changes with every deployment)
- Environment-specific configuration differences required (dev/prod)
- Environment-specific resources (HTTPRoute, Secrets, etc.)

**Structure:**
```
component/
├── base/
│   ├── kustomization.yaml
│   ├── deployment.yaml       # Image uses placeholder
│   ├── service.yaml
│   └── ...
└── overlays/
    ├── dev/
    │   ├── kustomization.yaml  # namespace, newTag, dev-specific resources
    │   └── dev-specific.yaml
    └── prod/
        ├── kustomization.yaml  # namespace, newTag, prod-specific resources
        └── prod-specific.yaml
```

**kustomization.yaml example:**
```yaml
# overlays/prod/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: platform-auth-prod

resources:
- ../../base
- httproute.yaml              # prod-specific resource
- postgresql-sealed-secret.yaml

images:
- name: quay.io/keycloak/keycloak
  newTag: 26.0.7               # <- Image tag update point

patches:
- path: keycloak-patch.yaml    # Environment-specific replicas, resources, etc.
  target:
    kind: Deployment
    name: keycloak
```

### Tier 2: Kustomize Recommended (Frequent Updates, No Environment Separation)

**Target Components:**
- Backstage (`backstage`)

**Rationale:**
- Frequent image tag updates (continuous deployment of Backstage app)
- Production only (IDP does not need environment separation)
- Tag updates can be automated with `kustomize edit set image`

**Structure:**
```
backstage/
├── base/
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ...
└── overlays/
    └── prod/
        └── kustomization.yaml  # Manages newTag only
```

**kustomization.yaml example:**
```yaml
# overlays/prod/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../base

images:
- name: ghcr.io/your-org/backstage
  newTag: v0.0.5               # <- Auto-updated by CI/CD
```

**Update command:**
```bash
# Run in CI/CD pipeline or Makefile
cd infrastructure/backstage/overlays/prod
kustomize edit set image ghcr.io/your-org/backstage:v0.0.6

# Or
sed -i 's/newTag: v.*/newTag: v0.0.6/' kustomization.yaml

git add kustomization.yaml
git commit -m "Update Backstage to v0.0.6"
git push  # -> Argo CD auto-syncs
```

### Tier 3: Flat YAML (Infrastructure Layer, Low Update Frequency)

**Target Components:**
- Cilium CNI + LoadBalancer (`cilium/`)
- Istio Control Plane + Gateways (`istio/`)
- Cert-Manager (`cert-manager/`)
- Prometheus Stack (`prometheus/`)
- Sealed Secrets Controller (`sealed-secret/`)
- Gateway API CRDs (`gateway-api/`)
- Local Path Provisioner (`local-path-provisioner/`)
- Namespace definitions (`app-dev/`, `app-prod/`, `kube-system/`)

**Rationale:**
- Extremely low update frequency (quarterly to annual version upgrades)
- No environment differences (single instance)
- Large generated manifests (Helm output, operator CRDs, etc.)
- Not worth introducing additional complexity

**Structure:**
```
component/
├── component.yaml            # Helm template output or operator manifests
├── namespace.yaml
├── httproute.yaml
└── additional-config.yaml
```

**Exception: Istio number prefixes:**
```
istio/
├── 00-namespace.yaml         # Explicit deployment order
├── 01-istio-base.yaml        # CRDs
├── 02-istiod.yaml            # Control Plane
├── gateway-platform.yaml
├── gateway-dev.yaml
└── gateway-prod.yaml
```

Numeric prefixes are used when Argo CD sync order matters (CRDs -> resources).

## Decision Flowchart

```
When adding a component:
  |
  ├─ Does it need environment-specific (dev/prod) configuration?
  │   └─ YES -> Kustomize (Tier 1)
  |
  ├─ Are image tags updated frequently? (once a week or more)
  │   └─ YES -> Kustomize (Tier 2)
  |
  ├─ Is there a possibility of adding environments in the future? (staging, etc.)
  │   └─ YES -> Kustomize (Tier 1)
  |
  └─ All of the above are NO -> Flat YAML (Tier 3)
```

## Referencing in Argo CD Application CRs

### Kustomize-Based:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: keycloak-prod
spec:
  source:
    repoURL: https://github.com/your-org/kensan-lab.git
    targetRevision: main
    path: infrastructure/security/keycloak/overlays/prod  # <- Point to overlays
```

### Flat YAML-Based:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cilium
spec:
  source:
    repoURL: https://github.com/your-org/kensan-lab.git
    targetRevision: main
    path: infrastructure/network/cilium                   # <- Point to directory directly
    directory:
      recurse: true
```

## Best Practices

### 1. When Using Kustomize

**DO:**
- Place environment-independent resources in `base/`
- Place environment-specific configuration in `overlays/`
- Manage image tags in `kustomization.yaml`
- Apply only minimal diffs with patches

**DON'T:**
- Hardcode environment-specific values in `base/`
- Redefine all resources in overlays (use patches instead)
- Overuse complex Kustomize features (vars, generators, etc.)

### 2. When Using Flat YAML

**DO:**
- Clearly note the generation source (`# Generated by: helm template ...`)
- Consider splitting large files (see the Istio example)
- Use numeric prefixes to control order as needed

**DON'T:**
- Manually edit large YAML files (prefer regeneration)
- Force Kustomize where it's not needed (YAGNI principle)

### 3. Migration Guidelines

When migrating existing flat YAML to Kustomize:

1. **Evaluate the need for environment separation**
   - Current: Operating in a single environment
   - Future: Planning to add dev/staging/prod environments -> Consider migration

2. **Check update frequency**
   - More than 5 image tag updates in the last 3 months -> Consider migration
   - Only annual version upgrades -> Maintain current approach

3. **Migrate gradually**
   ```bash
   # Step 1: Move existing files to base/
   mkdir -p component/base
   mv component/*.yaml component/base/

   # Step 2: Create kustomization.yaml
   cat > component/base/kustomization.yaml <<EOF
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization
   resources:
   - namespace.yaml
   - deployment.yaml
   - service.yaml
   EOF

   # Step 3: Create overlays/prod/
   mkdir -p component/overlays/prod
   cat > component/overlays/prod/kustomization.yaml <<EOF
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization
   resources:
   - ../../base
   images:
   - name: registry/image
     newTag: v1.0.0
   EOF

   # Step 4: Update the Argo CD Application CR
   # path: infrastructure/component -> infrastructure/component/overlays/prod
   ```

## Summary

| Tier | Target | Structure | Update Frequency | Environment Separation |
|------|--------|------|----------|----------|
| **1** | Keycloak, Custom apps | Kustomize (base + overlays/dev + overlays/prod) | High | Yes |
| **2** | Backstage | Kustomize (base + overlays/prod) | High | No |
| **3** | Cilium, Istio, Cert-Manager, Prometheus, etc. | Flat YAML | Low | No |

**Principle**: Use Kustomize only when necessary. Avoid excessive abstraction and keep things simple (KISS principle).
