# Observability

OpenTelemetry を中心に、メトリクス・トレース・ログを統一的に収集・保存・可視化する。

クラスタ健全性（ノード死活・probe・dead-man's switch）の設計は [cluster-health-monitoring.md](./cluster-health-monitoring.md) を参照。

## 監視の全体像

監視は「**何を観るか**（テレメトリ 3 本柱）」と「**どう観るか**（健全性 3 層）」の 2 軸で構成される。

```
   何を観るか (テレメトリ)               どう観るか (健全性監視の層)
┌──────────────────────────┐   ┌────────────────────────────────────────┐
│ Metrics → Prometheus     │   │ ① 受動: scrape できたか                │
│ Traces  → Tempo          │   │    node-exporter / kube-state-metrics  │
│ Logs    → Loki           │   │ ② 能動: 外から叩いて応答するか        │
│      (OTel Collector 経由)│   │    blackbox-exporter (ICMP/HTTPS)      │
└────────────┬─────────────┘   │ ③ 監視の監視: 監視自体は生きてるか    │
             │                  │    Grafana Cloud remote_write          │
             ▼                  │    + no-data アラート                  │
       Grafana (可視化)         └────────────────┬───────────────────────┘
             │                                   │
             ▼                                   ▼
   Alertmanager → Slack #k8s-alerts    Grafana Cloud → メール
   (クラスタ内で完結する通知)           (クラスタ外からの dead-man's switch)
```

| 層 | 主担当 | 検知できる障害の例 | 限界 |
|---|---|---|---|
| ① 受動 | kube-prometheus-stack (node-exporter, KSM) + Cluster Health ダッシュボード | リソース枯渇、NotReady、Pi 温度 / microSD 残量、Pod 異常 | scrape 不能の原因 (OS 死亡 vs kubelet 死亡) を区別できない |
| ② 能動 | blackbox-exporter + Probe CRD | OS 死活、有線断 → WiFi 縮退、Gateway/cert/認証経路のエンドツーエンド断 | probe 自体がクラスタ内なので、クラスタ全停止では沈黙 |
| ③ 監視の監視 | Grafana Cloud free tier (厳選 ~1.4k series を remote_write) | 監視スタックごと沈黙 (m4neo SPOF)、宅内回線断 | 詳細メトリクスは送っていない (一望ビュー相当のみ) |

3 層が互いの限界を埋める設計。導入経緯と Phase 分割は [cluster-health-monitoring.md](./cluster-health-monitoring.md)、実例は m4neo 障害 #2（8 日間の沈黙、層 ③ の動機）と 2026-06-06 Vault OIDC incident（認証経路の 3 週間潜伏、層 ② の HTTPS probe 対象選定の根拠）。

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

probe ベースのアラート (`kubernetes/observability/blackbox-exporter/resources/blackbox-alerts.yaml`):

- `NodeICMPUnreachable`: 有線 + WiFi 両系統 ICMP 断 5 分 (critical、ノード物理死)
- `NodeWiredLinkDown`: 有線断 + WiFi 生存 10 分 (warning、fallback 縮退運転)
- `PlatformEndpointDown`: platform HTTPS probe 断 5 分 (warning、経路レベル故障)

クラスタ外の通知 (dead-man's switch) は Grafana Cloud 側の no-data アラートが担当（上記「監視の全体像」参照）。Alertmanager 経由ではない。

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
