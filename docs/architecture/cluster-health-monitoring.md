# Cluster Health Monitoring design

The design for seeing kensan-lab's health at a glance in Grafana (node liveness, resources, endpoint reachability) and notifying Slack on anomalies. Built in 3 phases on top of the existing observability stack ([observability.md](./observability.md)).

## Background and gaps

The existing stack **already has the data and the alert path**:

- node-exporter (DaemonSet) + kube-state-metrics → Prometheus (retention 7d)
- kube-prometheus-stack defaultRules (`node` / `nodeExporterAlerting` enabled) → NodeNotReady etc. already notify Slack `#k8s-alerts`
- Grafana sidecar auto-imports ConfigMap dashboards (existing: controlplane / longhorn / claude-code / otel-apm)

What was missing:

| # | Gap | Phase |
|---|------|-----------|
| 1 | No single dashboard for cluster health (node liveness, temperature, microSD headroom, wired/WiFi link state) | Phase 1 |
| 2 | Liveness monitoring is only "did Prometheus scrape succeed". Can't distinguish kubelet death from OS death, or a wired-link drop from a total outage. HTTP reachability of the UIs behind the Gateway is unmonitored | Phase 2 |
| 3 | **The monitoring stack itself is a single-node SPOF**. Prometheus / Alertmanager / Grafana require `hardware-class=high-performance` = pinned to one node. If that node dies, monitoring goes silent with it — no alert fires | Phase 3 |

## Phase 1 — Cluster Health dashboard

No deployment changes; one new dashboard built purely from existing metrics.

- **Location**: `kubernetes/observability/grafana/resources/cluster-health-dashboard.yaml` (ConfigMap, `grafana_dashboard: "1"` + `grafana_folder: "Platform"`, same format as controlplane-dashboard)
- **Approach**: hand-built rather than importing community dashboards (Node Exporter Full etc.) — tuned to a 4-node homelab with "the whole picture in one glance" density

### Panel layout

**Row 1: node liveness**

| Panel | Query | Type |
|--------|--------|----|
| Node Ready | `kube_node_status_condition{condition="Ready",status="true"}` | stat (per node, green/red) |
| node-exporter up history | `up{job="node-exporter"}` | state timeline |
| Node Uptime | `node_time_seconds - node_boot_time_seconds` | stat |
| Pressure conditions | `kube_node_status_condition{condition=~"MemoryPressure\|DiskPressure\|PIDPressure",status="true"}` | table / stat |

**Row 2: resources**

| Panel | Query | Notes |
|--------|--------|------|
| CPU usage | `1 - avg by(instance)(rate(node_cpu_seconds_total{mode="idle"}[5m]))` | per-node timeseries |
| Memory usage | `1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes` | per-node timeseries |
| Disk usage (root) | `1 - node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{...}` | **primarily detects Pi microSD exhaustion**; thresholds 80% yellow / 90% red |
| CPU temperature | `node_hwmon_temp_celsius` (fallback to `node_thermal_zone_temp` if the Pi doesn't expose it) | thermal-throttling watch for the Pis (80°C threshold) |

**Row 3: network (visualizing the dual wired/WiFi setup)**

| Panel | Query | Notes |
|--------|--------|------|
| Wired link state | `node_network_carrier{device=~"eth0\|eno1\|enp4s0"}` | detects a wired drop falling back to WiFi |
| WiFi link state | `node_network_carrier{device=~"wlan0\|wlp3s0"}` | |
| Interface traffic | `rate(node_network_receive_bytes_total{device=~"eth.*\|en.*\|wlan.*\|wlp.*"}[5m])` | which path traffic is flowing over |

**Row 4: cluster overview**

| Panel | Query | Notes |
|--------|--------|------|
| Abnormal pod count | `sum(kube_pod_status_phase{phase=~"Pending\|Failed\|Unknown"})` | stat, green at 0 |
| Container restarts | `sum by(namespace)(increase(kube_pod_container_status_restarts_total[1h]))` | early CrashLoop detection |
| PVC usage | `kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes` | Longhorn and local-path alike |
| Pods per node | `sum by(node)(kube_pod_info)` | skew check |

Implementation checkpoints:

- [ ] Confirm the temperature metric name on the Pis (`node_hwmon_temp_celsius` vs `node_thermal_zone_temp`) against live Prometheus
- [ ] Confirm `node_network_carrier` works for WiFi interfaces (fall back to `node_network_up` if not)

## Phase 2 — blackbox-exporter (probe-based liveness)

Adds active ICMP / HTTP probes on top of the Prometheus-internal view.

- **Location**: `kubernetes/observability/blackbox-exporter/` (`config.json` + `values.yaml`). Auto-discovered by the observability ApplicationSet (git file generator)
- **Chart**: `prometheus-community/prometheus-blackbox-exporter`
- **Note**: ICMP probes need the `NET_RAW` capability (granted via the chart's `securityContext`)

### Probe targets

Defined with Prometheus Operator's `Probe` CRD (placed in `resources/`):

| Kind | Target | What it tells you |
|------|------|-----------|
| ICMP | wired IPs: 192.168.0.107–110 | OS-level liveness (distinguished from kubelet death) |
| ICMP | WiFi IPs: 192.168.0.207–210 | whether the WiFi fallback is alive when the wired link drops |
| HTTP | `https://{grafana,argocd,backstage,vault,prometheus}.platform.yu-min3.com` | end-to-end reachability: Gateway → cert → backend |

### Added alerts (PrometheusRule)

| Alert | Condition | Severity |
|----------|------|----------|
| NodeICMPUnreachable | ICMP probes failing on wired *and* WiFi for 5 min | critical |
| NodeWiredLinkDown | wired probe failing while the WiFi probe succeeds for 10 min | warning (running on fallback) |
| PlatformEndpointDown | HTTP probe failing for 5 min | warning |

A probe-results row is added to the dashboard (updating the Phase 1 JSON).

## Phase 3 — Grafana Cloud remote_write (countering the single-node SPOF)

Puts "monitoring of the monitoring, plus a dashboard that stays visible during an outage" outside the cluster. Changed from the original healthchecks.io idea (Watchdog forwarding only) to **filtered remote_write into the Grafana Cloud free tier** — beyond the dead-man's switch (no-data alert), it also buys "metrics up to the moment of failure remain viewable externally even after a total cluster outage".

- **Mechanism**: the local Prometheus keeps scraping entirely in-cluster, and `remote_write` (outbound HTTPS push, no inbound required) replicates a curated set of series to Grafana Cloud's Mimir (Tokyo region)
- **Free tier**: 10k active series / 14-day retention / 3 users. The allowlist measures ~1.4k series (14%)
- **Implementation**:
  1. Create the Grafana Cloud account + stack (manual, no credit card). Issue a remote_write token (`metrics:write` scope) into Vault static (`secret/monitoring/grafana-cloud/remote-write`)
  2. Sync it via ESO into the `grafana-cloud-remote-write` Secret (monitoring ns)
  3. Configure `prometheusSpec.remoteWrite` with the endpoint + basicAuth + `writeRelabelConfigs` (keep the cluster-health dashboard's series, drop non-idle cpu modes)
  4. Replicate the Cluster Health dashboard on the Cloud side (only the datasource uid changes)
  5. Add a **no-data alert** on the Cloud side (notify when `up{job="node-exporter"}` stops arriving) = the dead-man's switch
- **Failures covered**: monitoring-node down, Prometheus / Alertmanager down, total cluster outage, home network / uplink loss
- **Phase 4 candidate (optional)**: external HTTP probes via a Cloudflare Workers cron + a public status page (external-viewpoint separation of "home uplink down" vs "Gateway broken")

### Current Grafana Cloud state

Verified via the Grafana Cloud API on 2026-07-10.

**Alert rule**

| Field | Value |
|---|---|
| Folder | `kensan-lab` |
| Rule group | `cluster-help` |
| Rule | `kensan-lab cluster silent` |
| Evaluation interval | `1m` |
| Pending period | `5m` |
| Datasource | `grafanacloud-prom` |
| Query | `up{job="node-exporter"}` |
| Condition | last value `< 0.5` |
| No data state | `Alerting` |
| Execution error state | `Error` |
| Receiver | `mymail` |

This is the external dead-man's switch for the homelab. It fires when the
node-exporter `up` series stops arriving in Grafana Cloud, which covers local
Prometheus failure, remote_write failure, total cluster loss, and home network
loss. The contact point is email-based; the address is intentionally not stored
in this repository.

**Dashboard**

Grafana Cloud has a custom dashboard named `Cluster Health (Cloud)` with UID
`cluster-health-cloud`. It mirrors the local Cluster Health dashboard at Cloud
datasource scope. The panels currently cover:

- node liveness, Node Ready, Node Uptime, and pressure conditions
- node-exporter `up` history
- CPU, memory, root disk, and CPU temperature
- wired link / WiFi fallback state and interface traffic
- abnormal pods, container restarts, PVC usage, and pod count per node

The other dashboards visible in the stack are Grafana Cloud built-ins such as
Usage Insights, Cardinality management, Billing/Usage, and Incident Insights.

## PR breakdown

| PR | Content | Deployment impact |
|----|------|-------------|
| 1 | this design doc + the Phase 1 dashboard | none (ConfigMap only) |
| 2 | Phase 2 blackbox-exporter + Probes + alerts + dashboard update | 1 new pod (Deployment) |
| 3 | Phase 3 Grafana Cloud remote_write (ExternalSecret + values.yaml) | Prometheus config reload |

## Related

- The stack this builds on: [observability.md](./observability.md)
- Real incidents that motivated layers ② and ③: [docs/incidents](../incidents/index.md)
- Node topology & labels: `.claude/rules/kubernetes-cluster.md`
- The dual wired/WiFi network: [network.md](./network.md)
