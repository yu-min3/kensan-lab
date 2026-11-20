provider "vault" {
  address = var.vault_address
  token   = var.vault_token
}

# core と oidc は bootstrap-tf.sh の同一の private state に対して順に apply する。
# - 動作確認後に finish で初期 root token を失効し、state を破棄する
# - 再実行は新規 cluster 構築時のみ (既存 Vault に再 apply するなら terraform import が要る)
# - 永続的な Vault 設定は vault-config-operator (CRD) が ArgoCD sync で管理
