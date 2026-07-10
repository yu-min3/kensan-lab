# Stage 1: Vault HA Bootstrap

Brings up the Vault HA cluster with AWS KMS auto-unseal. The only step that must happen manually before the GitOps deploy is sealing the AWS KMS credentials into a SealedSecret.

## Prerequisites

- An AWS account (KMS Encrypt/Decrypt permission)
- `kubeseal` CLI installed (assumes the Sealed Secrets controller is already deployed from `kubernetes/secrets/sealed-secrets/`)
- A KMS key `alias/vault-unseal-kensan` created in the ap-northeast-1 region

## Vault chart configuration (reference)

Key parameters already defined in `kubernetes/secrets/vault/values.yaml`:

| Item | Value | Notes |
|---|---|---|
| replicas | 3 | spread across nodes via anti-affinity |
| storage | local-path (10Gi × 2 PVC) | to be switched after Longhorn lands ([roadmap](../roadmap/storage-longhorn.md)) |
| seal | AWS KMS auto-unseal | `alias/vault-unseal-kensan` (ap-northeast-1) |
| image tag | `2.0.0` (explicit pin) | [ADR-011](../adr/011-vault-version-pinning.md) |
| Agent Injector | disabled | secrets bridge to K8s Secrets via ESO instead |
| audit device | enabled separately by bootstrap TF | `vault audit enable file path=/vault/audit/audit.log` |

## Manual setup steps

### 1. Create the AWS IAM user

Create an IAM user allowing only KMS Encrypt / Decrypt, and issue an access key.

### 2. Generate the raw secret under temp/

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

`temp/*-raw.yaml` is gitignored. Never commit it.

### 3. Seal with kubeseal

```bash
kubeseal --format=yaml < temp/vault-aws-kms-credentials-raw.yaml \
  > kubernetes/secrets/vault/resources/aws-kms-credentials-sealed.yaml
```

### 4. Commit

Commit only the sealed YAML. Discard the raw file, or leave it in temp/ (git-ignored).

## After deployment

Once the Vault pods are up, run the following via the bootstrap TF (`bootstrap/vault/`):

- `vault operator init`
- `vault audit enable file path=/vault/audit/audit.log`
- Enable the auth methods (kubernetes / oidc)
- Create the root policy

Details: `bootstrap/vault/README.md`.

## Troubleshooting

- Pods stay Sealed and won't join → [vault-raft-join.md](../runbooks/vault-raft-join.md)
- Rotating the AWS KMS credentials → regenerate the SealedSecret, overwrite it in resources/ → ArgoCD sync
