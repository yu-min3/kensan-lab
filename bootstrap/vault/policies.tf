# Vault Policies
# 永続的な per-app policy は vault-config-operator が CRD で管理する
# ここでは Bootstrap で必要な信頼ルートの policy のみ作る

# admin: Vault 全権限 (人間 platform-admin + emergency-admin が持つ)
resource "vault_policy" "admin" {
  name   = "admin"
  policy = <<-EOT
    path "*" {
      capabilities = ["create", "read", "update", "delete", "list", "sudo", "patch"]
    }
  EOT
}

# vco-admin: vault-config-operator が Vault 設定を管理するための policy
# admin と等価 (VCO は auth methods / mounts / policies / identity すべて触る)
resource "vault_policy" "vco_admin" {
  name   = "vco-admin"
  policy = <<-EOT
    # Auth methods
    path "sys/auth"      { capabilities = ["read", "list"] }
    path "sys/auth/*"    { capabilities = ["create", "read", "update", "delete", "list", "sudo"] }

    # Secret engines
    path "sys/mounts"    { capabilities = ["read", "list"] }
    path "sys/mounts/*"  { capabilities = ["create", "read", "update", "delete", "list", "sudo"] }

    # Policies (new API)
    path "sys/policies/acl"      { capabilities = ["read", "list"] }
    path "sys/policies/acl/*"    { capabilities = ["create", "read", "update", "delete", "list"] }
    # Policies (legacy API — VCO's Policy CRD uses /sys/policy/{name})
    path "sys/policy"            { capabilities = ["read", "list"] }
    path "sys/policy/*"          { capabilities = ["create", "read", "update", "delete", "list"] }

    # Identity
    path "identity/*"  { capabilities = ["create", "read", "update", "delete", "list"] }

    # Auth method configurations (e.g. kubernetes/role/*, oidc/role/*)
    path "auth/*"  { capabilities = ["create", "read", "update", "delete", "list", "sudo"] }

    # Secret data manage (KV, PKI, Database etc.)
    path "secret/*"     { capabilities = ["create", "read", "update", "delete", "list"] }
    path "kv1/*"        { capabilities = ["create", "read", "update", "delete", "list"] }
    path "kubernetes/*" { capabilities = ["create", "read", "update", "delete", "list"] }
    path "database/*"   { capabilities = ["create", "read", "update", "delete", "list"] }

    # Audit (status read のみ、enable は bootstrap 範囲)
    path "sys/audit"  { capabilities = ["read", "list"] }
  EOT
}

# eso-read policy is managed by Vault Config Operator (VCO) via Policy CR
# (infrastructure/security/vault-config-operator/resources/database-engine/policy-eso-read.yaml).
# Bootstrap chain で必要な policy は admin / vco-admin のみ。eso-read は VCO 起動後に作成される。

# platform-dev: 将来の dev チーム用 placeholder。Phase 1 では readonly 程度に絞る
resource "vault_policy" "platform_dev" {
  name   = "platform-dev"
  policy = <<-EOT
    # 自分のトークン情報のみ
    path "auth/token/lookup-self" {
      capabilities = ["read"]
    }
    # secret/dev/* 配下のみ read (将来 VCO で path 設計を詰める想定)
    path "secret/data/dev/*" {
      capabilities = ["read", "list"]
    }
  EOT
}
