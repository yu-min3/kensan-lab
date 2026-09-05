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

# Working files, and where every generated credential lives for the length of
# this script. Removed on exit however it exits.
TMP_DIR="$(mktemp -d)"

# Whatever kubectl was pointing at before this ran.
#
# On success the kind context is left current on purpose — every command in the
# walkthrough is a plain `kubectl`, and the closing message says so. On failure
# it is put back: a run that stopped halfway has no business leaving somebody
# aimed at a cluster that may not exist, especially when the message explaining
# the switch never printed.
PREVIOUS_CONTEXT="$(kubectl config current-context 2>/dev/null || true)"
on_exit() {
  local status=$?
  rm -rf "${TMP_DIR}"
  # The seed's port-forward, if one is still running. Held here rather than
  # behind its own trap: replacing this one would drop the context restore
  # below, and the failure would be a kubectl pointing at a cluster that no
  # longer exists.
  if [[ -n "${GITEA_PF_PID:-}" ]]; then
    kill "${GITEA_PF_PID}" 2>/dev/null || true
    wait "${GITEA_PF_PID}" 2>/dev/null || true
  fi
  if [[ "$status" -ne 0 && -n "$PREVIOUS_CONTEXT" ]]; then
    kubectl config use-context "$PREVIOUS_CONTEXT" >/dev/null 2>&1 || true
  fi
  return "$status"
}
trap on_exit EXIT

CLUSTER_NAME="kensan-lab-explore"
# Where Argo CD syncs from: a Gitea inside the cluster, seeded from this
# checkout. Nothing has to be pushed anywhere, a fork is not required, and the
# golden path can create repositories without a GitHub token — which is the
# whole reason it is here. REPO_URL and REVISION are set once that server
# exists, further down.
GITEA_INTERNAL_URL="http://gitea-http.gitea.svc.cluster.local:3000"
GITEA_USERNAME="demo"
GITEA_REPO_PATH="${GITEA_USERNAME}/kensan-lab.git"
EXPLORE_DEMO_IMAGE="kensan-lab/explore-template:local"
EXPLORE_BACKSTAGE_IMAGE="kensan-lab/backstage-explore:local"
WAIT_TIMEOUT=900
# This is a local, disposable account rather than a credential carried into a
# persistent environment. Keep the walkthrough memorable; callers may still
# override it when exercising password handling itself.
DEMO_PASSWORD="${DEMO_PASSWORD:-demo}"

usage() {
  cat <<'EOF'
usage: scripts/explore-up.sh [--timeout SECONDS]

  --timeout SEC    How long to wait for every Application to go healthy

This checkout is what runs: it is pushed into a Gitea inside the cluster and
Argo CD reads from there. No account, no token, no fork, and nothing to push.

The demo account is `demo` / `demo`. Set DEMO_PASSWORD to override it.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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
     Linux:  see docs/getting-started/try-it-with-kind.md#1-start-the-platform"
fi

docker info >/dev/null 2>&1 || fail "the Docker daemon is not running. Start Docker Desktop (or dockerd) and try again."

# Istio's control plane plus Kyverno's webhook plus Argo CD's five components do
# not fit in the 4GB Docker Desktop hands out by default. Finding that out as a
# Pending pod twenty minutes in is the worst version of this failure.
#
# Measured in MiB rather than GiB: `docker info` reports what the Linux VM sees,
# which is a few hundred MiB below what Docker Desktop was told to allocate
# (kernel reservations). Rounding that down to whole GiB rejects a correctly
# configured 8 GiB machine, so the floor sits below the nominal setting.
MIN_MEM_MIB=7400
mem_bytes="$(docker info --format '{{.MemTotal}}' 2>/dev/null || echo 0)"
mem_mib=$(( mem_bytes / 1024 / 1024 ))
if [[ "$mem_bytes" -gt 0 && "$mem_mib" -lt "$MIN_MEM_MIB" ]]; then
  fail "Docker can use ${mem_mib}MiB of memory; this needs the equivalent of 8GiB allocated.
     Docker Desktop: Settings -> Resources -> Memory, then restart Docker.
     Keycloak is what moved this from 6 GiB to 8: an identity provider is a JVM,
     and the single sign-on it enables is worth more than the two gigabytes."
fi

# A shallow checkout cannot be pushed. Git refuses to send a history with no
# beginning and the receiving end has no way to complete it:
#
#   ! [remote rejected] HEAD -> main (shallow update not allowed)
#
# Checked here rather than where it happens, which is ninety seconds in, after
# the cluster is up and Gitea is installed.
if [[ "$(git -C "${REPO_ROOT}" rev-parse --is-shallow-repository 2>/dev/null)" == true ]]; then
  fail "this is a shallow clone, and the cluster's git server cannot be seeded from one.
     Complete the history first:
       git -C . fetch --unshallow"
fi

# Reusing a cluster cannot work, so it is refused rather than half-attempted.
#
# Every runtime client credential below is generated fresh on each run, and
# Keycloak reads its admin password once — at first startup, into an in-memory
# database. A second
# run therefore hands Kubernetes new secrets while Keycloak keeps the old ones,
# and the bootstrap cannot even log in to fix it:
#
#   Logging into http://localhost:8080 as user admin of realm master
#   Invalid user credentials [invalid_grant]
#
# Reconciling the two would mean either keeping the old secrets (and printing a
# password this run did not create) or restarting Keycloak (which drops the
# realm, since the database is in memory). Neither is worth the machinery for a
# cluster whose whole point is being thrown away.
if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  fail "a '${CLUSTER_NAME}' cluster already exists, and this script cannot reuse it.
     Every run generates fresh client credentials, and Keycloak only reads its own at
     first startup — so a second run leaves the two disagreeing.

     Remove it and start again:
       make explore-down && make try"
fi
cluster_exists=false

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

# The built-in demo is available before somebody exercises the Golden Path.
# Generated services do not reuse it: Gitea Actions builds each repository and
# pushes a commit-SHA image to the disposable registry created below.
info "building the built-in demo image from this checkout"
docker build --quiet --tag "${EXPLORE_DEMO_IMAGE}" \
  "${REPO_ROOT}/backstage/templates/fastapi-template/skeleton" >/dev/null

# Explore templates are loaded from Gitea at runtime, but custom scaffolder
# actions are backend code. Building the portal here makes this checkout the
# source of truth for both; a published image tag must never hide a local edit.
info "building the Explore Backstage image from this checkout"
docker build --quiet --tag "${EXPLORE_BACKSTAGE_IMAGE}" \
  --file "${REPO_ROOT}/backstage/packages/backend/Dockerfile" \
  "${REPO_ROOT}/backstage" >/dev/null

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

info "loading the built-in demo image into kind"
# `kind load` reads the node's containerd config, and environments/kind/
# kind-cluster.yaml pins the node image by digest. A kind older than that image
# cannot parse it, and says so as "unknown containerd config version" — after
# the cluster is already up, which reads like a problem with this repository
# rather than with the tool that created it.
kind load docker-image --name "${CLUSTER_NAME}" "${EXPLORE_DEMO_IMAGE}" >/dev/null \
  || fail "could not load the built-in demo image into the cluster.
     If the error above mentions an unknown containerd config version, kind is
     older than the node image this repository pins. Upgrade it and try again —
     v0.32.0 is the version this is tested against:
       kind version
       brew upgrade kind    # macOS"

info "loading the Explore Backstage image into kind"
kind load docker-image --name "${CLUSTER_NAME}" "${EXPLORE_BACKSTAGE_IMAGE}" >/dev/null \
  || fail "could not load the locally built Backstage image into the cluster."

# kind ships `standard` as the default StorageClass and the explore layer adds
# `longhorn` pointing at the same provisioner, so that the repository's PVCs
# bind unmodified. Two defaults would make every PVC fail to bind, so the
# built-in one steps down here rather than being deleted — kind owns it.
info "demoting kind's built-in StorageClass so 'longhorn' can be the default"
kubectl patch storageclass standard \
  -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}' \
  >/dev/null

# The replacement is applied in the same breath as the demotion, rather than
# left to arrive with the Applications. Gitea is installed before Argo CD is
# pointed anywhere and it claims a volume — with the built-in class demoted and
# `longhorn` not yet created, that claim has nothing to bind to and the pod sits
# Pending until the timeout. Argo CD owns the same manifest a few minutes later
# and finds it already matching.
kubectl apply -f "${ENV_DIR}/resources/storageclass-longhorn.yaml" >/dev/null

# The one name that has to mean two different things.
#
# Keycloak signs every token with an `iss` claim of
# https://auth.127-0-0-1.sslip.io/realms/kensan, and whoever validates that
# token has to fetch the issuer's keys from that exact URL — a different URL for
# the same issuer is a different issuer. From a browser the name resolves to
# 127.0.0.1 and the kind port mapping carries it to the gateway. From inside a
# pod, 127.0.0.1 is the pod itself, and the fetch would hit the pod's own
# loopback and fail.
#
# So CoreDNS is taught to answer this one name with the gateway's address.
# Nothing else changes: sslip.io still resolves normally for every other host,
# and the rewrite is invisible to the browser, which never asks CoreDNS.
#
# On bare metal the same split exists and is solved the same way — CoreDNS
# hosts entries for auth.yu-mins.com — because a LAN client and a pod also
# reach the gateway by different routes.
info "teaching CoreDNS to resolve the issuer to the gateway"
kubectl -n kube-system get configmap coredns -o jsonpath='{.data.Corefile}' \
  > "${TMP_DIR}/Corefile"
if ! grep -q "auth.127-0-0-1.sslip.io" "${TMP_DIR}/Corefile"; then
  # Ahead of `kubernetes`, so the rewritten name is resolved as the cluster
  # service it now names rather than being forwarded upstream.
  awk '
    /^    kubernetes cluster.local/ && !done {
      print "    rewrite name auth.127-0-0-1.sslip.io gateway-explore-istio.istio-system.svc.cluster.local"
      done = 1
    }
    { print }
  ' "${TMP_DIR}/Corefile" > "${TMP_DIR}/Corefile.new"
  kubectl -n kube-system create configmap coredns \
    --from-file=Corefile="${TMP_DIR}/Corefile.new" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  # CoreDNS reloads on its own within ~30s (the `reload` plugin), but a restart
  # makes the change take effect before anything asks.
  kubectl -n kube-system rollout restart deployment/coredns >/dev/null
fi

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

# ---------------------------------------------------------------------------
# Single sign-on credentials
# ---------------------------------------------------------------------------
#
# Every client and admin secret below is generated here, now, and never written
# to the repository. The disposable demo user's password is the only exception.
# A client secret committed to a public repository would be a working credential
# against every cluster anybody ever started from it.
#
# They are created *before* Keycloak exists, and Keycloak is then told to use
# them, rather than the other way round. The reverse order deadlocks: the
# oauth2-proxy pod will not start without its Secret, so it never becomes
# healthy, so the bootstrap that would have created the Secret never runs.
rand() { openssl rand -base64 24 | tr -d '/+=' | head -c 24; }

KEYCLOAK_ADMIN_PASSWORD="$(rand)"
DEMO_USER_PASSWORD="${DEMO_PASSWORD}"
ARGOCD_CLIENT_SECRET="$(rand)"
GRAFANA_CLIENT_SECRET="$(rand)"
OAUTH2_PROXY_CLIENT_SECRET="$(rand)"
BACKSTAGE_CLIENT_SECRET="$(rand)"
BACKSTAGE_SESSION_SECRET="$(rand)"
# oauth2-proxy requires exactly 16, 24 or 32 bytes here and refuses to start
# otherwise, with a message that does not mention the length.
OAUTH2_PROXY_COOKIE_SECRET="$(openssl rand -base64 32 | tr -d '\n' | head -c 32)"

info "generating the SSO credentials (they live as long as the cluster does)"
kubectl create namespace platform-auth-prod --dry-run=client -o yaml \
  | kubectl apply -f - >/dev/null
kubectl create namespace auth-system --dry-run=client -o yaml \
  | kubectl apply -f - >/dev/null
## The real namespace, with its ADR-006 labels, arrives with the Application a
## few minutes from now; this is only so the Secret below has somewhere to land.
kubectl apply -f "${REPO_ROOT}/kubernetes/backstage/namespace.yaml" >/dev/null
kubectl -n platform-auth-prod create secret generic keycloak-explore-admin \
  --from-literal=KEYCLOAK_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
# The `app.kubernetes.io/part-of: argocd` label is not decoration: Argo CD only
# dereferences `$name:key` against Secrets carrying it, and without the label
# the OIDC config silently keeps the literal string as the client secret.
kubectl -n argocd create secret generic argocd-oidc-secret \
  --from-literal=client-secret="${ARGOCD_CLIENT_SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n argocd label secret argocd-oidc-secret \
  app.kubernetes.io/part-of=argocd --overwrite >/dev/null
kubectl -n backstage create secret generic backstage-secret \
  --from-literal=AUTH_OIDC_CLIENT_ID=backstage \
  --from-literal=AUTH_OIDC_CLIENT_SECRET="${BACKSTAGE_CLIENT_SECRET}" \
  --from-literal=AUTH_SESSION_SECRET="${BACKSTAGE_SESSION_SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n auth-system create secret generic oauth2-proxy-secret \
  --from-literal=client-id=oauth2-proxy \
  --from-literal=client-secret="${OAUTH2_PROXY_CLIENT_SECRET}" \
  --from-literal=cookie-secret="${OAUTH2_PROXY_COOKIE_SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# What the golden path writes with. The Gitea credential is created a few lines
# further down, so the Secret is written there; this one exists now because the
# realm's admin password is already generated.
kubectl -n backstage create secret generic backstage-explore-keycloak \
  --from-literal=username=admin \
  --from-literal=password="${KEYCLOAK_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# Grafana reads its admin credentials from a Secret. Bare metal fills that from
# Vault; here it is generated, printed at the end, and gone when the cluster is.
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f - >/dev/null
GRAFANA_PASSWORD="$(openssl rand -base64 18 | tr -d '/+=' | head -c 20)"
kubectl -n monitoring create secret generic grafana-explore-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="${GRAFANA_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
# Grafana reads the OIDC client through envFrom, so the key names are the
# environment variable names its config interpolates.
kubectl -n monitoring create secret generic grafana-oidc-explore \
  --from-literal=GF_AUTH_GENERIC_OAUTH_CLIENT_ID=grafana \
  --from-literal=GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET="${GRAFANA_CLIENT_SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# ---------------------------------------------------------------------------
# Gitea — the git server Argo CD reads from, and the one the portal writes to
# ---------------------------------------------------------------------------
#
# Installed out of band for the same reason Argo CD is: it cannot be described
# by an Application that Argo CD would have to read out of it.
#
# Everything else in the slice mirrors bare metal. This does not: the real
# cluster reads from GitHub. It is the fourth substitution, and it buys two
# things nothing else can. A visitor never pushes anything — what runs is the
# checkout they are sitting in. And the golden path can create a repository and
# open a pull request without a credential for somebody else's account.
#
# There is no second path. Argo CD reading a repository over the network was the
# original design, and `--repo` survived the move as an option nobody had reason
# to pass: a fork runs `make try` like everyone else.
info "installing Gitea ${GITEA_CHART_VERSION} (the cluster's own git server)"
GITEA_ADMIN_PASSWORD="${DEMO_USER_PASSWORD}"
# What the portal publishes with.
kubectl -n backstage create secret generic backstage-explore-gitea \
  --from-literal=username="${GITEA_USERNAME}" \
  --from-literal=password="${GITEA_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

kubectl create namespace gitea --dry-run=client -o yaml | kubectl apply -f - >/dev/null
helm repo add gitea-charts "${GITEA_CHART_REPO}" >/dev/null 2>&1 || true
helm repo update gitea-charts >/dev/null
helm upgrade --install gitea gitea-charts/gitea \
  --version "${GITEA_CHART_VERSION}" \
  --namespace gitea \
  --values "${ENV_DIR}/values/gitea.yaml" \
  --set "gitea.admin.password=${GITEA_ADMIN_PASSWORD}" \
  --wait --timeout 8m >/dev/null

# The gateway route for Gitea arrives with the Applications, minutes from now,
# so the seed cannot go through it. A port-forward is the one path that exists
# at this point and it needs nothing from Istio.
info "seeding it with this checkout (nothing has to be pushed anywhere)"
kubectl -n gitea port-forward svc/gitea-http 3999:3000 >/dev/null 2>&1 &
GITEA_PF_PID=$!

gitea_local="http://127.0.0.1:3999"
for _ in $(seq 1 40); do
  curl -sf -o /dev/null --max-time 2 "${gitea_local}/api/healthz" && break
  sleep 1
done

curl -sf -o /dev/null --max-time 30 \
  -u "${GITEA_USERNAME}:${GITEA_ADMIN_PASSWORD}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"kensan-lab","private":false,"auto_init":false}' \
  "${gitea_local}/api/v1/user/repos" \
  || fail "Gitea did not accept the repository that holds this checkout.
   It answered on the port-forward, so this is the API rather than the pod:
     kubectl -n gitea logs deploy/gitea --tail=50"

# http.postBuffer is not optional. Above the default 1 MiB git switches to a
# chunked request with no Content-Length, and the receiving end refuses it —
# measured against both GitHub and Gitea, so it is git's behaviour and not a
# property of either server. Without this the push dies with
# "unexpected disconnect while reading sideband packet" after writing 100%.
git -C "${REPO_ROOT}" -c http.postBuffer=524288000 push --quiet --force \
  "http://${GITEA_USERNAME}:${GITEA_ADMIN_PASSWORD}@127.0.0.1:3999/${GITEA_REPO_PATH}" \
  "HEAD:refs/heads/main" \
  || fail "could not seed Gitea with this checkout.
   The repository was created, so this is the push itself:
     git -C . log --oneline -1"

# A generated repository must run its own code, not the built-in demo image.
# The runner builds inside DinD, pushes to the registry's fixed ClusterIP, and
# commits the resulting source SHA to the repository's values file. Both are
# disposable and reachable only inside this kind cluster.
info "starting the Golden Path image registry and Gitea Actions runner"
RUNNER_TOKEN_RESPONSE="$(curl -sf --max-time 30 -X POST \
  -u "${GITEA_USERNAME}:${GITEA_ADMIN_PASSWORD}" \
  "${gitea_local}/api/v1/admin/actions/runners/registration-token")"
RUNNER_TOKEN="$(printf '%s' "${RUNNER_TOKEN_RESPONSE}" \
  | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
[[ -n "${RUNNER_TOKEN}" ]] \
  || fail "Gitea did not issue an Actions runner registration token."

kubectl create namespace explore-build --dry-run=client -o yaml \
  | kubectl apply -f - >/dev/null
kubectl -n explore-build create secret generic act-runner-registration \
  --from-literal=token="${RUNNER_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply -f "${ENV_DIR}/resources-gitea/build-infrastructure.yaml" >/dev/null
kubectl -n explore-build rollout status deployment/registry --timeout=3m >/dev/null
kubectl -n explore-build rollout status deployment/act-runner --timeout=5m >/dev/null

runner_online=false
for _ in $(seq 1 60); do
  runners="$(curl -sf --max-time 5 \
    -u "${GITEA_USERNAME}:${GITEA_ADMIN_PASSWORD}" \
    "${gitea_local}/api/v1/admin/actions/runners" || true)"
  if printf '%s' "${runners}" | tr -d '[:space:]' \
    | grep -q '"name":"explore-builder".*"status":"online"'; then
    runner_online=true
    break
  fi
  sleep 2
done
[[ "${runner_online}" == true ]] \
  || fail "the Explore Actions runner registered but did not come online.
     Inspect it with:
       kubectl -n explore-build logs deploy/act-runner -c runner --tail=100"

kill "${GITEA_PF_PID}" 2>/dev/null || true
wait "${GITEA_PF_PID}" 2>/dev/null || true
GITEA_PF_PID=""

# The labelled namespace. Applied here rather than left to Argo CD because it
# describes a component that only exists on this path — syncing it where Gitea
# was never installed leaves resources with nothing behind them.
#
# The route is *not* applied here. `kind: HTTPRoute` does not resolve until
# the Gateway API CRDs exist, and those arrive with the Applications, minutes
# from now. It goes on further down, once they do.
kubectl apply -f "${ENV_DIR}/resources-gitea/namespace.yaml" >/dev/null

# Argo CD reads over the Service rather than the gateway: the hostname only
# resolves outside the cluster, and going through the gateway would mean
# teaching repo-server about the self-signed CA for no gain.
REPO_URL="${GITEA_INTERNAL_URL}/${GITEA_REPO_PATH%.git}"
REVISION=main

info "applying the AppProjects"
kubectl apply -f "${REPO_ROOT}/kubernetes/argocd/projects/" >/dev/null

# app-project admits sources from github.com/yu-min3 and nowhere else, which is
# the right rule on bare metal and wrong here: the Applications now come from a
# git server inside the cluster. Without this the scaffolded services — and the
# demo app, which is one of them — sit at Unknown with "repo ... is not
# permitted in project 'app-project'", a message that says nothing about
# AppProjects being the thing to look at.
#
# Patched rather than committed, because the restriction is real protection on
# the cluster that matters and this exception belongs to the throwaway one.
# Admit repositories owned by the disposable Gitea admin. The platform repo and
# every repository the Explore template creates live under exactly this prefix;
# nothing outside this one in-cluster server is widened.
kubectl -n argocd patch appproject app-project --type json \
  -p "[{\"op\": \"add\", \"path\": \"/spec/sourceRepos/-\", \"value\": \"${GITEA_INTERNAL_URL}/${GITEA_USERNAME}/*\"}]" \
  >/dev/null

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
# repoURL and revision appear in both root sources, plus the helm parameter
# every fixed child Application inherits.
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
# Keycloak realm
# ---------------------------------------------------------------------------
#
# Bare metal runs bootstrap/keycloak/setup.sh once by hand after the platform is
# up, using kcadm against the running pod. This is the same thing at a smaller
# scale, run automatically because there is nobody to run it and nothing worth
# keeping if it goes wrong.
#
# It cannot be a realm file in git. Keycloak does not substitute environment
# variables during realm import — verified: a `${VAR}` in a client secret is
# stored as the literal six characters — so a committed realm would have to
# carry working client secrets in a repository anyone can read.

# This runs *before* the wait for every Application, and it has to.
# oauth2-proxy fetches the realm's discovery document at startup and exits if it
# is not there, so it crash-loops until this code has run — and if this code
# waited for every Application to be healthy first, it would be waiting for
# oauth2-proxy, which is waiting for it. The realm is the thing that breaks the
# circle, so it is created as soon as Keycloak can answer.
info "waiting for Keycloak, then creating its realm"
for _ in $(seq 1 60); do
  kubectl -n platform-auth-prod get deployment keycloak >/dev/null 2>&1 && break
  sleep 10
done
kubectl -n platform-auth-prod rollout status deployment/keycloak --timeout=600s >/dev/null

kcadm() {
  kubectl -n platform-auth-prod exec -i deployment/keycloak -c keycloak -- \
    /opt/keycloak/bin/kcadm.sh "$@"
}

# kcadm keeps its session in ~/.keycloak inside the pod, so this login covers
# every call below.
kcadm config credentials \
  --server http://localhost:8080 \
  --realm master \
  --user admin \
  --password "${KEYCLOAK_ADMIN_PASSWORD}" >/dev/null

if kcadm get realms/kensan >/dev/null 2>&1; then
  info "the 'kensan' realm already exists — leaving it alone"
else
  kcadm create realms \
    -s realm=kensan \
    -s enabled=true \
    -s displayName="kensan-lab (explore)" \
    -s sslRequired=none \
    >/dev/null

  # The same two groups bare metal has. Grafana maps them to Admin and Editor
  # (see values/grafana.yaml); nothing else reads them yet, and they are here so
  # that the claim exists to be read.
  # The admin group's id is captured at creation. Looking it up afterwards with
  # `-q search=` matches by prefix and returns every hit, so the answer would
  # depend on the order Keycloak happens to list them in.
  admin_group_id="$(kcadm create groups -r kensan -s name=platform-admin -i)"
  kcadm create groups -r kensan -s name=platform-dev >/dev/null

  # Without this, `groups` never appears in any token, and every consumer that
  # maps groups to roles sees a user with no groups at all. It is a client
  # scope rather than a per-client mapper so that all three clients get it.
  scope_id="$(kcadm create client-scopes -r kensan \
    -s name=groups -s protocol=openid-connect \
    -s 'attributes."include.in.token.scope"=true' \
    -i)"
  kcadm create "client-scopes/${scope_id}/protocol-mappers/models" -r kensan \
    -s name=groups \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-group-membership-mapper \
    -s 'config."claim.name"=groups' \
    -s 'config."full.path"=false' \
    -s 'config."id.token.claim"=true' \
    -s 'config."access.token.claim"=true' \
    -s 'config."userinfo.token.claim"=true' \
    >/dev/null

  # Each client is created with the secret this script generated earlier and
  # already wrote into Kubernetes, rather than letting Keycloak generate one and
  # copying it back. The pods that need these secrets start before this code
  # runs; going the other way would mean they start without one.
  kc_client() {
    local client_id="$1" secret="$2"; shift 2
    local id
    id="$(kcadm create clients -r kensan \
      -s "clientId=${client_id}" \
      -s enabled=true \
      -s protocol=openid-connect \
      -s publicClient=false \
      -s standardFlowEnabled=true \
      -s "secret=${secret}" \
      -s "redirectUris=[$(printf '"%s",' "$@" | sed 's/,$//')]" \
      -i)"
    # The groups scope has to be attached through its own sub-resource. Setting
    # `defaultClientScopes` on the client itself is accepted and then ignored —
    # the client comes back with the built-in scopes and no groups, so every
    # token is missing the claim that decides what the user may do. Verified:
    # the update returns success and `get clients/<id>/default-client-scopes`
    # shows no groups.
    kcadm update "clients/${id}/default-client-scopes/${scope_id}" -r kensan \
      >/dev/null
  }

  kc_client argocd "${ARGOCD_CLIENT_SECRET}" \
    "https://argocd.127-0-0-1.sslip.io/auth/callback" \
    "https://argocd.127-0-0-1.sslip.io/applications"
  kc_client grafana "${GRAFANA_CLIENT_SECRET}" \
    "https://grafana.127-0-0-1.sslip.io/login/generic_oauth"
  ## Backstage's OIDC provider posts back to this exact path. It is not behind
  ## the gateway's gate — it holds its own client, like Argo CD and Grafana.
  kc_client backstage "${BACKSTAGE_CLIENT_SECRET}" \
    "https://backstage.127-0-0-1.sslip.io/api/auth/oidc/handler/frame"
  # One client for every host the proxy guards, and the hosts are listed rather
  # than matched with a wildcard.
  #
  # `https://*.127-0-0-1.sslip.io/oauth2/callback` looks like it should work and
  # does not: Keycloak's wildcard matches inside the path, never inside the
  # host. A browser sent to the authorization endpoint comes back with
  # "Invalid parameter: redirect_uri" and nothing in the cluster logs an error,
  # because Keycloak considers refusing an unregistered callback to be its job
  # rather than a fault.
  #
  # Adding a protected host therefore means editing two files: this list, and
  # the hosts in resources/authorizationpolicy-explore-oauth2.yaml.
  kc_client oauth2-proxy "${OAUTH2_PROXY_CLIENT_SECRET}" \
    "https://demo.127-0-0-1.sslip.io/oauth2/callback" \
    "https://backstage.127-0-0-1.sslip.io/oauth2/callback"

  # `skip_jwt_bearer_tokens` lets a command-line client present a token it
  # already holds instead of walking the browser flow, but only for tokens whose
  # audience matches what oauth2-proxy was told to expect. Keycloak does not put
  # that audience in by default, so it takes a mapper. Bare metal has the same
  # one under a different name; without it the feature is configured and inert.
  o2p_id="$(kcadm get clients -r kensan -q clientId=oauth2-proxy \
    --fields id --format csv --noquotes | head -1)"
  kcadm create "clients/${o2p_id}/protocol-mappers/models" -r kensan \
    -s name=istio-gateway-audience \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-audience-mapper \
    -s 'config."included.custom.audience"=istio-gateway-explore' \
    -s 'config."access.token.claim"=true' \
    -s 'config."id.token.claim"=false' \
    >/dev/null

  # One user, in the admin group, with a password printed at the end. There is
  # no identity provider behind this one and no directory to import — a person
  # trying the platform needs exactly one account that works.
  #
  # The email is not an address at this hostname, and that is deliberate:
  # Backstage's oauth2Proxy sign-in uses the emailMatchingUserEntityProfileEmail
  # resolver, so it has to equal the email of a User already in the catalog or
  # the login succeeds at Keycloak and then fails at the portal. This is the
  # demo account backstage/catalog/explore/demo.yaml ships.
  user_id="$(kcadm create users -r kensan \
    -s username=demo \
    -s enabled=true \
    -s emailVerified=true \
    -s email=demo-admin@example.com \
    -s firstName=Demo -s lastName=User \
    -i)"
  kcadm set-password -r kensan --userid "${user_id}" \
    --new-password "${DEMO_USER_PASSWORD}" >/dev/null
  kcadm update "users/${user_id}/groups/${admin_group_id}" -r kensan \
    -s realm=kensan -s userId="${user_id}" -s groupId="${admin_group_id}" \
    -n >/dev/null
fi

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
     Troubleshooting: docs/getting-started/kind-explained.md#troubleshooting"
  fi
  sleep 10
done

# Gitea's route, now that the Gateway API CRDs are in place. Browsers need it;
# Argo CD does not, which is why the cluster came up without it.
kubectl apply -f "${ENV_DIR}/resources-gitea/httproute.yaml" >/dev/null

# Argo CD started before cert-manager had signed anything, so its CA mount
# resolved to nothing. Now that the Secret exists, the pod has to be replaced to
# see it — until then every OIDC login fails on an unknown authority.
info "restarting argocd-server so it picks up the CA and the OIDC client"
kubectl -n argocd rollout restart deployment/argocd-server >/dev/null
kubectl -n argocd rollout status deployment/argocd-server --timeout=300s >/dev/null

# Backstage discovers the issuer once, at startup, and caches the outcome —
# including a failure. Keycloak's Application reports Healthy while its JVM is
# still coming up, so a portal that asked first holds a rejected discovery
# promise for the life of the pod: it stays Ready, Argo CD stays green, and
# every sign-in answers 500. Wait for the document to be servable, then replace
# the pod so it asks again.
info "waiting for Keycloak's realm to be servable, then restarting Backstage"
discovery_deadline=$(( $(date +%s) + 300 ))
while :; do
  code="$(curl -sSk -o /dev/null -w '%{http_code}' --max-time 10 \
    --resolve "auth.127-0-0-1.sslip.io:443:127.0.0.1" \
    "https://auth.127-0-0-1.sslip.io/realms/kensan/.well-known/openid-configuration" \
    || echo 000)"
  [[ "$code" == "200" ]] && break
  if [[ $(date +%s) -ge $discovery_deadline ]]; then
    fail "Keycloak never served the realm's discovery document (last: HTTP ${code}).
     The realm was created, so this is Keycloak or its route rather than SSO:
       kubectl -n platform-auth-prod logs deploy/keycloak -c keycloak --tail=50
       kubectl -n platform-auth-prod get httproute
     Troubleshooting: docs/getting-started/kind-explained.md#troubleshooting"
  fi
  sleep 5
done
kubectl -n backstage rollout restart deployment/backstage >/dev/null
kubectl -n backstage rollout status deployment/backstage --timeout=300s >/dev/null

# oauth2-proxy exits when discovery fails, so it recovers by itself — but by now
# it can be deep in CrashLoopBackOff, which would otherwise be charged to the
# visitor as minutes of waiting.
if ! kubectl -n auth-system get deploy/oauth2-proxy \
     -o jsonpath='{.status.readyReplicas}' 2>/dev/null | grep -q '^[1-9]'; then
  kubectl -n auth-system rollout restart deployment/oauth2-proxy >/dev/null
  kubectl -n auth-system rollout status deployment/oauth2-proxy --timeout=300s >/dev/null
fi

# ---------------------------------------------------------------------------
# Smoke test
# ---------------------------------------------------------------------------

# Sent with an explicit Host header rather than the sslip.io name, so this works
# with no DNS at all. The browser instructions below do need DNS; the offline
# fallback is in the docs.
# The demo app is behind the gateway's ext_authz gate, so a request with no
# session must come back as a redirect to Keycloak — not a 200. A 200 here would
# mean the application is being served to anyone who asks, which is the failure
# this whole phase exists to prevent, and it is the failure that looks like
# success in a browser.
info "checking that the gateway sends an unauthenticated request to Keycloak"
location="$(curl -sS -o /dev/null -w '%{redirect_url}' --max-time 15 \
  -H 'Host: demo.127-0-0-1.sslip.io' http://127.0.0.1/ || echo '')"
case "$location" in
  *auth.127-0-0-1.sslip.io*) : ;;
  '')
    fail "the demo app served a response instead of redirecting to Keycloak.
     The route works, so this is the authorization policy or oauth2-proxy:
       kubectl -n istio-system get authorizationpolicy explore-oauth2-authz -o yaml
       kubectl -n auth-system logs deploy/oauth2-proxy --tail=50
     Troubleshooting: docs/getting-started/kind-explained.md#troubleshooting" ;;
  *)
    fail "the demo app redirected somewhere other than Keycloak: ${location}" ;;
esac

# And again over TLS. This is a separate assertion because it can fail on its
# own: the route can be perfect while the certificate is still being issued, in
# which case Istio serves the listener with no certificate at all.
# Same request over TLS. A separate assertion because it fails on its own: the
# route can be perfect while the certificate is still being issued, in which
# case Istio serves the listener with no certificate at all.
info "checking that the gateway answers the same host over TLS"
code="$(curl -sSk -o /dev/null -w '%{http_code}' --max-time 15 \
  --resolve "demo.127-0-0-1.sslip.io:443:127.0.0.1" \
  https://demo.127-0-0-1.sslip.io/ || echo 000)"
if [[ "$code" != "302" ]]; then
  fail "the demo app answered HTTP ${code} over TLS, not the expected redirect.
     Plain HTTP redirects correctly, so the route and the gate are fine and the
     certificate is not:
       kubectl -n istio-system get certificate explore-wildcard
       kubectl -n cert-manager get certificate explore-ca
     Troubleshooting: docs/getting-started/kind-explained.md#troubleshooting"
fi

# Keycloak has to answer as itself, at the name it puts in its own tokens. If
# the discovery document names a different issuer, every login fails later with
# a message about the issuer not matching, far from the cause.
info "checking that Keycloak's discovery document names the right issuer"
issuer="$(curl -sSk --max-time 15 \
  --resolve "auth.127-0-0-1.sslip.io:443:127.0.0.1" \
  "https://auth.127-0-0-1.sslip.io/realms/kensan/.well-known/openid-configuration" \
  | sed -n 's/.*"issuer":"\([^"]*\)".*/\1/p')"
if [[ "$issuer" != "https://auth.127-0-0-1.sslip.io/realms/kensan" ]]; then
  fail "Keycloak reports its issuer as '${issuer}'.
     Every token it signs will carry that, and no client is configured for it:
       kubectl -n platform-auth-prod get configmap keycloak-env-config -o yaml"
fi

# The redirect the browser is about to follow has to be one Keycloak will
# accept. This is checked by following it, because the failure is invisible from
# every other angle: the gate redirects correctly, Keycloak answers, the token
# endpoint works for a direct grant, and the only sign of trouble is an error
# page a person sees after clicking. A wildcard host in the client's redirect
# URIs produces exactly this.
info "checking that Keycloak accepts the callback the gateway sends people to"
for host in demo; do
  target="$(curl -sSk -o /dev/null -w '%{redirect_url}' --max-time 15 \
    --resolve "${host}.127-0-0-1.sslip.io:443:127.0.0.1" \
    "https://${host}.127-0-0-1.sslip.io/" || echo '')"
  body="$(curl -sSk --max-time 15 \
    --resolve "auth.127-0-0-1.sslip.io:443:127.0.0.1" "${target}" || echo '')"
  if grep -qi "Invalid parameter: redirect_uri" <<<"$body"; then
    fail "Keycloak refuses the callback for ${host}.127-0-0-1.sslip.io.
     The host is guarded by the gateway but is not a registered redirect URI on
     the oauth2-proxy client. Keycloak cannot wildcard a host, so both lists
     have to name it:
       scripts/explore-up.sh                (kc_client oauth2-proxy ...)
       environments/kind/resources/authorizationpolicy-explore-oauth2.yaml"
  fi
  grep -qi "kc-login\|<form" <<<"$body" || fail "the login page for ${host} did not render a form.
     Keycloak answered, but not with something a person can sign in to:
       kubectl -n platform-auth-prod logs deploy/keycloak -c keycloak | tail -30"
done

# And the whole login, walked the way a person walks it: redirect to Keycloak,
# post the form, return through /oauth2/callback, come back holding a session
# cookie. Every earlier check stops before the callback, and the callback is
# where two separate faults land — a redirect URI Keycloak will not accept, and
# a CSRF cookie that never reached the client. Both looked like a working
# cluster from every other angle.
info "checking that a person can actually sign in"
code="$("${REPO_ROOT}/scripts/explore-login-check.sh" \
  demo.127-0-0-1.sslip.io demo "${DEMO_USER_PASSWORD}" 2>&1 || true)"
if [[ "$code" != "200" ]]; then
  fail "signing in to demo.127-0-0-1.sslip.io did not work: ${code}
     The gate redirects and Keycloak answers, so this is the round trip rather
     than the setup. Run it by hand to see which step stopped:
       scripts/explore-login-check.sh demo.127-0-0-1.sslip.io demo '<password>'"
fi

# Backstage signs people in itself now, so its round trip is a different shape:
# no CSRF cookie, no /oauth2/callback, and a handler that answers with a page
# rather than a status code. Checked separately for that reason — and checked at
# all because the two failures it catches (a client Keycloak does not know, a
# discovery document Node will not trust) both look like a healthy pod.
info "checking that a person can sign in to Backstage"
result="$("${REPO_ROOT}/scripts/explore-backstage-login-check.sh" \
  backstage.127-0-0-1.sslip.io demo "${DEMO_USER_PASSWORD}" 2>&1 || true)"
case "$result" in
  200*) : ;;
  *)
    fail "signing in to Backstage did not complete: ${result}
     The portal is up and its plugins started, so this is the OIDC round trip:
       kubectl -n backstage logs deploy/backstage -c backstage | grep -i oidc
     A self-signed CA that Node was not told about fails here as a certificate
     error; a missing Keycloak client fails as an unknown client_id." ;;
esac

# Backstage's probe is a liveness check on the HTTP router, not on the plugins
# behind it. A backend whose catalog failed to start still serves the frontend
# shell and still answers /healthcheck with 200, so the pod is Ready, the
# Application is Healthy, and the portal is useless.
#
# Asked from the pod's own log rather than over HTTP, because the portal now
# sits behind the SSO gate: an unauthenticated request never reaches the
# backend, so a 302 says nothing about whether the plugins came up. The backend
# announces both outcomes, and one line distinguishes them.
info "checking that Backstage's plugins all initialised"
plugins=""
for attempt in $(seq 1 60); do
  logs="$(kubectl -n backstage logs deploy/backstage -c backstage 2>/dev/null || true)"
  if grep -q "threw an error during startup" <<<"$logs"; then
    fail "a Backstage plugin failed to start. The pod stays Ready and the portal
     serves its shell either way, so nothing else would have noticed:
$(grep -o "Plugin '[a-z]*' threw an error during startup[^\"]*" <<<"$logs" | sort -u | head -3 | sed 's/^/       /')
     A config value the backend type-checks at boot is the usual cause."
  fi
  plugins="$(grep -o "Plugin initialization complete[^\"]*" <<<"$logs" | tail -1)"
  [[ -n "$plugins" ]] && break
  sleep 5
done
if [[ -z "$plugins" ]]; then
  fail "Backstage never reported its plugins as initialised.
     Neither a success nor a failure line appeared, so it is still starting or
     stuck before the plugins run:
       kubectl -n backstage logs deploy/backstage -c backstage | tail -30"
fi

# The Explore template is read from the checkout seeded into Gitea, not from a
# pre-published Backstage image. That is what lets a template edit be exercised
# by the very next `make try`. A missing catalog location is non-fatal to the
# backend, so prove the source is reachable from the consumer pod and reject a
# processing error from the log.
template_url="${GITEA_INTERNAL_URL}/${GITEA_USERNAME}/kensan-lab/src/branch/main/backstage/templates/fastapi-template/template-explore.yaml"
template_raw_url="${GITEA_INTERNAL_URL}/${GITEA_USERNAME}/kensan-lab/raw/branch/main/backstage/templates/fastapi-template/template-explore.yaml"
info "checking that Backstage can read the Explore software template from this checkout"
if ! kubectl -n backstage exec deploy/backstage -c backstage -- \
  node -e 'fetch(process.argv[1]).then(async r => process.exit(r.ok && (await r.text()).includes("kind: Template") ? 0 : 1)).catch(() => process.exit(1))' \
  "$template_raw_url"; then
  fail "Backstage cannot read the Explore template from the seeded checkout:
       ${template_url}"
fi
if grep -Fq "$template_url" <<<"$logs" && grep -Eq "Unable to read|processing.*error" <<<"$logs"; then
  fail "Backstage reached the Explore template but could not register it:
       ${template_url}"
fi

admin_password="$(kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' 2>/dev/null | base64 --decode || echo '(already rotated)')"

GITEA_NOTE="
  5. Look at the git server          https://gitea.127-0-0-1.sslip.io
     user 'demo', password '${GITEA_ADMIN_PASSWORD}'.
     Argo CD reads this cluster from here rather than from GitHub. That is why
     nothing had to be pushed for your checkout to be what is running, and why
     'Create' in the portal can make a repository without a token.
"

cat <<EOF

  The platform is up. The walkthrough is here:
  docs/getting-started/try-it-with-kind.md

  Every URL below is https, and your browser will warn about the certificate.
  cert-manager issued it from a CA this cluster generated a minute ago, and
  nothing on your machine has a reason to trust that — there is no domain here
  to prove ownership of. Clicking through is the intended path.

  One memorable credential pair reaches every screen:

       user 'demo', password '${DEMO_USER_PASSWORD}'

  Argo CD, Backstage, Grafana and the apps share a Keycloak session. Gitea uses
  a separate local account with the same credentials; it does not share SSO.

  1. Open the demo app               https://demo.127-0-0-1.sslip.io
     You will land on Keycloak first. The app has no authentication code and
     does not know it is protected — the gateway asked oauth2-proxy about your
     request before the pod ever saw it. One line of values turned that on.

  2. Open the GitOps tree            https://argocd.127-0-0-1.sslip.io
     'Log in via Keycloak' — you are already signed in, so it will not ask
     again. Or use the local admin account: password '${admin_password}'.
     The 'explore-root' application is the app-of-apps: open it and every
     component below it is one Application, exactly as on the real cluster.

  3. Open Grafana                    https://grafana.127-0-0-1.sslip.io
     'Sign in with Keycloak', again without being asked. You arrive as an Admin
     because the 'demo' user is in the platform-admin group and Grafana maps
     that group to that role. The break-glass local account is still there:
     user 'admin', password '${GRAFANA_PASSWORD}'.
     The 'Cluster Health' dashboard is the bare-metal one, unedited. The panels
     that stay empty are reading a Raspberry Pi's temperature sensor.

  Keycloak itself is at https://auth.127-0-0-1.sslip.io — user 'admin',
  password '${KEYCLOAK_ADMIN_PASSWORD}'.

  4. Open the developer portal       https://backstage.127-0-0-1.sslip.io
     Backstage signs you in against the same Keycloak, then resolves you to a
     user in its catalog. 'Create' holds the golden path template — the demo
     app above shows its shape. Each created repository is tested and built by
     the local Gitea Actions runner before Argo CD deploys its own image.
${GITEA_NOTE}
  6. See what the policy engine thinks
       kubectl get policyreport -A
     Kyverno is running the production policies in Audit mode, so violations
     are reported rather than blocked (ADR-012).

  What this cluster deliberately cannot show you — the L2 load balancer,
  storage replication, multi-architecture scheduling — and why:
  docs/getting-started/kind-explained.md#what-kind-cannot-demonstrate

  Your kubectl now points at this cluster, not whatever it pointed at before.
  'make explore-down' removes the context along with the cluster.

  Tear it down with:  make explore-down

EOF
