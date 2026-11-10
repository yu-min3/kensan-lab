# Cluster Watch

`python3 scripts/cluster-watch/build.py` packages the reviewed JSON snapshots under `validated-dashboards/` into Grafana ConfigMaps. Snapshots originate from the local prototype at commit `b93b54c`. They are Grafana documents, not rendered Helm manifests.

## Screens

| Entry | Content |
|---|---|
| `cluster-watch` | Compact four-node trends and seven foundation cards. Infrastructure is real; the app table is explicitly SAMPLE. Cilium/Istio readiness does not imply functional connectivity. |
| `cw-function-normal`, `cw-function-incident`, `cw-function-stale` | Communication metrics, communication age, and apps are **fictional TestData fixtures**. Other infrastructure is real. Header switches scenarios; card links explain the observed scope and provisional thresholds. |
| `cluster-watch-classic` | Previous operational dashboard preserved, including storage/HTTPS/alert views. |

Sample dashboards are grouped in one ConfigMap. Built-in TestData datasource UID `app-status-sample` is provisioned separately from Prometheus. No metrics are written to Prometheus; no collectors, probes, or recovery tests are installed. The production entry does not mark Cilium/Istio functional traffic healthy from sample data.

Four detail source snapshots originate from kube-prometheus-stack 79.5.0 / kubernetes-mixin (Apache-2.0). Production copies use `cw-detail-*` UIDs to avoid ownership conflicts. Links retain namespace/pod and absolute timestamps. Prometheus UID is `prometheus`; cluster label is empty for this single-cluster setup. All referenced sample detail dashboards are included.

## Validation and limits

The prototype was checked at 906px for normal, failed and stale communication, six component drilldowns and three app drilldowns with zero query/page errors. Staleness in communication scenarios is simulated; it does not detect transport errors of every production panel. Production functional collection and Argo CD Application metrics are not introduced. Logs and Events are not connected. Zero traffic, policy-denied traffic and missing telemetry require separate scoped evaluation before real collection can replace fixtures.

Regenerate snapshots and manifests together when the UI changes. The packager rejects duplicate UIDs/titles and ConfigMaps approaching the 1MiB limit.

## Rollback

Revert this deployment PR to restore the previous entry and remove added dashboard ConfigMaps through GitOps. Only ConfigMaps change; no stateful resources are pruned. The existing Prometheus/Loki/Tempo datasource entries are preserved. Remove the sample datasource through Grafana provisioning `deleteDatasources` if it also needs to be removed; removing its YAML alone does not guarantee deletion of an already provisioned datasource.
