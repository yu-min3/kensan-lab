# Observability integration & troubleshooting

Operational companion to the [observability architecture page](../architecture/observability.md): how to point an application at the telemetry pipeline, verify the stack is healthy, and debug the usual "my data isn't showing up" cases.

## Instrumenting an application

The only contract an app needs is OTLP to the collector. Minimum environment variables:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector-opentelemetry-collector.monitoring.svc:4318
OTEL_SERVICE_NAME=my-application
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production
```

The collector adds `cluster.name=kensan-lab` and `deployment.environment=<k8s.namespace.name>` to every pipeline, so per-app attributes stay minimal.

### Python example

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

## Health checks

```bash
# OTel Collector
kubectl exec -n monitoring deployment/otel-collector-opentelemetry-collector -- wget -O- http://localhost:13133/

# Tempo
kubectl exec -n monitoring tempo-0 -c tempo -- wget -O- http://localhost:3200/ready

# Loki
kubectl exec -n monitoring loki-0 -c loki -- wget -O- http://localhost:3100/ready
```

### Storage usage

```bash
kubectl get pvc -n monitoring
kubectl exec -n monitoring tempo-0 -c tempo -- du -sh /var/tempo/traces /var/tempo/wal
kubectl exec -n monitoring loki-0 -c loki -- du -sh /var/loki/chunks /var/loki/wal
```

## Troubleshooting

### No data arriving at all

1. App pod → collector connectivity: `kubectl exec <pod> -- curl http://otel-collector-opentelemetry-collector.monitoring.svc:4318/v1/traces`
2. Collector error log: `kubectl logs -n monitoring deployment/otel-collector-opentelemetry-collector | grep -i error`
3. Backend liveness: `kubectl get pods -n monitoring -l app.kubernetes.io/name=tempo` (and loki / prometheus equivalents)

### Metrics missing in Grafana

- Confirm Prometheus is receiving remote write from the collector
- Check the ServiceMonitor definition (label selectors, port names)
- Check the Grafana datasource URL

### Traces missing

- Tempo `/ready` check (above)
- Collector → Tempo connection errors in the collector log
- Remember Tempo is queried by TraceID — it is not label-searchable like LogQL

### Logs missing

- Loki `/ready` check (above)
- Verify the LogQL label selector (e.g. `{namespace="kensan"}`)

## Performance tuning

### OTel Collector

```yaml
# batching
batch:
  send_batch_size: 1000   # smaller: lower latency / larger: higher throughput
  timeout: 10s

# memory
memory_limiter:
  limit_mib: 512   # keep below the Pod memory limit
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
