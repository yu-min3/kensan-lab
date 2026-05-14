---
description: Secret management (Vault + ESO / Sealed Secrets / Transit), cert-manager, Reloader, GHCR pull secrets
globs: "**/sealed-secret*, kubernetes/auth/**, kubernetes/secrets/**"
---

# Security & Secrets

## Secret Management Methods

4 方式を用途で使い分ける（詳細は [`docs/secret-management/index.md`](../../docs/secret-management/index.md) が SoT）。

| 方式 | 主用途 | ローテ | 配布 |
|---|---|---|---|
| **Vault dynamic (VDBE)** | Postgres app 接続 cred | TTL 24h / 12h rotation | Vault Database Engine が動的生成 |
| **Vault static (ESO)** | API key / admin password / GHCR token / Slack webhook | Vault に新値を書けば最大 1h で配布 | External Secrets Operator → K8s Secret |
| **Vault Transit** | DB カラムに保存する PII の暗号鍵 (envelope encryption) | 手動 rotate + `transit/rewrap` | アプリが K8s SA auth で Vault に login → Transit API 直叩き |
| **SealedSecret** | Vault 自身の起動依存 / 内部 PKI / cert-manager / image pull token | 手動 (低頻度) | Sealed Secrets controller が in-cluster で復号 |

### 選び方
- Postgres app 接続 cred → **Vault dynamic**
- ローテが必要だが頻度低めの API key 系 → **Vault static (ESO)**
- DB に保存する PII → **Vault Transit**（鍵を Pod / DB に降ろさない）
- **Vault に依存できない低レイヤ** (Vault unseal cred / cert-manager / Cilium PKI) → **SealedSecret**

## Reloader

`stakater/reloader`（`reloader` ns）が ConfigMap / Secret の更新を検知して対応 Deployment / StatefulSet を rollout する。
ESO が Vault → K8s Secret 更新 → Reloader が pod 再起動、というチェーンで動的ローテが完結する。

`Deployment` 等の annotation:
```yaml
metadata:
  annotations:
    reloader.stakater.com/auto: "true"
    # または個別指定:
    secret.reloader.stakater.com/reload: "kensan-db-credentials"
```

## Sealed Secrets Workflow

1. Raw secret を作る → `temp/<name>-raw.yaml`（git-ignored）
2. Seal: `kubeseal --format=yaml < temp/<name>-raw.yaml > kubernetes/<cat>/<comp>/resources/<name>-sealed.yaml`
3. Sealed YAML を commit（Git に保存して安全）
4. Argo CD sync → Sealed Secrets controller が in-cluster で復号

## ExternalSecret Workflow (Vault static)

1. Vault に値を put: `vault kv put secret/<path> KEY=VALUE`
2. `ExternalSecret` CR を `kubernetes/<cat>/<comp>/resources/` に置く（`ClusterSecretStore: vault-backend` を参照）
3. ESO が refreshInterval (既定 1h) で Vault → K8s Secret に同期
4. Reloader annotation のある Deployment は Secret 更新時に rollout

## File Safety Rules

- **NEVER commit**: `temp/*-raw.yaml`, `.env`, `*credentials*` の生ファイル
- **Safe to commit**: `*-sealed.yaml`, `ExternalSecret` CR, `VaultStaticSecret` 等の CR
- `temp/*-raw.yaml` は `.gitignore` でブロック

## cert-manager

- **ClusterIssuer**: Let's Encrypt (production + staging)
- **Certificates**: 自動更新、`kubernetes/secrets/cert-manager/resources/` で定義
- Wildcard 証明書: `wildcard-platform-tls`（`*.platform.yu-min3.com`）と `wildcard-apps-tls`（`*.app.yu-min3.com`）
- DNS-01 challenge 用の IAM credential は SealedSecret（`route53-credentials`、`cert-manager` ns）
- Vault PKI は採用していない（理由: ADR-007）

## GHCR Pull Secrets

- `ghcr-pull-secret`（`.dockerconfigjson`）は Vault static (ESO) で配布。Vault path: `secret/ghcr/pull-token`
- 各 app ns（`app-prod`, `backstage`, `kensan`）が `ExternalSecret` で受け取り、ServiceAccount が `imagePullSecrets: [ghcr-pull-secret]` で参照

## Keycloak Authentication

- OIDC IdP: `auth.platform.yu-min3.com`（`platform-auth-prod` ns）
- Gateway-level 認可: Istio Gateway + oauth2-proxy を ext_authz として使う（`auth-system` ns）
- Postgres backing store の cred は Vault dynamic（ADR-008）
