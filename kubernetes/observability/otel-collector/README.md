# OpenTelemetry Collector

## Overview

The OpenTelemetry Collector is the central data pipeline that receives, processes,
and forwards telemetry — metrics, traces, and logs — from applications.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Application Pods                              │
│           (app-{name} namespaces — app-kensan, for example)      │
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
          │  │  1. memory_limiter (prevents OOM)      │ │
          │  │  2. batch (batching)                   │ │
          │  │  3. resource (adds metadata)           │ │
          │  │  4. attributes (strips secrets)        │ │
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
          │  │    Self-metrics (port 8888)            │ │
          │  │    - scraped by Prometheus             │ │
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

## Data flow

### 1. Metrics pipeline

```
App (OTel SDK)
  → OTLP receiver (4317)
  → [memory_limiter → batch → resource]
  → Prometheus Remote Write
  → Prometheus
```

### 2. Traces pipeline

```
App (OTel SDK)
  → OTLP receiver (4317)
  → [memory_limiter → batch → resource → attributes]
  → OTLP exporter
  → Tempo
```

### 3. Logs pipeline

```
App (OTel SDK)
  → OTLP receiver (4317)
  → [memory_limiter → batch → resource]
  → Loki exporter
  → Loki
```

## The intent behind values.yaml

### Deployment mode

```yaml
mode: deployment
replicaCount: 1
```

- **deployment mode**: at current traffic volumes a single instance is enough
- **Room to grow**: raise `replicaCount` to scale out when it is needed
- **Alternative**: daemonset mode, running on every node, is unnecessary at this cluster's size

### Receivers

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
```

- **OTLP gRPC (4317)**: the primary protocol — efficient and fast
- **OTLP HTTP (4318)**: a fallback, for environments with network constraints
- **Jaeger / Zipkin**: enabled by default but unused; left in place for future compatibility

### Processors

#### 1. memory_limiter (highest priority)

```yaml
memory_limiter:
  check_interval: 1s
  limit_mib: 512
```

- **Purpose**: keep the OOM killer away
- **Position**: always first, ahead of every other processor
- **Limit**: 50% of the container's memory limit (1Gi)
- **Behaviour**: drops data once memory use passes 512 MiB

#### 2. batch (throughput)

```yaml
batch:
  timeout: 10s
  send_batch_size: 1000
```

- **Purpose**: fewer sends to the backend, better throughput
- **timeout**: flush every 10 seconds, balancing latency against throughput
- **send_batch_size**: send immediately once 1000 items accumulate

#### 3. resource (metadata)

```yaml
resource:
  attributes:
    - key: cluster.name
      value: kensan-lab
      action: upsert
    - key: deployment.environment
      from_attribute: k8s.namespace.name
      action: upsert
```

- **cluster.name**: identifies the cluster if there is ever more than one
- **deployment.environment**: derived from the namespace name. Historical — with per-app namespaces there is now a single prod environment
- **upsert**: overwrite an existing attribute, add it when absent

#### 4. attributes (security)

```yaml
attributes:
  actions:
    - key: http.request.header.authorization
      action: delete
```

- **Purpose**: stop secrets leaking into telemetry
- **What is removed**: the Authorization header — bearer tokens, API keys, and the like
- **Where it applies**: the traces pipeline only, since HTTP traces are what carry it

### Exporters

#### 1. Prometheus Remote Write

```yaml
prometheusremotewrite:
  endpoint: http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090/api/v1/write
  tls:
    insecure: true
```

- **Protocol**: the Remote Write API, an efficient way to ship metrics
- **TLS off**: unnecessary for in-cluster traffic, and performance comes first
- **Endpoint**: the service name the Prometheus Operator deploys

#### 2. OTLP/Tempo

```yaml
otlp/tempo:
  endpoint: tempo.monitoring.svc:4317
  tls:
    insecure: true
```

- **Protocol**: OTLP gRPC, Tempo's native protocol
- **TLS off**: unnecessary for in-cluster traffic
- **Naming**: `otlp/tempo` makes it explicit that this exporter is for Tempo

#### 3. Loki

```yaml
loki:
  endpoint: http://loki.monitoring.svc:3100/loki/api/v1/push
```

- **Protocol**: the Loki push API
- **Endpoint**: Loki's standard port, 3100

### Service and ports

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

- **otlp-grpc / otlp-http**: where applications send their data
- **metrics (8888)**: the collector's own metrics, scraped by Prometheus
- **health (13133)**: the endpoint the liveness and readiness probes use

### Health checks

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

- **liveness**: wait 30 s, then check every 10 s, allowing for start-up time
- **readiness**: wait 10 s, then check every 5 s, so readiness is noticed early
- **Endpoint**: the `/` path served by the health_check extension

### Resources

```yaml
resources:
  requests:
    cpu: 200m
    memory: 512Mi
  limits:
    cpu: 500m
    memory: 1Gi
```

- **requests**: the guaranteed floor, used for scheduling
- **limits**: the ceiling during a burst
- **CPU**: 200m–500m — usually light, with headroom for spikes
- **Memory**: 512Mi–1Gi, with memory_limiter at 512 MiB, half the limit

### ServiceMonitor

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

- **enabled: true**: lets the Prometheus Operator discover the collector
- **interval: 30s**: how often the collector's own metrics are scraped
- **extraLabels**: the label the Prometheus Operator matches on to find this ServiceMonitor

## Pipeline design principles

### 1. Processor order matters

```yaml
processors: [memory_limiter, batch, resource, attributes]
```

1. **memory_limiter** first, so memory pressure is caught early
2. **batch** groups the data
3. **resource** adds metadata after batching, which is cheaper
4. **attributes** strips secrets last, so nothing slips past it

### 2. Per-pipeline tuning

- **metrics**: no `attributes` processor — there are no HTTP headers to strip
- **traces**: `attributes` processor required, because HTTP traces can carry secrets
- **logs**: no `attributes` processor — structured logs only, for now

## Security considerations

### 1. Stripping secrets

- The Authorization header is removed automatically
- More can be added later, cookies and API keys among them

### 2. In-cluster traffic only

- TLS is off, because in-cluster traffic is protected by network policy
- There is no direct access from outside

### 3. Resource limits

- memory_limiter keeps the collector out of OOM
- CPU and memory ceilings are set

## Operational considerations

### 1. Scalability

`replicaCount: 1` today, with two ways to grow.

**Vertically:**

```yaml
resources:
  requests:
    cpu: 500m
    memory: 1Gi
  limits:
    cpu: 1000m
    memory: 2Gi
```

**Horizontally:**

```yaml
replicaCount: 3
```

### 2. Monitoring the collector

- The ServiceMonitor ships the collector's own metrics to Prometheus
- The ones worth watching:
  - `otelcol_receiver_accepted_spans` — spans received
  - `otelcol_exporter_sent_spans` — spans sent on
  - `otelcol_processor_dropped_spans` — spans dropped

### 3. Troubleshooting

**Raising the log level:**

```yaml
config:
  service:
    telemetry:
      logs:
        level: debug  # info, warn, error, debug
```

**Enabling the debug exporter:**

```yaml
exporters:
  debug:
    verbosity: detailed
```

## Planned extensions

1. **Sampling**, to handle large trace volumes efficiently
2. **Tail sampling**, keeping only traces that contain an error
3. **The k8sattributes processor**, adding pod and node metadata automatically
4. **Multiple backends**, sending to more than one Prometheus or Tempo instance

## References

- [OpenTelemetry Collector documentation](https://opentelemetry.io/docs/collector/)
- [Processor configuration reference](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor)
- [Exporter configuration reference](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter)
- [Helm chart values](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector)
