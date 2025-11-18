#!/bin/bash
# Generate Grafana OTel Dashboards ConfigMap
# This script downloads dashboard JSONs from Grafana.com and creates a ConfigMap

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/../../../infrastructure/observability/grafana"
OUTPUT_FILE="$OUTPUT_DIR/dashboards.yaml"

echo "Downloading OpenTelemetry dashboards from Grafana.com..."

# Download OpenTelemetry APM dashboard (ID: 19419)
echo "  - OpenTelemetry APM (19419)..."
curl -s https://grafana.com/api/dashboards/19419/revisions/6/download \
  | sed 's/${DS_PROMETHEUS}/prometheus/g' \
  > /tmp/otel-apm-dashboard.json

# Download OpenTelemetry for HTTP Services dashboard (ID: 21587)
echo "  - OpenTelemetry for HTTP Services (21587)..."
curl -s https://grafana.com/api/dashboards/21587/revisions/1/download \
  | sed 's/${DS_PROMETHEUS}/prometheus/g' \
  | sed 's/${DS_TEMPO}/tempo/g' \
  | sed 's/${DS_LOKI}/loki/g' \
  > /tmp/otel-http-services-dashboard.json

echo "Creating dashboards ConfigMap..."

cat > "$OUTPUT_FILE" << 'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-otel-dashboards
  namespace: monitoring
  labels:
    grafana_dashboard: "1"  # Grafana sidecar が自動検出
    grafana_folder: "OpenTelemetry"
data:
EOF

# Add OpenTelemetry APM dashboard
echo "  otel-apm-dashboard.json: |" >> "$OUTPUT_FILE"
sed 's/^/    /' /tmp/otel-apm-dashboard.json >> "$OUTPUT_FILE"

# Add OpenTelemetry for HTTP Services dashboard
echo "  otel-http-services-dashboard.json: |" >> "$OUTPUT_FILE"
sed 's/^/    /' /tmp/otel-http-services-dashboard.json >> "$OUTPUT_FILE"

# Clean up temp files
rm /tmp/otel-apm-dashboard.json /tmp/otel-http-services-dashboard.json

echo "✓ Grafana dashboards ConfigMap generated successfully!"
echo "  Output: $OUTPUT_FILE"
