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
- 画像 build に single-arch tag を push しない — multi-arch (linux/amd64 + linux/arm64) manifest list で push する (Pi5 + amd64 worker 混在のため)

## Change Workflow

1. Edit `values.yaml` or `resources/` files
2. Commit and push to Git
3. Argo CD auto-syncs (or manual sync via UI/CLI)

## App of Apps Pattern

- Root Applications: `kubernetes/argocd/root-apps/`
- Each root app discovers Application CRs in `kubernetes/argocd/applications/<category>/`
- New apps added by Backstage auto-commit to `applications/apps/`

## Container Runtime

- **Cluster**: CRI-O
- **Image builds**: Docker buildx (multi-arch、 `linux/amd64,linux/arm64` manifest list を default)
  - 各 Makefile に `CONTAINER_RUNTIME ?= docker` 変数あり、 `make ... CONTAINER_RUNTIME=podman` で podman 切替も可
  - `apps/kensan/Makefile` の `k8s-build-*` は `docker buildx build --platform=linux/amd64,linux/arm64 --push` で build + GHCR push を atomic に
- Backstage / kensan アプリ image: `make build TAG=v1.0.0`

## Script Output Rule

When presenting shell commands for the user to copy-paste, write them to a script file in `temp/` directory (e.g., `temp/fix-xyz.sh`) instead of inline text. This prevents line-break corruption in the terminal.

## Infrastructure Dependencies (Deploy Order)

1. Cilium (CNI) — must be first
2. Gateway API CRDs
3. Istio (base → istiod → resources)
4. cert-manager → Certificate resources
5. Sealed Secrets controller → SealedSecret resources
6. Everything else
