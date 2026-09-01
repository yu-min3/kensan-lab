#!/usr/bin/env bash
# Backstage's own OIDC round trip, without a browser.
#
# Different from the gateway flow: Backstage is not behind oauth2-proxy, so
# there is no CSRF cookie and no /oauth2/callback. It starts the flow itself at
# /api/auth/oidc/start and finishes at /api/auth/oidc/handler/frame.
set -euo pipefail

HOST="${1:-backstage.127-0-0-1.sslip.io}"
USER_NAME="${2:-demo}"
PASSWORD="${3:-demo}"
JAR="$(mktemp)"
trap 'rm -f "$JAR"' EXIT
C=(/usr/bin/curl -sk --cookie-jar "$JAR" --cookie "$JAR")

start="https://${HOST}/api/auth/oidc/start?scope=openid%20profile%20email&origin=https%3A%2F%2F${HOST}&flow=popup&env=production"
auth_url="$("${C[@]}" -o /dev/null -w '%{redirect_url}' "$start")"
case "$auth_url" in
  *client_id=backstage*) ;;
  *) echo "start did not redirect to Keycloak as the backstage client: ${auth_url:-<none>}" >&2; exit 1 ;;
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

# The handler answers with a page that posts the result back to the opener.
# A refused sign-in says so in that payload rather than in the status code.
body="$("${C[@]}" "$handler")"
if grep -qi "error" <<<"$body" && ! grep -qi "backstageIdentity\|profile" <<<"$body"; then
  echo "sign-in refused:"; echo "$body" | grep -oiE '"error[^,]{0,120}' | head -2
  exit 1
fi
grep -qi "backstageIdentity\|profile" <<<"$body" \
  && echo "200 (Backstage issued an identity)" \
  || { echo "the handler returned no identity"; echo "$body" | head -c 200; exit 1; }
