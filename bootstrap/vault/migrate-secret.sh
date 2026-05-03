#!/bin/bash
# 既存 K8s Secret の値を Vault KV v2 に投入する汎用スクリプト
#
# 使い方:
#   ./bootstrap/vault/migrate-secret.sh \
#       --source-ns monitoring \
#       --source-secret grafana-admin-secret \
#       --vault-path secret/monitoring/grafana/admin \
#       --keys admin-user,admin-password
#
# 何をするか:
#   1. K8s Secret から指定 key の値を base64 decode で取り出し
#   2. Vault KV v2 (default mount: secret/) に PUT
#   3. 確認: 投入後の version と key 一覧を表示 (値は出さない)
#
# 前提:
#   - kubectl context が kensan-lab cluster
#   - bw login + unlock 済み (BW_SESSION 設定済み)
#   - /etc/hosts に vault.platform.yu-min3.com の解決を入れてある
#     (or curl が name resolve できる経路がある)
#   - jq, curl がローカル install 済み
#   - Vault root token を Bitwarden の "kensan-lab/vault/root-token" に保管済み
#
# セキュリティ:
#   - root token は変数で扱い stdout に echo しない
#   - 移行する値も stdout に echo しない (key 一覧のみ表示)
#   - 終了時に環境変数を unset
#
# 冪等性: 同じ vault-path に再投入すると新 version が作成される (KV v2 仕様)

set -euo pipefail

# === デフォルト ===
VAULT_ADDR="${VAULT_ADDR:-https://vault.platform.yu-min3.com}"
BW_ROOT_TOKEN_ITEM="kensan-lab/vault/root-token"

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

# === 前提チェック ===
for cmd in kubectl bw curl jq; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "ERROR: $cmd not found"; exit 1
  fi
done

bw_status=$(bw status 2>/dev/null | jq -r '.status' 2>/dev/null || echo "unknown")
if [ "$bw_status" != "unlocked" ]; then
  echo "ERROR: bw が unlock 状態じゃない (status: $bw_status)"
  echo "  export BW_SESSION=\$(bw unlock --raw)"
  exit 1
fi

# === 既存 K8s Secret 確認 ===
echo "==> Source: $SOURCE_NS/$SOURCE_SECRET"
if ! kubectl -n "$SOURCE_NS" get secret "$SOURCE_SECRET" &> /dev/null; then
  echo "ERROR: K8s Secret が無い"; exit 1
fi

# === root token 取得 ===
echo "==> Pulling Vault root token from Bitwarden..."
VAULT_TOKEN=$(bw get item "$BW_ROOT_TOKEN_ITEM" 2>/dev/null | jq -r '.login.password' || echo "")
if [ -z "$VAULT_TOKEN" ] || [ "$VAULT_TOKEN" = "null" ]; then
  echo "ERROR: Bitwarden から root token 取得失敗"; exit 1
fi
echo "    OK"

# === Vault 到達確認 ===
echo ""
echo "==> Vault health check at $VAULT_ADDR"
# standbyok=true: HA cluster で Istio LB が standby pod に振っても 200 を返させる
# (Vault default は standby pod だと 429。curl -f が落ちる)
if ! curl -sf --max-time 10 \
    -H "X-Vault-Token: $VAULT_TOKEN" \
    "$VAULT_ADDR/v1/sys/health?standbyok=true" > /dev/null; then
  echo "ERROR: Vault に届かない or token 無効"
  echo "  /etc/hosts と TLS cert を確認してね"
  unset VAULT_TOKEN
  exit 1
fi
echo "    OK"

# === K8s Secret から値取り出して JSON 構築 ===
echo ""
echo "==> Reading keys from $SOURCE_NS/$SOURCE_SECRET"
DATA_JSON='{"data":{}}'
IFS=',' read -ra KEY_ARR <<< "$KEYS"
for key in "${KEY_ARR[@]}"; do
  value=$(kubectl -n "$SOURCE_NS" get secret "$SOURCE_SECRET" \
    -o jsonpath="{.data.$key}" 2>/dev/null | base64 -d || echo "")
  if [ -z "$value" ]; then
    echo "ERROR: K8s Secret に key '$key' が無い or 空"
    unset VAULT_TOKEN
    exit 1
  fi
  DATA_JSON=$(echo "$DATA_JSON" | jq --arg k "$key" --arg v "$value" '.data[$k] = $v')
  echo "    $key: <hidden, ${#value} chars>"
done

# === Vault に PUT ===
echo ""
echo "==> PUT to $VAULT_ADDR/v1/$VAULT_PATH"
# KV v2 は path に "data" を挿入する: secret/foo → secret/data/foo
KV_API_PATH=$(echo "$VAULT_PATH" | sed 's|^\([^/]*\)/|\1/data/|')
RESP=$(curl -sf --max-time 10 \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  -H "Content-Type: application/json" \
  -X POST \
  -d "$DATA_JSON" \
  "$VAULT_ADDR/v1/$KV_API_PATH")

VERSION=$(echo "$RESP" | jq -r '.data.version')
if [ -z "$VERSION" ] || [ "$VERSION" = "null" ]; then
  echo "ERROR: PUT 失敗"
  echo "$RESP" | head -c 300
  unset VAULT_TOKEN
  exit 1
fi
echo "    OK (version: $VERSION)"

# === 確認 (key 一覧のみ、値は出さない) ===
echo ""
echo "==> Verify (key 一覧のみ)"
KEY_LIST=$(curl -sf --max-time 10 \
  -H "X-Vault-Token: $VAULT_TOKEN" \
  "$VAULT_ADDR/v1/$KV_API_PATH" | jq -r '.data.data | keys | join(", ")')
echo "    keys: $KEY_LIST"

# === cleanup ===
unset VAULT_TOKEN DATA_JSON

echo ""
echo "==> Done."
echo ""
echo "==> 次のステップ:"
echo "    1. ESO の ExternalSecret CR を target ns に置く"
echo "       (target.name = $SOURCE_SECRET で生成すれば既存と同名で切替)"
echo "    2. VCO で grafana-read 等の Vault Policy / KubernetesAuthEngineRole を投入"
echo "    3. 既存 SealedSecret YAML を git から削除して PR"
echo "    4. ArgoCD sync で SealedSecret 削除 + ESO 投入が同時に走る"
