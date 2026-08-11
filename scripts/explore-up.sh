#!/usr/bin/env bash
#
# Bring up the kind explore cluster. See docs/getting-started/try-it-with-kind.md.
#
# The design brief for this script is that every way it can fail should fail
# *before* the ten minute wait, and should say what to do in a sentence a person
# can act on. Everything after preflight is meant to be boring.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_DIR="${REPO_ROOT}/environments/kind"
# shellcheck source=../environments/kind/versions.sh
source "${ENV_DIR}/versions.sh"

CLUSTER_NAME="kensan-lab-explore"
REPO_URL="https://github.com/yu-min3/kensan-lab"
REVISION="main"
# Argo CD needs to pull manifests from a repository it can reach. Everything
# below is read from git over HTTPS, so a fork has to be pushed before it can be
# explored — there is no local-path mode, and pretending otherwise would only
# fail later and less clearly.
WAIT_TIMEOUT=900
# Optional. Backstage's scaffolder publishes to GitHub, and no platform can
# hand a visitor a credential for somebody else's account — so this is the one
# thing the explore layer asks you to bring. Without it everything still runs
# and the golden path template is visible; only pressing Create stops short.
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

usage() {
  cat <<'EOF'
usage: scripts/explore-up.sh [--repo URL] [--rev REVISION] [--timeout SECONDS]

  --repo URL       Git repository Argo CD syncs from (default: upstream)
  --rev REVISION   Branch, tag or SHA to sync (default: main)
  --timeout SEC    How long to wait for every Application to go healthy

Set GITHUB_TOKEN in the environment to let Backstage's scaffolder create
repositories and open pull requests. Everything works without it except the
Create button.

CI passes --rev "$GITHUB_SHA" so the cluster proves the pull request rather
than main. A fork passes --repo its own URL.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO_URL="$2"; shift 2 ;;
    --rev) REVISION="$2"; shift 2 ;;
    --timeout) WAIT_TIMEOUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

info() { printf '\033[36m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[31mcannot continue:\033[0m %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Preflight
# ---------------------------------------------------------------------------

info "checking prerequisites"

missing=()
for tool in docker kind kubectl helm curl; do
  command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
done
if [[ ${#missing[@]} -gt 0 ]]; then
  fail "these tools are not installed: ${missing[*]}
     macOS:  brew install ${missing[*]}
     Linux:  see docs/getting-started/try-it-with-kind.md#prerequisites"
fi

docker info >/dev/null 2>&1 || fail "the Docker daemon is not running. Start Docker Desktop (or dockerd) and try again."

# Istio's control plane plus Kyverno's webhook plus Argo CD's five components do
# not fit in the 4GB Docker Desktop hands out by default. Finding that out as a
# Pending pod twenty minutes in is the worst version of this failure.
#
# Measured in MiB rather than GiB: `docker info` reports what the Linux VM sees,
# which is a few hundred MiB below what Docker Desktop was told to allocate
# (kernel reservations). Rounding that down to whole GiB rejects a correctly
# configured 6 GiB machine, so the floor sits below the nominal setting.
MIN_MEM_MIB=5600
mem_bytes="$(docker info --format '{{.MemTotal}}' 2>/dev/null || echo 0)"
mem_mib=$(( mem_bytes / 1024 / 1024 ))
if [[ "$mem_bytes" -gt 0 && "$mem_mib" -lt "$MIN_MEM_MIB" ]]; then
  fail "Docker can use ${mem_mib}MiB of memory; this needs the equivalent of 6GiB allocated.
     Docker Desktop: Settings -> Resources -> Memory, then restart Docker.
     8GiB is worth setting if you have it — the Phase 1 slice fits in 6, with nothing to spare."
fi

cluster_exists=false
if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  cluster_exists=true
  info "reusing the existing '${CLUSTER_NAME}' cluster (run 'make explore-down' for a clean one)"
fi

# The gateway is published on host ports 80 and 443. Anything already bound
# there wins, and the demo URL would quietly hit the wrong server.
if [[ "$cluster_exists" == false ]] && command -v nc >/dev/null 2>&1; then
  for port in 80 443; do
    if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
      fail "something is already listening on port ${port}, which the gateway needs.
     Find it with:  sudo lsof -nP -iTCP:${port} -sTCP:LISTEN"
    fi
  done
fi

# ---------------------------------------------------------------------------
# Cluster
# ---------------------------------------------------------------------------

if [[ "$cluster_exists" == false ]]; then
  info "creating the kind cluster (this is the slow part)"
  kind create cluster --config "${ENV_DIR}/kind-cluster.yaml" --wait 120s
fi

# kind wrote a context for the new cluster and made it current. Switching
# explicitly anyway, so that every kubectl below is aimed at the throwaway
# cluster no matter what the caller had selected — the one thing this script
# must never do is touch somebody's real cluster.
kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

# kind ships `standard` as the default StorageClass and the explore layer adds
# `longhorn` pointing at the same provisioner, so that the repository's PVCs
# bind unmodified. Two defaults would make every PVC fail to bind, so the
# built-in one steps down here rather than being deleted — kind owns it.
info "demoting kind's built-in StorageClass so 'longhorn' can be the default"
kubectl patch storageclass standard \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}' \
  >/dev/null

# ---------------------------------------------------------------------------
# Argo CD
# ---------------------------------------------------------------------------

# Argo CD is the only component installed out of band: nothing can sync it
# before it exists. The chart version is pinned in environments/kind/versions.sh
# and CI fails if it drifts from the bare-metal Application.
info "installing Argo CD ${ARGOCD_CHART_VERSION}"
kubectl apply -f "${REPO_ROOT}/kubernetes/argocd/resources/namespace.yaml" >/dev/null
helm repo add argo "${ARGOCD_CHART_REPO}" >/dev/null 2>&1 || true
helm repo update argo >/dev/null
helm upgrade --install argocd argo/argo-cd \
  --version "${ARGOCD_CHART_VERSION}" \
  --namespace argocd \
  --values "${REPO_ROOT}/kubernetes/argocd/values.yaml" \
  --values "${ENV_DIR}/values/argocd.yaml" \
  --wait --timeout 10m >/dev/null

# The scaffolder needs a token to reach GitHub, and there is nowhere for one to
# come from: Vault is not part of this slice, and a credential for the visitor's
# own account cannot be generated. So it is passed in or the feature is absent —
# the Secret is created either way, holding an empty string, so that the
# Deployment's reference always resolves.
#
# The placeholder is not decoration. Backstage type-checks its config at
# startup, and `integrations.github[0].token` set to an empty string fails that
# check — which does not stop the process or fail the probe, it stops the
# catalog, scaffolder and techdocs plugins from ever initialising. The portal
# then serves its shell, answers /healthcheck, reports Healthy to Argo CD, and
# returns 503 for every catalog request. A syntactically valid token that GitHub
# will reject keeps all three plugins running and moves the failure to where it
# belongs: the moment somebody presses Create.
if [[ -n "$GITHUB_TOKEN" ]]; then
  info "wiring the GitHub token into Backstage (scaffolder enabled)"
else
  info "no GITHUB_TOKEN set — Backstage runs without the scaffolder's publish step"
  GITHUB_TOKEN="ghp_0000000000000000000000000000000000no-token-was-supplied"
fi
kubectl apply -f "${REPO_ROOT}/kubernetes/backstage/namespace.yaml" >/dev/null
kubectl -n backstage create secret generic backstage-explore-github \
  --from-literal=GITHUB_TOKEN="${GITHUB_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# Grafana reads its admin credentials from a Secret. Bare metal fills that from
# Vault; here it is generated, printed at the end, and gone when the cluster is.
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f - >/dev/null
GRAFANA_PASSWORD="$(openssl rand -base64 18 | tr -d '/+=' | head -c 20)"
kubectl -n monitoring create secret generic grafana-explore-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="${GRAFANA_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

info "applying the AppProjects"
kubectl apply -f "${REPO_ROOT}/kubernetes/argocd/projects/" >/dev/null

# The AppProjects warn about resources no Application manages. On bare metal
# that is a real signal. On kind it is 50-odd objects that kind itself created,
# and the warning badge lands on every application in the tree — the first thing
# anyone opens. Turned off here rather than in the shared manifest, because the
# signal is worth keeping where it means something. Nothing syncs the projects
# in the explore slice, so this is not drift waiting to be reverted.
for project in platform-project app-project; do
  kubectl -n argocd patch appproject "$project" --type merge \
    -p '{"spec":{"orphanedResources":null}}' >/dev/null
done

# ---------------------------------------------------------------------------
# Hand over to GitOps
# ---------------------------------------------------------------------------

info "handing the cluster to Argo CD (repo ${REPO_URL}, revision ${REVISION})"

# The committed root app names upstream/main so that it stays a valid, lintable
# manifest on its own. The substitution happens on the way in rather than as a
# patch afterwards: applying it first and correcting it second would give Argo
# CD a few seconds in which it is syncing main, and in CI that would silently
# validate the wrong commit.
#
# repoURL appears twice (the chart source, and the helm parameter every child
# Application inherits) and the revision once in each of the same two places.
root_app="$(sed \
  -e "s|https://github.com/yu-min3/kensan-lab|${REPO_URL}|g" \
  -e "s|targetRevision: main|targetRevision: ${REVISION}|" \
  -e "s|value: main|value: ${REVISION}|" \
  "${ENV_DIR}/explore-root-app.yaml")"

if ! printf '%s' "$root_app" | grep -q "targetRevision: ${REVISION}"; then
  fail "could not set the revision in explore-root-app.yaml — the file's shape changed.
     Expected a line reading 'targetRevision: main' to substitute."
fi

printf '%s' "$root_app" | kubectl apply -f - >/dev/null

# ---------------------------------------------------------------------------
# Wait
# ---------------------------------------------------------------------------

# `argocd app wait` would be nicer, but it means installing the CLI, and
# `kubectl wait` cannot read Argo CD's health status. Polling the two status
# fields is the honest version and it prints what is still moving.
info "waiting for every Application to go Synced/Healthy (up to ${WAIT_TIMEOUT}s)"
deadline=$(( $(date +%s) + WAIT_TIMEOUT ))
last_report=""
while :; do
  status="$(kubectl -n argocd get applications.argoproj.io \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.sync.status}{" "}{.status.health.status}{"\n"}{end}' \
    2>/dev/null || true)"

  pending="$(printf '%s\n' "$status" | awk 'NF && !($2 == "Synced" && $3 == "Healthy") {print "     " $1 " (" $2 "/" $3 ")"}')"

  if [[ -n "$status" && -z "$pending" ]]; then
    info "all $(printf '%s\n' "$status" | grep -c . ) Applications are Synced/Healthy"
    break
  fi

  if [[ "$pending" != "$last_report" ]]; then
    printf '  still working:\n%s\n' "$pending"
    last_report="$pending"
  fi

  if [[ $(date +%s) -ge $deadline ]]; then
    printf '\n' >&2
    fail "timed out. What is still not healthy:
${pending}

     Look at it with:
       kubectl -n argocd get applications
       kubectl -n argocd describe application <name>
     Troubleshooting: docs/getting-started/try-it-with-kind.md#troubleshooting"
  fi
  sleep 10
done

# ---------------------------------------------------------------------------
# Smoke test
# ---------------------------------------------------------------------------

# Sent with an explicit Host header rather than the sslip.io name, so this works
# with no DNS at all. The browser instructions below do need DNS; the offline
# fallback is in the docs.
info "checking that the gateway actually serves the demo app"
code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 \
  -H 'Host: demo.127-0-0-1.sslip.io' http://127.0.0.1/ || echo 000)"
if [[ "$code" != "200" ]]; then
  fail "the demo app answered HTTP ${code} through the gateway, not 200.
     Every Application is healthy, so this is routing rather than workloads:
       kubectl -n istio-system get gateway gateway-explore -o yaml
       kubectl -n app-demo get httproute demo -o yaml
     Troubleshooting: docs/getting-started/try-it-with-kind.md#troubleshooting"
fi

# And again over TLS. This is a separate assertion because it can fail on its
# own: the route can be perfect while the certificate is still being issued, in
# which case Istio serves the listener with no certificate at all.
info "checking that the gateway serves the same app over TLS"
code="$(curl -sSk -o /dev/null -w '%{http_code}' --max-time 15 \
  --resolve "demo.127-0-0-1.sslip.io:443:127.0.0.1" \
  https://demo.127-0-0-1.sslip.io/ || echo 000)"
if [[ "$code" != "200" ]]; then
  fail "the demo app answered HTTP ${code} over TLS, not 200.
     Plain HTTP works, so the route is fine and the certificate is not:
       kubectl -n istio-system get certificate explore-wildcard
       kubectl -n cert-manager get certificate explore-ca
     Troubleshooting: docs/getting-started/try-it-with-kind.md#troubleshooting"
fi

# Backstage's probe is a liveness check on the HTTP router, not on the plugins
# behind it. A backend whose catalog failed to start still serves the frontend
# shell and still answers /healthcheck with 200, so the pod is Ready, the
# Application is Healthy, and the portal is useless. The only way to know is to
# ask the catalog something.
info "checking that Backstage's catalog is actually serving"
for attempt in $(seq 1 30); do
  code="$(curl -sSk -o /dev/null -w '%{http_code}' --max-time 10 \
    --resolve "backstage.127-0-0-1.sslip.io:443:127.0.0.1" \
    "https://backstage.127-0-0-1.sslip.io/api/catalog/entities?limit=1" || echo 000)"
  [[ "$code" == "200" ]] && break
  sleep 5
done
if [[ "$code" != "200" ]]; then
  fail "Backstage's catalog answered HTTP ${code}, not 200.
     The pod is Ready and the frontend loads, so this is a backend plugin that
     failed to initialise rather than a routing or scheduling problem:
       kubectl -n backstage logs deploy/backstage -c backstage | grep -i 'during startup'
     A config value the backend type-checks at boot is the usual cause."
fi

admin_password="$(kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' 2>/dev/null | base64 --decode || echo '(already rotated)')"

cat <<EOF

  The platform is up. Four things worth doing, in order.

  Every URL below is https, and your browser will warn about the certificate.
  cert-manager issued it from a CA this cluster generated a minute ago, and
  nothing on your machine has a reason to trust that — there is no domain here
  to prove ownership of. Clicking through is the intended path.

  1. Open the GitOps tree            https://argocd.127-0-0-1.sslip.io
     user 'admin', password '${admin_password}'
     The 'explore-root' application is the app-of-apps: open it and every
     component below it is one Application, exactly as on the real cluster.

  2. Open Grafana                    https://grafana.127-0-0-1.sslip.io
     user 'admin', password '${GRAFANA_PASSWORD}'
     The 'Cluster Health' dashboard is the bare-metal one, unedited. The panels
     that stay empty are reading a Raspberry Pi's temperature sensor.

  3. Open the demo app               https://demo.127-0-0-1.sslip.io
     It reached the browser through Istio's Gateway API implementation, and it
     was deployed by charts/app-base — the same chart the real apps use, with
     no changes, only a values file.

  4. See what the policy engine thinks
       kubectl get policyreport -A
     Kyverno is running the production policies in Audit mode, so violations
     are reported rather than blocked (ADR-012).

  What this cluster deliberately cannot show you — the L2 load balancer, SSO,
  storage replication, multi-architecture scheduling — and why:
  docs/getting-started/try-it-with-kind.md#what-kind-cannot-show

  Your kubectl now points at this cluster, not whatever it pointed at before.
  'make explore-down' removes the context along with the cluster.

  Tear it down with:  make explore-down

EOF
