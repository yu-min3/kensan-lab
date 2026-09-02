#!/usr/bin/env bash
# The golden path, driven the way a visitor drives it — through the portal's
# API rather than by applying manifests.
#
# This exists because every failure this layer has produced looked healthy from
# the outside. Argo CD green, pods Ready, and the one thing a person came to do
# quietly broken. Scaffolding a service touches Gitea, the platform repository,
# Keycloak and the gateway in one go; nothing else in the smoke tests would
# notice if any of those stopped agreeing.
#
# Usage: explore-golden-path-check.sh <backstage-host> <user> <password> [name] [--deploy]
set -euo pipefail

HOST="${1:-backstage.127-0-0-1.sslip.io}"
USER_NAME="${2:-demo}"
PASSWORD="${3:-demo}"
NAME="${4:-checkservice}"
MODE="${5:-}"
if [[ -n "$MODE" && "$MODE" != "--deploy" ]]; then
  echo "fifth argument must be --deploy, got: ${MODE}" >&2
  exit 2
fi
JAR="$(mktemp)"
APP_PF_PID=""
cleanup() {
  rm -f "$JAR"
  if [[ -n "$APP_PF_PID" ]]; then
    kill "$APP_PF_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT
C=(/usr/bin/curl -sk --cookie-jar "$JAR" --cookie "$JAR")

# The portal issues its own token after Keycloak verifies the person, and the
# scaffolder API wants that token rather than the session cookie.
start="https://${HOST}/api/auth/oidc/start?scope=openid%20profile%20email&origin=https%3A%2F%2F${HOST}&flow=popup&env=production"
auth_url="$("${C[@]}" -o /dev/null -w '%{redirect_url}' "$start")"
case "$auth_url" in
  *client_id=backstage*) ;;
  *) echo "the portal did not start an OIDC flow: ${auth_url:-<none>}" >&2; exit 1 ;;
esac

form_action="$("${C[@]}" "$auth_url" \
  | sed -n 's/.*id="kc-form-login"[^>]*action="\([^"]*\)".*/\1/p' \
  | head -1 | sed 's/&amp;/\&/g')"
[ -n "$form_action" ] || { echo "no login form" >&2; exit 1; }

handler="$("${C[@]}" -o /dev/null -w '%{redirect_url}' \
  -d "username=${USER_NAME}" -d "password=${PASSWORD}" "$form_action")"
case "$handler" in
  *handler/frame*) ;;
  *) echo "Keycloak did not return the handler: ${handler:-<none>}" >&2; exit 1 ;;
esac

# The handler answers with a page that hands the result to its opener. The
# payload is percent-encoded inside a decodeURIComponent() call, so it has to be
# decoded before the token can be read out — grepping the response as it stands
# matches nothing, however successful the sign-in was.
#
# `|| true` on every extraction: under `set -e` with `pipefail`, a grep that
# matches nothing kills the script inside the command substitution, before the
# line below can say what went wrong. That is the failure this script exists to
# stop happening elsewhere.
frame="$("${C[@]}" "$handler" || true)"
payload="$(printf '%s' "$frame" \
  | sed -n "s/.*decodeURIComponent('\([^']*\)').*/\1/p" | head -1)"
[ -n "$payload" ] || { echo "the sign-in page carried no auth payload" >&2; exit 1; }
token="$(printf '%b' "${payload//%/\\x}" \
  | grep -oE '"backstageIdentity":\{"token":"[^"]+' \
  | grep -oE '"token":"[^"]+' | cut -d'"' -f4 | head -1 || true)"
[ -n "$token" ] || { echo "signed in, but no Backstage token came back" >&2; exit 1; }

echo "  signed in to the portal"

body=$(cat <<JSON
{"templateRef":"template:default/fastapi-app-template",
 "values":{"name":"${NAME}",
           "description":"created by explore-golden-path-check.sh",
           "owner":"group:default/platform-engineering",
           "repoUrl":"gitea.127-0-0-1.sslip.io?owner=gitea-admin&repo=${NAME}",
           "theme":"night","message":"created without a token",
           "domain":"platform","system":"developer-portal"}}
JSON
)

task="$("${C[@]}" -X POST "https://${HOST}/api/scaffolder/v2/tasks" \
  -H "Authorization: Bearer ${token}" \
  -H 'Content-Type: application/json' \
  -d "$body" | grep -oE '"id":"[^"]+' | cut -d'"' -f4 | head -1 || true)"
[ -n "$task" ] || { echo "the scaffolder refused the request" >&2; exit 1; }
echo "  scaffolder task ${task}"

for _ in $(seq 1 60); do
  status="$("${C[@]}" "https://${HOST}/api/scaffolder/v2/tasks/${task}" \
    -H "Authorization: Bearer ${token}" \
    | grep -oE '"status":"[^"]+' | cut -d'"' -f4 | head -1 || true)"
  case "$status" in
    completed) echo "  the task completed"; break ;;
    failed|cancelled)
      echo "the golden path failed at the scaffolder. What it logged:" >&2
      "${C[@]}" "https://${HOST}/api/scaffolder/v2/tasks/${task}/events" \
        -H "Authorization: Bearer ${token}" | tail -c 1200 >&2
      exit 1 ;;
  esac
  sleep 5
done
[ "$status" = completed ] || { echo "the scaffolder task never finished" >&2; exit 1; }

# What it was supposed to leave behind, checked where a person would look.
code="$(curl -sk -o /dev/null -w '%{http_code}' --max-time 15 \
  --resolve "gitea.127-0-0-1.sslip.io:443:127.0.0.1" \
  "https://gitea.127-0-0-1.sslip.io/gitea-admin/${NAME}")"
[ "$code" = 200 ] || { echo "the service repository is not on the git server (HTTP ${code})" >&2; exit 1; }
echo "  the repository exists on the git server"

pr="$(curl -sk --max-time 15 --resolve "gitea.127-0-0-1.sslip.io:443:127.0.0.1" \
  "https://gitea.127-0-0-1.sslip.io/api/v1/repos/gitea-admin/kensan-lab/pulls?state=open" \
  | grep -c "add-app-${NAME}" || true)"
[ "$pr" -gt 0 ] || { echo "no pull request was opened against the platform repository" >&2; exit 1; }
echo "  the platform pull request is open"

if [[ "$MODE" != "--deploy" ]]; then
  echo "200 (the golden path completed without a token)"
  exit 0
fi

# CI takes the one manual step in the walkthrough, then proves the result rather
# than treating a completed scaffolder task as a deployed application. The
# credentials belong to this disposable cluster and are the same ones the
# scaffolder used to open the pull request.
gitea_user="$(kubectl -n backstage get secret backstage-explore-gitea \
  -o go-template='{{index .data "username" | base64decode}}')"
gitea_password="$(kubectl -n backstage get secret backstage-explore-gitea \
  -o go-template='{{index .data "password" | base64decode}}')"
gitea_api="https://gitea.127-0-0-1.sslip.io/api/v1"

pulls="$(curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
  "${gitea_api}/repos/gitea-admin/kensan-lab/pulls?state=open")"
pr_number="$(printf '%s' "$pulls" | python3 -c '
import json, sys
branch = sys.argv[1]
for pull in json.load(sys.stdin):
    if pull.get("head", {}).get("ref") == branch:
        print(pull["number"])
        break
' "add-app-${NAME}")"
[ -n "$pr_number" ] || { echo "could not identify the platform pull request" >&2; exit 1; }

merge_code="$(curl -sk -o /dev/null -w '%{http_code}' --max-time 30 \
  -u "${gitea_user}:${gitea_password}" -H 'Content-Type: application/json' \
  -X POST -d '{"Do":"merge"}' \
  "${gitea_api}/repos/gitea-admin/kensan-lab/pulls/${pr_number}/merge")"
[ "$merge_code" = 200 ] || {
  echo "Gitea refused to merge platform PR #${pr_number} (HTTP ${merge_code})" >&2
  exit 1
}
echo "  merged platform PR #${pr_number}"

# Argo CD polls Git on its own, but waiting for that cache interval makes this
# check needlessly slow and can consume most of the application deadline. A
# human sees the same eventual reconciliation; CI asks for it immediately.
kubectl -n argocd annotate application explore-root \
  argocd.argoproj.io/refresh=hard --overwrite >/dev/null

deadline=$(( $(date +%s) + 360 ))
while :; do
  state="$(kubectl -n argocd get application "app-${NAME}" \
    -o jsonpath='{.status.sync.status}/{.status.health.status}' 2>/dev/null || true)"
  [[ "$state" == "Synced/Healthy" ]] && break
  if (( $(date +%s) >= deadline )); then
    echo "app-${NAME} did not become Synced/Healthy (last: ${state:-missing})" >&2
    kubectl -n argocd get application "app-${NAME}" -o yaml >&2 2>/dev/null || true
    exit 1
  fi
  sleep 5
done
echo "  app-${NAME} is Synced/Healthy"

kubectl -n "app-${NAME}" port-forward "svc/${NAME}" 18080:8000 >/dev/null 2>&1 &
APP_PF_PID=$!
for _ in $(seq 1 30); do
  curl -sf --max-time 2 http://127.0.0.1:18080/health >/dev/null 2>&1 && break
  sleep 1
done

config="$(curl -sf --max-time 5 http://127.0.0.1:18080/api/config || true)"
printf '%s' "$config" | grep -q '"appName":"'"${NAME}"'"' || {
  echo "the deployed app did not receive its name: ${config:-<no response>}" >&2; exit 1; }
printf '%s' "$config" | grep -q '"theme":"night"' || {
  echo "the deployed app did not receive its theme: ${config}" >&2; exit 1; }
printf '%s' "$config" | grep -q '"message":"created without a token"' || {
  echo "the deployed app did not receive its greeting: ${config}" >&2; exit 1; }
curl -sf --max-time 5 http://127.0.0.1:18080/ | grep -q '<div id="root"></div>' || {
  echo "the deployed app did not serve the React frontend" >&2; exit 1; }

login="$($(dirname "$0")/explore-login-check.sh \
  "${NAME}.127-0-0-1.sslip.io" "$USER_NAME" "$PASSWORD" 2>&1 || true)"
[ "$login" = 200 ] || {
  echo "the deployed app is not reachable through SSO: ${login}" >&2; exit 1; }

echo "200 (the golden path built, configured and deployed the app without a token)"
