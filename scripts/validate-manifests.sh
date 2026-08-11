#!/usr/bin/env bash
set -euo pipefail

mode="${1:-all}"

run_yamllint() {
  yamllint -c .yamllint kubernetes charts environments
}

run_kubeconform() {
  find kubernetes environments -name '*.yaml' \
    ! -name 'values.yaml' \
    ! -name 'Chart.yaml' \
    ! -path '*/chart/templates/*' \
    ! -path '*/platform-values/*' \
    ! -path 'environments/kind/applications/templates/*' \
    ! -path 'environments/kind/values/*' \
    -print0 | xargs -0 kubeconform -ignore-missing-schemas -summary
}

run_helm() {
  helm template app-kensan charts/app-base -f kubernetes/apps/app-kensan/values.yaml >/dev/null
  helm template explore environments/kind/applications >/dev/null
  helm template app-demo charts/app-base -f environments/kind/values/demo-app.yaml >/dev/null

  shopt -s nullglob
  local f name database_glob transit_glob
  database_glob='kubernetes/secrets/vault-database-engine/platform-values/vault-database/*.yaml'
  transit_glob='kubernetes/secrets/vault-transit-engine/platform-values/vault-transit/*.yaml'
  if ! compgen -G "$database_glob" >/dev/null; then
    helm template ci-smoke kubernetes/secrets/vault-database-engine/chart \
      --set name=ci-smoke --set ns=ci-smoke >/dev/null
  fi
  for f in $database_glob; do
    name="postgres-$(basename "$f" .yaml)"
    helm template "$name" kubernetes/secrets/vault-database-engine/chart \
      -f "$f" --set "name=$name" >/dev/null
  done

  if ! compgen -G "$transit_glob" >/dev/null; then
    helm template ci-smoke kubernetes/secrets/vault-transit-engine/chart \
      --set name=ci-smoke --set ns=ci-smoke --set keyName=ci-smoke >/dev/null
  fi
  for f in $transit_glob; do
    name="transit-$(basename "$f" .yaml)"
    helm template "$name" kubernetes/secrets/vault-transit-engine/chart \
      -f "$f" --set "name=$name" >/dev/null
  done
}

run_argocd() {
  python3 scripts/validate_argocd_apps.py
}

case "$mode" in
  yamllint) run_yamllint ;;
  kubeconform) run_kubeconform ;;
  helm) run_helm ;;
  argocd) run_argocd ;;
  all)
    run_yamllint
    run_kubeconform
    run_helm
    run_argocd
    ;;
  *)
    echo "usage: $0 [all|yamllint|kubeconform|helm|argocd]" >&2
    exit 2
    ;;
esac
