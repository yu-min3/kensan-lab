#!/usr/bin/env bash
# namespace.create の既定 false では Namespace を描かず、true では Platform のラベルと
# 保持設定つきで 1 つだけ描き、必須値の欠落・不正なラベル値を拒否することを確かめる。
set -euo pipefail

chart="$(cd "$(dirname "$0")/.." && pwd)"
base=(helm template demo "$chart" --namespace app-demo --set image.repository=example.invalid/demo --set image.tag=v1)

namespaces_of() { yq -r 'select(.kind == "Namespace")' -; }

# expect <rendered namespace> <yq path> <want>
expect() {
  local got
  got="$(printf '%s\n' "$1" | yq -r "$2")"
  if [ "$got" != "$3" ]; then
    echo "$2: want $3, got $got" >&2
    exit 1
  fi
}

# 既定では描かない
if [ -n "$("${base[@]}" | namespaces_of)" ]; then
  echo 'Namespace rendered by default' >&2
  exit 1
fi

ns="$("${base[@]}" --set namespace.create=true --set namespace.team=platform --set namespace.app=demo | namespaces_of)"
if [ "$(printf '%s\n' "$ns" | yq -r '.metadata.name' | grep -c .)" != 1 ]; then
  echo 'want exactly one Namespace' >&2
  exit 1
fi
expect "$ns" '.metadata.name' app-demo
expect "$ns" '.metadata.annotations["helm.sh/resource-policy"]' keep
expect "$ns" '.metadata.annotations["argocd.argoproj.io/sync-options"]' Prune=false
expect "$ns" '.metadata.annotations["argocd.argoproj.io/sync-wave"]' -2
expect "$ns" '.metadata.labels["pod-security.kubernetes.io/enforce"]' restricted
expect "$ns" '.metadata.labels["kensan-lab.platform/pss-level"]' restricted
expect "$ns" '.metadata.labels["kensan-lab.platform/tier"]' application
expect "$ns" '.metadata.labels["kensan-lab.platform/environment"]' production
expect "$ns" '.metadata.labels["kensan-lab.platform/team"]' platform
expect "$ns" '.metadata.labels["kensan-lab.platform/app"]' demo

# 必須値の欠落・ラベルに使えない値は render を失敗させる
for extra in '' '--set namespace.team=group:default/platform --set namespace.app=demo'; do
  # shellcheck disable=SC2086
  if "${base[@]}" --set namespace.create=true $extra >/dev/null 2>&1; then
    echo "invalid namespace values rendered: ${extra:-（team・app 未指定）}" >&2
    exit 1
  fi
done
