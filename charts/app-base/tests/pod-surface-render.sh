#!/usr/bin/env bash
# initContainers / extraContainers / podAnnotations / podLabels / httproute.filters が描画に入り、
# selector には podLabels が混ざらないことを確かめる。
set -euo pipefail

chart="$(cd "$(dirname "$0")/.." && pwd)"
base=(helm template pod-test "$chart" --set image.repository=example.invalid/widget --set image.tag=v1)

values="$(mktemp)"
trap 'rm -f "$values"' EXIT
cat > "$values" <<'EOF'
initContainers:
  - name: fetch
    image: example.invalid/fetch:v1
extraContainers:
  - name: sidecar
    image: example.invalid/sidecar:v1
podAnnotations:
  example.invalid/note: "on"
podLabels:
  example.invalid/role: viewer
EOF

deployment_of() { awk '/^kind: Deployment$/{f=1} f&&/^---$/{exit} f'; }

# expect <rendered deployment> <yq path> <want>
expect() {
  local got
  got="$(printf '%s\n' "$1" | yq -r "$2")"
  if [ "$got" != "$3" ]; then
    echo "$2: want $3, got $got" >&2
    exit 1
  fi
}

custom="$("${base[@]}" -f "$values" | deployment_of)"
expect "$custom" '.spec.template.spec.initContainers[0].name' fetch
expect "$custom" '.spec.template.spec.containers[0].name' app
expect "$custom" '.spec.template.spec.containers[1].name' sidecar
expect "$custom" '.spec.template.metadata.annotations["example.invalid/note"]' on
expect "$custom" '.spec.template.metadata.labels["example.invalid/role"]' viewer
# podLabels は selector に入らない（Deployment の selector は作り直さないと変えられない）
expect "$custom" '.spec.selector.matchLabels["example.invalid/role"]' null

# httproute.filters は app へのルールにだけ付き、/oauth2 のルートには付かない
routes="$("${base[@]}" --set httproute.enabled=true --set 'httproute.hostnames[0]=pod-test.app.yu-min3.com' \
  --set auth.gatewayOAuth2.enabled=true \
  --set 'httproute.filters[0].type=URLRewrite' --set 'httproute.filters[0].urlRewrite.hostname=localhost')"
app_route="$(printf '%s\n' "$routes" | yq 'select(.kind == "HTTPRoute" and .metadata.name == "pod-test")')"
oauth_route="$(printf '%s\n' "$routes" | yq 'select(.kind == "HTTPRoute" and .metadata.name == "pod-test-oauth2")')"
expect "$app_route" '.spec.rules[0].filters[0].urlRewrite.hostname' localhost
expect "$oauth_route" '.spec.rules[0].filters' null

# 既定では何も足されない
plain_route="$("${base[@]}" --set httproute.enabled=true --set 'httproute.hostnames[0]=pod-test.app.yu-min3.com' \
  | yq 'select(.kind == "HTTPRoute")')"
expect "$plain_route" '.spec.rules[0].filters' null
plain="$("${base[@]}" | deployment_of)"
expect "$plain" '.spec.template.spec.initContainers' null
expect "$plain" '.spec.template.spec.containers[1]' null
expect "$plain" '.spec.template.metadata.annotations' null
