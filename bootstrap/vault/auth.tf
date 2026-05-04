# =============================================================================
# Auth Methods Enable
# =============================================================================

# Kubernetes auth: Pod が SA JWT で Vault に認証
resource "vault_auth_backend" "kubernetes" {
  type        = "kubernetes"
  path        = "kubernetes"
  description = "Kubernetes ServiceAccount JWT authentication"
}

# Kubernetes auth backend の cluster 接続設定
# Vault Pod 自身が in-cluster な K8s API を叩いて TokenReview を行う
# kubernetes_host は in-cluster で resolvable な default URL
resource "vault_kubernetes_auth_backend_config" "this" {
  backend         = vault_auth_backend.kubernetes.path
  kubernetes_host = "https://kubernetes.default.svc.cluster.local:443"
  # token_reviewer_jwt と kubernetes_ca_cert は Vault 5.x で auto-detect (vault SA 経由)
}

# OIDC auth: 人間が Keycloak 経由で login
resource "vault_jwt_auth_backend" "oidc" {
  type               = "oidc"
  path               = "oidc"
  description        = "Keycloak OIDC for human login"
  oidc_discovery_url = var.keycloak_realm_url
  oidc_client_id     = var.keycloak_oidc_client_id
  oidc_client_secret = var.keycloak_oidc_client_secret
  default_role       = "platform-dev" # group claim 一致しない場合の fallback
}

# Userpass auth: break-glass (Keycloak 死亡時の緊急ログイン)
resource "vault_auth_backend" "userpass" {
  type        = "userpass"
  path        = "userpass"
  description = "Emergency userpass auth (break-glass only)"
}

# =============================================================================
# OIDC Roles (Keycloak group → Vault policy mapping)
# =============================================================================

resource "vault_jwt_auth_backend_role" "platform_admin" {
  backend         = vault_jwt_auth_backend.oidc.path
  role_name       = "platform-admin"
  role_type       = "oidc"
  user_claim      = "sub"
  bound_audiences = [var.keycloak_oidc_client_id]
  groups_claim    = "groups"
  bound_claims = {
    groups = "platform-admin"
  }
  token_policies = [vault_policy.admin.name]
  token_ttl      = 3600  # 1h
  token_max_ttl  = 28800 # 8h
  allowed_redirect_uris = var.vault_redirect_uris
}

resource "vault_jwt_auth_backend_role" "platform_dev" {
  backend         = vault_jwt_auth_backend.oidc.path
  role_name       = "platform-dev"
  role_type       = "oidc"
  user_claim      = "sub"
  bound_audiences = [var.keycloak_oidc_client_id]
  groups_claim    = "groups"
  bound_claims = {
    groups = "platform-dev"
  }
  token_policies = [vault_policy.platform_dev.name]
  token_ttl      = 3600
  token_max_ttl  = 28800
  allowed_redirect_uris = var.vault_redirect_uris
}

# =============================================================================
# Kubernetes Auth Roles
# =============================================================================

# vault-config-operator が VCO admin policy を取得するための role
# VCO は CR の spec.authentication.serviceAccount に基づいて TokenRequest API で
# 短命 token を発行し Vault に login する。SA は CR と同じ K8s ns から取られる。
# 本リポジトリは VCO CR (Policy / SecretEngineMount / DatabaseSecretEngineConfig 等)
# を vault ns に置く方針なので、bound SA は vault:default に揃える。
# VCO Pod 自身 (controller-manager:vault-config-operator) は Vault に直接 login しない。
resource "vault_kubernetes_auth_backend_role" "vault_config_operator" {
  backend                          = vault_auth_backend.kubernetes.path
  role_name                        = "vault-config-operator"
  bound_service_account_names      = ["default"]
  bound_service_account_namespaces = ["vault"]
  token_policies                   = [vault_policy.vco_admin.name]
  token_ttl                        = 3600  # 1h
  token_max_ttl                    = 86400 # 24h
}

# external-secrets が secret/data/* を read するための role
resource "vault_kubernetes_auth_backend_role" "external_secrets" {
  backend                          = vault_auth_backend.kubernetes.path
  role_name                        = "external-secrets"
  bound_service_account_names      = ["external-secrets"]
  bound_service_account_namespaces = ["external-secrets"]
  token_policies                   = [vault_policy.eso_read.name]
  token_ttl                        = 3600
  token_max_ttl                    = 86400
}

# =============================================================================
# Userpass: emergency-admin (break-glass)
# =============================================================================

resource "vault_generic_endpoint" "emergency_admin" {
  depends_on = [vault_auth_backend.userpass, vault_policy.admin]
  path       = "auth/userpass/users/emergency-admin"
  data_json = jsonencode({
    password       = var.emergency_admin_password
    token_policies = "admin"
    token_ttl      = "1h"
    token_max_ttl  = "8h"
  })
  ignore_absent_fields = true

  # password を update 検知の対象外にする (rotation は外で実施)
  write_fields = ["password"]
}
