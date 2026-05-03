terraform {
  required_version = ">= 1.6"

  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "~> 5.0" # 5.9.0 (2026-04-22) 時点最新
    }
  }
}
