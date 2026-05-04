#!/bin/bash
# Keycloak realm + user + Vault OIDC client セットアップ
#
# 使い方: ./bootstrap/keycloak/setup.sh
#
# 何をするか:
#   1. realm "kensan" 作成
#   2. groups "platform-admin", "platform-dev" 作成
#   3. user "yu" (email: ymisaki00@gmail.com) 作成 → platform-admin に assign
#   4. OIDC client "vault" 作成 (Vault Stage 1 Bootstrap TF が使う)
#   5. client_secret + user password を Bitwarden に保存
#
# 前提:
#   - kubectl context が kensan-lab cluster
#   - Keycloak が platform-auth-prod ns で稼働中 (auth.platform.yu-min3.com)
#   - bw login + unlock 済み (BW_SESSION 設定済み)
#   - host 側に jq, openssl install 済み
#
# 設計メモ:
#   - Keycloak image (quay.io/keycloak/keycloak:23.0) には jq が無いので、
#     kcadm の JSON 出力は host 側に流して host の jq で処理する。
#   - kcadm は pod 内で `~/.keycloak/kcadm.config` に session を保存するので、
#     login は 1 回だけで以降の exec は同じ session を使える。
#
# 冪等性: 既に存在するリソースは skip。再実行しても安全。

set -euo pipefail

NS="platform-auth-prod"
KC_DEPLOY="deployment/keycloak"
REALM="kensan"
USER_NAME="yu"
USER_EMAIL="ymisaki00@gmail.com"
USER_FIRST="Yu"
USER_LAST="Misaki"
ADMIN_GROUP="platform-admin"
DEV_GROUP="platform-dev"
CLIENT_ID="vault"
VAULT_HOSTNAME="vault.platform.yu-min3.com"

BW_CLIENT_ITEM="kensan-lab/keycloak/oidc-client-vault"
BW_USER_ITEM="kensan-lab/keycloak/user-yu"

# === 前提チェック ===
for cmd in kubectl jq bw openssl; do
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

if ! kubectl -n "$NS" get "$KC_DEPLOY" &> /dev/null; then
  echo "ERROR: $KC_DEPLOY not found in namespace $NS"; exit 1
fi

echo "==> kubectl context: $(kubectl config current-context)"
echo "==> 続行する場合は Enter、違うなら Ctrl-C"
read -r

# === Bitwarden 既存 item チェック ===
if bw get item "$BW_CLIENT_ITEM" &> /dev/null; then
  echo "WARNING: Bitwarden に '$BW_CLIENT_ITEM' が既に存在"
  echo "  上書きしたい場合は manual で archive:"
  echo "    bw delete item \$(bw get item \"$BW_CLIENT_ITEM\" | jq -r .id)"
  echo ""
  echo "==> 続行する? (既存 secret はそのまま、新しい secret は表示するだけになる)"
  echo "    続行=Enter, 中止=Ctrl-C"
  read -r
fi

# === admin password 取得 ===
echo ""
echo "==> Pulling Keycloak admin password from K8s secret..."
ADMIN_PW=$(kubectl -n "$NS" get secret keycloak-secret \
  -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d)
if [ -z "$ADMIN_PW" ]; then
  echo "ERROR: KEYCLOAK_ADMIN_PASSWORD が secret から取れない"; exit 1
fi
echo "    admin password 取得 OK"

# === user password 生成 ===
USER_PW=$(openssl rand -base64 24)

# === kcadm helper ===
# pod 内の kcadm を直接呼ぶ。第1引数以降が kcadm.sh の引数。
# 出力 (JSON 等) は stdout でホスト側に届く → ホストの jq で処理する。
kcadm() {
  kubectl -n "$NS" exec -i "$KC_DEPLOY" -- /opt/keycloak/bin/kcadm.sh "$@"
}

# === kcadm login (session は ~/.keycloak/kcadm.config に保存される) ===
echo ""
echo "==> Logging in to kcadm..."
kcadm config credentials \
  --server http://localhost:8080 \
  --realm master \
  --user admin \
  --password "$ADMIN_PW" > /dev/null
echo "    OK"

# === Realm 作成 ===
echo ""
echo "==> Ensuring realm '$REALM'..."
if kcadm get "realms/$REALM" > /dev/null 2>&1; then
  echo "    realm '$REALM' already exists, skip"
else
  kcadm create realms \
    -s "realm=$REALM" \
    -s enabled=true \
    -s "displayName=Kensan Lab" \
    -s loginWithEmailAllowed=true \
    -s registrationAllowed=false > /dev/null
  echo "    realm '$REALM' created"
fi

# === Groups 作成 ===
ensure_group() {
  local g="$1"
  echo ""
  echo "==> Ensuring group '$g' in realm '$REALM'..."
  local existing
  existing=$(kcadm get groups -r "$REALM" --fields name 2>/dev/null \
    | jq -r --arg g "$g" '.[] | select(.name == $g) | .name' || true)
  if [ -n "$existing" ]; then
    echo "    group '$g' already exists, skip"
  else
    kcadm create groups -r "$REALM" -s "name=$g" > /dev/null
    echo "    group '$g' created"
  fi
}
ensure_group "$ADMIN_GROUP"
ensure_group "$DEV_GROUP"

# === User 作成 ===
echo ""
echo "==> Ensuring user '$USER_NAME' in realm '$REALM'..."
USER_ID=$(kcadm get users -r "$REALM" --query "username=$USER_NAME" --fields id 2>/dev/null \
  | jq -r '.[0].id // empty' || true)

if [ -n "$USER_ID" ] && [ "$USER_ID" != "null" ]; then
  echo "    user '$USER_NAME' already exists (id: $USER_ID), skip create"
  echo "    NOTE: パスワード再設定は手動で。BW に古い PW が残ってる可能性あり"
  USER_PW=""  # 既存の場合は BW に save しない
else
  kcadm create users -r "$REALM" \
    -s "username=$USER_NAME" \
    -s "email=$USER_EMAIL" \
    -s "firstName=$USER_FIRST" \
    -s "lastName=$USER_LAST" \
    -s emailVerified=true \
    -s enabled=true > /dev/null
  USER_ID=$(kcadm get users -r "$REALM" --query "username=$USER_NAME" --fields id \
    | jq -r '.[0].id')
  echo "    user '$USER_NAME' created (id: $USER_ID)"

  echo "==> Setting password (temporary=false)..."
  kcadm set-password -r "$REALM" \
    --username "$USER_NAME" \
    --new-password "$USER_PW" > /dev/null
  echo "    password set"

  echo "==> Joining user to group '$ADMIN_GROUP'..."
  GROUP_ID=$(kcadm get groups -r "$REALM" --fields id,name \
    | jq -r --arg g "$ADMIN_GROUP" '.[] | select(.name == $g) | .id')
  kcadm update "users/$USER_ID/groups/$GROUP_ID" -r "$REALM" \
    -s "realm=$REALM" -s "userId=$USER_ID" -s "groupId=$GROUP_ID" -n > /dev/null
  echo "    user joined to '$ADMIN_GROUP'"
fi

# === OIDC Client 共通関数 ===
# 引数: <client_id> <name> <description> <redirect_uris_json> <web_origins_json> <post_logout_uri>
# 戻り値: stdout に client UUID を 1 行で出力。
# 副作用: 存在しなければ作成し groups protocol mapper を付ける。冪等。
ensure_oidc_client() {
  local cid="$1"
  local cname="$2"
  local cdesc="$3"
  local redirect_uris_json="$4"
  local web_origins_json="$5"
  local post_logout_uri="$6"

  echo "" >&2
  echo "==> Ensuring OIDC client '$cid' in realm '$REALM'..." >&2
  local uuid
  uuid=$(kcadm get clients -r "$REALM" --query "clientId=$cid" --fields id 2>/dev/null \
    | jq -r '.[0].id // empty' || true)

  if [ -n "$uuid" ] && [ "$uuid" != "null" ]; then
    echo "    client '$cid' already exists (id: $uuid), skip create" >&2
  else
    kcadm create clients -r "$REALM" \
      -s "clientId=$cid" \
      -s "name=$cname" \
      -s "description=$cdesc" \
      -s enabled=true \
      -s protocol=openid-connect \
      -s publicClient=false \
      -s standardFlowEnabled=true \
      -s directAccessGrantsEnabled=false \
      -s "redirectUris=$redirect_uris_json" \
      -s "webOrigins=$web_origins_json" \
      -s "attributes.\"post.logout.redirect.uris\"=$post_logout_uri" > /dev/null
    uuid=$(kcadm get clients -r "$REALM" --query "clientId=$cid" --fields id \
      | jq -r '.[0].id')
    echo "    client '$cid' created (id: $uuid)" >&2

    echo "==> Adding 'groups' protocol mapper to '$cid'..." >&2
    kcadm create "clients/$uuid/protocol-mappers/models" -r "$REALM" \
      -s name=groups \
      -s protocol=openid-connect \
      -s protocolMapper=oidc-group-membership-mapper \
      -s 'config."claim.name"=groups' \
      -s 'config."full.path"=false' \
      -s 'config."id.token.claim"=true' \
      -s 'config."access.token.claim"=true' \
      -s 'config."userinfo.token.claim"=true' > /dev/null
    echo "    groups mapper added" >&2
  fi

  echo "$uuid"
}

# === Vault OIDC client ===
VAULT_REDIRECT_URIS=$(jq -nc \
  --arg vault_cb "https://$VAULT_HOSTNAME/ui/vault/auth/oidc/oidc/callback" \
  --arg cli_cb "http://localhost:8250/oidc/callback" \
  '[$vault_cb, $cli_cb]')
VAULT_WEB_ORIGINS=$(jq -nc --arg origin "https://$VAULT_HOSTNAME" '[$origin]')

CLIENT_UUID=$(ensure_oidc_client \
  "$CLIENT_ID" \
  "Vault OIDC" \
  "Vault HA OIDC auth method (Stage 1)" \
  "$VAULT_REDIRECT_URIS" \
  "$VAULT_WEB_ORIGINS" \
  "https://$VAULT_HOSTNAME")

# === Istio Gateway OIDC client (Phase 1) — ADR-010 で Path A 採択 ===
# oauth2-proxy 経由で gateway-platform 配下の全 host を保護する。
# redirectUris は oauth2-proxy 自身の callback URL 1 本のみ
# (cookie domain .platform.yu-min3.com で全 host SSO 化、各 host への
# 個別 redirectUris 登録は不要)。
OAUTH2_PROXY_HOST="oauth2-proxy.platform.yu-min3.com"
GATEWAY_REDIRECT_URIS=$(jq -nc \
  --arg cb "https://$OAUTH2_PROXY_HOST/oauth2/callback" \
  '[$cb]')
GATEWAY_WEB_ORIGINS=$(jq -nc --arg origin "https://$OAUTH2_PROXY_HOST" '[$origin]')
GATEWAY_CLIENT_UUID=$(ensure_oidc_client \
  "istio-gateway-platform" \
  "Istio Gateway Platform OIDC" \
  "Phase 1 Gateway-level OIDC for *.platform.yu-min3.com (oauth2-proxy)" \
  "$GATEWAY_REDIRECT_URIS" \
  "$GATEWAY_WEB_ORIGINS" \
  "https://$OAUTH2_PROXY_HOST")

# === Vault client_secret 取得 ===
echo ""
echo "==> Fetching Vault client_secret..."
CLIENT_SECRET=$(kcadm get "clients/$CLIENT_UUID/client-secret" -r "$REALM" 2>/dev/null \
  | jq -r '.value')
if [ -z "$CLIENT_SECRET" ] || [ "$CLIENT_SECRET" = "null" ]; then
  echo "ERROR: client_secret が取れない"; exit 1
fi
echo "    client_secret 取得 OK (${CLIENT_SECRET:0:8}...)"

# === Bitwarden 保存 ===
save_bw() {
  local item_name="$1"
  local username="$2"
  local password="$3"
  local notes="$4"

  if bw get item "$item_name" &> /dev/null; then
    echo "    skip: '$item_name' は既に Bitwarden にある"
    return 0
  fi

  local item_json
  item_json=$(bw get template item | jq \
    --arg name "$item_name" \
    --arg notes "$notes" \
    --arg user "$username" \
    --arg pw "$password" \
    '.name = $name
     | .notes = $notes
     | .login = {uris: [], username: $user, password: $pw, totp: null}
     | .fields = []')

  echo "$item_json" | bw encode | bw create item > /dev/null
  echo "    saved: '$item_name'"
}

echo ""
echo "==> Saving secrets to Bitwarden..."

CLIENT_NOTES="Vault Stage 1 OIDC client.
Realm: $REALM
ClientID: $CLIENT_ID
DiscoveryURL: https://auth.platform.yu-min3.com/realms/$REALM
Created by: bootstrap/keycloak/setup.sh"
save_bw "$BW_CLIENT_ITEM" "$CLIENT_ID" "$CLIENT_SECRET" "$CLIENT_NOTES"

# === Istio Gateway client_secret 取得 + Bitwarden 保存 ===
echo ""
echo "==> Fetching istio-gateway-platform client_secret..."
GATEWAY_CLIENT_SECRET=$(kcadm get "clients/$GATEWAY_CLIENT_UUID/client-secret" -r "$REALM" 2>/dev/null \
  | jq -r '.value')
if [ -z "$GATEWAY_CLIENT_SECRET" ] || [ "$GATEWAY_CLIENT_SECRET" = "null" ]; then
  echo "ERROR: istio-gateway-platform client_secret が取れない"; exit 1
fi
echo "    gateway client_secret 取得 OK (${GATEWAY_CLIENT_SECRET:0:8}...)"

GATEWAY_NOTES="Istio Gateway Platform OIDC client (Phase 1 Gateway-level auth).
Realm: $REALM
ClientID: istio-gateway-platform
DiscoveryURL: https://auth.platform.yu-min3.com/realms/$REALM
Used by: oauth2-proxy in auth-system namespace
Created by: bootstrap/keycloak/setup.sh"
save_bw "kensan-lab/keycloak/oidc-client-istio-gateway-platform" \
  "istio-gateway-platform" "$GATEWAY_CLIENT_SECRET" "$GATEWAY_NOTES"

if [ -n "${USER_PW:-}" ]; then
  USER_NOTES="Keycloak user for kensan realm.
Realm: $REALM
Email: $USER_EMAIL
Group: $ADMIN_GROUP
Login URL: https://auth.platform.yu-min3.com/realms/$REALM/account
Created by: bootstrap/keycloak/setup.sh"
  save_bw "$BW_USER_ITEM" "$USER_NAME" "$USER_PW" "$USER_NOTES"
fi

# === 出力 ===
echo ""
echo "==> Done."
echo ""
echo "==> 確認:"
echo "    OIDC discovery: https://auth.platform.yu-min3.com/realms/$REALM/.well-known/openid-configuration"
echo "    bw get item \"$BW_CLIENT_ITEM\" | jq '{name, login: {username: .login.username}}'"
echo ""
echo "==> 次のステップ (Vault Stage 1):"
echo "    1. vault operator init (manual)"
echo "    2. root token + Recovery Keys を Bitwarden に保存"
echo "    3. bootstrap/vault/ で terraform.tfvars に下記を書く:"
echo "       keycloak_oidc_discovery_url = \"https://auth.platform.yu-min3.com/realms/$REALM\""
echo "       keycloak_oidc_client_id     = \"$CLIENT_ID\""
echo "       keycloak_oidc_client_secret = \"<bw get item $BW_CLIENT_ITEM | jq -r .login.password>\""
echo "    4. cd bootstrap/vault && terraform init && terraform apply"
echo ""
echo "==> 次のステップ (Gateway OIDC / oauth2-proxy):"
echo "    1. cookie_secret を生成: COOKIE=\$(openssl rand -base64 32 | tr -d '\\n')"
echo "    2. Vault に投入:"
echo "       vault kv put secret/platform-auth/istio-gateway/platform \\"
echo "         client-id=istio-gateway-platform \\"
echo "         client-secret=\$(bw get item kensan-lab/keycloak/oidc-client-istio-gateway-platform | jq -r .login.password) \\"
echo "         cookie-secret=\"\$COOKIE\""
echo "    3. ArgoCD で oauth2-proxy Application を sync (auto sync 有効なら自動)"
echo "    4. ExternalSecret 経由で auth-system/oauth2-proxy-secret 作成確認"
