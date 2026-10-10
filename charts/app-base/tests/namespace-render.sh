#!/usr/bin/env bash
set -euo pipefail
chart="$(cd "$(dirname "$0")/.." && pwd)"
render="$(mktemp)"
trap 'rm -f "$render"' EXIT
base=(helm template demo "$chart" --namespace app-demo --set image.repository=example.invalid/demo --set image.tag=v1)
"${base[@]}" > "$render"
if rg -q '^kind: Namespace$' "$render"; then exit 1; fi
"${base[@]}" --set namespace.create=true --set namespace.team=platform --set namespace.app=demo > "$render"
ruby -ryaml -e '
docs=YAML.load_stream(File.read(ARGV[0])); ns=docs.select{|d| d && d["kind"]=="Namespace"}; abort "namespace count" unless ns.size==1
m=ns[0]["metadata"]; abort "wrong namespace" unless m["name"]=="app-demo"
a=m["annotations"]; abort "retention" unless a["helm.sh/resource-policy"]=="keep" && a["argocd.argoproj.io/sync-options"]=="Prune=false" && a["argocd.argoproj.io/sync-wave"]=="-2"
l=m["labels"]; abort "security" unless l["pod-security.kubernetes.io/enforce"]=="restricted" && l["kensan-lab.platform/pss-level"]=="restricted"
abort "labels" unless l["kensan-lab.platform/team"]=="platform" && l["kensan-lab.platform/app"]=="demo" && l["kensan-lab.platform/environment"]=="production"
' "$render"
if "${base[@]}" --set namespace.create=true >/dev/null 2>&1; then exit 1; fi
if "${base[@]}" --set namespace.create=true --set namespace.team='group:default/platform' --set namespace.app=demo >/dev/null 2>&1; then exit 1; fi
