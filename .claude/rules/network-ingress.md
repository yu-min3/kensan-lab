---
description: Cilium L2, LoadBalancer IPs, Istio Gateways, domain architecture, and certificate mapping
globs: "infrastructure/network/**, **/httproute*, **/gateway*"
---

# Network & Ingress

## Cilium LoadBalancer

- **L2 Announcements** enabled on WiFi: regex `^wlan.*`, `^wlp.*`
- **IP Pool**: 192.168.0.240–249 (must not overlap DHCP)
- **Devices**: auto-detect (supports mixed `wlan0` / `wlp3s0`)
- kube-proxy replacement enabled

## LoadBalancer IP Assignments

| Gateway | IP |
|---------|----|
| gateway-platform | 192.168.0.242 |
| gateway-dev | 192.168.0.241 |
| gateway-prod | 192.168.0.243 |

## Gateway Architecture

Three Istio Gateways in `infrastructure/network/istio/resources/`:

| Gateway | Domain Pattern | Certificate Secret |
|---------|---------------|--------------------|
| gateway-platform | `*.platform.yu-min3.com` | `wildcard-platform-tls` |
| gateway-prod | `*.app.yu-min3.com` | `wildcard-apps-tls` |
| gateway-dev | `*.app.yu-min3.com` | `wildcard-apps-tls` |

## Domain Examples

- Backstage: `backstage.platform.yu-min3.com`
- Argo CD: `argocd.platform.yu-min3.com`
- Prometheus: `prometheus.platform.yu-min3.com`
- Grafana: `grafana.platform.yu-min3.com`
- Keycloak (prod): `auth.platform.yu-min3.com`
- Keycloak (dev): `auth-dev.platform.yu-min3.com`
- Apps: `<appname>.app.yu-min3.com`

## HTTPRoute Convention

Each component's HTTPRoute is placed in its `resources/` directory and attaches to the appropriate Gateway via `parentRefs`.

## Known Issue

Cilium L2 lease renewal can timeout on WiFi (`context deadline exceeded`), causing LoadBalancer IPs to become unreachable. Fix: restart Cilium daemonset.
