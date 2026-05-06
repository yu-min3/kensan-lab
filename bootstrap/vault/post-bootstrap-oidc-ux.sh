#!/bin/bash
# Vault OIDC ログイン UX 改善のための one-shot 設定スクリプト
#
# 役割:
#   1. auth/oidc/config の `default_role` を "default" に切り替え
#      → UI で Role 欄を空のまま Sign in できるようにする
#   2. sys/config/ui/login/default-auth で OIDC を Method の default に
#      → UI を開いた瞬間 OIDC method が選択された状態
#
# なぜ VCO じゃないか:
#   - VCO の JWTOIDCAuthEngineConfig CRD は default_role field を持たない (2026-05-06 時点)
#   - VCO に sys/config/ui/login/default-auth 用 CRD が無い
#   詳細は projects/kensan-lab/vault-config-management-strategy.md
#
# 前提:
#   - VCO で `JWTOIDCAuthEngineRole "default"` が apply 済 (本 PR の他 yaml)
#   - vault CLI 利用可、root token を import 済
#       export VAULT_ADDR=https://vault.platform.yu-min3.com
#       export VAULT_TOKEN=$(bw get item kensan-lab/vault/root-token | jq -r .login.password)
#
# 冪等性: 何回流しても同じ結果。値が同じなら no-op。

set -euo pipefail

if ! command -v vault >/dev/null 2>&1; then
  echo "ERROR: vault CLI not found"; exit 1
fi
if [ -z "${VAULT_ADDR:-}" ]; then
  echo "ERROR: VAULT_ADDR not set"; exit 1
fi
if ! vault token lookup >/dev/null 2>&1; then
  echo "ERROR: vault token invalid (login or re-export VAULT_TOKEN)"; exit 1
fi

echo "==> Setting auth/oidc/config default_role=default"
# 重要: auth/oidc/config エンドポイントは:
# - vault write は HTTP PUT で full-replace (指定しない field は消える)
# - vault patch は HTTP PATCH 405 Method Not Allowed (実機検証 2026-05-06)
# したがって全 field を毎回明示する full-write しか選択肢なし。
# oidc_client_secret は Bitwarden から runtime 取得 (script 内に平文残さない)。
if ! command -v bw >/dev/null 2>&1; then
  echo "ERROR: bw CLI not found (Bitwarden CLI が必要)"; exit 1
fi
if [ "$(bw status 2>/dev/null | jq -r '.status' 2>/dev/null)" != "unlocked" ]; then
  echo "ERROR: bw が unlock 状態じゃない (export BW_SESSION=\$(bw unlock --raw))"; exit 1
fi
VAULT_OIDC_CLIENT_SECRET=$(bw get item kensan-lab/keycloak/oidc-client-vault | jq -r '.login.password')
if [ -z "$VAULT_OIDC_CLIENT_SECRET" ] || [ "$VAULT_OIDC_CLIENT_SECRET" = "null" ]; then
  echo "ERROR: BW から vault client_secret 取得失敗"; exit 1
fi
vault write auth/oidc/config \
  oidc_discovery_url="https://auth.platform.yu-min3.com/realms/kensan" \
  oidc_client_id="vault" \
  oidc_client_secret="$VAULT_OIDC_CLIENT_SECRET" \
  default_role="default"
echo "    OK"

echo ""
echo "==> UI default-auth はスキップ (Vault Enterprise 専用 endpoint、OSS では 404)"
echo "    OSS 代替: ブックマーク URL で OIDC method を pre-select する:"
echo "      https://vault.platform.yu-min3.com/ui/vault/auth?with=oidc/"
echo "    開いたら Role 欄空のまま Sign in (default_role=default で通る)"

echo ""
echo "==> Verify"
echo "  - auth/oidc/config:"
vault read auth/oidc/config 2>&1 | grep -E "default_role|oidc_discovery_url" | sed 's/^/      /'

echo ""
echo "==> Done. ブラウザで下記を開けば 1-click login:"
echo "    https://vault.platform.yu-min3.com/ui/vault/auth?with=oidc/"
