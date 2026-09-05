# Stage 1: Vault HA Bootstrap

Bring up the Vault HA cluster with AWS KMS auto-unseal. The only manual step needed before GitOps deployment is sealing the AWS KMS credentials into a SealedSecret.

## Prerequisites

- An AWS account with KMS Encrypt/Decrypt permissions
- `kubeseal` CLI installed (assumes the Sealed Secrets controller is already deployed via `kubernetes/secrets/sealed-secrets/`)
- A KMS key `alias/vault-unseal-kensan` already created in ap-northeast-1

## Vault chart configuration (reference)

Key parameters already defined in `kubernetes/secrets/vault/values.yaml`:

| Item | Value | Notes |
|---|---|---|
| replicas | 3 | Spread across nodes via anti-affinity |
| storage | local-path (10Gi × 2 PVC) | Planned switch after Longhorn adoption ([roadmap](../roadmap/storage-longhorn.md)) |
| seal | AWS KMS auto-unseal | `alias/vault-unseal-kensan` (ap-northeast-1) |
| image tag | `2.0.0` (explicit pin) | [ADR-011](../adr/011-vault-version-pinning.md) |
| Agent Injector | disabled | Design bridges to K8s Secrets via ESO instead |
| audit device | enabled separately via bootstrap Terraform | `vault audit enable file path=/vault/audit/audit.log` |

## Manual setup steps

### 1. Create an AWS IAM user

Create an IAM user restricted to KMS Encrypt / Decrypt only, and issue an access key.

### 2. Generate the raw secret into temp/

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

`temp/*-raw.yaml` is already `.gitignore`d. Don't commit it.

### 3. Seal it with kubeseal

```bash
kubeseal --format=yaml < temp/vault-aws-kms-credentials-raw.yaml \
  > kubernetes/secrets/vault/resources/aws-kms-credentials-sealed.yaml
```

### 4. Commit

Commit only the sealed YAML. The raw file can be discarded or left in `temp/` (it's git-ignored either way).

## Post-deploy operations

Once the Vault pods are up, run the following via bootstrap Terraform (`bootstrap/vault/`):

- `vault operator init`
- `vault audit enable file path=/vault/audit/audit.log`
- Enable the auth methods (kubernetes / oidc)
- Create the root policy

See `bootstrap/vault/README.md` for details.

## Troubleshooting

- Pod stuck Sealed, won't join → [vault-raft-join.md](../runbooks/vault-raft-join.md)
- Need to rotate the AWS KMS credentials → regenerate the SealedSecret, overwrite the file in `resources/`, let Argo CD sync
