# secrets

機密 (DB credential / API key / TLS 証明書 / 暗号鍵) の管理を担うコンポーネント群。Vault を中核とした 4 方式 + bootstrap 用 Sealed Secrets で構成。

## 構成

| dir | 役割 |
|---|---|
| `vault/` | Vault server (HA 3 replica、KMS auto-unseal、KV / Database / Transit / OIDC mount) |
| `vault-config-operator/` | Vault を K8s CR で declarative に設定する operator |
| `vault-database-engine/` | DB dynamic credential の chart + 共通リソース。restart / reload を許容できる per-DB instance だけ ApplicationSet で auto 生成 |
| `vault-transit-engine/` | encryption-as-a-service の鍵 + policy + auth role |
| `external-secrets/` | Vault → K8s Secret bridge (ESO operator) |
| `sealed-secrets/` | Vault 起動前から必要な secret (循環依存回避) を Git に暗号化保存 |
| `cert-manager/` | TLS 証明書管理 (Let's Encrypt DNS-01 wildcard)。Vault と独立 |
| `reloader/` | Secret / ConfigMap 変更検知 → Pod auto-restart (主に dynamic cred rotation 用) |

## 4 方式の使い分け (要約)

- **Vault dynamic** (Database engine) — restart / credential reload を許容できる Postgres app cred (TTL 24h)
- **Vault static** (ESO 経由) — API key / 管理者 pw (1h refresh)
- **Vault Transit** — DB カラム PII の envelope 暗号化 (鍵は Vault 内に常駐)
- **SealedSecret** — Vault 起動依存 / 低レイヤ PKI (Vault に依存できない場合のみ)

## 関連

- 詳細マトリクス・全 inventory・運用フロー: [`docs/secret-management/index.md`](../../docs/secret-management/index.md)
- 設計判断: [ADR-007](../../docs/adr/007-no-vault-pki.md) / [ADR-008](../../docs/adr/008-keycloak-db-credentials.md) / [ADR-011](../../docs/adr/011-vault-version-pinning.md)
