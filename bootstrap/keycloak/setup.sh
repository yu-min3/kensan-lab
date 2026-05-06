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
    echo "    client '$cid' already exists (id: $uuid)" >&2
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

  # script を SSoT として、redirectUris / webOrigins / post-logout を毎回 update。
  # Keycloak UI で手動編集した内容は上書きされる (script が正)。
  echo "    syncing redirectUris / webOrigins / post-logout (script is SSoT)" >&2
  kcadm update "clients/$uuid" -r "$REALM" \
    -s "redirectUris=$redirect_uris_json" \
    -s "webOrigins=$web_origins_json" \
    -s "attributes.\"post.logout.redirect.uris\"=$post_logout_uri" > /dev/null

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

# === Istio Gateway OIDC client (Phase 2) — ADR-010 Path A + AuthZ binding ===
# oauth2-proxy が Keycloak へ redirect する際の redirect_uri は、
# request 元 host の `/oauth2/callback` になる (cookie_domain で
# .platform.yu-min3.com 横断で session 共有しつつ、callback は host ごと)。
# したがって全 protected host を Keycloak に登録する必要がある。
# 新 protected host を追加するときは `GATEWAY_PROTECTED_HOSTS` に追記して
# script を再実行する (ensure_oidc_client が既存 client を update する)。
GATEWAY_PROTECTED_HOSTS=(
  backstage.platform.yu-min3.com
  prometheus.platform.yu-min3.com
  hubble.platform.yu-min3.com
  longhorn.platform.yu-min3.com
)
# Grafana は auth.generic_oauth で Keycloak 直結 (Path B)。Authorization Bearer
# 解釈の衝突を避けるため gateway-platform は bypass、別 client で OIDC する。
GATEWAY_REDIRECT_URIS=$(printf '%s\n' "${GATEWAY_PROTECTED_HOSTS[@]}" \
  | jq -Rnc '[inputs | "https://\(.)/oauth2/callback"]')
GATEWAY_WEB_ORIGINS=$(printf '%s\n' "${GATEWAY_PROTECTED_HOSTS[@]}" \
  | jq -Rnc '[inputs | "https://\(.)"]')
# post-logout は "+" で redirectUris を継承する Keycloak 特殊記法
GATEWAY_CLIENT_UUID=$(ensure_oidc_client \
  "istio-gateway-platform" \
  "Istio Gateway Platform OIDC" \
  "Gateway-level OIDC for *.platform.yu-min3.com (oauth2-proxy ext_authz)" \
  "$GATEWAY_REDIRECT_URIS" \
  "$GATEWAY_WEB_ORIGINS" \
  "+")

# === Grafana OIDC client (Path B — Grafana 自前 OIDC) ===
# Grafana auth.generic_oauth で直接 Keycloak と connect するための専用 client。
# gateway-platform 経由で Authorization Bearer を Grafana に渡すと内部 API key
# として誤解釈されて 403 になる問題を避けるため、Grafana は AuthorizationPolicy
# Category 1 で bypass し、自前 client で OIDC させる。
# redirect_uri: Grafana の generic_oauth handler の固定 path /login/generic_oauth
GRAFANA_HOSTNAME="grafana.platform.yu-min3.com"
GRAFANA_REDIRECT_URIS=$(jq -nc \
  --arg cb "https://$GRAFANA_HOSTNAME/login/generic_oauth" \
  '[$cb]')
GRAFANA_WEB_ORIGINS=$(jq -nc --arg origin "https://$GRAFANA_HOSTNAME" '[$origin]')
GRAFANA_CLIENT_UUID=$(ensure_oidc_client \
  "grafana" \
  "Grafana OIDC" \
  "Grafana auth.generic_oauth (Path B — gateway-platform bypass)" \
  "$GRAFANA_REDIRECT_URIS" \
  "$GRAFANA_WEB_ORIGINS" \
  "https://$GRAFANA_HOSTNAME/login")

# === ArgoCD OIDC client (Path B — ArgoCD 自前 OIDC) ===
# ArgoCD は argocd-cm の oidc.config で直接 Keycloak と connect する。
# Vault / ArgoCD は Category 1 (bypass) で扱い、app native auth に任せる方針通り。
# redirect_uri: ArgoCD の OIDC callback path /auth/callback (UI), /pkce/verify (CLI)
ARGOCD_HOSTNAME="argocd.platform.yu-min3.com"
ARGOCD_REDIRECT_URIS=$(jq -nc \
  --arg ui_cb "https://$ARGOCD_HOSTNAME/auth/callback" \
  --arg cli_cb "http://localhost:8085/auth/callback" \
  --arg pkce_cb "https://$ARGOCD_HOSTNAME/pkce/verify" \
  '[$ui_cb, $cli_cb, $pkce_cb]')
ARGOCD_WEB_ORIGINS=$(jq -nc --arg origin "https://$ARGOCD_HOSTNAME" '[$origin]')
ARGOCD_CLIENT_UUID=$(ensure_oidc_client \
  "argocd" \
  "ArgoCD OIDC" \
  "ArgoCD argocd-cm.oidc.config (Path B — gateway-platform bypass)" \
  "$ARGOCD_REDIRECT_URIS" \
  "$ARGOCD_WEB_ORIGINS" \
  "https://$ARGOCD_HOSTNAME")

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

# === Grafana client_secret 取得 + Bitwarden 保存 ===
echo ""
echo "==> Fetching grafana client_secret..."
GRAFANA_CLIENT_SECRET=$(kcadm get "clients/$GRAFANA_CLIENT_UUID/client-secret" -r "$REALM" 2>/dev/null \
  | jq -r '.value')
if [ -z "$GRAFANA_CLIENT_SECRET" ] || [ "$GRAFANA_CLIENT_SECRET" = "null" ]; then
  echo "ERROR: grafana client_secret が取れない"; exit 1
fi
echo "    grafana client_secret 取得 OK (${GRAFANA_CLIENT_SECRET:0:8}...)"

GRAFANA_NOTES="Grafana OIDC client (Path B — gateway bypass + Grafana 自前 generic_oauth).
Realm: $REALM
ClientID: grafana
DiscoveryURL: https://auth.platform.yu-min3.com/realms/$REALM
Used by: Grafana in monitoring namespace (auth.generic_oauth)
Created by: bootstrap/keycloak/setup.sh"
save_bw "kensan-lab/keycloak/oidc-client-grafana" \
  "grafana" "$GRAFANA_CLIENT_SECRET" "$GRAFANA_NOTES"

# === ArgoCD client_secret 取得 + Bitwarden 保存 ===
echo ""
echo "==> Fetching argocd client_secret..."
ARGOCD_CLIENT_SECRET=$(kcadm get "clients/$ARGOCD_CLIENT_UUID/client-secret" -r "$REALM" 2>/dev/null \
  | jq -r '.value')
if [ -z "$ARGOCD_CLIENT_SECRET" ] || [ "$ARGOCD_CLIENT_SECRET" = "null" ]; then
  echo "ERROR: argocd client_secret が取れない"; exit 1
fi
echo "    argocd client_secret 取得 OK (${ARGOCD_CLIENT_SECRET:0:8}...)"

ARGOCD_NOTES="ArgoCD OIDC client (Path B — gateway bypass + argocd-cm.oidc.config 直結).
Realm: $REALM
ClientID: argocd
DiscoveryURL: https://auth.platform.yu-min3.com/realms/$REALM
Used by: ArgoCD in argocd namespace (configs.cm.oidc.config)
Created by: bootstrap/keycloak/setup.sh"
save_bw "kensan-lab/keycloak/oidc-client-argocd" \
  "argocd" "$ARGOCD_CLIENT_SECRET" "$ARGOCD_NOTES"

if [ -n "${USER_PW:-}" ]; then
  USER_NOTES="Keycloak user for kensan realm.
Realm: $REALM
Email: $USER_EMAIL
Group: $ADMIN_GROUP
Login URL: https://auth.platform.yu-min3.com/realms/$REALM/account
Created by: bootstrap/keycloak/setup.sh"
  save_bw "$BW_USER_ITEM" "$USER_NAME" "$USER_PW" "$USER_NOTES"
fi

# === Vault KV 投入 (oauth2-proxy 用) ===
# vault CLI + 有効 token があれば自動投入。なければ手動手順を案内。
# KV path: secret/platform-auth/istio-gateway/platform
# 投入 keys: client-id / client-secret / cookie-secret
# 既存値があればそれを尊重 (再実行で session を破壊しない)。
GATEWAY_VAULT_PATH="secret/platform-auth/istio-gateway/platform"   # KV v2 は mount prefix 必須

vault_populate_gateway_kv() {
  echo ""
  echo "==> Populating Vault KV: $GATEWAY_VAULT_PATH"

  # 既存 KV 取得 (失敗しても気にしない)
  local existing_keys=""
  local existing_cookie=""
  if vault kv get -format=json "$GATEWAY_VAULT_PATH" >/dev/null 2>&1; then
    existing_keys=$(vault kv get -format=json "$GATEWAY_VAULT_PATH" \
      | jq -r '.data.data | keys | join(",")')
    existing_cookie=$(vault kv get -format=json "$GATEWAY_VAULT_PATH" \
      | jq -r '.data.data["cookie-secret"] // empty')
    echo "    既存 KV あり (keys: $existing_keys)"
  else
    echo "    KV path 未存在、新規作成"
  fi

  # cookie-secret は既存があれば流用 (rotate したい場合は事前に kv delete する)
  local cookie
  if [ -n "$existing_cookie" ]; then
    cookie="$existing_cookie"
    echo "    cookie-secret: 既存値を流用"
  else
    # oauth2-proxy は AES key として 16/24/32 byte を要求。先に URL-safe base64
    # decode を試み、長さ一致したら raw bytes として使う動作のため、URL-safe
    # アルファベット (-_) で出力する必要がある。標準アルファベット (+/) だと
    # decode 失敗 → 入力文字列 (44 byte) が raw 扱いされ AES key 長違反で起動失敗。
    cookie=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=' | tr -d '\n')
    echo "    cookie-secret: 新規生成 (URL-safe base64)"
  fi

  vault kv put "$GATEWAY_VAULT_PATH" \
    client-id=istio-gateway-platform \
    client-secret="$GATEWAY_CLIENT_SECRET" \
    cookie-secret="$cookie" > /dev/null
  echo "    populated OK"

  echo ""
  echo "    確認 (keys のみ):"
  vault kv get -format=json "$GATEWAY_VAULT_PATH" | jq -r '.data.data | keys | "      " + (. | tostring)'
}

# === Vault KV 投入 (Grafana auth.generic_oauth 用) ===
# KV path: secret/observability/grafana/oidc
# 投入 keys: client-id / client-secret
# ESO の external-secrets policy が secret/data/* read 可能なので追加 policy 不要。
GRAFANA_OIDC_VAULT_PATH="secret/observability/grafana/oidc"

vault_populate_grafana_oidc_kv() {
  echo ""
  echo "==> Populating Vault KV: $GRAFANA_OIDC_VAULT_PATH"

  if vault kv get -format=json "$GRAFANA_OIDC_VAULT_PATH" >/dev/null 2>&1; then
    local existing_keys
    existing_keys=$(vault kv get -format=json "$GRAFANA_OIDC_VAULT_PATH" \
      | jq -r '.data.data | keys | join(",")')
    echo "    既存 KV あり (keys: $existing_keys) — 上書き"
  else
    echo "    KV path 未存在、新規作成"
  fi

  vault kv put "$GRAFANA_OIDC_VAULT_PATH" \
    client-id=grafana \
    client-secret="$GRAFANA_CLIENT_SECRET" > /dev/null
  echo "    populated OK"
}

# === Vault KV 投入 (ArgoCD argocd-cm.oidc.config 用) ===
# KV path: secret/security/argocd/oidc
# 投入 keys: client-id / client-secret
# ESO の external-secrets-read policy で read 可能、追加 policy 不要。
ARGOCD_OIDC_VAULT_PATH="secret/security/argocd/oidc"

vault_populate_argocd_oidc_kv() {
  echo ""
  echo "==> Populating Vault KV: $ARGOCD_OIDC_VAULT_PATH"

  if vault kv get -format=json "$ARGOCD_OIDC_VAULT_PATH" >/dev/null 2>&1; then
    local existing_keys
    existing_keys=$(vault kv get -format=json "$ARGOCD_OIDC_VAULT_PATH" \
      | jq -r '.data.data | keys | join(",")')
    echo "    既存 KV あり (keys: $existing_keys) — 上書き"
  else
    echo "    KV path 未存在、新規作成"
  fi

  vault kv put "$ARGOCD_OIDC_VAULT_PATH" \
    client-id=argocd \
    client-secret="$ARGOCD_CLIENT_SECRET" > /dev/null
  echo "    populated OK"
}

print_gateway_vault_manual() {
  cat <<MSG

==> Vault 自動投入をスキップ ($1)。手動投入手順:

    # 0. 必要なら Vault に login
    export VAULT_ADDR=https://vault.platform.yu-min3.com
    vault login -method=oidc

    # 1. 各 secret 用意
    COOKIE=\$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=' | tr -d '\\n')   # URL-safe
    CSEC=\$(bw get item kensan-lab/keycloak/oidc-client-istio-gateway-platform | jq -r .login.password)
    GSEC=\$(bw get item kensan-lab/keycloak/oidc-client-grafana | jq -r .login.password)
    ASEC=\$(bw get item kensan-lab/keycloak/oidc-client-argocd | jq -r .login.password)

    # 2. Vault に投入 (Gateway oauth2-proxy 用)
    vault kv put $GATEWAY_VAULT_PATH \\
      client-id=istio-gateway-platform \\
      client-secret="\$CSEC" \\
      cookie-secret="\$COOKIE"

    # 2'. Vault に投入 (Grafana 自前 OIDC 用)
    vault kv put $GRAFANA_OIDC_VAULT_PATH \\
      client-id=grafana \\
      client-secret="\$GSEC"

    # 2''. Vault に投入 (ArgoCD 自前 OIDC 用)
    vault kv put $ARGOCD_OIDC_VAULT_PATH \\
      client-id=argocd \\
      client-secret="\$ASEC"

    # 3. 確認
    vault kv get -format=json $GATEWAY_VAULT_PATH | jq '.data.data | keys'
    vault kv get -format=json $GRAFANA_OIDC_VAULT_PATH | jq '.data.data | keys'
    vault kv get -format=json $ARGOCD_OIDC_VAULT_PATH | jq '.data.data | keys'
MSG
}

if ! command -v vault &>/dev/null; then
  print_gateway_vault_manual "vault CLI なし"
elif [ -z "${VAULT_ADDR:-}" ]; then
  print_gateway_vault_manual "VAULT_ADDR 未設定"
elif ! vault token lookup &>/dev/null; then
  print_gateway_vault_manual "vault token 無効"
else
  vault_populate_gateway_kv
  vault_populate_grafana_oidc_kv
  vault_populate_argocd_oidc_kv
fi

# === 出力 ===
echo ""
echo "==> Done."
echo ""
echo "==> 確認:"
echo "    OIDC discovery: https://auth.platform.yu-min3.com/realms/$REALM/.well-known/openid-configuration"
echo "    bw get item \"$BW_CLIENT_ITEM\" | jq '{name, login: {username: .login.username}}'"
echo ""
echo "==> 次のステップ (Vault Stage 1, 未実施なら):"
echo "    1. vault operator init (manual)"
echo "    2. root token + Recovery Keys を Bitwarden に保存"
echo "    3. bootstrap/vault/ で terraform.tfvars に下記を書く:"
echo "       keycloak_oidc_discovery_url = \"https://auth.platform.yu-min3.com/realms/$REALM\""
echo "       keycloak_oidc_client_id     = \"$CLIENT_ID\""
echo "       keycloak_oidc_client_secret = \"<bw get item $BW_CLIENT_ITEM | jq -r .login.password>\""
echo "    4. cd bootstrap/vault && terraform init && terraform apply"
echo ""
echo "==> 次のステップ (Gateway OIDC / oauth2-proxy, このスクリプト以後):"
echo "    1. PR #269 を ready-for-review → merge"
echo "    2. ArgoCD で oauth2-proxy Application が sync 完了するか確認"
echo "       kubectl -n argocd get application oauth2-proxy"
echo "       kubectl -n auth-system get pods -l app=oauth2-proxy"
echo "    3. provider 登録確認:"
echo "       kubectl -n istio-system get cm istio -o yaml | yq '.data.mesh' | yq '.extensionProviders'"
