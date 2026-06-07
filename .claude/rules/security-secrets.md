---
description: Secret management workflows (Vault + ESO / Sealed Secrets / Reloader / cert-manager / GHCR)
globs: "**/sealed-secret*, kubernetes/auth/**, kubernetes/secrets/**"
---

# Security & Secrets

## 方式と SoT

4 方式 (Vault dynamic / Vault static via ESO / Vault Transit / SealedSecret) の使い分け・全 inventory・選び方は SoT を見る:

→ [`docs/secret-management/index.md`](../../docs/secret-management/index.md)

ここでは **AI が作業時に踏む operational rules** だけ書く。

## Sealed Secrets Workflow

1. Raw secret → `temp/<name>-raw.yaml`（git-ignored）
2. Seal: `kubeseal --format=yaml < temp/<name>-raw.yaml > kubernetes/<cat>/<comp>/resources/<name>-sealed.yaml`
3. Sealed YAML を commit
4. Argo CD sync → Sealed Secrets controller が in-cluster で復号

## ExternalSecret Workflow (Vault static)

1. Vault に値を put: `vault kv put secret/<path> KEY=VALUE`
2. `ExternalSecret` CR を `kubernetes/<cat>/<comp>/resources/` に置く（`ClusterSecretStore: vault-backend` を参照）
3. ESO が refreshInterval (既定 1h) で Vault → K8s Secret に同期
4. Reloader annotation のある Deployment は Secret 更新時に rollout

## Reloader Annotation

`stakater/reloader`（`reloader` ns）が ConfigMap / Secret 更新を検知して Deployment / StatefulSet を rollout する。ESO 経由の dynamic rotation を完結させるために必須。

```yaml
metadata:
  annotations:
    reloader.stakater.com/auto: "true"
    # または個別指定:
    secret.reloader.stakater.com/reload: "kensan-db-credentials"
```

## File Safety Rules

- **NEVER commit**: `temp/*-raw.yaml`、`.env`、`*credentials*` の生ファイル
- **Safe to commit**: `*-sealed.yaml`、`ExternalSecret` CR、`VaultStaticSecret` 等の CR
- `temp/*-raw.yaml` は `.gitignore` でブロック

## cert-manager

- **ClusterIssuer**: Let's Encrypt (production + staging)
- **Wildcard 証明書**: `wildcard-platform-tls`（`*.platform.yu-min3.com`）/ `wildcard-apps-tls`（`*.app.yu-min3.com`）
- **Certificate 定義**: `kubernetes/secrets/cert-manager/resources/`
- **DNS-01 IAM cred**: SealedSecret (`route53-credentials`、`cert-manager` ns)
- **Vault PKI は採用していない**: 理由は [ADR-007](../../docs/adr/007-no-vault-pki.md)

## GHCR Pull Secrets

- `ghcr-pull-secret`（`.dockerconfigjson`）は Vault static (ESO) で配布。Vault path: `secret/ghcr/pull-token`
- 各 app ns（`app-prod`、`backstage`、`kensan`、`app-kensan`）が `ExternalSecret` で受け取り、ServiceAccount が `imagePullSecrets: [ghcr-pull-secret]` で参照（`app-kensan` は app-base chart の `externalsecret-ghcr.yaml` 経由）

## Keycloak Authentication

- OIDC IdP: `auth.platform.yu-min3.com`（`platform-auth-prod` ns）
- Gateway-level 認可: Istio Gateway + oauth2-proxy を ext_authz として使う（`auth-system` ns、ADR-010）
- Postgres backing store の cred は Vault dynamic（[ADR-008](../../docs/adr/008-keycloak-db-credentials.md)）
