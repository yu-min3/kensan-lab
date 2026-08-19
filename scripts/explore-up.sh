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
  fi
  if [[ "$status" -ne 0 && -n "$PREVIOUS_CONTEXT" ]]; then
    kubectl config use-context "$PREVIOUS_CONTEXT" >/dev/null 2>&1 || true
  fi
  return "$status"
}
trap on_exit EXIT

CLUSTER_NAME="kensan-lab-explore"
# Where Argo CD syncs from. Taken from this checkout rather than hardcoded,
# because a fork that synced upstream would be a fork in name only: you would
# edit your copy, run `make try`, and watch somebody else's main come up.
#
# `origin` is whatever `git clone` set, so a fork clone points at the fork. The
# SSH form is rewritten because Argo CD reads over HTTPS and has no key here.
# Falls back to upstream when there is no git checkout at all — someone who
# downloaded a tarball still gets a working cluster.
REPO_URL="$(git -C "${REPO_ROOT}" remote get-url origin 2>/dev/null || true)"
REPO_URL="${REPO_URL:-https://github.com/yu-min3/kensan-lab}"
REPO_URL="${REPO_URL%.git}"
case "$REPO_URL" in
  # git@github.com:owner/repo -> https://github.com/owner/repo
  git@*) REPO_URL="https://$(printf '%s' "${REPO_URL#git@}" | sed 's|:|/|')" ;;
  ssh://git@*) REPO_URL="https://${REPO_URL#ssh://git@}" ;;
esac

# And the branch you are actually on, so a change is one `git push` away from
# being live. Detached HEAD or no checkout falls back to main.
REVISION="$(git -C "${REPO_ROOT}" symbolic-ref --short HEAD 2>/dev/null || true)"
# Whether REVISION names a branch of this checkout, which is what makes the
# "did you push it?" check meaningful. `--rev` clears it: the caller chose the
# revision and knows where it lives.
REVISION_IS_LOCAL_BRANCH=true
[[ -n "$REVISION" ]] || REVISION_IS_LOCAL_BRANCH=false
REVISION="${REVISION:-main}"

# Where Argo CD reads from.
#
# By default: an in-cluster Gitea, seeded from this checkout. Nothing has to be
# pushed anywhere, a fork is not required, and the golden path can create
# repositories without a GitHub token — which is the whole reason it is here.
#
# With --repo: that URL instead, and no Gitea. CI uses this to point the cluster
# at the commit under review, where the pushed commit *is* the thing being
# proved.
EXTERNAL_REPO=false
GITEA_INTERNAL_URL="http://gitea-http.gitea.svc.cluster.local:3000"
GITEA_REPO_PATH="gitea-admin/kensan-lab.git"
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
# Optional. The demo account's password is generated and printed unless one is
# supplied, which is what CI does so that it can drive a login without scraping
# this script's output.
DEMO_PASSWORD="${DEMO_PASSWORD:-}"

usage() {
  cat <<'EOF'
usage: scripts/explore-up.sh [--repo URL] [--rev REVISION] [--timeout SECONDS]

  --repo URL       Git repository Argo CD syncs from (default: upstream)
  --rev REVISION   Branch, tag or SHA to sync (default: main)
  --timeout SEC    How long to wait for every Application to go healthy

Set GITHUB_TOKEN in the environment to let Backstage's scaffolder create
repositories and open pull requests. Everything works without it except the
Create button.

Set DEMO_PASSWORD to choose the demo account's password instead of having one
generated. Useful for scripting against the cluster; the generated one is
printed either way.

CI passes --rev "$GITHUB_SHA" so the cluster proves the pull request rather
than main. A fork passes --repo its own URL.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO_URL="$2"; EXTERNAL_REPO=true; shift 2 ;;
    --rev) REVISION="$2"; REVISION_IS_LOCAL_BRANCH=false; shift 2 ;;
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

# Reusing a cluster cannot work, so it is refused rather than half-attempted.
#
# Every credential below is generated fresh on each run, but Keycloak reads its
# admin password once — at first startup, into an in-memory database. A second
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
     Every run generates fresh credentials, and Keycloak only reads its own at
     first startup — so a second run leaves the two disagreeing.

     Remove it and start again:
       make explore-down && make try"
fi
# Argo CD reads the repository over the network, not this directory. A commit
# that only exists locally is invisible to it, and the symptom is a cluster that
# comes up sixty seconds behind whatever you just wrote — or an Application that
# never syncs at all because the branch is not there yet.
# Only when the revision is a branch name from this checkout. A caller that
# passed --rev explicitly may well have named a commit SHA — CI does — and a SHA
# is not something `ls-remote --heads` can find even when it is perfectly
# fetchable. Checking it as a branch would refuse a run that would have worked.
# Only in external mode. Seeding Gitea from the checkout removes the question
# entirely: what runs is what is on disk, pushed or not.
if [[ "$EXTERNAL_REPO" == true && "$REVISION_IS_LOCAL_BRANCH" == true ]] \
  && git -C "${REPO_ROOT}" rev-parse --git-dir >/dev/null 2>&1; then
  if ! git -C "${REPO_ROOT}" ls-remote --exit-code --heads origin "$REVISION" >/dev/null 2>&1; then
    fail "the branch '${REVISION}' is not on ${REPO_URL} yet.
     Argo CD syncs from the repository, not from this directory, so it has
     nothing to read. Push it first:
       git push -u origin ${REVISION}"
  fi
  local_head="$(git -C "${REPO_ROOT}" rev-parse HEAD 2>/dev/null || true)"
  remote_head="$(git -C "${REPO_ROOT}" ls-remote origin "$REVISION" 2>/dev/null | cut -f1)"
  if [[ -n "$local_head" && -n "$remote_head" && "$local_head" != "$remote_head" ]]; then
    info "note: local HEAD and origin/${REVISION} differ — the cluster will run what is pushed"
  fi
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
# Every secret below is generated here, now, and never written to the
# repository. That is not squeamishness: a kind cluster publishes ports 80 and
# 443 on the host, so a client secret committed to a public repository would be
# a working credential against every cluster anybody ever started from it.
#
# They are created *before* Keycloak exists, and Keycloak is then told to use
# them, rather than the other way round. The reverse order deadlocks: the
# oauth2-proxy pod will not start without its Secret, so it never becomes
# healthy, so the bootstrap that would have created the Secret never runs.
rand() { openssl rand -base64 24 | tr -d '/+=' | head -c 24; }

KEYCLOAK_ADMIN_PASSWORD="$(rand)"
DEMO_USER_PASSWORD="${DEMO_PASSWORD:-$(rand)}"
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
kubectl -n backstage create secret generic backstage-explore-github \
  --from-literal=GITHUB_TOKEN="${GITHUB_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# What the golden path writes with. The Gitea credential is created a few lines
# further down, so the Secret is written there; this one exists now because the
# realm's admin password is already generated.
kubectl -n backstage create secret generic backstage-explore-keycloak \
  --from-literal=username=admin \
  --from-literal=password="${KEYCLOAK_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# The credential the portal publishes with. Created on both paths, because the
# Deployment names it either way and a secretKeyRef to a Secret that does not
# exist stops the pod before it starts — with an error about a missing Secret
# rather than about the git server nobody installed.
#
# On --repo there is no Gitea, so this is a placeholder: the integration it
# configures points at a host that does not resolve, and nothing on that path
# ever calls it. The same shape the GitHub token takes a few lines below, for
# the same reason.
GITEA_ADMIN_PASSWORD="${GITEA_ADMIN_PASSWORD:-no-gitea-on-this-path}"
kubectl -n backstage create secret generic backstage-explore-gitea \
  --from-literal=username=gitea-admin \
  --from-literal=password="${GITEA_ADMIN_PASSWORD}" \
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
if [[ "$EXTERNAL_REPO" == false ]]; then
  info "installing Gitea ${GITEA_CHART_VERSION} (the cluster's own git server)"
  GITEA_ADMIN_PASSWORD="$(rand)"
  # Replaces the placeholder written above, now that there is a real password.
  kubectl -n backstage create secret generic backstage-explore-gitea \
    --from-literal=username=gitea-admin \
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
    -u "gitea-admin:${GITEA_ADMIN_PASSWORD}" \
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
    "http://gitea-admin:${GITEA_ADMIN_PASSWORD}@127.0.0.1:3999/${GITEA_REPO_PATH}" \
    "HEAD:refs/heads/main" \
    || fail "could not seed Gitea with this checkout.
     The repository was created, so this is the push itself:
       git -C . log --oneline -1"

  kill "${GITEA_PF_PID}" 2>/dev/null || true
  GITEA_PF_PID=""

  # The labelled namespace and the browser-facing route. Applied here rather
  # than left to Argo CD: they describe a component that only exists on this
  # path, and syncing them where Gitea was never installed leaves a route with
  # no backend and an Application stuck Degraded.
  kubectl apply -f "${ENV_DIR}/resources-gitea/gitea.yaml" >/dev/null

  # Argo CD reads over the Service rather than the gateway: the hostname only
  # resolves outside the cluster, and going through the gateway would mean
  # teaching repo-server about the self-signed CA for no gain.
  REPO_URL="${GITEA_INTERNAL_URL}/${GITEA_REPO_PATH%.git}"
  REVISION=main
fi

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
if [[ "$EXTERNAL_REPO" == false ]]; then
  # The exact URL, not a glob. Argo CD matches sourceRepos with a glob that does
  # not cross '/', so `${GITEA_INTERNAL_URL}/*` misses a two-segment path and the
  # Application stays Unknown with InvalidSpecError. Measured.
  kubectl -n argocd patch appproject app-project --type json \
    -p "[{\"op\": \"add\", \"path\": \"/spec/sourceRepos/-\", \"value\": \"${REPO_URL}\"}]" \
    >/dev/null
fi

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
# carry real client secrets and a real user password, in a repository anyone can
# read, for a cluster that publishes ports 80 and 443.

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
  # demo account backstage/catalog/organizations/teams.yaml ships.
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
     Troubleshooting: docs/getting-started/try-it-with-kind.md#troubleshooting"
  fi
  sleep 10
done

# Argo CD started before cert-manager had signed anything, so its CA mount
# resolved to nothing. Now that the Secret exists, the pod has to be replaced to
# see it — until then every OIDC login fails on an unknown authority.
info "restarting argocd-server so it picks up the CA and the OIDC client"
kubectl -n argocd rollout restart deployment/argocd-server >/dev/null
kubectl -n argocd rollout status deployment/argocd-server --timeout=300s >/dev/null

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
     Troubleshooting: docs/getting-started/try-it-with-kind.md#troubleshooting" ;;
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
     Troubleshooting: docs/getting-started/try-it-with-kind.md#troubleshooting"
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

admin_password="$(kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' 2>/dev/null | base64 --decode || echo '(already rotated)')"

# Composed here rather than written inline, so that --repo — which skips Gitea
# entirely — does not print a section about a component nobody installed.
if [[ "$EXTERNAL_REPO" == false ]]; then
  GITEA_NOTE="
  5. Look at the git server          https://gitea.127-0-0-1.sslip.io
     user 'gitea-admin', password '${GITEA_ADMIN_PASSWORD}'.
     Argo CD reads this cluster from here rather than from GitHub. That is why
     nothing had to be pushed for your checkout to be what is running, and why
     'Create' in the portal can make a repository without a token.
"
else
  GITEA_NOTE=""
fi

cat <<EOF

  The platform is up. Five things worth doing, in order.

  Every URL below is https, and your browser will warn about the certificate.
  cert-manager issued it from a CA this cluster generated a minute ago, and
  nothing on your machine has a reason to trust that — there is no domain here
  to prove ownership of. Clicking through is the intended path.

  Single sign-on works here. One account reaches all of it:

       user 'demo', password '${DEMO_USER_PASSWORD}'

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
     app above is what it produces.
${GITEA_NOTE}
  6. See what the policy engine thinks
       kubectl get policyreport -A
     Kyverno is running the production policies in Audit mode, so violations
     are reported rather than blocked (ADR-012).

  What this cluster deliberately cannot show you — the L2 load balancer,
  storage replication, multi-architecture scheduling — and why:
  docs/getting-started/try-it-with-kind.md#what-kind-cannot-show

  Your kubectl now points at this cluster, not whatever it pointed at before.
  'make explore-down' removes the context along with the cluster.

  Tear it down with:  make explore-down

EOF
