#!/bin/bash
# Prometheus Helm リポジトリ追加
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# kube-prometheus-stack マニフェスト生成
helm template prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --include-crds \
  --values ../../base-infra/prometheus/prometheus-values.yaml \
  > ../../base-infra/prometheus/prometheus-stack.yaml
