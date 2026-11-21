#!/usr/bin/env bash
# Populate Vault for Langfuse and the OTel collector's external receiver.
#
#   secret/observability/langfuse        Langfuse app, stores, konro project keys, owner login
#   secret/observability/otel-collector  bearer token for the otlp/external receiver
#
# Idempotent: existing values are kept and only missing keys are generated.
# SALT and ENCRYPTION_KEY must never change once Langfuse has started (API keys
# and encrypted columns depend on them), so there is no rotate option here.
# Secrets are never printed; only key names are shown.
#
# Prerequisites: vault CLI logged in with write access to secret/
#   (vault login -method=oidc role=platform-admin), jq, openssl.
#
# Usage: bootstrap/vault/llmops-secrets.sh --owner-email you@example.com [--dry-run]
set -euo pipefail

LANGFUSE_PATH="secret/observability/langfuse"
COLLECTOR_PATH="secret/observability/otel-collector"
owner_email=""
dry_run=false
while [ $# -gt 0 ]; do
  case "$1" in
    --owner-email) owner_email="$2"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
for cmd in vault jq openssl; do command -v "$cmd" >/dev/null || { echo "$cmd is required" >&2; exit 1; }; done
vault token lookup >/dev/null 2>&1 || { echo "vault is not logged in (vault login -method=oidc role=platform-admin)" >&2; exit 1; }

hex() { openssl rand -hex "$1"; }
alnum() { openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | head -c "$1"; }

# Prints the existing data object of a path, or {} when missing.
existing() { vault kv get -format=json "$1" 2>/dev/null | jq -c '.data.data' || echo '{}'; }

# merge PATH key=generator ...: keep existing values, fill missing ones.
merge() {
  local path="$1"; shift
  local current; current=$(existing "$path")
  local updated="$current" added=()
  for spec in "$@"; do
    local key="${spec%%=*}" gen="${spec#*=}"
    if [ "$(jq -r --arg k "$key" '.[$k] // empty' <<<"$current")" = "" ]; then
      local value; value=$(eval "$gen")
      [ -n "$value" ] || { echo "empty value generated for $key" >&2; exit 1; }
      updated=$(jq -c --arg k "$key" --arg v "$value" '. + {($k): $v}' <<<"$updated")
      added+=("$key")
    fi
  done
  if [ ${#added[@]} -eq 0 ]; then
    echo "$path: all keys present, unchanged"
    return
  fi
  echo "$path: adding ${added[*]}"
  if $dry_run; then
    echo "  (dry run, not written)"
    return
  fi
  # Values go through stdin, never argv.
  jq -c . <<<"$updated" | vault kv put "$path" - >/dev/null
}

current_email=$(existing "$LANGFUSE_PATH" | jq -r '."init-user-email" // empty')
if [ -z "$current_email" ] && [ -z "$owner_email" ]; then
  echo "--owner-email is required on first run (Langfuse owner login)" >&2
  exit 2
fi

merge "$LANGFUSE_PATH" \
  "salt=hex 32" \
  "encryption-key=hex 32" \
  "nextauth-secret=hex 32" \
  "postgres-password=alnum 40" \
  "postgres-admin-password=alnum 40" \
  "redis-password=alnum 40" \
  "clickhouse-password=alnum 40" \
  "s3-password=alnum 40" \
  "konro-public-key=echo pk-lf-\$(hex 16)" \
  "konro-secret-key=echo sk-lf-\$(hex 24)" \
  "init-user-email=echo $owner_email" \
  "init-user-password=alnum 24"

merge "$COLLECTOR_PATH" \
  "otlp-external-token=hex 32"

echo
echo "keys now stored:"
for p in "$LANGFUSE_PATH" "$COLLECTOR_PATH"; do
  printf '  %s: %s\n' "$p" "$(existing "$p" | jq -r 'keys | join(", ")')"
done
echo
echo "Langfuse owner password: vault kv get -field=init-user-password $LANGFUSE_PATH"
