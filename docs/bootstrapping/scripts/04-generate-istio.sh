#!/bin/bash
# Istio Helm リポジトリ追加
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update

# istio-base (CRDs) を生成
helm template istio-base istio/base \
  --namespace istio-system \
  --version 1.27.3 \
  > ../../base-infra/istio/01-istio-base.yaml

# istiod (Control Plane) を生成（Gateway API サポート有効化）
helm template istiod istio/istiod \
  --namespace istio-system \
  --version 1.27.3 \
  --set pilot.env.PILOT_ENABLE_GATEWAY_API=true \
  --set pilot.env.PILOT_ENABLE_GATEWAY_API_STATUS=true \
  --set pilot.env.PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER=true \
  > ../../base-infra/istio/02-istiod.yaml
