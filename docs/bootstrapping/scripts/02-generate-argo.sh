#!/bin/bash
# Helm リポジトリ追加
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update

# マニフェスト生成
helm template argocd argo/argo-cd \
  --namespace argocd \
  --set server.service.type=ClusterIP \
  > ../../base-infra/argocd/argocd-install.yaml

echo "⚠️ IMPORTANT: You need to manually edit base-infra/argocd/argocd-install.yaml"
echo "Add the following to the argocd-cmd-params-cm ConfigMap data section:"
echo '  server.insecure: "true"  # Run server without TLS (TLS termination is handled by Istio Gateway)'
