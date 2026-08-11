#!/usr/bin/env bash
#
# Tear down the kind explore cluster.
#
# `kind delete` removes the node containers and the kubeconfig entry, but leaves
# the images it pulled in the local Docker cache — several gigabytes of Istio,
# Kyverno and Argo CD that the person who tried this repository never asked for.
# Cleaning those up is the difference between "I tried it" and "it left a mess",
# so it is offered here and never done silently: the same images may be in use
# by someone's other clusters.
set -euo pipefail

CLUSTER_NAME="kensan-lab-explore"
PRUNE_IMAGES=false

usage() {
  cat <<'EOF'
usage: scripts/explore-down.sh [--prune-images]

  --prune-images   also remove the container images this cluster pulled
                   (istio, kyverno, argo-cd, keycloak). Images shared with your
                   other clusters are removed too — Docker cannot tell them
                   apart. Left off by default.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prune-images) PRUNE_IMAGES=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

info() { printf '\033[36m==>\033[0m %s\n' "$*"; }

command -v kind >/dev/null 2>&1 || { echo "kind is not installed; nothing to delete." >&2; exit 0; }

if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  info "deleting the '${CLUSTER_NAME}' cluster"
  kind delete cluster --name "$CLUSTER_NAME"
else
  info "no '${CLUSTER_NAME}' cluster to delete"
fi

if [[ "$PRUNE_IMAGES" == true ]]; then
  info "removing the images this cluster pulled"
  # Matched by repository name rather than by dangling status: the images are
  # tagged and in use by nothing once the cluster is gone, so `docker image
  # prune` would not touch them.
  for pattern in \
    'docker.io/istio/' \
    'ghcr.io/kyverno/' \
    'quay.io/argoproj/' \
    'ghcr.io/dexidp/' \
    'ghcr.io/yu-min3/kensan-lab/explore-demo' \
    'quay.io/keycloak/' \
    'kindest/node'
  do
    ids="$(docker images --filter "reference=${pattern}*" -q | sort -u)"
    [[ -n "$ids" ]] && docker rmi -f $ids >/dev/null 2>&1 || true
  done
  info "done — 'docker system df' will show what is left"
else
  cat <<'EOF'

  The cluster is gone. Its container images are still cached, which makes the
  next `make try` much faster. To reclaim that disk instead:

    scripts/explore-down.sh --prune-images

EOF
fi
