# Secret Management Guide

This platform uses [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) to securely store sensitive information in the Git repository. This guide explains the procedures for creating and encrypting secrets needed by various components.

## Prerequisites

- `kubeseal` CLI is installed.
- Sealed Secrets controller is deployed to the cluster (`infrastructure/security/sealed-secret/controller.yaml`).
- `kubectl` can connect to the cluster.

## How Sealed Secrets Work

1. Generate a raw Secret (YAML) using `kubectl create secret --dry-run=client`.
2. Pipe the raw Secret to the `kubeseal` CLI for encryption.
3. An encrypted `SealedSecret` resource (YAML) is generated.
4. Commit this `SealedSecret` to Git and deploy it with Argo CD.
5. The Sealed Secrets controller running in the cluster decrypts the `SealedSecret` and creates a regular `Secret` resource.

**Important**: Never commit raw (unencrypted) Secrets to Git. Ensure the `temp/` directory is included in `.gitignore`.

---

## 1. GHCR Image Pull Secret

Authentication credentials for pulling private container images from GitHub Container Registry (GHCR).

Run the following script. Replace the placeholders (`<github-username>`, `<PAT>`, etc.) with your own information before executing.

```bash
./scripts/05-create-ghcr-secret.sh
```

This generates an encrypted `SealedSecret` such as `infrastructure/security/sealed-secret/ghcr-pull-secret-prod.yaml`.

---

## 2. Grafana Admin Password

Set the admin password for Grafana included in the Prometheus stack.

Running the following script generates a random password and saves the encrypted `SealedSecret` to `infrastructure/observability/prometheus/grafana-sealed-secret.yaml`.

```bash
./scripts/06a-create-grafana-secret.sh
```

---

## 3. Backstage Secrets

Set up the PostgreSQL database credentials and GitHub Personal Access Token used by Backstage.

Run the following script. Replace the placeholders (`<strong-password>`, `<github-pat>`) with your own information before executing.

```bash
./scripts/07b-create-backstage-secrets.sh
```

This generates the following two encrypted files:
- `backstage/manifests/base/postgresql-secret.yaml`
- `backstage/manifests/base/backstage-secret.yaml`
