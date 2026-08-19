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
# Usage: explore-golden-path-check.sh <backstage-host> <user> <password>
set -euo pipefail

HOST="${1:-backstage.127-0-0-1.sslip.io}"
USER_NAME="${2:-demo}"
PASSWORD="${3:-local-demo-password}"
NAME="${4:-checkservice}"
JAR="$(mktemp)"
trap 'rm -f "$JAR"' EXIT
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

# The handler answers with a page that posts the result back to the opener; the
# token is in that payload.
token="$("${C[@]}" "$handler" \
  | grep -oE '"backstageIdentity":\{[^}]*"token":"[^"]+' \
  | grep -oE '"token":"[^"]+' | cut -d'"' -f4 | head -1)"
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
  -d "$body" | grep -oE '"id":"[^"]+' | cut -d'"' -f4 | head -1)"
[ -n "$task" ] || { echo "the scaffolder refused the request" >&2; exit 1; }
echo "  scaffolder task ${task}"

for _ in $(seq 1 60); do
  status="$("${C[@]}" "https://${HOST}/api/scaffolder/v2/tasks/${task}" \
    -H "Authorization: Bearer ${token}" \
    | grep -oE '"status":"[^"]+' | cut -d'"' -f4 | head -1)"
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

echo "200 (the golden path completed without a token)"
