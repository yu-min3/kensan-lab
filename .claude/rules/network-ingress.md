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
| gateway-platform | `*.platform.example.com` | `wildcard-platform-tls` |
| gateway-prod | `*.apps.example.com` | `wildcard-apps-tls` |
| gateway-dev | `*.apps.example.com` | `wildcard-apps-tls` |

## Domain Examples

- Backstage: `backstage.platform.example.com`
- Argo CD: `argocd.platform.example.com`
- Prometheus: `prometheus.platform.example.com`
- Grafana: `grafana.platform.example.com`
- Keycloak (prod): `auth.platform.example.com`
- Keycloak (dev): `auth-dev.platform.example.com`
- Apps: `<appname>.apps.example.com`

## HTTPRoute Convention

Each component's HTTPRoute is placed in its `resources/` directory and attaches to the appropriate Gateway via `parentRefs`.

## Known Issue

Cilium L2 lease renewal can timeout on WiFi (`context deadline exceeded`), causing LoadBalancer IPs to become unreachable. Fix: restart Cilium daemonset.
