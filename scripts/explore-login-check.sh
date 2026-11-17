#!/usr/bin/env bash
# Sign in through the gateway the way a browser does, and print the status code
# the front door finally answers with. Used by scripts/explore-up.sh and by
# Explore CI; safe to run by hand against a live cluster:
#
#   scripts/explore-login-check.sh demo.127-0-0-1.sslip.io demo demo
#
# Sign in through the gateway the way a browser does: follow the redirect to
# Keycloak, post the login form, come back through /oauth2/callback, and end up
# holding a session cookie.
#
# Not a Bearer token. oauth2-proxy is deliberately no longer given the
# Authorization header (kubernetes/network/istio/istiod/values.yaml) so that an
# application's own token survives the gate — which means a Bearer token no
# longer opens it, and a test that used one would be testing a path nobody
# takes.
set -euo pipefail

HOST="${1:-demo.127-0-0-1.sslip.io}"
USER_NAME="${2:-demo}"
PASSWORD="${3:-demo}"
CHECK_REFRESH="${4:-}"
[[ -z "$CHECK_REFRESH" || "$CHECK_REFRESH" = --refresh ]] || exit 2
JAR="$(mktemp)"
trap 'rm -f "$JAR"' EXIT

C=(curl -sk --cookie-jar "$JAR" --cookie "$JAR")

# 1. The gate sends us to Keycloak and sets the CSRF cookie on the way.
auth_url="$("${C[@]}" -o /dev/null -w '%{redirect_url}' "https://${HOST}/")"
case "$auth_url" in
  *auth.127-0-0-1.sslip.io*) ;;
  *) echo "the gate did not redirect to Keycloak: ${auth_url:-<none>}" >&2; exit 1 ;;
esac

# 2. Keycloak's login page carries the URL its form posts to.
form_action="$("${C[@]}" "$auth_url" \
  | sed -n 's/.*id="kc-form-login"[^>]*action="\([^"]*\)".*/\1/p' \
  | head -1 | sed 's/&amp;/\&/g')"
[ -n "$form_action" ] || { echo "no login form on the Keycloak page" >&2; exit 1; }

# 3. Posting it hands back a redirect to the callback, carrying the code.
callback="$("${C[@]}" -o /dev/null -w '%{redirect_url}' \
  -d "username=${USER_NAME}" -d "password=${PASSWORD}" "$form_action")"
case "$callback" in
  *oauth2/callback*) ;;
  *) echo "Keycloak did not return a callback: ${callback:-<none>}" >&2; exit 1 ;;
esac

# 4. The callback is where oauth2-proxy checks the CSRF cookie and, if it is
#    happy, writes the session cookie. This is the step that fails when the
#    CSRF cookie never reached the client.
"${C[@]}" -o /dev/null "$callback"

# 5. And now the front door should simply open.
code="$("${C[@]}" -o /dev/null -w '%{http_code}' "https://${HOST}/")"
if [[ "$CHECK_REFRESH" = --refresh ]]; then
  [ "$code" = 200 ] || { echo "initial login failed: $code" >&2; exit 1; }
  # Include every split session cookie, without printing credential values.
  session() {
    awk -F '\t' '$6 ~ /^_oauth2_proxy(_[0-9]+)?$/ {print $6 "=" $7}' "$JAR" | sort
  }
  before="$(session)"
  [ -n "$before" ] || { echo "no session cookies" >&2; exit 1; }
  # Keycloak's default token lifetime is five minutes. No requests during this
  # wait: otherwise an earlier refresh would hide the expired-session path.
  sleep 310
  code="$("${C[@]}" -o /dev/null -w '%{http_code}' "https://${HOST}/")"
  [ "$code" = 200 ] || { echo "expired session returned $code" >&2; exit 1; }
  renewed="$(session)"
  [ "$renewed" != "$before" ] || { echo "refreshed cookies did not reach the client" >&2; exit 1; }
  [ "$(printf '%s\n' "$renewed" | cut -d= -f1)" = "$(printf '%s\n' "$before" | cut -d= -f1)" ] \
    || { echo "split session cookies changed unexpectedly" >&2; exit 1; }
  for _ in 1 2 3; do
    code="$("${C[@]}" -o /dev/null -w '%{http_code}' "https://${HOST}/api/config")"
    [ "$code" = 200 ] && [ "$(session)" = "$renewed" ] \
      || { echo "renewed session was not retained" >&2; exit 1; }
  done
fi
echo "$code"
