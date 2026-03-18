# Grafana Independent Deployment

## Overview

Grafana has been separated from kube-prometheus-stack and independently deployed as the unified visualization layer for the entire Observability stack (Prometheus/Tempo/Loki).

It is currently managed as an Argo CD Helm multi-source Application.
- Application CR: `infrastructure/gitops/argocd/applications/observability/grafana/app.yaml`
- Helm values: `infrastructure/observability/grafana/values.yaml`
- Custom resources: `infrastructure/observability/grafana/resources/`

## Key Grafana Values Settings

File: `infrastructure/observability/grafana/values.yaml`

- `admin.existingSecret`: Retrieve admin credentials from Sealed Secret
- `sidecar.datasources.enabled`: Auto-discover datasources from ConfigMaps
- `sidecar.dashboards.enabled`: Auto-discover dashboards from ConfigMaps
- `sidecar.datasources.defaultDatasourceEnabled: false`: Disable the built-in Prometheus datasource

## Sealed Secret Creation Steps

The Grafana admin password is managed as a Sealed Secret. It must be recreated for new clusters.

```bash
# Create raw secret
kubectl create secret generic grafana-admin-secret \
  --namespace=monitoring \
  --from-literal=admin-user=admin \
  --from-literal=admin-password=<YOUR_PASSWORD> \
  --dry-run=client -o yaml > temp/grafana-admin-secret-raw.yaml

# Seal the secret
kubeseal --controller-name=sealed-secrets \
  --controller-namespace=kube-system \
  --format=yaml \
  < temp/grafana-admin-secret-raw.yaml \
  > infrastructure/observability/grafana/resources/grafana-admin-sealed-secret.yaml

# Delete the raw secret
rm temp/grafana-admin-secret-raw.yaml
```

## OTel Dashboard Updates

Script to fetch official dashboards from Grafana.com and update ConfigMaps:

```bash
docs/bootstrapping/scripts/13-generate-grafana-dashboards.sh
```

Output: `infrastructure/observability/grafana/resources/dashboards.yaml`

## Verification

### Datasources

Grafana UI -> Configuration -> Data Sources should display:
- **Prometheus** (default, exemplars enabled)
- **Tempo** (tracing)
- **Loki** (logs)

### Dashboards

Grafana UI -> Dashboards -> Browse should display:
- **OpenTelemetry** folder: OTel APM, OTel for HTTP Services
- **General** folder: Kubernetes monitoring dashboards

### Sidecar Logs

```bash
kubectl logs -n monitoring deployment/grafana -c grafana-sc-datasources --tail=10
kubectl logs -n monitoring deployment/grafana -c grafana-sc-dashboard --tail=10
```

## Troubleshooting

### Grafana Pod in CrashLoopBackOff

**Symptom**: `Only one datasource per organization can be marked as default`

**Fix**: Set `sidecar.datasources.defaultDatasourceEnabled: false` in `values.yaml`

### Datasources Not Showing

ConfigMap requires the `grafana_datasource: "1"` label.

```bash
kubectl get configmap -n monitoring -l grafana_datasource=1
```

### Dashboards Not Showing

ConfigMap requires the `grafana_dashboard: "1"` label.

```bash
kubectl get configmap -n monitoring -l grafana_dashboard=1
```

## File Structure

```
infrastructure/observability/grafana/
├── values.yaml                         # Helm values
└── resources/
    ├── datasources.yaml                # Prometheus/Tempo/Loki datasource definitions
    ├── dashboards.yaml                 # OTel dashboard ConfigMap
    ├── grafana-admin-sealed-secret.yaml
    └── httproute.yaml

infrastructure/observability/prometheus/
├── values.yaml                         # grafana.enabled: false
└── resources/
    ├── httproute-prometheus.yaml
    └── grafana-sealed-secret.yaml
```

## References

- [Grafana Official Helm Chart](https://github.com/grafana/helm-charts/tree/main/charts/grafana)
- [Grafana Sidecar Documentation](https://github.com/kiwigrid/k8s-sidecar)
- [OpenTelemetry APM Dashboard](https://grafana.com/grafana/dashboards/19419)
- [OpenTelemetry for HTTP Services](https://grafana.com/grafana/dashboards/21587)
