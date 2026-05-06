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
# 重要: `vault write` は full-replace なので、default_role だけ送ると
# oidc_discovery_url / oidc_client_id 等が消える (HTTP PUT semantics)。
# `vault patch` (Vault 1.16+ で導入された partial update) を使い、
# 既存 field を維持したまま default_role だけ更新する。
vault patch auth/oidc/config default_role=default
echo "    OK"

echo ""
echo "==> Setting UI default login method = oidc (root namespace)"
# ルール名は任意。namespace_path 空文字 = root namespace。
# backup_auth_types は空 (token method は常に裏に居るので)。
vault write sys/config/ui/login/default-auth/oidc-default \
  namespace_path="" \
  default_auth_type="oidc" \
  disable_inheritance=false \
  backup_auth_types=""
echo "    OK"

echo ""
echo "==> Verify"
echo "  - auth/oidc/config:"
vault read auth/oidc/config 2>&1 | grep -E "default_role|oidc_discovery_url" | sed 's/^/      /'
echo ""
echo "  - sys/config/ui/login/default-auth/oidc-default:"
vault read sys/config/ui/login/default-auth/oidc-default 2>&1 | grep -E "default_auth_type|namespace_path" | sed 's/^/      /'

echo ""
echo "==> Done. UI を開くと OIDC method 既定 + Role 空欄で Sign in 可能になる。"
