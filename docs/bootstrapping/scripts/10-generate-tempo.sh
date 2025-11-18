#!/bin/bash
# Grafana Tempo Helm リポジトリ追加
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update grafana

# Grafana Tempo マニフェスト生成
helm template tempo grafana/tempo \
  --version 1.24.0 \
  --namespace monitoring \
  --values ../../infrastructure/observability/tempo/values.yaml \
  > ../../infrastructure/observability/tempo/tempo-manifests.yaml 2>/dev/null

echo "Grafana Tempo manifests generated successfully!"
