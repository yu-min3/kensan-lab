# Cilium WiFi cluster stability tuning

> **Updated 2026-05-07**: kensan-lab has since moved to wired LAN as the primary network. WiFi is kept as a fallback, so the relaxed values documented here (`leaseDuration: 60s`, etc.) are still in effect (they give breathing room during a fallback failover). This document is kept as historical reference from the WiFi-only era.

On a homelab cluster connecting over WiFi (the RPi 5 fleet on `wlan0`, the Bosgame M4 Neo on `wlp3s0`), latency spikes frequently time out lease renewals and API calls. `kubernetes/network/cilium/values.yaml` relaxes several defaults to accommodate this.

## API client rate limit

Raise Cilium's client rate limit against the API server. At the default (qps=10 / burst=20), a WiFi spike causes throttling, and the reconcile burst after a Pod recovers then makes the apiserver unreachable.

```yaml
k8sClientRateLimit:
  qps: 20
  burst: 40
```

## L2 announcement lease timing

The LoadBalancer IP's lease renewal was frequently throwing `context deadline exceeded` over WiFi, so these were relaxed step by step until landing here:

```yaml
l2announcements:
  enabled: true
  leaseDuration: 60s
  leaseRenewDeadline: 40s
  leaseRetryPeriod: 8s
```

### Trial history

| Setting | Result |
|---|---|
| Cilium chart default (15s / 10s / 2s) | Lease renewal failures every few minutes → LoadBalancer IP unreachable |
| 30s / 20s / 4s | Improved, but a timeout still occurred roughly once every few dozen minutes |
| 60s / 40s / 8s (current) | Stable |

### Trade-off

Extending renewDeadline means a longer delay before IP take-over if the lease-holder Pod crashes. At homelab scale (4 nodes), that's well within tolerance. On a production-grade cluster, staying closer to the default is recommended.

## Related

- [Cilium update strategy](./cilium-update-strategy.md)
- Known issue: Cilium L2 lease instability over WiFi (also documented in CLAUDE.md's Known Issue section)
