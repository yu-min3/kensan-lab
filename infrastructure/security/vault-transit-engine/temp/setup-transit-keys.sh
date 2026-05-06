#!/usr/bin/env bash
# ============================================================================
# Vault Transit key bootstrap (Stage 6, 1 度きり)
# ============================================================================
#
# なぜ手動?
#   redhat-cop/vault-config-operator は 2026-05 時点で TransitSecretEngine CR を
#   持たない。mount + policy + auth role は GitOps、key の create / rotate /
#   config だけ手動の hybrid 構成。
#
# 実行タイミング
#   PR vault-stage6-transit/vco-config が merge され、ArgoCD で
#   `vault-transit-engine-shared` が Healthy になった後 1 度だけ。
#
# 実行方法 (Mac から)
#   ./infrastructure/security/vault-transit-engine/temp/setup-transit-keys.sh
#
# 前提
#   - kubectl context が kensan-lab cluster
#   - bw login 済み (BW_SESSION 未設定なら本 script が unlock を促す)
#   - Bitwarden item: "kensan-lab/vault/root-token"
#   - vault CLI install (brew install vault)
#
# 何が起きるか
#   1. bw から root token 取得
#   2. vault svc に port-forward (8200, バックグラウンド)
#   3. transit/keys/users-name を作成 (既存なら skip)
#   4. 確認 (vault read transit/keys/users-name)
#   5. port-forward 停止 + VAULT_TOKEN unset
#
# 冪等性: key が既に存在する場合は skip する (再実行可)。
#
# Key rotation (運用)
#   vault write -f transit/keys/users-name/rotate

set -euo pipefail

KEY_NAME="${KEY_NAME:-users-name}"
BW_ROOT_TOKEN_ITEM="kensan-lab/vault/root-token"
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
for cmd in kubectl bw vault jq; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd not found" >&2
    exit 1
  fi
done

# === bw unlock ===
if [[ -z "${BW_SESSION:-}" ]]; then
  echo "==> bw unlock (master password を入力)"
  BW_SESSION=$(bw unlock --raw)
  export BW_SESSION
fi

bw_status=$(bw status 2>/dev/null | jq -r '.status' 2>/dev/null || echo "unknown")
if [[ "$bw_status" != "unlocked" ]]; then
  echo "ERROR: bw が unlock 状態じゃない (status: $bw_status)" >&2
  exit 1
fi

# === root token ===
if ! bw get item "$BW_ROOT_TOKEN_ITEM" &>/dev/null; then
  echo "ERROR: Bitwarden item '$BW_ROOT_TOKEN_ITEM' が無い" >&2
  exit 1
fi
VAULT_TOKEN=$(bw get password "$BW_ROOT_TOKEN_ITEM")
export VAULT_TOKEN

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

if ! vault status >/dev/null 2>&1; then
  echo "ERROR: vault に到達できない ($VAULT_ADDR)" >&2
  exit 1
fi

# === key create (冪等) ===
if vault read "transit/keys/${KEY_NAME}" >/dev/null 2>&1; then
  echo "==> transit/keys/${KEY_NAME} 既に存在 → skip"
else
  echo "==> Creating Vault Transit key: ${KEY_NAME}"
  vault write -f "transit/keys/${KEY_NAME}" \
    type=aes256-gcm96 \
    exportable=false \
    allow_plaintext_backup=false \
    derived=false \
    convergent_encryption=false
fi

# === verify ===
echo
echo "==> Verify:"
vault read "transit/keys/${KEY_NAME}"

echo
echo "==> Done. Next: PR #285 (go-impl) を merge → user-service rollout."
