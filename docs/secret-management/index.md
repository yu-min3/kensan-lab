# Secret Management Guide

This platform uses [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) to securely store sensitive information in the Git repository.

## How Sealed Secrets Work

1. Generate a raw Secret (YAML) using `kubectl create secret --dry-run=client`.
2. Pipe the raw Secret to the `kubeseal` CLI for encryption.
3. An encrypted `SealedSecret` resource (YAML) is generated.
4. Commit this `SealedSecret` to Git and deploy it with Argo CD.
5. The Sealed Secrets controller running in the cluster decrypts the `SealedSecret` and creates a regular `Secret` resource.

**Important**: Never commit raw (unencrypted) Secrets to Git.

## Prerequisites

- `kubeseal` CLI installed
- Sealed Secrets controller deployed (`infrastructure/security/sealed-secrets/`)
- `kubectl` configured to connect to the cluster

## Creating a Sealed Secret

```bash
# 1. Create a raw secret (dry-run)
kubectl create secret generic <secret-name> \
  --namespace <namespace> \
  --from-literal=<key>=<value> \
  --dry-run=client -o yaml > /tmp/secret.yaml

# 2. Encrypt with kubeseal
kubeseal --format yaml < /tmp/secret.yaml > <output-path>/sealed-secret.yaml

# 3. Clean up the raw secret
rm /tmp/secret.yaml
```

## Managed Secrets

Sealed Secrets are stored alongside the components that use them:

| Component | Location |
|-----------|----------|
| GHCR pull secrets | `infrastructure/environments/<env>/ghcr-pull-secret.yaml` |
| Grafana | `infrastructure/observability/grafana/resources/` |
| AlertManager (Slack) | `infrastructure/observability/prometheus/resources/` |
| Backstage | `backstage/manifests/base/` |
| Keycloak | `infrastructure/security/keycloak/overlays/` |
| Cloudflare Tunnel | `infrastructure/network/cloudflare-tunnel/resources/` |
