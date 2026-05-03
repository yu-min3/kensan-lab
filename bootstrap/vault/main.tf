provider "vault" {
  address = var.vault_address
  token   = var.vault_token
}

# このディレクトリは Pattern A' (TF 使い捨て) として運用する
# - 1 回 apply、state 破棄
# - 再実行は新規 cluster 構築時のみ (既存 Vault に再 apply するなら terraform import が要る)
# - 永続的な Vault 設定は vault-config-operator (CRD) が ArgoCD sync で管理
