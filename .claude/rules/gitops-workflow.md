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
- **inner repo の main / 並行作業 branch で直接 commit する作業をしない** — 必ず worktree を切って独立した working tree で作業する (下記参照)

## Worktree-based Development

開発 (新規 feature / fix / refactor) は **必ず git worktree** で行う。並行作業中の branch と working tree を共有しない。

### Path 規約

```
~/kensan-lab.worktrees/<short-feature-name>
```

例: `~/kensan-lab.worktrees/vault-transit-chartify`, `~/kensan-lab.worktrees/longhorn-phase1`

### Workflow

```bash
# 1. origin から最新を fetch
git fetch origin

# 2. worktree を origin/main から派生して作成 (新 branch も同時に)
git worktree add ~/kensan-lab.worktrees/<name> -b <branch-name> origin/main

# 3. その worktree dir で作業 / commit
cd ~/kensan-lab.worktrees/<name>
# ... edit, commit ...

# 4. push + PR (worktree dir から)
git push -u origin <branch-name>
gh pr create ...

# 5. PR merge 後、worktree を片付ける
cd <元 repo の path>
git worktree remove ~/kensan-lab.worktrees/<name>
```

### なぜ worktree を必須にするか

- 並行作業（同じ inner repo で複数 branch が走る）時、`git checkout` で branch を切替えると working tree の状態が壊れる
- 特に他の作業者 (Yu 含む) が同じ inner repo を別端末から触ってる時、checkout の race で fatal な事故が起きる (PR #340 で実際に発生、stash + branch 再作成で復旧)
- worktree なら branch ごとに独立した working tree が確保され、checkout 不要で並行作業可能

### 例外

- 一行 typo fix のように main 直 commit でも安全な極小修正で、かつ並行作業が無いと確信できる場合
- ただし迷ったら worktree を切る (cost が低いので、不要な事故より worktree のオーバーヘッドを優先)

## Change Workflow

1. Worktree を切って branch を作る (上記)
2. Edit `values.yaml` or `resources/` files
3. Commit and push to Git
4. Argo CD auto-syncs (or manual sync via UI/CLI)

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
