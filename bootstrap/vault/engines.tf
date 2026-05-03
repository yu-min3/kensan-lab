# =============================================================================
# Secret Engines
# =============================================================================

# KV v2 at "secret/" — App human-managed static secret 用
# (Stage 2 で Grafana admin pw 等をここに置く)
resource "vault_mount" "kv_v2" {
  path        = "secret"
  type        = "kv"
  options     = { version = "2" }
  description = "KV v2 secret store for app human-managed static secrets"
}

# =============================================================================
# Audit Devices
# =============================================================================

# Audit device #1: file (PVC mounted at /vault/audit, see Helm values)
# Promtail が tail して Loki に送る
resource "vault_audit" "file" {
  type        = "file"
  description = "File audit device #1 (primary, PVC backed)"
  options = {
    file_path = "/vault/audit/audit.log"
  }
}

# Audit device #2: stdout (container log として Promtail で捕捉、Loki へ)
# 1 個目が write 失敗しても Vault 全停止しないように冗長化
resource "vault_audit" "stdout" {
  path        = "stdout"
  type        = "file"
  description = "File audit device #2 (backup, stdout)"
  options = {
    file_path = "stdout"
  }
}
