# Cluster Health Monitoring 設計

kensan-lab の健全性（ノード死活・リソース・エンドポイント疎通）を Grafana で一望し、異常を Slack に通知するための設計。既存の observability スタック（[observability.md](./observability.md)）の上に 3 フェーズで積む。

## 背景と課題

既存スタックで **データとアラート経路はすでに揃っている**:

- node-exporter（DaemonSet）+ kube-state-metrics → Prometheus（retention 7d）
- kube-prometheus-stack の defaultRules（`node` / `nodeExporterAlerting` 有効）→ NodeNotReady 等は Slack `#k8s-alerts` に通知済み
- Grafana sidecar による ConfigMap ダッシュボード自動取込（既存: controlplane / longhorn / claude-code / otel-apm）

欠けているもの:

| # | 課題 | 対応 Phase |
|---|------|-----------|
| 1 | クラスタ健全性を一望するダッシュボードがない（ノード死活・温度・microSD 残量・有線/WiFi 状態） | Phase 1 |
| 2 | 死活監視が「Prometheus が scrape できたか」のみ。kubelet 死亡と OS 死亡、有線断と完全断の区別ができない。Gateway 経由の各 UI の HTTP 疎通も未監視 | Phase 2 |
| 3 | **監視スタック自体が m4neo SPOF**。Prometheus / Alertmanager / Grafana は `hardware-class=high-performance` 必須 = m4neo 固定。m4neo が死ぬと監視ごと沈黙し、アラートも飛ばない | Phase 3 |

## Phase 1 — Cluster Health ダッシュボード

デプロイ変更なし。既存メトリクスのみでダッシュボードを 1 枚追加する。

- **配置**: `kubernetes/observability/grafana/resources/cluster-health-dashboard.yaml`（ConfigMap、`grafana_dashboard: "1"` + `grafana_folder: "Platform"`。controlplane-dashboard と同形式）
- **方針**: コミュニティ製（Node Exporter Full 等）の import ではなく自作。4 ノード homelab に合わせ「一目で全体が分かる」密度に絞る

### パネル構成

**Row 1: ノード死活**

| パネル | クエリ | 型 |
|--------|--------|----|
| Node Ready | `kube_node_status_condition{condition="Ready",status="true"}` | stat（ノード別、緑/赤） |
| node-exporter up 履歴 | `up{job="node-exporter"}` | state timeline |
| Node Uptime | `node_time_seconds - node_boot_time_seconds` | stat |
| Pressure conditions | `kube_node_status_condition{condition=~"MemoryPressure\|DiskPressure\|PIDPressure",status="true"}` | table / stat |

**Row 2: リソース**

| パネル | クエリ | 備考 |
|--------|--------|------|
| CPU 使用率 | `1 - avg by(instance)(rate(node_cpu_seconds_total{mode="idle"}[5m]))` | ノード別 timeseries |
| メモリ使用率 | `1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes` | ノード別 timeseries |
| Disk 使用率 (root) | `1 - node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{...}` | **Pi の microSD 枯渇検知が主目的**。閾値 80% 黄 / 90% 赤 |
| CPU 温度 | `node_hwmon_temp_celsius` （Pi で取れない場合 `node_thermal_zone_temp` に fallback） | Pi のサーマルスロットリング監視（80°C 閾値） |

**Row 3: ネットワーク（有線/WiFi 2 系統運用の可視化）**

| パネル | クエリ | 備考 |
|--------|--------|------|
| 有線リンク状態 | `node_network_carrier{device=~"eth0\|eno1\|enp4s0"}` | 有線断で WiFi fallback に落ちたことを検知 |
| WiFi リンク状態 | `node_network_carrier{device=~"wlan0\|wlp3s0"}` | |
| Interface トラフィック | `rate(node_network_receive_bytes_total{device=~"eth.*\|en.*\|wlan.*\|wlp.*"}[5m])` | どの経路でトラフィックが流れているか |

**Row 4: クラスタ概況**

| パネル | クエリ | 備考 |
|--------|--------|------|
| 異常 Pod 数 | `sum(kube_pod_status_phase{phase=~"Pending\|Failed\|Unknown"})` | stat、0 で緑 |
| コンテナ再起動 | `sum by(namespace)(increase(kube_pod_container_status_restarts_total[1h]))` | CrashLoop の早期発見 |
| PVC 使用率 | `kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes` | Longhorn / local-path 双方 |
| Pod 数 / ノード | `sum by(node)(kube_pod_info)` | 偏り確認 |

実装時の確認事項:

- [ ] Pi 上の温度メトリクス名（`node_hwmon_temp_celsius` か `node_thermal_zone_temp`）を実機 Prometheus で確認
- [ ] `node_network_carrier` が WiFi インターフェースで取れるか確認（取れなければ `node_network_up`）

## Phase 2 — blackbox-exporter（probe ベースの死活監視）

Prometheus 内部視点に加え、ICMP / HTTP の能動 probe を追加する。

- **配置**: `kubernetes/observability/blackbox-exporter/`（`config.json` + `values.yaml`）。observability ApplicationSet（git file generator）が自動検出
- **Chart**: `prometheus-community/prometheus-blackbox-exporter`
- **注意**: ICMP probe には `NET_RAW` capability が必要（chart の `securityContext` で付与）

### Probe 対象

Prometheus Operator の `Probe` CRD で定義（`resources/` に配置）:

| 種別 | 対象 | 分かること |
|------|------|-----------|
| ICMP | 有線 IP: 192.168.0.107–110 | OS レベルの死活（kubelet 死亡と区別） |
| ICMP | WiFi IP: 192.168.0.207–210 | 有線断時に WiFi fallback が生きているか |
| HTTP | `https://{grafana,argocd,backstage,vault,prometheus}.platform.yu-min3.com` | Gateway → cert → backend のエンドツーエンド疎通 |

### アラート追加（PrometheusRule）

| アラート | 条件 | severity |
|----------|------|----------|
| NodeICMPUnreachable | 有線 + WiFi 両系統の ICMP probe 失敗 5 分継続 | critical |
| NodeWiredLinkDown | 有線 probe 失敗 & WiFi probe 成功 10 分継続 | warning（fallback 運用中） |
| PlatformEndpointDown | HTTP probe 失敗 5 分継続 | warning |

ダッシュボードに probe 結果の Row を追加（Phase 1 の JSON を更新）。

## Phase 3 — Dead-man's switch（m4neo SPOF 対策）

「監視の監視」をクラスタ外部に置く。

- **仕組み**: kube-prometheus-stack の `Watchdog` アラート（常時発火するハートビート、現在 `null` receiver に捨てている）を [healthchecks.io](https://healthchecks.io)（無料枠）へ転送。ping が途絶えたら healthchecks.io 側からメール / Slack 通知
- **実装**:
  1. healthchecks.io で check を作成（period 5m / grace 5m 程度）
  2. ping URL を Vault static（`secret/monitoring/alertmanager/healthchecks`）に格納、ESO で Alertmanager secret に同期（既存 `alertmanager-slack-secret` と同パターン）
  3. Alertmanager config の `Watchdog` route を `null` → `healthchecks` receiver（`webhook_configs` + `url_file`）に変更。`repeat_interval: 1m`
- **カバーされる障害**: m4neo ダウン、Prometheus / Alertmanager 停止、クラスタ全体停止、宅内ネットワーク / 回線断

## PR 分割

| PR | 内容 | デプロイ影響 |
|----|------|-------------|
| 1 | 本設計 doc + Phase 1 ダッシュボード | なし（ConfigMap 追加のみ） |
| 2 | Phase 2 blackbox-exporter + Probe + アラート + ダッシュボード更新 | 新 Pod 1（Deployment） |
| 3 | Phase 3 Watchdog → healthchecks.io | Alertmanager config 変更 |

## 関連

- 既存スタック全体像: [observability.md](./observability.md)
- ノード構成・ラベル: `.claude/rules/kubernetes-cluster.md`
- ネットワーク 2 系統運用: [network.md](./network.md)
