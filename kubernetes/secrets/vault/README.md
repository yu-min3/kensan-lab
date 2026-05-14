# vault

Vault HA server (3 replica、AWS KMS auto-unseal、Raft storage)。KV / Database / Transit / OIDC の各 secret engine をホストする secret 管理の中核。

## 構成

- `values.yaml` — HashiCorp 公式 chart の override (HA、KMS auto-unseal、Raft retry_join、image pin)
- `resources/`
  - `namespace.yaml` — `vault` ns
  - `aws-kms-credentials-sealed.yaml` — KMS 用 IAM (SealedSecret、Vault 自身の起動に必要なので Vault 由来にできない)
  - `httproute.yaml` — UI / API (`vault.platform.yu-min3.com`)

## 注意点

- **`spec.syncOptions: Prune=false`**: Application 削除で raft データの PV が蒸発するのを防ぐ (resources-finalizer も外している)
- **image tag を明示 pin** (latest 不可): 理由は ADR-011
- **TLS disabled**: cluster 内通信は Istio mTLS でカバー。Phase 2 以降で再検討

## 関連

- 4 方式の使い分けと運用詳細: [`docs/secret-management/index.md`](../../../docs/secret-management/index.md)
- bootstrap (初回 unseal、root token): [`docs/bootstrapping/12-vault-stage1.md`](../../../docs/bootstrapping/12-vault-stage1.md)
- ADR: [007 No Vault PKI](../../../docs/adr/007-no-vault-pki.md) / [011 Version Pinning](../../../docs/adr/011-vault-version-pinning.md)
