#!/bin/bash
# OpenTelemetry Collector Helm リポジトリ追加
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update open-telemetry

# OpenTelemetry Collector マニフェスト生成
helm template otel-collector open-telemetry/opentelemetry-collector \
  --version 0.97.1 \
  --namespace monitoring \
  --values ../../infrastructure/observability/otel-collector/values.yaml \
  > ../../infrastructure/observability/otel-collector/otel-collector-manifests.yaml

echo "OpenTelemetry Collector manifests generated successfully!"
