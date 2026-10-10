#!/usr/bin/env bash
set -euo pipefail
chart="$(cd "$(dirname "$0")/.." && pwd)"
for enabled in true false; do
  output="$(helm template app-smoke "$chart" --namespace app-smoke \
    --set image.repository=example.invalid/smoke --set image.tag=v1 \
    --set httproute.enabled=true --set httproute.hostnames[0]=smoke.app.yu-min3.com \
    --set auth.gatewayOAuth2.enabled=true --set httproute.dns.enabled="$enabled")"
  ignore=true
  if [[ "$enabled" == true ]]; then ignore=false; fi
  [[ $(printf '%s\n' "$output" | grep -cx 'kind: HTTPRoute') == 2 ]]
  [[ $(printf '%s\n' "$output" | grep -cF "k8s-gateway.dns/ignore: \"$ignore\"") == 2 ]]
done
printf '%s\n' 'PASS: DNS opt-out applies to normal and OAuth2 routes together'
