#!/usr/bin/env bash
# Run independent session-expiry and golden-path checks concurrently. The login
# helper owns a private cookie jar, so scaffolding cannot refresh its session.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG="$(mktemp)"
REFRESH_PID=""
cleanup() {
  if [[ -n "$REFRESH_PID" ]]; then
    kill "$REFRESH_PID" 2>/dev/null || true
    wait "$REFRESH_PID" 2>/dev/null || true
  fi
  rm -f "$LOG"
}
trap cleanup EXIT

"${ROOT}/scripts/explore-login-check.sh" \
  demo.127-0-0-1.sslip.io demo demo --refresh >"$LOG" 2>&1 &
REFRESH_PID=$!

"${ROOT}/scripts/explore-golden-path-check.sh" \
  backstage.127-0-0-1.sslip.io demo demo checkservice --deploy

if ! wait "$REFRESH_PID"; then
  REFRESH_PID=""
  cat "$LOG" >&2
  echo "expired-session check failed" >&2
  exit 1
fi
REFRESH_PID=""
[ "$(cat "$LOG")" = 200 ] || {
  cat "$LOG" >&2
  echo "expired-session check did not return 200" >&2
  exit 1
}
echo "200 (expired session and golden path both passed)"
