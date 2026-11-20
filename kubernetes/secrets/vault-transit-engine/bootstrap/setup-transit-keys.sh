#!/usr/bin/env bash
# Run after the Transit mount is ready. Use an existing authorized Vault session.
# Example: VAULT_ADDR=https://vault.platform.yu-min3.com vault login -method=oidc role=platform-admin
set -euo pipefail

KEY_NAME="${KEY_NAME:-users-name}"
VAULT_NS="vault"
VAULT_SVC="vault-active"
LOCAL_PORT="${LOCAL_PORT:-8200}"
PF_PID=""

cleanup() {
  if [[ -n "$PF_PID" ]] && kill -0 "$PF_PID" 2>/dev/null; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
  fi
  unset VAULT_TOKEN || true
}
trap cleanup EXIT

# === 前提コマンド ===
for cmd in kubectl vault; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd not found" >&2
    exit 1
  fi
done

# === port-forward ===
echo "==> port-forward svc/$VAULT_SVC :$LOCAL_PORT (ns=$VAULT_NS)"
kubectl -n "$VAULT_NS" port-forward "svc/$VAULT_SVC" "${LOCAL_PORT}:8200" >/dev/null 2>&1 &
PF_PID=$!

export VAULT_ADDR="http://127.0.0.1:${LOCAL_PORT}"

# port-forward が listen するまで待つ
for _ in $(seq 1 20); do
  if vault status >/dev/null 2>&1; then
    break
  fi
  sleep 0.3
done

if ! kill -0 "$PF_PID" 2>/dev/null || ! vault status >/dev/null 2>&1; then
  echo "ERROR: vault に到達できない ($VAULT_ADDR)" >&2
  exit 1
fi

vault token lookup >/dev/null || { echo "Login to the target Vault first." >&2; exit 1; }

# === key create (冪等) ===
if key_status=$(vault read -format=json "transit/keys/${KEY_NAME}" 2>&1); then
  echo "==> transit/keys/${KEY_NAME} 既に存在 → skip"
elif [[ "$key_status" == "No value found at transit/keys/$KEY_NAME" ]]; then
  echo "==> Creating Vault Transit key: ${KEY_NAME}"
  vault write -f "transit/keys/${KEY_NAME}" \
    type=aes256-gcm96 \
    exportable=false \
    allow_plaintext_backup=false \
    derived=false \
    convergent_encryption=false
else
  echo "Transit key lookup failed. Check the Vault session and permissions before retrying." >&2
  exit 1
fi

# === verify ===
echo
echo "==> Verify:"
vault read "transit/keys/${KEY_NAME}"

echo
echo "==> Done."
