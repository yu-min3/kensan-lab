# Cluster Health Monitoring design

A design for surfacing kensan-lab's health (node liveness, resources, endpoint reachability) in a single Grafana view and notifying Slack on anomalies. Builds in three phases on top of the existing observability stack ([observability.md](./observability.md)).

## Background and gaps

The existing stack **already has the data path and alert path in place**:

- node-exporter (DaemonSet) + kube-state-metrics → Prometheus (7d retention)
- kube-prometheus-stack's defaultRules (`node` / `nodeExporterAlerting` enabled) → NodeNotReady and similar already notify Slack's `#k8s-alerts`
- Grafana's sidecar auto-imports ConfigMap dashboards (existing: controlplane / longhorn / claude-code / otel-apm)

What's missing:

| # | Gap | Phase |
|---|------|-----------|
| 1 | No single dashboard for overall cluster health (node liveness, temperature, remaining microSD capacity, wired/WiFi state) | Phase 1 |
| 2 | Liveness monitoring only means "could Prometheus scrape it." Can't distinguish a dead kubelet from a dead OS, or a wired-link drop from a total outage. HTTP reachability of each UI through the Gateway is also unmonitored | Phase 2 |
| 3 | **The monitoring stack itself is an m4neo SPOF**. Prometheus / Alertmanager / Grafana all require `hardware-class=high-performance`, which pins them to m4neo. If m4neo dies, monitoring goes silent along with it — no alert fires | Phase 3 |

## Phase 1 — Cluster Health dashboard

No deployment changes. Adds a single dashboard using only existing metrics.

- **Location**: `kubernetes/observability/grafana/resources/cluster-health-dashboard.yaml` (a ConfigMap with `grafana_dashboard: "1"` + `grafana_folder: "Platform"`; same shape as the controlplane-dashboard)
- **Approach**: hand-built rather than importing a community dashboard (e.g. Node Exporter Full) — tuned to a density where a 4-node homelab is legible at a glance

### Panel layout

**Row 1: Node liveness**

| Panel | Query | Type |
|--------|--------|----|
| Node Ready | `kube_node_status_condition{condition="Ready",status="true"}` | stat (per node, green/red) |
| node-exporter up history | `up{job="node-exporter"}` | state timeline |
| Node Uptime | `node_time_seconds - node_boot_time_seconds` | stat |
| Pressure conditions | `kube_node_status_condition{condition=~"MemoryPressure\|DiskPressure\|PIDPressure",status="true"}` | table / stat |

**Row 2: Resources**

| Panel | Query | Notes |
|--------|--------|------|
| CPU usage | `1 - avg by(instance)(rate(node_cpu_seconds_total{mode="idle"}[5m]))` | Per-node timeseries |
| Memory usage | `1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes` | Per-node timeseries |
| Disk usage (root) | `1 - node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{...}` | **Primary purpose: catching microSD exhaustion on the Pis**. Thresholds: yellow at 80%, red at 90% |
| CPU temperature | `node_hwmon_temp_celsius` (falls back to `node_thermal_zone_temp` if unavailable on the Pis) | Watches for thermal throttling on the Pis (80°C threshold) |

**Row 3: Network (visualizing the wired/WiFi dual-path setup)**

| Panel | Query | Notes |
|--------|--------|------|
| Wired link state | `node_network_carrier{device=~"eth0\|eno1\|enp4s0"}` | Detects a wired drop that fell back to WiFi |
| WiFi link state | `node_network_carrier{device=~"wlan0\|wlp3s0"}` | |
| Interface traffic | `rate(node_network_receive_bytes_total{device=~"eth.*\|en.*\|wlan.*\|wlp.*"}[5m])` | Shows which path traffic is actually flowing over |

**Row 4: Cluster overview**

| Panel | Query | Notes |
|--------|--------|------|
| Abnormal pod count | `sum(kube_pod_status_phase{phase=~"Pending\|Failed\|Unknown"})` | stat, green at 0 |
| Container restarts | `sum by(namespace)(increase(kube_pod_container_status_restarts_total[1h]))` | Early detection of CrashLoops |
| PVC usage | `kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes` | Covers both Longhorn and local-path |
| Pods per node | `sum by(node)(kube_pod_info)` | Checking for scheduling skew |

Things to confirm during implementation:

- [ ] Confirm the actual temperature metric name on the Pis (`node_hwmon_temp_celsius` vs `node_thermal_zone_temp`) against live Prometheus
- [ ] Confirm `node_network_carrier` is available on the WiFi interfaces (fall back to `node_network_up` if not)

## Phase 2 — blackbox-exporter (probe-based liveness monitoring)

Adds active ICMP / HTTP probes on top of Prometheus's internal-view scraping.

- **Location**: `kubernetes/observability/blackbox-exporter/` (`config.json` + `values.yaml`). Auto-discovered by the observability ApplicationSet (git file generator)
- **Chart**: `prometheus-community/prometheus-blackbox-exporter`
- **Note**: ICMP probes need the `NET_RAW` capability (granted via the chart's `securityContext`)

### Probe targets

Defined via the Prometheus Operator's `Probe` CRD (placed in `resources/`):

| Type | Target | What it reveals |
|------|------|-----------|
| ICMP | Wired IPs: 192.168.0.107–110 | OS-level liveness (distinguishes this from a dead kubelet) |
| ICMP | WiFi IPs: 192.168.0.207–210 | Whether the WiFi fallback is alive when the wired link drops |
| HTTP | `https://{grafana,argocd,backstage,vault,prometheus}.platform.yu-min3.com` | End-to-end reachability through Gateway → cert → backend |

### New alerts (PrometheusRule)

| Alert | Condition | Severity |
|----------|------|----------|
| NodeICMPUnreachable | Both wired and WiFi ICMP probes fail, sustained 5 minutes | critical |
| NodeWiredLinkDown | Wired probe fails & WiFi probe succeeds, sustained 10 minutes | warning (running on fallback) |
| PlatformEndpointDown | HTTP probe fails, sustained 5 minutes | warning |

Add a probe-results row to the dashboard (update the Phase 1 JSON).

## Phase 3 — Grafana Cloud remote_write (mitigating the m4neo SPOF)

Puts "monitoring the monitoring, plus a dashboard that stays visible during an outage" outside the cluster. Changed from the original plan of healthchecks.io (Watchdog forwarding only) to a **filtered remote_write into Grafana Cloud's free tier** — this adds a dead-man's-switch (no-data alert) plus the ability to "view metrics from just before a total cluster outage, from outside the cluster."

- **Mechanism**: local Prometheus scraping stays entirely in-cluster as before; `remote_write` (outbound HTTPS push, no inbound needed) replicates a carefully-chosen subset of series to Mimir on Grafana Cloud (Tokyo region)
- **Free tier**: 10k active series / 14-day retention / 3 users. The allowlist measures out to ~1.4k series (14%) in practice
- **Implementation**:
  1. Create a Grafana Cloud account + stack (manual, no credit card required). Issue a remote_write token (`metrics:write` scope) into Vault static (`secret/monitoring/grafana-cloud/remote-write`)
  2. Sync it into a `grafana-cloud-remote-write` Secret (`monitoring` namespace) via ESO
  3. Add the endpoint + basicAuth + `writeRelabelConfigs` to `prometheusSpec.remoteWrite` (keep the cluster-health dashboard's series, drop non-idle CPU modes)
  4. Replicate the Cluster Health dashboard on the Cloud side (only the datasource uid needs to change)
  5. Set up a **no-data alert** on the Cloud side (`up{job="node-exporter"}` no longer arriving) — this is the dead-man's-switch
- **Failure modes covered**: m4neo going down, Prometheus / Alertmanager stopping, a total cluster outage, home network / ISP outage
- **Candidate Phase 4 (optional)**: an external HTTP probe via Cloudflare Workers cron + a public status page (to distinguish a home-ISP outage from a Gateway failure from an outside vantage point)

## PR split

| PR | Content | Deploy impact |
|----|------|-------------|
| 1 | This design doc + Phase 1 dashboard | None (ConfigMap addition only) |
| 2 | Phase 2 blackbox-exporter + Probes + alerts + dashboard update | One new Pod (Deployment) |
| 3 | Phase 3 Grafana Cloud remote_write (ExternalSecret + values.yaml) | Prometheus config reload |

## Related

- Overview of the existing stack: [observability.md](./observability.md)
- Node topology and labels: `.claude/rules/kubernetes-cluster.md`
- The wired/WiFi dual-path setup: [network.md](./network.md)
