# OpenTelemetry Collector

## 概要

OpenTelemetry Collector は、アプリケーションからのテレメトリデータ（メトリクス/トレース/ログ）を収集・処理・転送する中央集約型のデータパイプラインです。

## アーキテクチャ

```
┌─────────────────────────────────────────────────────────────────┐
│                    Application Pods                              │
│              (app-prod namespace / app-dev namespace)            │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   FastAPI    │  │   FastAPI    │  │   FastAPI    │          │
│  │   App 1      │  │   App 2      │  │   App 3      │          │
│  │              │  │              │  │              │          │
│  │  OTel SDK    │  │  OTel SDK    │  │  OTel SDK    │          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │
│         │                  │                  │                   │
└─────────┼──────────────────┼──────────────────┼───────────────────┘
          │                  │                  │
          │ OTLP/gRPC        │ OTLP/gRPC        │ OTLP/gRPC
          │ (4317)           │ (4317)           │ (4317)
          │                  │                  │
          └──────────────────┴──────────────────┘
                             │
                             ▼
          ┌──────────────────────────────────────────────┐
          │   OpenTelemetry Collector                    │
          │   (monitoring namespace)                     │
          │                                              │
          │  ┌────────────────────────────────────────┐ │
          │  │         Receivers                      │ │
          │  │  - OTLP gRPC (4317)                    │ │
          │  │  - OTLP HTTP (4318)                    │ │
          │  └──────────────┬─────────────────────────┘ │
          │                 │                            │
          │  ┌──────────────▼─────────────────────────┐ │
          │  │         Processors                     │ │
          │  │  1. memory_limiter (OOM防止)          │ │
          │  │  2. batch (バッチ処理)                │ │
          │  │  3. resource (メタデータ付加)         │ │
          │  │  4. attributes (機密情報削除)         │ │
          │  └──────────────┬─────────────────────────┘ │
          │                 │                            │
          │  ┌──────────────▼─────────────────────────┐ │
          │  │         Exporters                      │ │
          │  │  - Prometheus Remote Write (metrics)   │ │
          │  │  - OTLP/Tempo (traces)                 │ │
          │  │  - Loki (logs)                         │ │
          │  └──────────────┬─────────────────────────┘ │
          │                 │                            │
          │  ┌──────────────▼─────────────────────────┐ │
          │  │    Self-Metrics (port 8888)            │ │
          │  │    - Prometheus scraping               │ │
          │  └────────────────────────────────────────┘ │
          └──────────────────┬───────────────────────────┘
                             │
          ┌──────────────────┼────────────────────────┐
          │                  │                        │
          ▼                  ▼                        ▼
    ┌─────────┐      ┌──────────┐           ┌──────────┐
    │Prometheus│      │  Tempo   │           │   Loki   │
    │ (metrics)│      │ (traces) │           │  (logs)  │
    └─────────┘      └──────────┘           └──────────┘
```

## データフロー

### 1. Metrics Pipeline
```
App (OTel SDK)
  → OTLP Receiver (4317)
  → [memory_limiter → batch → resource]
  → Prometheus Remote Write
  → Prometheus
```

### 2. Traces Pipeline
```
App (OTel SDK)
  → OTLP Receiver (4317)
  → [memory_limiter → batch → resource → attributes]
  → OTLP Exporter
  → Tempo
```

### 3. Logs Pipeline
```
App (OTel SDK)
  → OTLP Receiver (4317)
  → [memory_limiter → batch → resource]
  → Loki Exporter
  → Loki
```

## values.yaml 設計意図

### Deployment Mode

```yaml
mode: deployment
replicaCount: 1
```

**設計意図:**
- **deployment モード**: 現時点では単一インスタンスで十分なトラフィック量
- **将来の拡張性**: 必要に応じて `replicaCount` を増やしてスケールアウト可能
- **代替案**: daemonset モード（各ノードで実行）は、現在のクラスタ規模では不要

### Receivers設計

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
```

**設計意図:**
- **OTLP gRPC (4317)**: 主要プロトコル。効率的でパフォーマンスが高い
- **OTLP HTTP (4318)**: フォールバック用。ネットワーク制約がある環境向け
- **Jaeger/Zipkin**: デフォルトで有効だが、現時点では使用しない（将来の互換性のため残す）

### Processors設計

#### 1. memory_limiter（最優先）

```yaml
memory_limiter:
  check_interval: 1s
  limit_mib: 512
```

**設計意図:**
- **目的**: OOMキラーを防ぐ
- **配置順序**: 必ず最初に配置（他のprocessorより前）
- **制限値**: コンテナのメモリ制限（1Gi）の50%を設定
- **動作**: メモリ使用量が512MiBを超えるとデータをドロップ

#### 2. batch（パフォーマンス最適化）

```yaml
batch:
  timeout: 10s
  send_batch_size: 1000
```

**設計意図:**
- **目的**: バックエンドへの送信回数を削減してパフォーマンス向上
- **timeout**: 10秒ごとにバッチを送信（レイテンシとスループットのバランス）
- **send_batch_size**: 1000件に達したら即座に送信

#### 3. resource（メタデータ付加）

```yaml
resource:
  attributes:
    - key: cluster.name
      value: goldship
      action: upsert
    - key: deployment.environment
      from_attribute: k8s.namespace.name
      action: upsert
```

**設計意図:**
- **cluster.name**: マルチクラスタ環境での識別用（将来的な拡張を見据えて）
- **deployment.environment**: namespace名から環境を自動判定（app-prod → prod, app-dev → dev）
- **upsert**: 既存の属性がある場合は上書き、ない場合は追加

#### 4. attributes（セキュリティ）

```yaml
attributes:
  actions:
    - key: http.request.header.authorization
      action: delete
```

**設計意図:**
- **目的**: 機密情報の漏洩防止
- **削除対象**: Authorization ヘッダー（Bearer トークン、API キーなど）
- **配置**: traces パイプラインのみに適用（HTTPトレースが対象）

### Exporters設計

#### 1. Prometheus Remote Write

```yaml
prometheusremotewrite:
  endpoint: http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090/api/v1/write
  tls:
    insecure: true
```

**設計意図:**
- **プロトコル**: Remote Write API（効率的なメトリクス転送）
- **TLS無効**: クラスタ内通信のため不要（パフォーマンス優先）
- **エンドポイント**: Prometheus Operatorがデプロイしたサービス名

#### 2. OTLP/Tempo

```yaml
otlp/tempo:
  endpoint: tempo.monitoring.svc:4317
  tls:
    insecure: true
```

**設計意図:**
- **プロトコル**: OTLP gRPC（Tempo のネイティブプロトコル）
- **TLS無効**: クラスタ内通信のため不要
- **命名**: `otlp/tempo` で Tempo 専用エクスポーターであることを明示

#### 3. Loki

```yaml
loki:
  endpoint: http://loki.monitoring.svc:3100/loki/api/v1/push
```

**設計意図:**
- **プロトコル**: Loki Push API
- **エンドポイント**: Loki の標準ポート（3100）

### Service & Ports設計

```yaml
ports:
  otlp-grpc:
    enabled: true
    containerPort: 4317
    servicePort: 4317
  otlp-http:
    enabled: true
    containerPort: 4318
    servicePort: 4318
  metrics:
    enabled: true
    containerPort: 8888
    servicePort: 8888
  health:
    enabled: true
    containerPort: 13133
    servicePort: 13133
```

**設計意図:**
- **otlp-grpc/http**: アプリケーションからのデータ受信用
- **metrics (8888)**: Collectorの自己監視メトリクス（Prometheusでスクレイピング）
- **health (13133)**: liveness/readiness probe用のヘルスチェックエンドポイント

### Health Checks設計

```yaml
livenessProbe:
  httpGet:
    port: 13133
    path: /
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    port: 13133
    path: /
  initialDelaySeconds: 10
  periodSeconds: 5
```

**設計意図:**
- **liveness**: 30秒待機後、10秒ごとにチェック（起動時間を考慮）
- **readiness**: 10秒待機後、5秒ごとにチェック（早期の準備完了検出）
- **エンドポイント**: health_check extension が提供する `/` パス

### Resources設計

```yaml
resources:
  requests:
    cpu: 200m
    memory: 512Mi
  limits:
    cpu: 500m
    memory: 1Gi
```

**設計意図:**
- **requests**: 保証リソース（スケジューリング時の最小要件）
- **limits**: 上限（バースト時の最大使用量）
- **CPU**: 200m〜500m（通常は低負荷、スパイク時に対応）
- **Memory**: 512Mi〜1Gi（memory_limiter は512MiB = limits の50%）

### ServiceMonitor設計

```yaml
serviceMonitor:
  enabled: true
  metricsEndpoints:
    - port: metrics
      path: /metrics
      interval: 30s
  extraLabels:
    release: prometheus
```

**設計意図:**
- **enabled: true**: Prometheus Operatorによる自動検出を有効化
- **interval: 30s**: Collectorの自己監視メトリクスのスクレイピング間隔
- **extraLabels**: Prometheus Operatorが ServiceMonitor を検出するためのラベル

## Pipeline設計の原則

### 1. Processor の順序が重要

```yaml
processors: [memory_limiter, batch, resource, attributes]
```

**理由:**
1. **memory_limiter**: 最優先。メモリ不足を早期検出
2. **batch**: バッチ処理でデータをまとめる
3. **resource**: バッチ後にメタデータを付加（効率的）
4. **attributes**: 最後に機密情報を削除（確実に削除）

### 2. Pipeline ごとの最適化

- **metrics**: `attributes` processor 不要（HTTPヘッダーが含まれない）
- **traces**: `attributes` processor 必要（HTTPトレースに機密情報が含まれる可能性）
- **logs**: `attributes` processor 不要（現時点では構造化ログのみ）

## セキュリティ考慮事項

### 1. 機密情報の削除

- Authorization ヘッダーを自動削除
- 将来的には他の機密情報も追加（例: Cookie, API keys）

### 2. クラスタ内通信のみ

- TLS無効化（クラスタ内通信はネットワークポリシーで保護）
- 外部からの直接アクセスは不可

### 3. Resource Limits

- OOM を防ぐための memory_limiter
- CPU/Memory の上限設定

## 運用考慮事項

### 1. スケーラビリティ

現在は `replicaCount: 1` だが、必要に応じて以下の対応が可能：

**垂直スケーリング:**
```yaml
resources:
  requests:
    cpu: 500m
    memory: 1Gi
  limits:
    cpu: 1000m
    memory: 2Gi
```

**水平スケーリング:**
```yaml
replicaCount: 3
```

### 2. 監視

Collector 自体を監視するために：
- ServiceMonitor で自己メトリクスを Prometheus に送信
- 重要なメトリクス:
  - `otelcol_receiver_accepted_spans`: 受信したスパン数
  - `otelcol_exporter_sent_spans`: 送信したスパン数
  - `otelcol_processor_dropped_spans`: ドロップされたスパン数

### 3. トラブルシューティング

**ログレベルの変更:**
```yaml
config:
  service:
    telemetry:
      logs:
        level: debug  # info, warn, error, debug
```

**Debug Exporter の有効化:**
```yaml
exporters:
  debug:
    verbosity: detailed
```

## 今後の拡張予定

1. **Sampling の導入**: 大量のトレースデータを効率的に処理
2. **Tail Sampling**: エラーを含むトレースのみを保存
3. **K8s Attributes Processor**: Pod/Node メタデータの自動付加
4. **複数バックエンド対応**: 複数の Prometheus/Tempo インスタンスへの送信

## 参考リンク

- [OpenTelemetry Collector 公式ドキュメント](https://opentelemetry.io/docs/collector/)
- [Processor 設定リファレンス](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor)
- [Exporter 設定リファレンス](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter)
- [Helm Chart Values](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector)
