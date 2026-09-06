# otel-collector

The single telemetry intake. Applications learn one endpoint — OTLP to this
collector — and everything after that is decided here: batching, memory
protection, enrichment, secret scrubbing, and the fan-out to Prometheus, Tempo,
and Loki. Why the platform has exactly one chokepoint is in
[`kubernetes/observability/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/observability/README.md).

## Layout

- `values.yaml` — overrides on the OpenTelemetry chart: the OTLP receivers (gRPC 4317, HTTP 4318), the four processors, the three exporters, ports, probes, and the ServiceMonitor
- `config.json` — chart version and Helm parameters for the observability ApplicationSet (the version pinned here is the source of truth)

## Things to watch

- **Processor order is load-bearing**: `[memory_limiter, batch, resource, attributes]`. `memory_limiter` has to run first so pressure is caught before anything else allocates, and `attributes` has to run last so nothing added later escapes the scrub. Reordering the list silently changes both properties
- **`limit_mib: 512` is half of the container's 1Gi limit.** They move together — raising the limit alone leaves the collector dropping data long before it is actually short of memory
- **The secret scrub applies to traces only.** `http.request.header.authorization` is deleted in the traces pipeline, because that is the pipeline that carries HTTP headers. Adding a header-bearing source to metrics or logs means extending the processor to that pipeline too
- **`extraLabels: {release: prometheus}`** is what makes the Prometheus Operator pick up the ServiceMonitor. Without it the collector's own metrics (port 8888) go unscraped and the pipeline stops being observable
- **Jaeger and Zipkin receivers are enabled by the chart default and unused.** Left in place for compatibility; nothing routes to them

## Related

- The pipeline, the enrichment and scrubbing rationale, and the three health layers: [`kubernetes/observability/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/observability/README.md)
- Instrumenting an app, health checks, "no data arriving", and tuning: [`docs/runbooks/observability-integration.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/runbooks/observability-integration.md)
- Upstream configuration reference: [Collector docs](https://opentelemetry.io/docs/collector/configuration/) · [processors](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor)
