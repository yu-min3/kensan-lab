# Cluster Watch

`python3 scripts/cluster-watch/build.py` packages the reviewed JSON snapshots under `validated-dashboards/` into Grafana ConfigMaps. Snapshots originate from the local prototype at commit `b93b54c`. They are Grafana documents, not rendered Helm manifests.

## Screens

| Entry | Content |
|---|---|
| `cluster-watch` | Compact four-node trends and seven foundation cards. Infrastructure and Argo CD application Sync/Health are real. The table dynamically selects `app-project`; its name links open the real Argo CD application. HTTP route health is unmeasured. Cilium/Istio readiness does not imply functional connectivity. |
| `cw-function-normal`, `cw-function-incident`, `cw-function-stale` | Communication metrics, communication age, and apps are **fictional TestData fixtures**. Other infrastructure is real. Header switches scenarios; card links explain the observed scope and provisional thresholds. |
| `cluster-watch-classic` | Previous operational dashboard preserved, including storage/HTTPS/alert views. |

Sample dashboards are grouped in one ConfigMap. Built-in TestData datasource UID `app-status-sample` is provisioned separately from Prometheus. These samples do not write metrics to Prometheus. The separate Application Controller PodMonitor scrapes `argocd_app_info` for the operational table; no traffic probes or recovery tests are installed. The production entry does not mark Cilium/Istio functional traffic healthy from sample data.

Four detail source snapshots originate from kube-prometheus-stack 79.5.0 / kubernetes-mixin (Apache-2.0). Production copies use `cw-detail-*` UIDs to avoid ownership conflicts. Links retain namespace/pod and absolute timestamps. Prometheus UID is `prometheus`; cluster label is empty for this single-cluster setup. All referenced sample detail dashboards are included.

## Validation and limits

The prototype was checked at 906px for normal, failed and stale communication, six component drilldowns and three app drilldowns with zero query/page errors. Staleness in communication scenarios is simulated; it does not detect transport errors of every production panel. Production functional traffic collection is not introduced. Application state is collected from the existing controller metrics port, without a controller restart. Logs and Events are not connected. Zero traffic, policy-denied traffic and missing telemetry require separate scoped evaluation before real collection can replace fixtures.

Regenerate snapshots and manifests together when the UI changes. The packager rejects duplicate UIDs/titles and ConfigMaps approaching the 1MiB limit.

## Rollback

Revert this deployment PR to restore the previous entry and remove added dashboard ConfigMaps through GitOps. Only ConfigMaps change; no stateful resources are pruned. The existing Prometheus/Loki/Tempo datasource entries are preserved. Remove the sample datasource through Grafana provisioning `deleteDatasources` if it also needs to be removed; removing its YAML alone does not guarantee deletion of an already provisioned datasource.

## Live application state

`apps.py` replaces only the production table during packaging. The reviewed sample snapshots stay available under explicit SAMPLE UIDs. The table uses `argocd_app_info{project="app-project"}` from `job="argocd-application-controller", namespace="argocd"`; platform applications remain in Argo CD via the panel link. Names are discovered dynamically, rather than kept as a list of four applications.

Health and Sync retain all Argo CD states. An unknown state is not mapped to Healthy. Health describes Kubernetes resources, not end-to-end HTTP success: the route column stays **unmeasured**. The observation timestamp is the Prometheus sample timestamp, not the last Git sync or reconcile time.

A failed/missing controller target, an up/app sample at least 120 seconds old, or no app info yields one explicit unavailable row with no retained Healthy status. Every target must be up. Identical controller samples are deduplicated. One missing app cannot be distinguished from a deleted app using this metric alone; this view does not assert an expected inventory size.

Run `python3 scripts/cluster-watch/test_apps.py` for packaging and PromQL regression checks (Docker, `prom/prometheus:v3.1.0`, PyYAML). The cases exercise stale/missing/failed collection, separate Health/Sync, project scoping, status transitions and deduplication.

For rollback of live state, revert `apps.py`, its `build.py` integration and the generated operational ConfigMap together, plus the PodMonitor. No credentials, public routes, workloads or persistent volumes change. A dashboard rollback restores explicitly labelled sample data, not live monitoring.

Source: [Argo CD Application Controller metrics](https://argo-cd.readthedocs.io/en/stable/operator-manual/metrics/).
