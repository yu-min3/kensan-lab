# observability

OpenTelemetry を中心としたメトリクス・トレース・ログ + Grafana で可視化。`monitoring` ns に集約。

## 構成

| dir | 役割 |
|---|---|
| `otel-collector/` | アプリからのテレメトリ集約 (OTLP gRPC/HTTP)、3 パイプラインを Prometheus / Tempo / Loki に分配 |
| `prometheus/` | メトリクス TSDB + Alertmanager (kube-prometheus-stack)。Slack 通知 + Grafana Cloud remote_write |
| `tempo/` | 分散トレーシング (Single Binary、OTLP 受信、TraceID query、7 日保持) |
| `loki/` | ログ集約 (SingleBinary、HTTP push、LogQL、7 日保持) |
| `grafana/` | ダッシュボード + Explore + Alerting (Keycloak OIDC で直結) |
| `blackbox-exporter/` | 能動 probe (ノード ICMP 有線/WiFi、platform HTTPS)。Probe CRD で対象宣言 |

ApplicationSet で量産 (`applications/observability/applicationset.yaml`、`config.json` でパラメータ化)。

## 健全性監視の 3 層

| 層 | 問い | 実装 | 検知できるもの |
|---|---|---|---|
| 受動 | scrape できたか | node-exporter / kube-state-metrics → Cluster Health ダッシュボード | リソース枯渇、NotReady、温度、Pod 異常 |
| 能動 | 外から叩いて応答するか | blackbox-exporter (ICMP / HTTPS probe) | OS 死活、有線断→WiFi 縮退、Gateway 経路断、認証経路の潜伏故障 |
| 監視の監視 | 監視自体は生きてるか | Grafana Cloud remote_write (~1.4k series) + no-data アラート | 監視スタックごと沈黙 (m4neo SPOF)、回線断 |

## アプリ側の最低限の統合

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector-opentelemetry-collector.monitoring.svc:4318
OTEL_SERVICE_NAME=<app>
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=<env>
```

## 関連

- アーキ全体図 / パイプライン詳細 / SDK 例 / トラブルシューティング: [`docs/architecture/observability.md`](../../docs/architecture/observability.md)
- 健全性監視 3 層の設計 (Phase 1-3): [`docs/architecture/cluster-health-monitoring.md`](../../docs/architecture/cluster-health-monitoring.md)
- アラートルール: `prometheus/resources/apiserver-etcd-alerts.yaml`、`blackbox-exporter/resources/blackbox-alerts.yaml`
- 各 component の固有設定: 配下の README (`otel-collector/`, `tempo/`)
