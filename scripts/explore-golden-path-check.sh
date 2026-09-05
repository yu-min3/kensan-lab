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
REGISTRY_PF_PID=""
cleanup() {
  rm -f "$JAR"
  if [[ -n "$APP_PF_PID" ]]; then
    kill "$APP_PF_PID" 2>/dev/null || true
    wait "$APP_PF_PID" 2>/dev/null || true
  fi
  if [[ -n "$REGISTRY_PF_PID" ]]; then
    kill "$REGISTRY_PF_PID" 2>/dev/null || true
    wait "$REGISTRY_PF_PID" 2>/dev/null || true
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
           "repoUrl":"gitea.127-0-0-1.sslip.io?owner=demo&repo=${NAME}",
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

for _ in $(seq 1 180); do
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
  "https://gitea.127-0-0-1.sslip.io/demo/${NAME}")"
[ "$code" = 200 ] || { echo "the service repository is not on the git server (HTTP ${code})" >&2; exit 1; }
echo "  the repository exists on the git server"

pr="$(curl -sk --max-time 15 --resolve "gitea.127-0-0-1.sslip.io:443:127.0.0.1" \
  "https://gitea.127-0-0-1.sslip.io/api/v1/repos/demo/kensan-lab/pulls?state=open" \
  | grep -c "add-app-${NAME}" || true)"
[ "$pr" -gt 0 ] || { echo "no pull request was opened against the platform repository" >&2; exit 1; }
echo "  the platform pull request is open"

# The scaffolder now waits for the generated repository's workflow before it
# opens the platform pull request. Read back the immutable tag and registry
# manifest as an independent end-to-end assertion of that contract.
gitea_user="$(kubectl -n backstage get secret backstage-explore-gitea \
  -o go-template='{{index .data "username" | base64decode}}')"
gitea_password="$(kubectl -n backstage get secret backstage-explore-gitea \
  -o go-template='{{index .data "password" | base64decode}}')"
gitea_api="https://gitea.127-0-0-1.sslip.io/api/v1"

image_tag() {
  curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
    "${gitea_api}/repos/demo/${NAME}/contents/deploy/values.yaml?ref=main" \
    | python3 -c '
import base64, json, re, sys
try:
    text = base64.b64decode(json.load(sys.stdin)["content"]).decode()
except Exception:
    sys.exit(0)
match = re.search(r"^  tag: ([0-9a-f]{40})$", text, re.MULTILINE)
if match:
    print(match.group(1))
'
}

first_image_tag=""
deadline=$(( $(date +%s) + 900 ))
while [[ -z "${first_image_tag}" ]]; do
  first_image_tag="$(image_tag)"
  if (( $(date +%s) >= deadline )); then
    echo "the generated repository did not produce an immutable image tag" >&2
    curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
      "${gitea_api}/repos/demo/${NAME}/actions/tasks" | tail -c 2000 >&2 || true
    exit 1
  fi
  [[ -n "${first_image_tag}" ]] || sleep 5
done

kubectl -n explore-build port-forward svc/registry 15000:5000 >/dev/null 2>&1 &
REGISTRY_PF_PID=$!
for _ in $(seq 1 30); do
  curl -sf --max-time 2 http://127.0.0.1:15000/v2/ >/dev/null 2>&1 && break
  sleep 1
done
curl -sf --max-time 10 \
  "http://127.0.0.1:15000/v2/demo/${NAME}/manifests/${first_image_tag}" \
  -o /dev/null \
  || { echo "the workflow recorded an image that is absent from the registry" >&2; exit 1; }
echo "  Gitea Actions built ${NAME}:${first_image_tag:0:12} from its own source"

if [[ "$MODE" != "--deploy" ]]; then
  echo "200 (the golden path built its repository without an external token)"
  exit 0
fi

# CI takes the one manual step in the walkthrough, then proves the result rather
# than treating a completed build as a deployed application.

pulls="$(curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
  "${gitea_api}/repos/demo/kensan-lab/pulls?state=open")"
pr_number="$(printf '%s' "$pulls" | python3 -c '
import json, sys
branch = sys.argv[1]
for pull in json.load(sys.stdin):
    if pull.get("head", {}).get("ref") == branch:
        print(pull["number"])
        break
' "add-app-${NAME}")"
[ -n "$pr_number" ] || { echo "could not identify the platform pull request" >&2; exit 1; }

# Gitea computes mergeability asynchronously after opening a pull request. A
# human has spent a few seconds opening the PR page by this point; CI reaches
# the merge endpoint immediately and receives 405 until that calculation is
# ready. Retry only that transient response — authentication and API errors
# still fail without being hidden.
merge_code=""
for _ in $(seq 1 30); do
  merge_code="$(curl -sk -o /dev/null -w '%{http_code}' --max-time 30 \
    -u "${gitea_user}:${gitea_password}" -H 'Content-Type: application/json' \
    -X POST -d '{"Do":"merge"}' \
    "${gitea_api}/repos/demo/kensan-lab/pulls/${pr_number}/merge")"
  [[ "$merge_code" == 200 ]] && break
  [[ "$merge_code" == 405 ]] || break
  sleep 2
done
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

deployed_image="$(kubectl -n "app-${NAME}" get deployment "${NAME}" \
  -o jsonpath='{.spec.template.spec.containers[0].image}')"
case "${deployed_image}" in
  "10.96.0.50:5000/demo/${NAME}:${first_image_tag}") ;;
  *) echo "the deployment does not use its repository image: ${deployed_image}" >&2; exit 1 ;;
esac
first_pod_uid="$(kubectl -n "app-${NAME}" get pod \
  -l "app.kubernetes.io/name=${NAME}" -o jsonpath='{.items[0].metadata.uid}')"

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

# Prove this is a continuing delivery path, not a one-off scaffold build. Edit
# the generated Python source through Gitea, then require a second image, a new
# pod and the new endpoint. A shared or mutable image can pass none of these.
source_file="$(curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
  "${gitea_api}/repos/demo/${NAME}/contents/app/main.py?ref=main")"
update_body="$(printf '%s' "${source_file}" | python3 -c '
import base64, json, sys
item = json.load(sys.stdin)
source = base64.b64decode(item["content"]).decode()
source += "\n\n@app.get(\"/build-proof\")\nasync def build_proof():\n    return {\"source\": \"rebuilt-by-gitea-actions\"}\n"
print(json.dumps({
    "branch": "main",
    "message": "Prove generated source rebuilds",
    "sha": item["sha"],
    "content": base64.b64encode(source.encode()).decode(),
}))
')"
update_code="$(curl -sk -o /dev/null -w '%{http_code}' --max-time 30 \
  -u "${gitea_user}:${gitea_password}" -H 'Content-Type: application/json' \
  -X PUT -d "${update_body}" \
  "${gitea_api}/repos/demo/${NAME}/contents/app/main.py")"
[[ "${update_code}" == 200 || "${update_code}" == 201 ]] || {
  echo "could not commit the source change to the generated repository (HTTP ${update_code})" >&2
  exit 1
}

second_image_tag=""
deadline=$(( $(date +%s) + 900 ))
while [[ -z "${second_image_tag}" || "${second_image_tag}" == "${first_image_tag}" ]]; do
  second_image_tag="$(image_tag)"
  if (( $(date +%s) >= deadline )); then
    echo "the source change did not produce a second immutable image" >&2
    exit 1
  fi
  [[ -n "${second_image_tag}" && "${second_image_tag}" != "${first_image_tag}" ]] || sleep 5
done

curl -sf --max-time 10 \
  "http://127.0.0.1:15000/v2/demo/${NAME}/manifests/${second_image_tag}" \
  -o /dev/null \
  || { echo "the rebuilt image is absent from the registry" >&2; exit 1; }

kubectl -n argocd annotate application "app-${NAME}" \
  argocd.argoproj.io/refresh=hard --overwrite >/dev/null
deadline=$(( $(date +%s) + 360 ))
while :; do
  deployed_image="$(kubectl -n "app-${NAME}" get deployment "${NAME}" \
    -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || true)"
  state="$(kubectl -n argocd get application "app-${NAME}" \
    -o jsonpath='{.status.sync.status}/{.status.health.status}' 2>/dev/null || true)"
  [[ "${deployed_image}" == "10.96.0.50:5000/demo/${NAME}:${second_image_tag}" \
    && "${state}" == "Synced/Healthy" ]] && break
  if (( $(date +%s) >= deadline )); then
    echo "the rebuilt image did not roll out (image=${deployed_image}, state=${state})" >&2
    exit 1
  fi
  sleep 5
done
kubectl -n "app-${NAME}" rollout status deployment/"${NAME}" --timeout=3m >/dev/null
second_pod_uid="$(kubectl -n "app-${NAME}" get pod \
  -l "app.kubernetes.io/name=${NAME}" -o jsonpath='{.items[0].metadata.uid}')"
[[ "${second_pod_uid}" != "${first_pod_uid}" ]] \
  || { echo "the image changed without replacing the application pod" >&2; exit 1; }

kill "${APP_PF_PID}" 2>/dev/null || true
wait "${APP_PF_PID}" 2>/dev/null || true
APP_PF_PID=""
kubectl -n "app-${NAME}" port-forward "svc/${NAME}" 18080:8000 >/dev/null 2>&1 &
APP_PF_PID=$!
for _ in $(seq 1 30); do
  proof="$(curl -sf --max-time 2 http://127.0.0.1:18080/build-proof 2>/dev/null || true)"
  [[ "${proof}" == *rebuilt-by-gitea-actions* ]] && break
  sleep 1
done
[[ "${proof:-}" == *rebuilt-by-gitea-actions* ]] \
  || { echo "the rebuilt pod does not serve the committed source change" >&2; exit 1; }

echo "  source edit produced ${NAME}:${second_image_tag:0:12} and a new pod"

# A red build must leave the last good image running. Commit a deliberate Ruff
# failure, wait for Actions to reject it, then restore the good source with a
# skip marker so the disposable repository is not left broken.
good_source_file="$(curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
  "${gitea_api}/repos/demo/${NAME}/contents/app/main.py?ref=main")"
bad_update_body="$(printf '%s' "${good_source_file}" | python3 -c '
import base64, json, sys
item = json.load(sys.stdin)
source = base64.b64decode(item["content"]).decode() + "\nBROKEN = undefined_name\n"
print(json.dumps({
    "branch": "main",
    "message": "Exercise the failed-build guard",
    "sha": item["sha"],
    "content": base64.b64encode(source.encode()).decode(),
}))
')"
bad_update="$(curl -sk --max-time 30 -u "${gitea_user}:${gitea_password}" \
  -H 'Content-Type: application/json' -X PUT -d "${bad_update_body}" \
  "${gitea_api}/repos/demo/${NAME}/contents/app/main.py")"
bad_commit="$(printf '%s' "${bad_update}" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("commit",{}).get("sha",""))')"
[[ -n "${bad_commit}" ]] || { echo "could not create the failed-build fixture" >&2; exit 1; }

bad_status=""
deadline=$(( $(date +%s) + 300 ))
while [[ "${bad_status}" != "failure" ]]; do
  bad_status="$(curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
    "${gitea_api}/repos/demo/${NAME}/actions/tasks" \
    | python3 -c '
import json, sys
sha = sys.argv[1]
for run in json.load(sys.stdin).get("workflow_runs", []):
    if run.get("head_sha") == sha:
        print(run.get("status", ""))
        break
' "${bad_commit}")"
  [[ "${bad_status}" == "failure" ]] && break
  [[ "${bad_status}" != "success" ]] \
    || { echo "the deliberately invalid source passed its build" >&2; exit 1; }
  if (( $(date +%s) >= deadline )); then
    echo "the deliberately invalid source build never finished" >&2
    exit 1
  fi
  sleep 5
done

[[ "$(image_tag)" == "${second_image_tag}" ]] \
  || { echo "a failed build changed the deploy image tag" >&2; exit 1; }
current_pod_uid="$(kubectl -n "app-${NAME}" get pod \
  -l "app.kubernetes.io/name=${NAME}" -o jsonpath='{.items[0].metadata.uid}')"
[[ "${current_pod_uid}" == "${second_pod_uid}" ]] \
  || { echo "a failed build replaced the last good pod" >&2; exit 1; }

bad_source_file="$(curl -sk --max-time 15 -u "${gitea_user}:${gitea_password}" \
  "${gitea_api}/repos/demo/${NAME}/contents/app/main.py?ref=main")"
restore_body="$(python3 -c '
import json, sys
good = json.loads(sys.argv[1])
bad = json.loads(sys.argv[2])
print(json.dumps({
    "branch": "main",
    "message": "Restore the last good source [skip ci]",
    "sha": bad["sha"],
    "content": good["content"],
}))
' "${good_source_file}" "${bad_source_file}")"
restore_code="$(curl -sk -o /dev/null -w '%{http_code}' --max-time 30 \
  -u "${gitea_user}:${gitea_password}" -H 'Content-Type: application/json' \
  -X PUT -d "${restore_body}" \
  "${gitea_api}/repos/demo/${NAME}/contents/app/main.py")"
[[ "${restore_code}" == 200 || "${restore_code}" == 201 ]] \
  || { echo "could not restore the failed-build fixture" >&2; exit 1; }
echo "  failed source was rejected; the last good image and pod stayed running"

echo "200 (the golden path built, deployed and rebuilt the generated source)"
