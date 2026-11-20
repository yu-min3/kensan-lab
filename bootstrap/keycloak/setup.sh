#!/usr/bin/env bash
# Reconcile the Keycloak realm, then publish issuer-owned OIDC secrets to Vault.
set +x
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/read-vault-client-secret.sh"

REALM="kensan"
USER_NAME="yu"
USER_EMAIL="ymisaki00@gmail.com"
USER_FIRST="Yu"
USER_LAST="Misaki"
ADMIN_GROUP="platform-admin"
DEV_GROUP="platform-dev"
CLIENT_ID="vault"
VAULT_HOSTNAME="vault.platform.yu-min3.com"
SSO_SESSION_IDLE_SECONDS=259200   # 3 days
SSO_SESSION_MAX_SECONDS=2592000  # 30 days

for cmd in kubectl jq openssl; do
  command -v "$cmd" >/dev/null || { echo "Missing $cmd" >&2; exit 1; }
done
KC_CONTEXT=$(kubectl config current-context)
echo "==> kubectl context: $KC_CONTEXT"
echo "==> 続行する場合は Enter、違うなら Ctrl-C"
read -r
keycloak_open_session

# === Realm 作成 ===
echo ""
echo "==> Ensuring realm '$REALM'..."
current_realm=$(kcadm get realms --fields realm | jq -r --arg realm "$REALM" '.[] | select(.realm == $realm) | .realm')
if [[ -n $current_realm ]]; then
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

# ADR-016 defines the inner identity session as idle 3 days / max 30 days.
# This used to be an imperative Admin Console setting, which meant a recreated
# realm silently fell back to Keycloak's 30 minute / 10 hour defaults. Reconcile
# it on every run so bare metal and kind intentionally share the same policy.
kcadm update "realms/$REALM" \
  -s "ssoSessionIdleTimeout=$SSO_SESSION_IDLE_SECONDS" \
  -s "ssoSessionMaxLifespan=$SSO_SESSION_MAX_SECONDS" > /dev/null
echo "    SSO session policy: idle=3d, max=30d"

# === Groups 作成 ===
ensure_group() {
  local g="$1"
  echo ""
  echo "==> Ensuring group '$g' in realm '$REALM'..."
  local existing
  existing=$(kcadm get groups -r "$REALM" --fields name \
    | jq -r --arg g "$g" '.[] | select(.name == $g) | .name')
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
USER_ID=$(kcadm get users -r "$REALM" --query "username=$USER_NAME" --query exact=true --fields id,username |
  jq -r --arg user "$USER_NAME" '.[] | select(.username == $user) | .id')
if [[ -n "$USER_ID" ]]; then
  echo "    existing user; preserving password and memberships"
else
  : "${KENSAN_OP_VAULT:?Set KENSAN_OP_VAULT to the dedicated 1Password vault}"
  # Create user, credential and membership in one request. A failed lookup
  # must never result in a passwordless user that a retry would skip.
  user_json=$(op read --no-newline "op://${KENSAN_OP_VAULT}/keycloak-user-yu/password" |
    jq -Rsec --arg user "$USER_NAME" --arg email "$USER_EMAIL" --arg first "$USER_FIRST" \
      --arg last "$USER_LAST" --arg group "/$ADMIN_GROUP" '
        select(length > 0) |
        {username:$user, email:$email, firstName:$first, lastName:$last,
         enabled:true, emailVerified:true, groups:[$group],
         credentials:[{type:"password", temporary:false, value:.}] }')
  printf '%s' "$user_json" | kcadm create users -r "$REALM" -f - >/dev/null
  unset user_json
  echo "    created user with a password and group membership"
fi

# === OIDC Client 共通関数 ===
# 引数: <client_id> <name> <description> <redirect_uris_json> <web_origins_json> <post_logout_uri>
# 戻り値: stdout に client UUID を 1 行で出力。
# 副作用: 存在しなければ作成し groups protocol mapper を付ける。冪等。
ensure_oidc_client() (
  set -euo pipefail
  local cid="$1"
  local cname="$2"
  local cdesc="$3"
  local redirect_uris_json="$4"
  local web_origins_json="$5"
  local post_logout_uri="$6"

  echo "" >&2
  echo "==> Ensuring OIDC client '$cid' in realm '$REALM'..." >&2
  local uuid groups_mapper
  uuid=$(kcadm get clients -r "$REALM" --query "clientId=$cid" --fields id \
    | jq -r '.[0].id // empty')

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
      | jq -er '.[0].id | strings | select(length > 0)')
    echo "    client '$cid' created (id: $uuid)" >&2

  fi
  # A previous run may have stopped immediately after creating the client.
  groups_mapper=$(kcadm get "clients/$uuid/protocol-mappers/models" -r "$REALM" --fields name |
    jq -r '.[] | select(.name == "groups") | .name')
  if [[ -z "$groups_mapper" ]]; then
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
)

# === Vault OIDC client ===
VAULT_REDIRECT_URIS=$(jq -nc \
  --arg vault_cb "https://$VAULT_HOSTNAME/ui/vault/auth/oidc/oidc/callback" \
  --arg cli_cb "http://localhost:8250/oidc/callback" \
  '[$vault_cb, $cli_cb]')
VAULT_WEB_ORIGINS=$(jq -nc --arg origin "https://$VAULT_HOSTNAME" '[$origin]')

ensure_oidc_client \
  "$CLIENT_ID" \
  "Vault OIDC" \
  "Vault HA OIDC auth method (Stage 1)" \
  "$VAULT_REDIRECT_URIS" \
  "$VAULT_WEB_ORIGINS" \
  "https://$VAULT_HOSTNAME" >/dev/null

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
  # Cloudflare Tunnel 経由の外部公開 host (`.yu-mins.com`)。
  # hubble は yu-mins HTTPRoute 未配備のため除外。
  backstage.yu-mins.com
  prometheus.yu-mins.com
  longhorn.yu-mins.com
  # gateway-prod 側の protected host (新 kensan、ADR-010 の app 系への拡張)
  kensan.app.yu-min3.com # 本番 host（Phase 7 で legacy から継承, #394）
  kensan.yu-mins.com # kensan アプリの外部公開 (Cloudflare Tunnel → gateway-prod)
  koubou.app.yu-min3.com # 工房のApp入口（LANのみ）
  konro.app.yu-min3.com # konro（作り置き並行調理アプリ、LAN のみ・外部公開なし）
  kensan-markport.app.yu-min3.com # kensan の workspace ビューア（LAN のみ、ADR-023）
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
GRAFANA_HOSTNAME_TUNNEL="grafana.yu-mins.com"
GRAFANA_REDIRECT_URIS=$(jq -nc \
  --arg cb "https://$GRAFANA_HOSTNAME/login/generic_oauth" \
  --arg cb_tunnel "https://$GRAFANA_HOSTNAME_TUNNEL/login/generic_oauth" \
  '[$cb, $cb_tunnel]')
GRAFANA_WEB_ORIGINS=$(jq -nc \
  --arg origin "https://$GRAFANA_HOSTNAME" \
  --arg origin_tunnel "https://$GRAFANA_HOSTNAME_TUNNEL" \
  '[$origin, $origin_tunnel]')
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
ARGOCD_HOSTNAME_TUNNEL="argocd.yu-mins.com"
ARGOCD_REDIRECT_URIS=$(jq -nc \
  --arg ui_cb "https://$ARGOCD_HOSTNAME/auth/callback" \
  --arg cli_cb "http://localhost:8085/auth/callback" \
  --arg pkce_cb "https://$ARGOCD_HOSTNAME/pkce/verify" \
  --arg ui_cb_tunnel "https://$ARGOCD_HOSTNAME_TUNNEL/auth/callback" \
  --arg pkce_cb_tunnel "https://$ARGOCD_HOSTNAME_TUNNEL/pkce/verify" \
  '[$ui_cb, $cli_cb, $pkce_cb, $ui_cb_tunnel, $pkce_cb_tunnel]')
ARGOCD_WEB_ORIGINS=$(jq -nc \
  --arg origin "https://$ARGOCD_HOSTNAME" \
  --arg origin_tunnel "https://$ARGOCD_HOSTNAME_TUNNEL" \
  '[$origin, $origin_tunnel]')
ARGOCD_CLIENT_UUID=$(ensure_oidc_client \
  "argocd" \
  "ArgoCD OIDC" \
  "ArgoCD argocd-cm.oidc.config (Path B — gateway-platform bypass)" \
  "$ARGOCD_REDIRECT_URIS" \
  "$ARGOCD_WEB_ORIGINS" \
  "https://$ARGOCD_HOSTNAME")

# === Backstage OIDC client (Path B — Backstage native OIDC) ===
BACKSTAGE_HOSTNAME="backstage.platform.yu-min3.com"
BACKSTAGE_HOSTNAME_TUNNEL="backstage.yu-mins.com"
BACKSTAGE_REDIRECT_URIS=$(jq -nc \
  --arg cb "https://$BACKSTAGE_HOSTNAME/api/auth/oidc/handler/frame" \
  --arg cb_tunnel "https://$BACKSTAGE_HOSTNAME_TUNNEL/api/auth/oidc/handler/frame" \
  '[$cb, $cb_tunnel]')
BACKSTAGE_WEB_ORIGINS=$(jq -nc \
  --arg origin "https://$BACKSTAGE_HOSTNAME" \
  --arg origin_tunnel "https://$BACKSTAGE_HOSTNAME_TUNNEL" \
  '[$origin, $origin_tunnel]')
BACKSTAGE_CLIENT_UUID=$(ensure_oidc_client \
  "backstage" \
  "Backstage OIDC" \
  "Backstage native OIDC (Path B — gateway-platform bypass)" \
  "$BACKSTAGE_REDIRECT_URIS" \
  "$BACKSTAGE_WEB_ORIGINS" \
  "https://$BACKSTAGE_HOSTNAME")

# Publish the four current client secrets. Compare-and-set preserves concurrent
# changes and prevents silently re-creating a deleted KV version.
sync_client_to_vault() {
  local client=$1 uuid=$2 path=$3 session_field=${4:-}
  local current version old next session='' client_secret
  if current=$(vault kv get -format=json -mount=secret "$path" 2>&1); then
    version=$(printf '%s' "$current" | jq -er '.data.metadata.version | numbers | select(. > 0)')
    old=$(printf '%s' "$current" | jq -ceS '.data.data | objects')
    if [[ -n "$session_field" ]]; then
      session=$(printf '%s' "$old" | jq -er --arg key "$session_field" '.[$key] | strings | select(length > 0)') || {
        echo "Expected $session_field at secret/$path; restore the existing value before retrying" >&2
        return 1
      }
    fi
  elif [[ "$current" == "No value found at secret/data/$path" ]]; then
    version=0
    old='{}'
    case "$session_field" in
      cookie-secret) session=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n');;
      session-secret) session=$(openssl rand -hex 32);;
    esac
  else
    echo "Unable to read secret/$path; check Vault access and retry" >&2
    return 1
  fi
  client_secret=$(keycloak_client_secret "$uuid")
  # Build on existing fields so a sync never discards another consumer's data.
  next=$(printf '%s' "$old" | jq -cSe --arg client "$client" --arg field "$session_field" \
    --rawfile credential <(printf '%s' "$client_secret") --rawfile session <(printf '%s' "$session") '
    .["client-id"] = $client | .["client-secret"] = $credential |
    if $field != "" then .[$field] = $session else . end')
  if [[ "$old" == "$next" ]]; then
    echo "    secret/$path already current"
    return
  fi
  printf '%s' "$next" | vault kv put -mount=secret -cas="$version" "$path" - >/dev/null
  echo "    synced secret/$path"
}

if [[ -z ${VAULT_ADDR:-} ]]; then
  echo 'Keycloak clients ready. App KV sync pending: set VAULT_ADDR, sign in to Vault, and rerun.'
  exit 0
fi
vault token lookup >/dev/null
sync_client_to_vault istio-gateway-platform "$GATEWAY_CLIENT_UUID" platform-auth/istio-gateway/platform cookie-secret
sync_client_to_vault grafana "$GRAFANA_CLIENT_UUID" observability/grafana/oidc
sync_client_to_vault argocd "$ARGOCD_CLIENT_UUID" security/argocd/oidc
sync_client_to_vault backstage "$BACKSTAGE_CLIENT_UUID" backstage/oidc session-secret
echo "==> Keycloak and app KV sync complete. Configure Vault login with bootstrap/vault/post-bootstrap-oidc-ux.sh."
