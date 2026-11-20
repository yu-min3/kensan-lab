#!/bin/bash
# One-off import of an existing Kubernetes Secret using the operator's Vault session.
# For credentials managed in 1Password use sync-op.py.
set -euo pipefail
set +x
VAULT_ADDR="${VAULT_ADDR:-https://vault.platform.yu-min3.com}"
export VAULT_ADDR

# === 引数 parse ===
SOURCE_NS=""
SOURCE_SECRET=""
VAULT_PATH=""
KEYS=""

usage() {
  cat <<EOF
Usage: $0 --source-ns NS --source-secret NAME --vault-path PATH --keys KEY1,KEY2,...

Required:
  --source-ns      K8s Secret の namespace
  --source-secret  K8s Secret 名
  --vault-path     Vault KV path (例: secret/monitoring/grafana/admin)
  --keys           移行する key (カンマ区切り、Secret と Vault で同じ key 名を使う)

Optional:
  --vault-addr     Vault HTTPS endpoint (default: $VAULT_ADDR)
EOF
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --source-ns)     SOURCE_NS="$2"; shift 2;;
    --source-secret) SOURCE_SECRET="$2"; shift 2;;
    --vault-path)    VAULT_PATH="$2"; shift 2;;
    --keys)          KEYS="$2"; shift 2;;
    --vault-addr)    VAULT_ADDR="$2"; shift 2;;
    -h|--help)       usage;;
    *) echo "ERROR: unknown arg $1"; usage;;
  esac
done

[ -z "$SOURCE_NS" ] && usage
[ -z "$SOURCE_SECRET" ] && usage
[ -z "$VAULT_PATH" ] && usage
[ -z "$KEYS" ] && usage

# Read the Secret once. Values travel only over pipes.
kubectl -n "$SOURCE_NS" get secret "$SOURCE_SECRET" -o json |
  jq -ce --arg keys "$KEYS" '
    .data as $data | reduce ($keys | split(",")[]) as $k ({};
      if ($data[$k] // "") == "" then error("missing selected key")
      else .[$k] = ($data[$k] | @base64d) end)' |
  python3 "$(dirname "$0")/kv_sync.py" "$VAULT_PATH"
