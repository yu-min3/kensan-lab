# Configuration Guide

This repository uses real configuration values from the author's homelab. If you fork this repo, you'll need to replace them with your own.

## What to Replace

### 1. Domain Name

This repository uses `yu-min3.com`. Replace with your own domain.

**Search and replace across the repo:**

```bash
# Preview what will change
grep -r "yu-min3\.com" infrastructure/ backstage/ apps/ --include="*.yaml" --include="*.yml"

# Replace (Linux/Mac)
find infrastructure/ backstage/ apps/ -type f \( -name "*.yaml" -o -name "*.yml" \) \
  -exec sed -i '' 's/yu-min3\.com/your-domain.com/g' {} +
```

**Key files:**

| Component | File | Value |
|-----------|------|-------|
| Gateways | `infrastructure/network/istio/resources/gateway-*.yaml` | `*.platform.yu-min3.com`, `*.app.yu-min3.com` |
| Certificates | `infrastructure/security/cert-manager/resources/wildcard-certificate-*.yaml` | `*.yu-min3.com` |
| ClusterIssuer | `infrastructure/security/cert-manager/resources/clusterissuer.yaml` | `admin@yu-min3.com` |
| HTTPRoutes | `infrastructure/**/resources/httproute*.yaml` | `*.platform.yu-min3.com` |
| Keycloak | `infrastructure/security/keycloak/overlays/*/httproute.yaml` | `auth.platform.yu-min3.com` |
| Keycloak env | `infrastructure/security/keycloak/overlays/prod/keycloak-env-patch.yaml` | `KC_HOSTNAME` |

### 2. GitHub Organization

This repository uses `yu-min3`. Replace in Argo CD Application CRs and container image references.

```bash
grep -r "yu-min3" infrastructure/gitops/ backstage/ apps/ --include="*.yaml" --include="*.yml" | grep -v "yu-min3\.com"

find infrastructure/gitops/ backstage/ apps/ -type f \( -name "*.yaml" -o -name "*.yml" \) \
  -exec sed -i '' 's/yu-min3/your-github-org/g' {} +
```

**Key files:**

| Component | File | Value |
|-----------|------|-------|
| Argo CD Apps | `infrastructure/gitops/argocd/applications/**/app.yaml` | `repoURL` |
| Root App | `infrastructure/gitops/argocd/root-apps/platform-root-app.yaml` | `repoURL` |
| Container images | `apps/kensan/manifests/app/*.yaml` | `ghcr.io/yu-min3/...` |
| Backstage image | `backstage/manifests/overlays/prod/kustomization.yaml` | `ghcr.io/yu-min3/backstage` |

### 3. LoadBalancer IP Range

This repository uses `192.168.0.240-249`. Adjust to your local network.

**File:** `infrastructure/network/cilium/values.yaml`

Ensure the range does not overlap with your DHCP server's allocation.

### 4. Network Interface

Cilium L2 Announcements use WiFi interfaces (`^wlan.*`, `^wlp.*`). If your nodes use wired connections, update the interface regex.

**File:** `infrastructure/network/cilium/values.yaml`

### 5. Sealed Secrets

All Sealed Secrets are encrypted with this cluster's sealing key. **They cannot be decrypted by a different cluster.** You must regenerate every Sealed Secret.

```bash
# List all sealed secrets
find infrastructure/ backstage/ apps/ -name "*sealed*" -o -name "*secret*" | grep -v node_modules

# General pattern: create raw secret, then seal
kubectl create secret generic <name> \
  --namespace=<ns> \
  --from-literal=key=value \
  --dry-run=client -o yaml > temp/<name>-raw.yaml

kubeseal --format=yaml < temp/<name>-raw.yaml > <path>/<name>-sealed.yaml
rm temp/<name>-raw.yaml  # never commit raw secrets
```

### 6. cert-manager DNS Provider

This repository uses AWS Route 53 for DNS-01 challenges. If you use a different DNS provider (Cloudflare, Google Cloud DNS, etc.), update:

- `infrastructure/security/cert-manager/resources/clusterissuer.yaml` — solver configuration
- The corresponding credentials Sealed Secret

---

## Verification

After replacing all values:

```bash
# Check no author-specific values remain
grep -r "yu-min3" infrastructure/ backstage/ --include="*.yaml" | head -20

# Validate Argo CD Application CRs point to your repo
grep -rh "repoURL:" infrastructure/gitops/argocd/ --include="*.yaml" | sort -u
```
