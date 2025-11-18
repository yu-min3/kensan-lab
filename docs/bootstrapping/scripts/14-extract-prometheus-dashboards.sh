#!/bin/bash
# Extract Prometheus Dashboard ConfigMaps from kube-prometheus-stack
#
# Background:
# When grafana.enabled=false, the Helm chart doesn't generate dashboard ConfigMaps.
# This script temporarily enables Grafana to extract the dashboard ConfigMaps,
# then saves them separately for use with our independent Grafana deployment.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROMETHEUS_DIR="$SCRIPT_DIR/../../../infrastructure/observability/prometheus"
OUTPUT_FILE="$PROMETHEUS_DIR/dashboards-configmaps.yaml"

echo "Extracting Prometheus dashboard ConfigMaps..."
echo ""

# Check if Python is available
if ! command -v python3 &> /dev/null; then
    echo "Error: python3 is required but not installed"
    exit 1
fi

# Check if PyYAML is available
if ! python3 -c "import yaml" 2>/dev/null; then
    echo "Error: PyYAML is required. Install with: pip3 install pyyaml"
    exit 1
fi

# Step 1: Create temporary values file with Grafana enabled
echo "Step 1: Creating temporary values file with Grafana enabled..."
cat > /tmp/prometheus-values-with-grafana.yaml << 'EOF'
grafana:
  enabled: true
  defaultDashboardsEnabled: true
  defaultDashboardsTimezone: Asia/Tokyo
EOF

# Step 2: Generate full manifest with Grafana enabled
echo "Step 2: Generating Helm manifest with Grafana enabled..."
helm template prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values "$PROMETHEUS_DIR/prometheus-values.yaml" \
  --values /tmp/prometheus-values-with-grafana.yaml \
  2>/dev/null > /tmp/prometheus-with-dashboards.yaml

echo "  Generated: $(wc -l < /tmp/prometheus-with-dashboards.yaml) lines"

# Step 3: Extract dashboard ConfigMaps using Python
echo "Step 3: Extracting dashboard ConfigMaps..."
python3 << 'PYEOF'
import yaml
import sys

input_file = "/tmp/prometheus-with-dashboards.yaml"
output_file = "OUTPUT_FILE_PLACEHOLDER"

try:
    # Read all documents from the YAML file
    with open(input_file, 'r') as f:
        docs = list(yaml.safe_load_all(f))

    # Filter ConfigMaps with grafana_dashboard label
    dashboard_cms = []
    for doc in docs:
        if doc and doc.get('kind') == 'ConfigMap':
            labels = doc.get('metadata', {}).get('labels', {})
            if labels.get('grafana_dashboard') == '1':
                dashboard_cms.append(doc)

    # Write to output file
    with open(output_file, 'w') as f:
        yaml.dump_all(dashboard_cms, f, default_flow_style=False, sort_keys=False)

    print(f"  Extracted: {len(dashboard_cms)} dashboard ConfigMaps")

    # Print dashboard names
    print("\nDashboard ConfigMaps:")
    for cm in dashboard_cms:
        name = cm.get('metadata', {}).get('name', 'unknown')
        print(f"  - {name}")

except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
PYEOF

# Replace placeholder with actual output file path
sed -i "s|OUTPUT_FILE_PLACEHOLDER|$OUTPUT_FILE|g" /tmp/extract_dashboards.py 2>/dev/null || true

# Run the Python script directly
python3 << PYEOF
import yaml

input_file = "/tmp/prometheus-with-dashboards.yaml"
output_file = "$OUTPUT_FILE"

with open(input_file, 'r') as f:
    docs = list(yaml.safe_load_all(f))

dashboard_cms = []
for doc in docs:
    if doc and doc.get('kind') == 'ConfigMap':
        labels = doc.get('metadata', {}).get('labels', {})
        if labels.get('grafana_dashboard') == '1':
            dashboard_cms.append(doc)

with open(output_file, 'w') as f:
    yaml.dump_all(dashboard_cms, f, default_flow_style=False, sort_keys=False)

print(f"  Extracted: {len(dashboard_cms)} dashboard ConfigMaps")

print("\nDashboard ConfigMaps:")
for cm in dashboard_cms:
    name = cm.get('metadata', {}).get('name', 'unknown')
    print(f"  - {name}")
PYEOF

# Step 4: Clean up temporary files
echo ""
echo "Step 4: Cleaning up temporary files..."
rm -f /tmp/prometheus-values-with-grafana.yaml
rm -f /tmp/prometheus-with-dashboards.yaml

# Step 5: Verify output
echo ""
echo "✓ Prometheus dashboard ConfigMaps extracted successfully!"
echo "  Output: $OUTPUT_FILE"
echo "  Size: $(du -h "$OUTPUT_FILE" | cut -f1)"
echo "  Lines: $(wc -l < "$OUTPUT_FILE")"
echo ""
echo "To apply these dashboards:"
echo "  kubectl apply -f $OUTPUT_FILE"
