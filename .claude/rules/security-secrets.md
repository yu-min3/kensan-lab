---
description: Sealed Secrets workflow, cert-manager, GHCR pull secrets, and sensitive data handling
globs: "**/sealed-secret*, infrastructure/security/**"
---

# Security & Secrets

## Sealed Secrets Workflow

1. Create raw secret → save to `temp/<name>-raw.yaml` (git-ignored)
2. Seal: `kubeseal --format=yaml < temp/<name>-raw.yaml > infrastructure/<cat>/<comp>/resources/<name>-sealed.yaml`
3. Commit sealed YAML (safe to store in Git)
4. Argo CD syncs → Sealed Secrets controller decrypts in-cluster

## File Safety Rules

- **NEVER commit**: `temp/*-raw.yaml`, `.env`, `*credentials*`
- **Safe to commit**: `*-sealed.yaml`, sealed secret YAMLs in `resources/`
- Raw secrets in `temp/` are git-ignored via `.gitignore` pattern `*-raw.yaml`

## cert-manager

- **ClusterIssuer**: Let's Encrypt (production + staging)
- **Certificates**: auto-renewed, defined in `infrastructure/security/cert-manager/resources/`
- Wildcard certs: `wildcard-platform-tls` and `wildcard-apps-tls`
- Manual renewal (rarely needed): delete the cert secret, cert-manager recreates it

## GHCR Pull Secrets

- Each app namespace (`app-dev`, `app-prod`) has `ghcr-pull-secret` (SealedSecret)
- ServiceAccounts reference `imagePullSecrets: [ghcr-pull-secret]`
- Located in `infrastructure/environments/<env>/`

## Keycloak Authentication

- JWT-based auth for prod/dev environments
- Istio RequestAuthentication + AuthorizationPolicy per application
- Prod: `auth.platform.example.com`, Dev: `auth-dev.platform.example.com`

## Placeholder Secrets (Pre-Publication)

All Sealed Secrets currently contain `PLACEHOLDER_ENCRYPTED_DATA_REGENERATE_FOR_YOUR_CLUSTER`. Users must regenerate with their own cluster's sealing key.
