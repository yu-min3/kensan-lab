# Cluster Watch

`python3 scripts/cluster-watch/build.py` generates the production Grafana dashboard ConfigMaps in `kubernetes/observability/grafana/resources/`.

The overview retains UID `cluster-watch`. The previous overview is preserved as `cluster-watch-classic` and linked from the new overview. This preserves the previous storage/HTTPS/alert panels while the new overview focuses on node load, workloads and evidence.

Four detail source snapshots are from kube-prometheus-stack 79.5.0, ConfigMaps `prometheus-kube-prometheus-k8s-resources-{node,namespace,workload,pod}`. Upstream: kubernetes-sigs/kubernetes-mixin (Apache-2.0). Production copies use `cw-detail-*` UIDs so they do not conflict with dashboards managed by the Prometheus chart. Node tables retain namespace and pod; drilldowns carry absolute timestamps. The Prometheus datasource UID is `prometheus`; cluster is empty for this lab's unlabeled single-cluster metrics.

The six evidence dashboards distinguish workload readiness, metric collection and unmeasured functional/recovery tests. Dedicated etcd, Cilium and Istio metric collection remains unverified. No failure injection or new monitoring workload is installed. Layout and expected readiness counts are specific to this lab. On a network request error Grafana may retain old values; inspect panel errors and data age. Logs/Events are not connected; optional inherited metrics may show no data.

Only ConfigMaps change. Revert the deployment commit to restore the prior entry dashboard and remove the added dashboard copies through Argo CD. No stateful resources are pruned.
