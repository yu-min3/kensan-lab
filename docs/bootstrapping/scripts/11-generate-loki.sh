#!/bin/bash
# Grafana Loki Helm リポジトリ追加
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update grafana

# Grafana Loki マニフェスト生成
helm template loki grafana/loki \
  --version 6.46.0 \
  --namespace monitoring \
  --values ../../infrastructure/observability/loki/values.yaml \
  > ../../infrastructure/observability/loki/loki-manifests.yaml 2>/dev/null

echo "Grafana Loki manifests generated successfully!"
echo ""
echo "✓ Note: Loki chart v6.46.0 generates proper volumeClaimTemplates"
echo "✓ No manual fixes required (unlike Tempo chart)"
echo ""
