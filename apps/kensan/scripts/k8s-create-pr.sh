#!/bin/bash
set -euo pipefail

# Usage: scripts/k8s-create-pr.sh <dev|prod|all> <TAG>
# kustomization.yaml の newTag を更新し、main から分岐した deploy ブランチで PR を作成する。

ENV="${1:?Usage: k8s-create-pr.sh <dev|prod|all> <TAG>}"
TAG="${2:?Usage: k8s-create-pr.sh <dev|prod|all> <TAG>}"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# --- Validate ---
if [ "$ENV" != "dev" ] && [ "$ENV" != "prod" ] && [ "$ENV" != "all" ]; then
  echo "ERROR: ENV must be dev, prod, or all (got: $ENV)"
  exit 1
fi

if ! command -v gh &>/dev/null; then
  echo "ERROR: gh CLI is required. Install: brew install gh"
  exit 1
fi

# --- Save current branch ---
ORIGINAL_BRANCH=$(git rev-parse --abbrev-ref HEAD)

# --- Create deploy branch from main ---
BRANCH="deploy/${ENV}-${TAG}"
echo "Fetching latest main..."
git fetch origin main
echo "Creating branch: $BRANCH (from origin/main)"
git checkout -b "$BRANCH" origin/main

# --- Update newTag ---
update_tag() {
  local overlay="$1"
  local file="manifests/overlays/${overlay}/kustomization.yaml"
  if [ ! -f "$file" ]; then
    echo "ERROR: $file not found"
    git checkout "$ORIGINAL_BRANCH"
    git branch -D "$BRANCH"
    exit 1
  fi
  sed -i '' "s/newTag: .*/newTag: ${TAG}/" "$file"
  git add "$file"
  echo "Updated $file -> newTag: $TAG"
}

if [ "$ENV" = "dev" ] || [ "$ENV" = "all" ]; then
  update_tag dev
fi
if [ "$ENV" = "prod" ] || [ "$ENV" = "all" ]; then
  update_tag prod
fi

# --- Commit & Push ---
case "$ENV" in
  dev)  SCOPE="dev"  ;;
  prod) SCOPE="prod" ;;
  all)  SCOPE="dev/prod" ;;
esac

git commit -m "deploy(${SCOPE}): update kensan image tag to ${TAG}"
git push -u origin "$BRANCH"

# --- Create PR ---
case "$ENV" in
  dev)  TITLE="deploy(dev): kensan ${TAG}" ;;
  prod) TITLE="deploy(prod): kensan ${TAG}" ;;
  all)  TITLE="deploy: kensan ${TAG}" ;;
esac

BODY="## Summary
- Update kensan image tags to \`${TAG}\` for **${SCOPE}** environment(s)

## Images
- \`ghcr.io/yu-min3/kensan-frontend:${TAG}\`
- \`ghcr.io/yu-min3/kensan-user:${TAG}\`
- \`ghcr.io/yu-min3/kensan-task:${TAG}\`
- \`ghcr.io/yu-min3/kensan-timeblock:${TAG}\`
- \`ghcr.io/yu-min3/kensan-analytics:${TAG}\`
- \`ghcr.io/yu-min3/kensan-memo:${TAG}\`
- \`ghcr.io/yu-min3/kensan-note:${TAG}\`
- \`ghcr.io/yu-min3/kensan-ai:${TAG}\`

## Deploy
Merge this PR -> Argo CD auto-sync"

PR_URL=$(gh pr create --title "$TITLE" --body "$BODY" 2>&1)

# --- Return to original branch ---
echo ""
echo "Switching back to $ORIGINAL_BRANCH..."
git checkout "$ORIGINAL_BRANCH"
echo ""
echo "PR created: $PR_URL"
