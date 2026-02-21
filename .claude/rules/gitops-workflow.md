---
description: GitOps principles, forbidden operations, and workflow conventions
---

# GitOps Workflow Rules

## Core Principle

ALL infrastructure changes MUST go through Git → Argo CD sync. No exceptions.

## Forbidden Operations

- `kubectl apply` / `kubectl delete` on infrastructure namespaces (except initial bootstrapping)
- `helm install` / `helm upgrade` directly — Argo CD renders charts natively
- Committing rendered Helm manifests (`helm template` output) to Git
- `docker build` / `docker push` — use **Podman** instead

## Change Workflow

1. Edit `values.yaml` or `resources/` files
2. Commit and push to Git
3. Argo CD auto-syncs (or manual sync via UI/CLI)

## App of Apps Pattern

- Root Applications: `infrastructure/gitops/argocd/root-apps/`
- Each root app discovers Application CRs in `infrastructure/gitops/argocd/applications/<category>/`
- New apps added by Backstage auto-commit to `applications/apps/`

## Container Runtime

- **Cluster**: CRI-O
- **Image builds**: Podman (never Docker)
- Backstage Makefile uses Podman by default: `make build TAG=v1.0.0`

## Script Output Rule

When presenting shell commands for the user to copy-paste, write them to a script file in `temp/` directory (e.g., `temp/fix-xyz.sh`) instead of inline text. This prevents line-break corruption in the terminal.

## Infrastructure Dependencies (Deploy Order)

1. Cilium (CNI) — must be first
2. Gateway API CRDs
3. Istio (base → istiod → resources)
4. cert-manager → Certificate resources
5. Sealed Secrets controller → SealedSecret resources
6. Everything else
