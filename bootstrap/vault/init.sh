#!/bin/bash
# Vault Stage 1 初期化 + Bootstrap TF tfvars 生成
#
# 使い方: ./bootstrap/vault/init.sh
#
# 何をするか:
#   1. vault operator init (一度だけ。既に initialized なら skip)
#   2. Recovery Keys + root token を Bitwarden 保存
#   3. emergency_admin password 生成 + Bitwarden 保存
#   4. unsealed 確認 (3 pod 全部 Ready, auto-unseal 経由)
#   5. terraform.tfvars 生成 (Keycloak client_secret は bw から取得)
#
# terraform apply は手動 (plan 確認のため):
#   kubectl -n vault port-forward svc/vault 8200:8200 &
#   cd bootstrap/vault && terraform init && terraform plan && terraform apply
#
# 前提:
#   - kubectl context が kensan-lab cluster
#   - vault-0/1/2 が 1/2 Running (Sealed=true、init 待ち)
#   - bw login + unlock 済み (BW_SESSION 設定済み)
#   - bootstrap/keycloak/setup.sh 実行済み (kensan-lab/keycloak/oidc-client-vault 存在)
#
# セキュリティ:
#   - root token / Recovery Keys は -format=json で temp file に書き、変数経由で Bitwarden に流す
#   - stdout に機密値を echo しない
#   - trap で temp file を確実に削除 (Bitwarden 保存失敗時のみ温存して手動回収可能に)
#
# 冪等性: 既に initialized なら init を skip し tfvars だけ再生成する。再実行可。

set -euo pipefail

NS="vault"
RECOVERY_SHARES=5
RECOVERY_THRESHOLD=3

BW_ROOT_TOKEN_ITEM="kensan-lab/vault/root-token"
BW_RECOVERY_KEYS_ITEM="kensan-lab/vault/recovery-keys"
BW_EMERGENCY_ADMIN_ITEM="kensan-lab/vault/emergency-admin"
BW_KEYCLOAK_CLIENT_ITEM="kensan-lab/keycloak/oidc-client-vault"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TFVARS_FILE="$SCRIPT_DIR/terraform.tfvars"

# === 前提チェック ===
for cmd in kubectl bw jq openssl terraform; do
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

if ! kubectl -n "$NS" get pod vault-0 &> /dev/null; then
  echo "ERROR: vault-0 pod が無い。Vault が deploy されてるか確認"
  exit 1
fi

if ! bw get item "$BW_KEYCLOAK_CLIENT_ITEM" &> /dev/null; then
  echo "ERROR: '$BW_KEYCLOAK_CLIENT_ITEM' が Bitwarden に無い"
  echo "  → bootstrap/keycloak/setup.sh を先に実行してね"
  exit 1
fi

echo "==> kubectl context: $(kubectl config current-context)"
echo "==> 続行する場合は Enter、違うなら Ctrl-C"
read -r

# === Bitwarden 保存 helper ===
bw_save() {
  local name="$1"
  local username="$2"
  local password="$3"
  local notes="$4"

  bw get template item | jq \
    --arg name "$name" \
    --arg notes "$notes" \
    --arg user "$username" \
    --arg pw "$password" \
    '.name = $name
     | .notes = $notes
     | .login = {uris: [], username: $user, password: $pw, totp: null}
     | .fields = []' \
    | bw encode | bw create item > /dev/null
}

# === Vault initialized 状態判定 ===
echo ""
echo "==> Checking Vault initialized state..."
INIT_STATE=$(kubectl -n "$NS" exec vault-0 -c vault -- vault status -format=json 2>/dev/null \
  | jq -r '.initialized' 2>/dev/null || echo "unknown")
echo "    initialized: $INIT_STATE"

ROOT_TOKEN=""

if [ "$INIT_STATE" = "true" ]; then
  echo "==> Vault は既に initialized。Bitwarden から root token を読み込む"
  if ! bw get item "$BW_ROOT_TOKEN_ITEM" &> /dev/null; then
    echo "ERROR: Vault は initialized だが Bitwarden の '$BW_ROOT_TOKEN_ITEM' は無い。"
    echo "  → 過去 init した時の token がどこかにあるはず。手動で対応必要"
    exit 1
  fi
  ROOT_TOKEN=$(bw get item "$BW_ROOT_TOKEN_ITEM" | jq -r '.login.password')
  echo "    root token 取得 OK"
else
  # === Bitwarden 既存 item チェック (init 前なのに既存ならズレてる) ===
  for item in "$BW_ROOT_TOKEN_ITEM" "$BW_RECOVERY_KEYS_ITEM"; do
    if bw get item "$item" &> /dev/null; then
      echo "ERROR: Vault は uninitialized だが Bitwarden に '$item' が既に存在"
      echo "  → 整合性ズレ。手動で確認・clean してから再実行"
      exit 1
    fi
  done

  # === 初期化 ===
  echo ""
  echo "==> Initializing Vault (一度だけ。出力は機密、stdout に残さない)..."
  TEMP_OUT=$(mktemp)
  # default の trap: 正常終了でも異常終了でも temp file 削除
  trap "rm -f $TEMP_OUT" EXIT

  kubectl -n "$NS" exec vault-0 -c vault -- vault operator init \
    -recovery-shares="$RECOVERY_SHARES" \
    -recovery-threshold="$RECOVERY_THRESHOLD" \
    -format=json > "$TEMP_OUT"

  ROOT_TOKEN=$(jq -r '.root_token' < "$TEMP_OUT")
  RECOVERY_KEYS=$(jq -r '.recovery_keys_b64 | join("\n")' < "$TEMP_OUT")

  if [ -z "$ROOT_TOKEN" ] || [ "$ROOT_TOKEN" = "null" ]; then
    echo "ERROR: root token 抽出失敗。出力 → $TEMP_OUT に温存 (要手動回収、回収後 rm)"
    trap - EXIT  # 削除しない
    exit 1
  fi

  echo "    initialized OK"

  # === Bitwarden 保存 (失敗したら temp file 温存して手動回収を促す) ===
  echo ""
  echo "==> Saving root token to Bitwarden..."
  if ! bw_save "$BW_ROOT_TOKEN_ITEM" "root" "$ROOT_TOKEN" \
      "Vault Stage 1 root token. Created by bootstrap/vault/init.sh on $(date -Iseconds)"; then
    echo "ERROR: Bitwarden 保存失敗。出力 → $TEMP_OUT に温存 (要手動回収、回収後 rm)"
    trap - EXIT
    exit 1
  fi
  echo "    saved: $BW_ROOT_TOKEN_ITEM"

  echo ""
  echo "==> Saving Recovery Keys to Bitwarden..."
  RECOVERY_NOTES="Vault Stage 1 Recovery Keys (auto-unseal 失敗時に必要、threshold=$RECOVERY_THRESHOLD/$RECOVERY_SHARES)

$RECOVERY_KEYS

Generated by bootstrap/vault/init.sh on $(date -Iseconds)"
  if ! bw_save "$BW_RECOVERY_KEYS_ITEM" "" "" "$RECOVERY_NOTES"; then
    echo "ERROR: Recovery keys 保存失敗。出力 → $TEMP_OUT に温存 (要手動回収、回収後 rm)"
    trap - EXIT
    exit 1
  fi
  echo "    saved: $BW_RECOVERY_KEYS_ITEM"
fi

# === Emergency admin password ===
echo ""
echo "==> Ensuring emergency admin password..."
EMERGENCY_PW=""
if bw get item "$BW_EMERGENCY_ADMIN_ITEM" &> /dev/null; then
  echo "    skip: $BW_EMERGENCY_ADMIN_ITEM 既に存在 (既存パスワードを再利用)"
  EMERGENCY_PW=$(bw get item "$BW_EMERGENCY_ADMIN_ITEM" | jq -r '.login.password')
else
  EMERGENCY_PW=$(openssl rand -base64 32)
  bw_save "$BW_EMERGENCY_ADMIN_ITEM" "emergency-admin" "$EMERGENCY_PW" \
    "Vault userpass break-glass admin. Bootstrap TF が auth/userpass/users/emergency-admin として登録。
Created by bootstrap/vault/init.sh on $(date -Iseconds)"
  echo "    saved: $BW_EMERGENCY_ADMIN_ITEM"
fi

# === Pod Ready 待機 (auto-unseal 経由で全 pod が 2/2 Ready になる) ===
echo ""
echo "==> Waiting for vault-0/1/2 to be 2/2 Ready (auto-unseal 経由)..."
kubectl -n "$NS" wait --for=condition=Ready pod -l app.kubernetes.io/name=vault --timeout=180s || {
  echo "WARN: Ready 待機 timeout。手動で確認:"
  echo "  kubectl -n $NS get pod"
  echo "  kubectl -n $NS exec vault-0 -c vault -- vault status"
}
echo ""
kubectl -n "$NS" get pod

# === terraform.tfvars 生成 ===
echo ""
echo "==> Generating $TFVARS_FILE..."
KEYCLOAK_CLIENT_SECRET=$(bw get item "$BW_KEYCLOAK_CLIENT_ITEM" | jq -r '.login.password')

cat > "$TFVARS_FILE" <<EOF
# Generated by bootstrap/vault/init.sh on $(date -Iseconds)
# このファイルは .gitignore 済み。secret を含むので絶対 commit しないこと。
#
# Vault は port-forward 前提 (cluster 外から CLI で apply するため):
#   kubectl -n vault port-forward svc/vault 8200:8200 &
vault_address = "http://127.0.0.1:8200"
vault_token   = "$ROOT_TOKEN"

keycloak_realm_url = "https://auth.platform.yu-min3.com/realms/kensan"
keycloak_oidc_client_id     = "vault"
keycloak_oidc_client_secret = "$KEYCLOAK_CLIENT_SECRET"

emergency_admin_password = "$EMERGENCY_PW"
EOF
chmod 600 "$TFVARS_FILE"
echo "    saved: $TFVARS_FILE (mode 600, gitignored)"

# === 次の手順案内 ===
echo ""
echo "==> Done."
echo ""
echo "==> 次のステップ:"
echo "    1. port-forward を起動 (別 terminal):"
echo "       kubectl -n vault port-forward svc/vault 8200:8200"
echo ""
echo "    2. terraform plan で変更内容確認:"
echo "       cd bootstrap/vault && terraform init && terraform plan"
echo ""
echo "    3. terraform apply で設定反映:"
echo "       terraform apply"
echo ""
echo "    4. State 廃棄 (Pattern A': bootstrap 後は VCO に引き継ぐ):"
echo "       rm -f terraform.tfstate* terraform.tfvars"
echo ""
echo "    5. Bitwarden 確認:"
echo "       bw get item \"$BW_ROOT_TOKEN_ITEM\" | jq '{name}'"
echo "       bw get item \"$BW_RECOVERY_KEYS_ITEM\" | jq '{name}'"
echo "       bw get item \"$BW_EMERGENCY_ADMIN_ITEM\" | jq '{name}'"
