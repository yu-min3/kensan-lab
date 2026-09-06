# tempo

The trace store. Single-binary Grafana Tempo: it ingests OTLP from the
collector, writes blocks to a Longhorn PVC, and answers trace-ID lookups from
Grafana. Where it sits in the pipeline, and why single-binary with a seven-day
window was the right size, is in
[`kubernetes/observability/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/observability/README.md).

## Layout

- `values.yaml` — overrides on the Grafana chart: the OTLP receivers (gRPC 4317, HTTP 4318), ingester flush thresholds, compactor retention, the local storage backend, and the PVC
- `config.json` — chart version and Helm parameters for the observability ApplicationSet (the version pinned here is the source of truth)

## Things to watch

- **`block_retention: 168h` and `size: 10Gi` are one decision, not two.** Seven days costs roughly 1.4 GB a day. Changing the retention without resizing the PVC fills the volume instead of extending the window
- **`enableStatefulSetAutoDeletePVC: false`** — deleting the StatefulSet leaves the PVC behind, so traces are not lost to a resync. Removing them has to be deliberate
- **The ingester thresholds trade memory against flush frequency**: `trace_idle_period: 10s`, `max_block_bytes: 1_000_000`, `max_block_duration: 5m`. The 512Mi memory request assumes roughly 300 MB of ingester buffers plus 100 MB of WAL — raise the thresholds and the request has to follow
- **`backend: local`** — the exit ramp to S3-compatible object storage is a `storage.trace.backend` change, not a redesign. Nothing else here assumes local files
- **The image tag is pinned explicitly**, never `latest` (ADR-011)

## Related

- The pipeline, the sizing rationale, and the scale-out path: [`kubernetes/observability/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/kubernetes/observability/README.md)
- Health checks, "traces are missing", and tuning: [`docs/runbooks/observability-integration.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/runbooks/observability-integration.md)
- Recovering the volume: [`docs/runbooks/longhorn-restore-test.md`](https://github.com/yu-min3/kensan-lab/blob/main/docs/runbooks/longhorn-restore-test.md)
- Upstream configuration reference: [Tempo docs](https://grafana.com/docs/tempo/latest/configuration/)
