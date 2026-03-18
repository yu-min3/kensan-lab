# Observability Stack

## 概要

このプラットフォームの Observability スタックは、OpenTelemetry を中心とした統合的な可観測性基盤です。メトリクス・トレース・ログの3つの柱（Three Pillars of Observability）を統一的に収集・保存・可視化します。

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
│                │ │             │ │              │
│  - Pull模型    │ │ - OTLP受信  │ │ - HTTP受信   │
│  - Time Series │ │ - TraceID   │ │ - LogQL      │
│  - PromQL      │ │   索引      │ │ - Labels     │
│                │ │ - 7日保持   │ │ - 7日保持    │
└────────┬───────┘ └──────┬──────┘ └──────┬───────┘
         │                │                │
         │                │                │
         └────────────────┼────────────────┘
                          ↓
                   ┌──────────────┐
                   │   Grafana    │
                   │              │
                   │ - Dashboard  │
                   │ - Explore    │
                   │ - Alerting   │
                   └──────────────┘

### アラートパイプライン

```
Prometheus
  → AlertRules (PrometheusRule CR)
  → Alertmanager
      ├── route: Watchdog, InfoInhibitor → null (通知しない)
      └── route: default → slack-default
            → Slack Incoming Webhook
            → #k8s-alerts チャンネル
```

**カスタムアラートルール** (`resources/apiserver-etcd-alerts.yaml`):
- `KubeApiserverHighLatency`: apiserver LIST/WATCH の p99 > 5s
- `CiliumOperatorCrashLooping`: Cilium operator の再起動 > 3回/1h
- `EtcdSlowWalFsync`: etcd WAL fsync p99 > 100ms (microSD 劣化の先行指標)
- `EtcdSlowBackendCommit`: etcd backend commit p99 > 250ms

```

## コンポーネント

### 1. OpenTelemetry Collector

**役割**: テレメトリーデータの中央集約・処理・転送

**デプロイメント**:
- **Mode**: Deployment (1 replica)
- **Namespace**: monitoring
- **Image**: otel/opentelemetry-collector-contrib:0.93.0

**パイプライン**:

#### Metrics Pipeline
```yaml
receivers: [otlp]
processors: [memory_limiter, batch, resource]
exporters: [prometheusremotewrite]
```
- **受信**: OTLP gRPC/HTTP
- **処理**: メモリ制限、バッチ処理、リソース属性付加
- **転送**: Prometheus Remote Write API

#### Traces Pipeline
```yaml
receivers: [otlp]
processors: [memory_limiter, batch, resource, attributes]
exporters: [otlp/tempo]
```
- **受信**: OTLP gRPC/HTTP
- **処理**: メモリ制限、バッチ処理、リソース属性付加、機密情報削除
- **転送**: Tempo OTLP endpoint

#### Logs Pipeline
```yaml
receivers: [otlp]
processors: [memory_limiter, batch, resource]
exporters: [loki]
```
- **受信**: OTLP gRPC/HTTP
- **処理**: メモリ制限、バッチ処理、リソース属性付加
- **転送**: Loki HTTP API

**リソース属性付加**:
```yaml
cluster.name: kensan-lab
deployment.environment: <k8s.namespace.name>
```

**詳細**: [otel-collector/README.md](./otel-collector/README.md)

---

### 2. Prometheus + Alertmanager

**役割**: メトリクスの時系列データベース + アラート通知

**デプロイメント**:
- **Mode**: kube-prometheus-stack (Prometheus + Alertmanager 含む)
- **Namespace**: monitoring
- **Storage**: Prometheus 10Gi + Alertmanager 2Gi PVC (local-path)

**データソース**:
- OTel Collector からの Remote Write
- ServiceMonitor による自動スクレイピング（OTel Collector, Tempo, Loki 自身のメトリクス）

**保持期間**: 7日 (retentionSize: 9GB)

**Alertmanager 通知設定**:
- **通知先**: Slack Incoming Webhook (`api_url_file` で Secret から読み込み)
- **Webhook URL**: SealedSecret で管理 (`resources/alertmanager-slack-sealed-secret.yaml`)
- **ルーティング**: Watchdog/InfoInhibitor → null、その他 → Slack
- **再通知間隔**: 4時間
- **復旧通知**: 有効 (`send_resolved: true`)

---

### 3. Grafana Tempo

**役割**: 分散トレーシングバックエンド

**デプロイメント**:
- **Mode**: Single Binary (Monolithic)
- **Namespace**: monitoring
- **Image**: grafana/tempo:2.3.1
- **Storage**: 10Gi PVC (local-path)

**受信プロトコル**:
- OTLP gRPC: `tempo.monitoring.svc:4317`
- OTLP HTTP: `tempo.monitoring.svc:4318`

**保持期間**: 7日間（自動削除）

**機能**:
- TraceID ベースのクエリ
- WAL による耐障害性
- Compactor による自動圧縮

**詳細**: [tempo/README.md](./tempo/README.md)

---

### 4. Grafana Loki

**役割**: ログ集約システム

**デプロイメント**:
- **Mode**: SingleBinary (Monolithic)
- **Namespace**: monitoring
- **Image**: grafana/loki:3.5.7
- **Storage**: 10Gi PVC (local-path)

**受信エンドポイント**:
- HTTP API: `http://loki.monitoring.svc:3100/loki/api/v1/push`

**保持期間**: 7日間（自動削除）

**スキーマ**: TSDB v13

**機能**:
- LogQL によるクエリ
- Label ベースのインデックス
- Compactor による自動削除

---

## データフロー

### Metrics フロー
```
Application (SDK)
  → OTel Collector (OTLP 4317/4318)
  → Batch Processor (1000 items / 10s)
  → Resource Processor (cluster.name, deployment.environment)
  → Prometheus Remote Write (9090/api/v1/write)
  → Prometheus TSDB
  → Grafana Dashboard
```

### Traces フロー
```
Application (SDK)
  → OTel Collector (OTLP 4317/4318)
  → Batch Processor (1000 items / 10s)
  → Attributes Processor (http.request.header.authorization 削除)
  → Tempo OTLP (4317)
  → Tempo Ingester
  → WAL → Blocks → Storage (PVC)
  → Grafana Trace Viewer
```

### Logs フロー
```
Application (SDK)
  → OTel Collector (OTLP 4317/4318)
  → Batch Processor (1000 items / 10s)
  → Resource Processor (cluster.name, deployment.environment)
  → Loki HTTP API (3100/loki/api/v1/push)
  → Loki Ingester
  → WAL → Chunks → Storage (PVC)
  → Grafana Log Viewer (LogQL)
```

## アプリケーション統合

### OpenTelemetry SDK の設定

#### 環境変数
```bash
# OTel Collector エンドポイント
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector-opentelemetry-collector.monitoring.svc:4318

# サービス名
OTEL_SERVICE_NAME=my-application

# リソース属性
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production
```

#### Python 例
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

# Traces
trace.set_tracer_provider(TracerProvider(resource=resource))
trace.get_tracer_provider().add_span_processor(
    BatchSpanProcessor(OTLPSpanExporter())
)

# Metrics
metrics.set_meter_provider(MeterProvider(resource=resource))
```

### サービス検出

アプリケーション Podから OTel Collector にアクセス:
```yaml
# アプリケーション Deployment
env:
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector-opentelemetry-collector.monitoring.svc:4318"
```

## 運用

### ヘルスチェック

#### OTel Collector
```bash
kubectl exec -n monitoring deployment/otel-collector-opentelemetry-collector -- \
  wget -O- http://localhost:13133/
```

#### Tempo
```bash
kubectl exec -n monitoring tempo-0 -c tempo -- \
  wget -O- http://localhost:3200/ready
```

#### Loki
```bash
kubectl exec -n monitoring loki-0 -c loki -- \
  wget -O- http://localhost:3100/ready
```

### ストレージ監視

```bash
# PVC 使用状況
kubectl get pvc -n monitoring

# Tempo ストレージ
kubectl exec -n monitoring tempo-0 -c tempo -- \
  du -sh /var/tempo/traces /var/tempo/wal

# Loki ストレージ
kubectl exec -n monitoring loki-0 -c loki -- \
  du -sh /var/loki/chunks /var/loki/wal
```

### ログ確認

```bash
# OTel Collector
kubectl logs -n monitoring deployment/otel-collector-opentelemetry-collector -f

# Tempo
kubectl logs -n monitoring tempo-0 -c tempo -f

# Loki
kubectl logs -n monitoring loki-0 -c loki -f
```

## トラブルシューティング

### データが届かない

#### 1. OTel Collector への接続確認
```bash
# アプリケーション Pod から
kubectl exec -it <app-pod> -- curl http://otel-collector-opentelemetry-collector.monitoring.svc:4318/v1/traces
```

#### 2. OTel Collector のパイプライン確認
```bash
kubectl logs -n monitoring deployment/otel-collector-opentelemetry-collector | grep -i error
```

#### 3. バックエンドの稼働確認
```bash
kubectl get pods -n monitoring -l app.kubernetes.io/name=tempo
kubectl get pods -n monitoring -l app.kubernetes.io/name=loki
```

### メトリクスが表示されない

- Prometheus が OTel Collector から Remote Write を受信しているか確認
- ServiceMonitor が正しく設定されているか確認
- Grafana のデータソース設定を確認

### トレースが表示されない

- Tempo の /ready エンドポイントが正常か確認
- OTel Collector → Tempo の接続エラーがないか確認
- TraceID でクエリしているか確認（Tempo は TraceID ベース）

### ログが表示されない

- Loki の /ready エンドポイントが正常か確認
- OTel Collector → Loki の接続エラーがないか確認
- LogQL のラベルセレクタが正しいか確認

## パフォーマンスチューニング

### OTel Collector

**バッチサイズ調整**:
```yaml
batch:
  send_batch_size: 1000  # デフォルト
  timeout: 10s           # デフォルト
```
- 小さい値: レイテンシ低下、スループット低下
- 大きい値: レイテンシ増加、スループット向上

**メモリ制限**:
```yaml
memory_limiter:
  limit_mib: 512  # 現在値
```
- OOM を防ぐため、Pod の memory limit より小さく設定

### Tempo

**ブロックサイズ調整** (values.yaml):
```yaml
ingester:
  max_block_bytes: 1_000_000    # 1MB
  max_block_duration: 5m
```

### Loki

**Ingestion Rate 調整** (values.yaml):
```yaml
limits_config:
  ingestion_rate_mb: 10         # MB/s
  ingestion_burst_size_mb: 20
```

## セキュリティ

### クラスタ内通信のみ

- 全サービスが ClusterIP で外部非公開
- アプリケーションからのアクセスのみ許可

### 機密情報のフィルタリング

OTel Collector で自動削除:
```yaml
attributes:
  actions:
    - key: http.request.header.authorization
      action: delete
```

### リソース制限

全コンポーネントに CPU/Memory limits 設定済み:
- OTel Collector: 500m CPU, 1Gi Memory
- Tempo: 500m CPU, 1Gi Memory
- Loki: 500m CPU, 1Gi Memory

## 今後の拡張

### 分散モードへの移行

トラフィック増加時:
- **Tempo**: tempo-distributed chart へ移行
- **Loki**: loki-distributed chart へ移行

### オブジェクトストレージへの移行

本番環境:
```yaml
# Tempo
storage:
  trace:
    backend: s3
    s3:
      bucket: tempo-traces

# Loki
storage:
  type: s3
  s3:
    bucketname: loki-chunks
```

### Grafana 統合

- Tempo Datasource 自動設定
- Loki Datasource 自動設定
- 事前定義ダッシュボード追加

## 参考リンク

- [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/)
- [Grafana Tempo Documentation](https://grafana.com/docs/tempo/latest/)
- [Grafana Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Prometheus Documentation](https://prometheus.io/docs/)
