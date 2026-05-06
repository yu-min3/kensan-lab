# Stage 1: Vault HA Bootstrap

Vault HA cluster を AWS KMS auto-unseal で立ち上げる。GitOps デプロイ前に手動で済ませる必要があるのは AWS KMS 認証情報の SealedSecret 化のみ。

## 前提

- AWS アカウント (KMS Encrypt/Decrypt 権限)
- `kubeseal` CLI がインストール済み (Sealed Secrets controller は `infrastructure/security/sealed-secrets/` でデプロイ済み前提)
- ap-northeast-1 リージョンに `alias/vault-unseal-kensan` の KMS key を作成済み

## Vault chart 構成 (参考)

`infrastructure/security/vault/values.yaml` で定義済みの主要パラメータ:

| 項目 | 値 | 補足 |
|---|---|---|
| replicas | 3 | anti-affinity で各ノード分散 |
| storage | local-path (10Gi × 2 PVC) | Longhorn 導入後に切替予定 ([roadmap](../roadmap/storage-longhorn.md)) |
| seal | AWS KMS auto-unseal | `alias/vault-unseal-kensan` (ap-northeast-1) |
| image tag | `2.0.0` (明示 pin) | [ADR-011](../adr/011-vault-version-pinning.md) |
| Agent Injector | disabled | ESO 経由で K8s Secret に bridge する設計 |
| audit device | 別途 bootstrap TF で enable | `vault audit enable file path=/vault/audit/audit.log` |

## 手動セットアップ手順

### 1. AWS IAM user 作成

KMS Encrypt / Decrypt のみ許可した IAM user を作成し、access key を発行する。

### 2. raw secret を temp/ に生成

```bash
cat <<EOF > temp/vault-aws-kms-credentials-raw.yaml
apiVersion: v1
kind: Secret
metadata:
  name: vault-aws-kms-credentials
  namespace: vault
type: Opaque
stringData:
  aws_access_key_id: "<ACCESS_KEY>"
  aws_secret_access_key: "<SECRET_KEY>"
EOF
```

`temp/*-raw.yaml` は `.gitignore` 済み。commit しないこと。

### 3. kubeseal で seal

```bash
kubeseal --format=yaml < temp/vault-aws-kms-credentials-raw.yaml \
  > infrastructure/security/vault/resources/aws-kms-credentials-sealed.yaml
```

### 4. commit

sealed の YAML のみ commit する。raw は破棄するか temp/ に置いたままで OK (git-ignored)。

## デプロイ後の操作

Vault Pod が起動したら、以下を bootstrap TF (`bootstrap/vault/`) で実行する:

- `vault operator init`
- `vault audit enable file path=/vault/audit/audit.log`
- 認証メソッド (kubernetes / oidc) の有効化
- root policy の作成

詳細は `bootstrap/vault/README.md`。

## トラブルシューティング

- Pod が Sealed のまま join できない → [vault-raft-join.md](../runbooks/vault-raft-join.md)
- AWS KMS の認証情報を更新したい → SealedSecret を再生成して resources/ を上書き → ArgoCD sync
