#!/usr/bin/env bash
## ============================================================================
## Vault Transit key bootstrap (Stage 6, 1 回だけ実行)
## ============================================================================
##
## なぜ手動?
##   redhat-cop/vault-config-operator は 2026-05 時点で TransitSecretEngine CR を
##   持たない (mount は SecretEngineMount で declarative にできるが、key の create /
##   rotate / config 設定はできない)。本 capability では mount + policy + auth role
##   だけ GitOps、key は手動作成 (1 度きり) の hybrid 構成にする。
##
## 実行タイミング
##   1. PR vault-stage6-transit/vco-config が main に merge されて
##      Application "vault-transit-engine-shared" が Healthy になった後
##   2. Yu が手動で 1 度だけ実行
##
## 実行方法
##   kubectl exec -n vault vault-0 -c vault -- /bin/sh < <この script>
##   (もしくは vault CLI を直接持っている macOS シェルから VAULT_ADDR / VAULT_TOKEN
##    を export して実行)
##
## 確認
##   vault list transit/keys                    # users-name が見える
##   vault read transit/keys/users-name         # type: aes256-gcm96, latest_version: 1
##
## Key rotation (運用)
##   vault write -f transit/keys/users-name/rotate
##   後に user-service の rewrap loop (Stage 6 PR #2 で実装) が旧 ciphertext を
##   最新 version に rewrap する想定。

set -euo pipefail

KEY_NAME="${KEY_NAME:-users-name}"

echo "==> Creating Vault Transit key: ${KEY_NAME}"
vault write -f "transit/keys/${KEY_NAME}" \
  type=aes256-gcm96 \
  exportable=false \
  allow_plaintext_backup=false \
  derived=false \
  convergent_encryption=false

echo
echo "==> Created. Verify:"
vault read "transit/keys/${KEY_NAME}"

echo
echo "==> Done. Next step: deploy user-service with VAULT_* env (Stage 6 PR #2)."
