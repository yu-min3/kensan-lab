# observability

OpenTelemetry を中心としたメトリクス・トレース・ログ + Grafana で可視化。`monitoring` ns に集約。

## 構成

| dir | 役割 |
|---|---|
| `otel-collector/` | アプリからのテレメトリ集約 (OTLP gRPC/HTTP)、3 パイプラインを Prometheus / Tempo / Loki に分配 |
| `prometheus/` | メトリクス TSDB + Alertmanager (kube-prometheus-stack)。Slack 通知 |
| `tempo/` | 分散トレーシング (Single Binary、OTLP 受信、TraceID query、7 日保持) |
| `loki/` | ログ集約 (SingleBinary、HTTP push、LogQL、7 日保持) |
| `grafana/` | ダッシュボード + Explore + Alerting (Keycloak OIDC で直結) |

ApplicationSet で量産 (`applications/observability/applicationset.yaml`、`config.json` でパラメータ化)。

## アプリ側の最低限の統合

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector-opentelemetry-collector.monitoring.svc:4318
OTEL_SERVICE_NAME=<app>
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=<env>
```

## 関連

- アーキ全体図 / パイプライン詳細 / SDK 例 / トラブルシューティング: [`docs/architecture/observability.md`](../../docs/architecture/observability.md)
- アラートルール: `prometheus/resources/apiserver-etcd-alerts.yaml`
- 各 component の固有設定: 配下の README (`otel-collector/`, `tempo/`)
