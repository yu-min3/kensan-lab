# Bootstrap TF inputs
# 値は terraform.tfvars (gitignore 済み) で渡す

variable "vault_address" {
  description = "Vault HTTP API URL. ローカルから kubectl port-forward 経由で繋ぐなら http://localhost:8200"
  type        = string
}

variable "vault_token" {
  description = "Root token (vault operator init で取得した直後のもの)。bootstrap 完了後に revoke すること"
  type        = string
  sensitive   = true
}

# Keycloak 連携 (事前に Keycloak realm + OIDC client 作成済みの前提)
variable "keycloak_realm_url" {
  description = "Keycloak realm の OIDC discovery URL。例: https://auth.platform.yu-min3.com/realms/kensan"
  type        = string
}

variable "keycloak_oidc_client_id" {
  description = "Keycloak で作成した Vault 用 OIDC client の client_id"
  type        = string
}

variable "keycloak_oidc_client_secret" {
  description = "Keycloak の OIDC client secret"
  type        = string
  sensitive   = true
}

variable "vault_redirect_uris" {
  description = "OIDC callback URL のリスト。Keycloak client の Valid Redirect URIs と一致させる"
  type        = list(string)
  default = [
    "https://vault.platform.yu-min3.com/ui/vault/auth/oidc/oidc/callback",
    "http://localhost:8250/oidc/callback", # vault CLI の oidc auth method 用
  ]
}

# Break-glass 用 (1Password に保管)
variable "emergency_admin_password" {
  description = "userpass auth method の emergency-admin user の password"
  type        = string
  sensitive   = true
}
