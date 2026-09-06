# Grafana Tempo

## Overview

Grafana Tempo is the trace store — a trace TSDB. It holds the trace data the
OpenTelemetry Collector sends it and serves a query API keyed on trace ID.

## Architecture

```
┌──────────────────────────────────┐
│   OpenTelemetry Collector        │
│   (monitoring namespace)         │
└──────────────┬───────────────────┘
               │ OTLP (gRPC: 4317)
               ↓
┌──────────────────────────────────┐
│   Grafana Tempo StatefulSet      │
│   (monitoring namespace)         │
│                                  │
│  ┌────────────────────────────┐ │
│  │    Distributor             │ │
│  │  - Receives OTLP traces    │ │
│  └──────────┬─────────────────┘ │
│             │                    │
│  ┌──────────▼─────────────────┐ │
│  │    Ingester                │ │
│  │  - Writes to WAL           │ │
│  │  - Creates blocks          │ │
│  └──────────┬─────────────────┘ │
│             │                    │
│  ┌──────────▼─────────────────┐ │
│  │    Storage (local)         │ │
│  │  - /var/tempo/traces       │ │
│  │  - /var/tempo/wal          │ │
│  └──────────┬─────────────────┘ │
│             │                    │
│  ┌──────────▼─────────────────┐ │
│  │    Compactor               │ │
│  │  - Block retention: 7 days │ │
│  └────────────────────────────┘ │
│                                  │
│  ┌────────────────────────────┐ │
│  │    Query Frontend (3100)   │ │
│  │  - TraceID lookup          │ │
│  └────────────────────────────┘ │
└──────────────┬───────────────────┘
               │ PVC (10Gi)
               │ longhorn
               ↓
        ┌──────────────┐
        │ Persistent   │
        │ Storage      │
        └──────────────┘
               ↑
               │ HTTP API (3100)
        ┌──────────────┐
        │   Grafana    │
        │  Datasource  │
        └──────────────┘
```

## Data flow

### 1. Receiving traces (distributor)

```
OTel Collector
  → OTLP gRPC (4317)
  → Tempo distributor
  → trace spans received
```

### 2. Storing traces (ingester)

```
Distributor
  → ingester
  → write to the WAL (/var/tempo/wal)
  → create blocks (/var/tempo/traces)
```

### 3. Compacting traces (compactor)

```
Runs periodically
  → compacts older blocks
  → deletes blocks older than 7 days
```

### 4. Querying traces (query frontend)

```
Grafana
  → HTTP API (3100)
  → query by trace ID
  → trace spans returned
```

## The intent behind values.yaml

### Deployment mode

```yaml
replicas: 1
```

- **Single-binary mode**: Tempo runs as one process, which suits a development or small-scale environment
- **Simple to operate**: distributor, ingester, compactor, and query all live in one pod
- **Room to grow**: move to the `tempo-distributed` chart when traffic demands it

### Image

```yaml
tempo:
  repository: grafana/tempo
  tag: "2.3.1"
```

- **A stable release**: 2.3.1 is production-ready
- **An explicit tag**: avoids `latest`, so builds stay reproducible

### Tempo configuration

#### Server

```yaml
server:
  http_listen_port: 3100
```

- **Port 3100**: the same port Grafana Loki uses, keeping the scheme consistent
- **HTTP API**: used by Grafana queries and by health checks

#### Distributor (the OTLP receiver)

```yaml
distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
        http:
          endpoint: 0.0.0.0:4318
```

- **OTLP gRPC (4317)**: the main protocol, used by the OpenTelemetry Collector
- **OTLP HTTP (4318)**: a fallback
- **Binding 0.0.0.0**: receive on every interface inside the pod

#### Ingester (storing traces)

```yaml
ingester:
  trace_idle_period: 10s
  max_block_bytes: 1_000_000
  max_block_duration: 5m
```

- **trace_idle_period: 10s**: flush ten seconds after a trace goes quiet.
  The trade-off: shorter means flushing more often, longer means holding more in memory
- **max_block_bytes: 1MB**: the block size ceiling — small blocks reach disk sooner
- **max_block_duration: 5m**: cut a block at five minutes at the latest, so memory use stays bounded

#### Compactor

```yaml
compactor:
  compaction:
    block_retention: 168h  # 7 days
```

- **168h — seven days**: how long trace data is kept. Enough for recent troubleshooting, and it keeps storage down
- **Automatic deletion**: blocks older than seven days are removed

#### Storage backend

```yaml
storage:
  trace:
    backend: local
    local:
      path: /var/tempo/traces
    wal:
      path: /var/tempo/wal
    pool:
      max_workers: 100
      queue_depth: 10000
```

- **backend: local**: the local filesystem — simple to run, persisted by a PVC, and switchable to S3, GCS, or Azure Blob in a larger environment
- **traces path**: where compacted blocks live
- **wal path**: the write-ahead log, used to recover data after a crash
- **pool**:
  - `max_workers: 100` — concurrent write workers
  - `queue_depth: 10000` — how deep the write queue goes

### Persistence

```yaml
persistence:
  enabled: true
  accessModes:
    - ReadWriteOnce
  size: 10Gi
  storageClassName: longhorn
  enableStatefulSetAutoDeletePVC: false
```

- **enabled: true**: keep trace data across restarts
- **ReadWriteOnce**: a single StatefulSet pod uses it
- **size: 10Gi**: seven days of traces, at roughly 1.4 GB a day, with headroom
- **storageClassName: longhorn**: replicated block storage, the default storage class, giving dynamic PV provisioning on bare metal
- **enableStatefulSetAutoDeletePVC: false**: keep the PVC even if the StatefulSet is deleted, forcing deletion to be a deliberate act

### Service

```yaml
service:
  type: ClusterIP
  port: 3100
```

- **ClusterIP**: in-cluster traffic only — OTLP from the OpenTelemetry Collector, and queries from Grafana
- **port: 3100**: the main HTTP API port
  - `/ready` — health check
  - `/api/traces/{traceID}` — trace query

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

- **requests** — the guaranteed floor: 200m CPU is enough at normal load, and 512Mi covers the WAL and block buffers
- **limits** — the ceiling: 500m CPU for spikes, 1Gi memory to keep OOM away
- **How the memory number was reached**:
  - Ingester buffers: ~300 MB
  - WAL: ~100 MB
  - Everything else, the compactor included: ~100 MB
  - Around 500 MB in total, rounded up to 512Mi

## Architectural principles

### 1. Why single-binary mode

**Traffic today:**

- A small number of applications
- Thousands to tens of thousands of spans a day
- Queries are infrequent — only while troubleshooting

**What single-binary mode buys:**

- One pod to operate
- Efficient use of resources, with no inter-process traffic
- A simple deployment

**When to move to distributed mode:**

- Trace volume passes a million spans a day
- Query latency becomes a problem
- High availability becomes a requirement

### 2. Why local storage

The current choice is `backend: local` plus a PVC.

**In its favour:** simple configuration, no additional cloud service, lower cost.

**Against it:** limited scalability, and backups are manual.

**Moving to an object store:**

```yaml
storage:
  trace:
    backend: s3  # or gcs, azure
    s3:
      bucket: tempo-traces
      endpoint: s3.amazonaws.com
```

### 3. Why seven days of retention

- **Troubleshooting**: recent traces are what get looked at
- **Storage cost**: long-term retention buys nothing here
- **Compliance**: no regulatory requirement applies

**To change it:**

```yaml
compactor:
  compaction:
    block_retention: 336h  # 14 days
```

## Security considerations

### 1. In-cluster traffic only

- `type: ClusterIP` — nothing is exposed outside the cluster
- Only the OpenTelemetry Collector and Grafana can reach it

### 2. Protecting the PVC

- `enableStatefulSetAutoDeletePVC: false` guards against accidental deletion
- Removing the PVC has to be done deliberately

### 3. Resource limits

- The memory limit (1Gi) keeps the pod out of OOM
- The CPU limit (500m) protects the node's resources

## Operational considerations

### 1. Watching storage

The metric that matters is PVC utilisation:
`kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes`.
Alert when free space drops below 20%.

### 2. Watching block count

```bash
# count the blocks inside the Tempo pod
kubectl exec -n monitoring tempo-0 -- ls -lh /var/tempo/traces
```

### 3. Query performance

- Watch the latency of trace-ID lookups
- Alert when p95 passes one second

### 4. Backup strategy

**Today:** PVC snapshots, which depend on the CSI driver.

**Suggested:**

```bash
# a periodic backup
kubectl exec -n monitoring tempo-0 -- tar czf /tmp/tempo-backup.tar.gz /var/tempo/traces
kubectl cp monitoring/tempo-0:/tmp/tempo-backup.tar.gz ./tempo-backup-$(date +%Y%m%d).tar.gz
```

## Troubleshooting

### Problem 1: the pod stays Pending

**Symptom:**

```bash
kubectl get pods -n monitoring
# NAME      READY   STATUS    RESTARTS   AGE
# tempo-0   0/1     Pending   0          5m
```

**Causes and what to do:**

1. **The PVC is not bound**

   ```bash
   kubectl get pvc -n monitoring
   kubectl describe pvc tempo-storage-tempo-0 -n monitoring

   # check the Longhorn volume
   kubectl get volumes.longhorn.io -n longhorn-system | grep tempo
   ```

2. **The node is out of resources**

   ```bash
   kubectl describe node
   # look at: Allocated resources
   ```

### Problem 2: OTLP ingestion fails

**Symptom:**

```bash
kubectl logs -n monitoring -l app.kubernetes.io/name=otel-collector | grep tempo
# Error: connection refused
```

**What to do:**

1. **Check the Service**

   ```bash
   kubectl get svc tempo -n monitoring
   kubectl get endpoints tempo -n monitoring
   ```

2. **Check the pod**

   ```bash
   kubectl logs -n monitoring tempo-0 | grep -i otlp
   # "OTLP gRPC receiver started on :4317" should appear
   ```

### Problem 3: queries are slow

**Symptom:** searching for traces in Grafana takes a long time.

**What to do:**

1. **Count the blocks**

   ```bash
   kubectl exec -n monitoring tempo-0 -- find /var/tempo/traces -name "*.tar.gz" | wc -l
   # too many blocks means the compaction settings need revisiting
   ```

2. **Check memory use**

   ```bash
   kubectl top pod tempo-0 -n monitoring
   # close to the limit means the limit should go up
   ```

## Planned extensions

1. **Distributed mode**, once traffic grows
2. **An S3 backend**, for long-term retention and scalability
3. **TraceQL support**, for richer queries
4. **Automatic Grafana datasource configuration**, through Terraform or Helm

## References

- [Grafana Tempo documentation](https://grafana.com/docs/tempo/latest/)
- [Tempo Helm chart](https://github.com/grafana/helm-charts/tree/main/charts/tempo)
- [OTLP ingestion](https://grafana.com/docs/tempo/latest/configuration/network/otlp/)
- [Storage configuration](https://grafana.com/docs/tempo/latest/configuration/storage/)
