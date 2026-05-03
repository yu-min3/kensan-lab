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

    # Policies
    path "sys/policies/acl"      { capabilities = ["read", "list"] }
    path "sys/policies/acl/*"    { capabilities = ["create", "read", "update", "delete", "list"] }

    # Identity
    path "identity/*"  { capabilities = ["create", "read", "update", "delete", "list"] }

    # Auth method configurations (e.g. kubernetes/role/*, oidc/role/*)
    path "auth/*"  { capabilities = ["create", "read", "update", "delete", "list", "sudo"] }

    # Secret data manage (KV, PKI, Database etc.)
    path "secret/*"     { capabilities = ["create", "read", "update", "delete", "list"] }
    path "kubernetes/*" { capabilities = ["create", "read", "update", "delete", "list"] }

    # Audit (status read のみ、enable は bootstrap 範囲)
    path "sys/audit"  { capabilities = ["read", "list"] }
  EOT
}

# eso-read: External Secrets Operator が secret/data/* を read するための policy
resource "vault_policy" "eso_read" {
  name   = "eso-read"
  policy = <<-EOT
    # KV v2 read (data path)
    path "secret/data/*" {
      capabilities = ["read", "list"]
    }
    # KV v2 metadata read (バージョン情報取得)
    path "secret/metadata/*" {
      capabilities = ["read", "list"]
    }
    # token renew (lease 維持)
    path "auth/token/renew-self" {
      capabilities = ["update"]
    }
    path "auth/token/lookup-self" {
      capabilities = ["read"]
    }
  EOT
}

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
