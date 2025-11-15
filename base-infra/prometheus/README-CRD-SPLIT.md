# CRD Split

このディレクトリでは、CRDとその他のリソースを分離しています：

- **00-crds.yaml** (4.1MB): 10個のCustomResourceDefinitions
  - AlertmanagerConfig
  - Alertmanager
  - PodMonitor
  - Probe
  - PrometheusAgent
  - Prometheus
  - PrometheusRule
  - ScrapeConfig
  - ServiceMonitor
  - ThanosRuler

- **prometheus-stack.yaml** (626KB): 134個のリソース
  - Deployments, Services, ConfigMaps, ServiceMonitors, PrometheusRules等

## デプロイ順序

Argo CDはファイル名のアルファベット順でリソースを適用するため、`00-crds.yaml`が最初に適用されます。
`ServerSideApply=true`オプションにより、CRDの適用が確実に行われます。

## 再生成方法

Prometheus Stackを更新する場合：

```bash
# Helm chartから生成
helm template kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values prometheus-values.yaml \
  > /tmp/prometheus-all.yaml

# CRDとリソースを分離
python3 scripts/split_crds.py /tmp/prometheus-all.yaml 00-crds.yaml prometheus-stack.yaml
```
