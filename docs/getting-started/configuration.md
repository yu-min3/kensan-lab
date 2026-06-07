# Configuration Guide

This repository uses real configuration values from the author's homelab. If you fork this repo, you'll need to replace them with your own.

## What to Replace

### 1. Domain Names

This repository uses **two domains**, and you'll need to replace both:

- **`yu-min3.com`** — the primary domain. All internal/LAN-facing hosts live under
  `*.platform.yu-min3.com` and `*.app.yu-min3.com`, served by the Istio gateways
  with cert-manager-issued wildcard TLS.
- **`yu-mins.com`** — the **Cloudflare Tunnel public-facing** domain. A handful of
  components (Argo CD, Grafana, Prometheus, Longhorn, Backstage) are exposed to the
  internet via Cloudflare Tunnel under `*.yu-mins.com` (no `platform`/`app` prefix).
  TLS for these is terminated at the Cloudflare edge, so the in-cluster listener is
  plain HTTP. If you don't use Cloudflare Tunnel you can drop these references, but
  if you keep the pattern you must substitute your own second domain.

**Search and replace across the repo:**

```bash
# Preview what will change (both domains)
grep -r "yu-min3\.com" kubernetes/ backstage/ apps/ bootstrap/ --include="*.yaml" --include="*.yml"
grep -rn "yu-mins\.com" kubernetes/ backstage/ apps/ bootstrap/

# Replace (Linux/Mac)
find kubernetes/ backstage/ apps/ -type f \( -name "*.yaml" -o -name "*.yml" \) \
  -exec sed -i '' 's/yu-min3\.com/your-domain.com/g' {} +
find kubernetes/ backstage/ apps/ -type f \( -name "*.yaml" -o -name "*.yml" \) \
  -exec sed -i '' 's/yu-mins\.com/your-tunnel-domain.com/g' {} +
```

> Note: `yu-mins.com` also appears in `bootstrap/keycloak/setup.sh` (allowed redirect
> URIs / hostnames). Update that script too if you keep the tunnel pattern.

**Key files — primary domain (`yu-min3.com`):**

| Component | File | Value |
|-----------|------|-------|
| Gateways | `kubernetes/network/istio/gateway-*.yaml` | `*.platform.yu-min3.com`, `*.app.yu-min3.com` |
| Certificates | `kubernetes/secrets/cert-manager/resources/wildcard-certificate-*.yaml` | `*.yu-min3.com` |
| ClusterIssuer | `kubernetes/secrets/cert-manager/resources/clusterissuer.yaml` | `admin@yu-min3.com` |
| HTTPRoutes | `kubernetes/**/resources/httproute*.yaml` | `*.platform.yu-min3.com` |
| Keycloak | `kubernetes/auth/keycloak/httproute.yaml` | `auth.platform.yu-min3.com` |
| Keycloak env | `kubernetes/auth/keycloak/keycloak-env-config.yaml` | `KC_HOSTNAME` |

**Key files — Cloudflare Tunnel domain (`yu-mins.com`):**

| Component | File | Value |
|-----------|------|-------|
| Gateway listener | `kubernetes/network/istio/gateway-platform.yaml` | `*.yu-mins.com` (`http-tunnel` listener) |
| oauth2-proxy | `kubernetes/auth/oauth2-proxy/values.yaml` | `cookie_domains`, `whitelist_domains` |
| AuthorizationPolicy | `kubernetes/network/istio/authorizationpolicy-gateway-platform-allow.yaml`, `...-oauth2.yaml` | `*.yu-mins.com` hosts |
| Argo CD HTTPRoute | `kubernetes/argocd/resources/httproute.yaml` | `argocd.yu-mins.com` |
| Grafana HTTPRoute | `kubernetes/observability/grafana/resources/httproute.yaml` | `grafana.yu-mins.com` |
| Prometheus HTTPRoute | `kubernetes/observability/prometheus/resources/httproute-prometheus.yaml` | `prometheus.yu-mins.com` |
| Longhorn HTTPRoute | `kubernetes/storage/longhorn/resources/httproute.yaml` | `longhorn.yu-mins.com` |
| Backstage HTTPRoute | `backstage/manifests/httproute.yaml` | `backstage.yu-mins.com` |
| Keycloak bootstrap | `bootstrap/keycloak/setup.sh` | redirect URIs / hostnames |

### 2. GitHub Organization

This repository uses `yu-min3`. Replace in Argo CD Application CRs and container image references.

```bash
grep -r "yu-min3" kubernetes/argocd/ backstage/ apps/ --include="*.yaml" --include="*.yml" | grep -v "yu-min3\.com"

find kubernetes/argocd/ backstage/ apps/ -type f \( -name "*.yaml" -o -name "*.yml" \) \
  -exec sed -i '' 's/yu-min3/your-github-org/g' {} +
```

**Key files:**

| Component | File | Value |
|-----------|------|-------|
| Argo CD Apps | `kubernetes/argocd/applications/**/app.yaml` | `repoURL` |
| Root App | `kubernetes/argocd/root-apps/platform-root-app.yaml` | `repoURL` |
| Container images (legacy kensan) | `apps/kensan-legacy/manifests/services/*.yaml`, `apps/kensan-legacy/manifests/lakehouse/*.yaml` | `ghcr.io/yu-min3/...` |
| Container image (new kensan) | `kubernetes/apps/app-kensan/values.yaml` | `image.repository: ghcr.io/yu-min3/kensan` |
| Backstage image | `backstage/manifests/backstage-deployment.yaml` | `ghcr.io/yu-min3/backstage` |

### 3. LoadBalancer IP Range

This repository uses `192.168.0.240-249`. Adjust to your local network.

**File:** `kubernetes/network/cilium/resources/lb-ippool.yaml` (the `CiliumLoadBalancerIPPool`)

Ensure the range does not overlap with your DHCP server's allocation.

### 4. Network Interface

Cilium L2 Announcements default to wired interfaces (`^eth.*`, `^en.*`) with WiFi fallback (`^wlan.*`, `^wlp.*`). Adjust the interface regex to match your nodes' interface names if different.

**File:** `kubernetes/network/cilium/resources/lb-ippool.yaml`

### 5. Sealed Secrets

All Sealed Secrets are encrypted with this cluster's sealing key. **They cannot be decrypted by a different cluster.** You must regenerate every Sealed Secret.

```bash
# List all sealed secrets
find kubernetes/ backstage/ apps/ -name "*sealed*" -o -name "*secret*" | grep -v node_modules

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

- `kubernetes/secrets/cert-manager/resources/clusterissuer.yaml` — solver configuration
- The corresponding credentials Sealed Secret

---

## Verification

After replacing all values:

```bash
# Check no author-specific values remain (both domains + org)
grep -r "yu-min3" kubernetes/ backstage/ --include="*.yaml" | head -20
grep -rn "yu-mins" kubernetes/ backstage/ apps/ bootstrap/ | head -20

# Validate Argo CD Application CRs point to your repo
grep -rh "repoURL:" kubernetes/argocd/ --include="*.yaml" | sort -u
```
