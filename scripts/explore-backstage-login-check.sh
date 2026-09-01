#!/usr/bin/env bash
# Backstage's own OIDC round trip and Explore template lookup, without a browser.
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

# The handler answers with a page that posts a percent-encoded payload to the
# opener. Decode it before looking for the Backstage identity token.
frame="$("${C[@]}" "$handler" || true)"
payload="$(printf '%s' "$frame" \
  | sed -n "s/.*decodeURIComponent('\([^']*\)').*/\1/p" | head -1)"
[ -n "$payload" ] || { echo "the sign-in page carried no auth payload" >&2; exit 1; }
decoded="$(printf '%b' "${payload//%/\\x}")"
token="$(printf '%s' "$decoded" \
  | grep -oE '"backstageIdentity":\{"token":"[^"]+' \
  | grep -oE '"token":"[^"]+' | cut -d'"' -f4 | head -1 || true)"
[ -n "$token" ] || { echo "the handler returned no Backstage identity" >&2; exit 1; }

# A missing catalog file is not a plugin startup failure: Backstage stays
# Healthy and Create shows an empty page. Ask for the entity a person expects
# to see so that this test proves the experience, not just the login.
code="$("${C[@]}" -o /dev/null -w '%{http_code}' \
  -H "Authorization: Bearer ${token}" \
  "https://${HOST}/api/catalog/entities/by-name/template/default/fastapi-app-template")"
[ "$code" = 200 ] || {
  echo "the Explore software template is not registered (HTTP ${code})" >&2
  exit 1
}

echo "200 (Backstage issued an identity; Explore template is registered)"
