# Observability

OpenTelemetry を中心に、メトリクス・トレース・ログを統一的に収集・保存・可視化する。

クラスタ健全性（ノード死活・probe・dead-man's switch）の設計は [cluster-health-monitoring.md](./cluster-health-monitoring.md) を参照。

## アーキテクチャ

```
┌─────────────────────────────────────────────────────────────────┐
│                        Applications                              │
│              (Instrumented with OpenTelemetry SDK)               │
└────────────────────────┬────────────────────────────────────────┘
                         │ OTLP Protocol
                         │ (gRPC: 4317 / HTTP: 4318)
                         ↓
┌─────────────────────────────────────────────────────────────────┐
│              OpenTelemetry Collector                             │
│                   (monitoring namespace)                         │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Receivers  │  │  Processors  │  │   Exporters  │         │
│  ├──────────────┤  ├──────────────┤  ├──────────────┤         │
│  │ OTLP gRPC    │→ │ Memory       │→ │ Prometheus   │         │
│  │ OTLP HTTP    │  │ Limiter      │  │ Remote Write │         │
│  └──────────────┘  │ Batch        │  │              │         │
│                     │ Resource     │  │ Tempo (OTLP) │         │
│                     │ Attributes   │  │              │         │
│                     └──────────────┘  │ Loki         │         │
│                                        └──────────────┘         │
└────────┬────────────────┬────────────────┬────────────────────┘
         │                │                │
         │ Metrics        │ Traces         │ Logs
         ↓                ↓                ↓
┌────────────────┐ ┌─────────────┐ ┌──────────────┐
│  Prometheus    │ │    Tempo    │ │     Loki     │
│  - Pull模型    │ │ - OTLP受信  │ │ - HTTP受信   │
│  - PromQL      │ │ - 7日保持   │ │ - 7日保持    │
└────────┬───────┘ └──────┬──────┘ └──────┬───────┘
         │                │                │
         └────────────────┼────────────────┘
                          ↓
                   ┌──────────────┐
                   │   Grafana    │
                   │ - Dashboard  │
                   │ - Explore    │
                   │ - Alerting   │
                   └──────────────┘
```

### アラートパイプライン

```
Prometheus
  → AlertRules (PrometheusRule CR)
  → Alertmanager
      ├── route: Watchdog, InfoInhibitor → null
      └── route: default → slack-default
            → #k8s-alerts チャンネル
```

カスタムアラートルール (`kubernetes/observability/prometheus/resources/apiserver-etcd-alerts.yaml`):

- `KubeApiserverHighLatency`: apiserver LIST/WATCH の p99 > 5s
- `CiliumOperatorCrashLooping`: Cilium operator の再起動 > 3 回/1h
- `EtcdSlowWalFsync`: etcd WAL fsync p99 > 100ms (microSD 劣化の先行指標)
- `EtcdSlowBackendCommit`: etcd backend commit p99 > 250ms

## コンポーネント

### OpenTelemetry Collector

テレメトリーの中央集約・処理・転送。Deployment 1 replica、`monitoring` ns、`otel/opentelemetry-collector-contrib:0.93.0`。

3 つのパイプライン:

```yaml
# Metrics
receivers: [otlp]
processors: [memory_limiter, batch, resource]
exporters: [prometheusremotewrite]

# Traces (attributes processor で機密情報削除)
receivers: [otlp]
processors: [memory_limiter, batch, resource, attributes]
exporters: [otlp/tempo]

# Logs
receivers: [otlp]
processors: [memory_limiter, batch, resource]
exporters: [loki]
```

全パイプラインに `cluster.name=kensan-lab` と `deployment.environment=<k8s.namespace.name>` を付加。

詳細: [`kubernetes/observability/otel-collector/README.md`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/observability/otel-collector)

### Prometheus + Alertmanager

`kube-prometheus-stack` で Prometheus + Alertmanager を同梱。`monitoring` ns。

- 受信: OTel Collector からの Remote Write + ServiceMonitor 自動スクレイピング
- ストレージ: Prometheus 10Gi + Alertmanager 2Gi (Longhorn)
- 保持期間: 7 日 (`retentionSize: 9GB`)
- Slack 通知先は SealedSecret (`resources/alertmanager-slack-sealed-secret.yaml`)
- 再通知間隔 4h、復旧通知あり

### Grafana Tempo

分散トレーシングバックエンド。Single Binary mode、`monitoring` ns、`grafana/tempo:2.3.1`、10Gi PVC。

- OTLP gRPC: `tempo.monitoring.svc:4317`
- OTLP HTTP: `tempo.monitoring.svc:4318`
- 保持期間: 7 日

詳細: [`kubernetes/observability/tempo/README.md`](https://github.com/yu-min3/kensan-lab/tree/main/kubernetes/observability/tempo)

### Grafana Loki

ログ集約。SingleBinary mode、`monitoring` ns、`grafana/loki:3.5.7`、10Gi PVC、TSDB v13 schema、保持 7 日。

受信: `http://loki.monitoring.svc:3100/loki/api/v1/push`

### Grafana

ダッシュボード / Explore / Alerting。Keycloak OIDC で直結 (`auth.generic_oauth`)。

## アプリケーション側の統合

OpenTelemetry SDK の最低限の環境変数:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector-opentelemetry-collector.monitoring.svc:4318
OTEL_SERVICE_NAME=my-application
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production
```

### Python の例

```python
from opentelemetry import trace, metrics
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.resources import Resource

resource = Resource.create({
    "service.name": "my-application",
    "deployment.environment": "production"
})

trace.set_tracer_provider(TracerProvider(resource=resource))
trace.get_tracer_provider().add_span_processor(
    BatchSpanProcessor(OTLPSpanExporter())
)
metrics.set_meter_provider(MeterProvider(resource=resource))
```

## 運用

### ヘルスチェック

```bash
# OTel Collector
kubectl exec -n monitoring deployment/otel-collector-opentelemetry-collector -- wget -O- http://localhost:13133/

# Tempo
kubectl exec -n monitoring tempo-0 -c tempo -- wget -O- http://localhost:3200/ready

# Loki
kubectl exec -n monitoring loki-0 -c loki -- wget -O- http://localhost:3100/ready
```

### ストレージ確認

```bash
kubectl get pvc -n monitoring
kubectl exec -n monitoring tempo-0 -c tempo -- du -sh /var/tempo/traces /var/tempo/wal
kubectl exec -n monitoring loki-0 -c loki -- du -sh /var/loki/chunks /var/loki/wal
```

## トラブルシューティング

### データが届かない

1. アプリ Pod → OTel Collector の疎通: `kubectl exec <pod> -- curl http://otel-collector-opentelemetry-collector.monitoring.svc:4318/v1/traces`
2. OTel Collector のエラーログ: `kubectl logs -n monitoring deployment/otel-collector-opentelemetry-collector | grep -i error`
3. バックエンドの稼働: `kubectl get pods -n monitoring -l app.kubernetes.io/name=tempo` 等

### メトリクスが Grafana で見えない

- Prometheus の Remote Write 受信を確認
- ServiceMonitor の設定確認
- Grafana datasource の URL 確認

### トレースが見えない

- Tempo `/ready` 確認
- OTel Collector → Tempo の接続エラーチェック
- Tempo は TraceID クエリ。LogQL のように label 検索じゃない

### ログが見えない

- Loki `/ready` 確認
- LogQL の label selector を確認 (`{namespace="kensan"}` のように)

## パフォーマンスチューニング

### OTel Collector

```yaml
# バッチ
batch:
  send_batch_size: 1000   # 小: low-latency, 大: high-throughput
  timeout: 10s

# メモリ
memory_limiter:
  limit_mib: 512   # Pod memory limit より小さく
```

### Tempo / Loki

```yaml
# Tempo
ingester:
  max_block_bytes: 1_000_000
  max_block_duration: 5m

# Loki
limits_config:
  ingestion_rate_mb: 10
  ingestion_burst_size_mb: 20
```

## セキュリティ

- 全 Service は ClusterIP (外部非公開、UI は Istio Gateway 経由のみ)
- OTel Collector で `http.request.header.authorization` を attributes processor で削除
- CPU / Memory limits 全コンポーネント設定済み (各 500m / 1Gi)

## 将来の拡張

- **分散モード**: トラフィック増で `tempo-distributed` / `loki-distributed` chart に切替
- **オブジェクトストレージ**: Tempo / Loki を S3 (R2) backend に切替
- **追加 dashboard**: Argo CD、Cilium、Vault などのオフィシャル dashboard インポート
