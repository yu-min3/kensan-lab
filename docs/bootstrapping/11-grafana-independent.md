# Grafana Independent Deployment

## Overview

Grafana has been separated from kube-prometheus-stack and independently deployed as the unified visualization layer for the entire Observability stack (Prometheus/Tempo/Loki).

It is managed by the observability ApplicationSet, which discovers each component from a per-component `config.json` and renders a Helm multi-source Application.
- ApplicationSet: `kubernetes/argocd/applications/observability/applicationset.yaml` (generator globs `kubernetes/observability/*/config.json`)
- Component config: `kubernetes/observability/grafana/config.json` (chart repo/version, namespace, `hasResources`)
- Helm values: `kubernetes/observability/grafana/values.yaml`
- Custom resources: `kubernetes/observability/grafana/resources/`

There is no longer a hand-written `app.yaml` for grafana — the Application CR is generated from `config.json`.

## Key Grafana Values Settings

File: `kubernetes/observability/grafana/values.yaml`

- `admin.existingSecret`: Retrieve admin credentials from Sealed Secret
- `sidecar.datasources.enabled`: Auto-discover datasources from ConfigMaps
- `sidecar.dashboards.enabled`: Auto-discover dashboards from ConfigMaps
- `sidecar.datasources.defaultDatasourceEnabled: false`: Disable the built-in Prometheus datasource

## Admin Credentials (External Secrets → Vault)

The Grafana admin credentials are no longer a hand-sealed Secret. They are managed by the External Secrets Operator (ESO), which reads them from Vault and materializes the `grafana-admin-secret` that `values.yaml` references via `admin.existingSecret`.

- ExternalSecret CR: `kubernetes/observability/grafana/resources/external-secret.yaml`
  - Pulls `secret/monitoring/grafana/admin` from Vault via the `vault-backend` ClusterSecretStore and creates `grafana-admin-secret` in `monitoring`.
- OIDC client secret: `kubernetes/observability/grafana/resources/external-secret-oidc.yaml`

Prerequisites (see the ExternalSecret comments for the authoritative list):

- VCO `grafana-read` policy + `grafana` KubernetesAuthEngineRole applied
  (`kubernetes/secrets/vault-config-operator/resources/grafana.yaml`)
- The KV entry `secret/monitoring/grafana/admin` is populated
  (`bootstrap/vault/migrate-secret.sh`)

## OTel Dashboard Updates

Script to fetch official dashboards from Grafana.com and update ConfigMaps:

```bash
docs/bootstrapping/scripts/13-generate-grafana-dashboards.sh
```

Output: `kubernetes/observability/grafana/resources/dashboards.yaml`

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
kubernetes/observability/grafana/
├── config.json                         # ApplicationSet input (chart/version/namespace/hasResources)
├── values.yaml                         # Helm values
└── resources/
    ├── datasources.yaml                # Prometheus/Tempo/Loki datasource definitions
    ├── dashboards.yaml                 # OTel dashboard ConfigMap
    ├── claude-code-dashboard.yaml      # additional dashboard ConfigMaps
    ├── cluster-health-dashboard.yaml
    ├── controlplane-dashboard.yaml
    ├── longhorn-dashboard.yaml
    ├── external-secret.yaml            # Grafana admin credentials (External Secrets → Vault)
    ├── external-secret-oidc.yaml       # OIDC client secret
    └── httproute.yaml

kubernetes/observability/prometheus/
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
