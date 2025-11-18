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
echo ""
echo "⚠️  IMPORTANT: Manual fix required for volumeClaimTemplates"
echo "The Tempo Helm chart generates incomplete volumeClaimTemplates."
echo "Please manually edit tempo-manifests.yaml and fix the volumeClaimTemplates section:"
echo ""
echo "Replace:"
echo "  volumeClaimTemplates:"
echo "    - metadata:"
echo "        name: storage"
echo "        annotations:"
echo "          null"
echo "      spec:"
echo "        ..."
echo ""
echo "With:"
echo "  volumeClaimTemplates:"
echo "    - apiVersion: v1"
echo "      kind: PersistentVolumeClaim"
echo "      metadata:"
echo "        name: storage"
echo "      spec:"
echo "        ..."
echo "        volumeMode: Filesystem"
echo ""
