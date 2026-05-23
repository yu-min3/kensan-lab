#!/bin/bash
set -euo pipefail

# Usage: scripts/k8s-create-pr.sh <dev|prod|all> <TAG>
# kensan の flat manifest (manifests/services/*.yaml, manifests/lakehouse/dagster-*.yaml)
# の image tag を更新し、 main から分岐した deploy ブランチで PR を作成する。
#
# 注: kustomize overlay は PR #305 で廃止 (dev/prod 分離撤去)。
# dev/prod/all のいずれを指定しても同じ flat manifest を更新する (歴史的互換性のため引数は残す)。

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

# --- Update image tags in flat manifests ---
# 各 (service, image) ペアを (file, image-name) で表現。
# polaris-init-job.yaml は :latest 運用なので対象外。
declare -a TARGETS=(
  "manifests/services/frontend.yaml:kensan-frontend"
  "manifests/services/user-service.yaml:kensan-user"
  "manifests/services/task-service.yaml:kensan-task"
  "manifests/services/timeblock-service.yaml:kensan-timeblock"
  "manifests/services/analytics-service.yaml:kensan-analytics"
  "manifests/services/memo-service.yaml:kensan-memo"
  "manifests/services/note-service.yaml:kensan-note"
  "manifests/services/kensan-ai.yaml:kensan-ai"
  "manifests/lakehouse/dagster-webserver.yaml:kensan-dagster"
  "manifests/lakehouse/dagster-daemon.yaml:kensan-dagster"
  "manifests/lakehouse/dagster-user-code.yaml:kensan-dagster"
)

UPDATED=0
for entry in "${TARGETS[@]}"; do
  file="${entry%%:*}"
  image="${entry##*:}"
  if [ ! -f "$file" ]; then
    echo "WARNING: $file not found, skipping"
    continue
  fi
  # ghcr.io/yu-min3/<image>:<old> -> ghcr.io/yu-min3/<image>:<TAG>
  if grep -q "image: ghcr.io/yu-min3/${image}:" "$file"; then
    sed -i '' "s|image: ghcr.io/yu-min3/${image}:.*|image: ghcr.io/yu-min3/${image}:${TAG}|g" "$file"
    git add "$file"
    echo "Updated $file -> ghcr.io/yu-min3/${image}:${TAG}"
    UPDATED=$((UPDATED + 1))
  else
    echo "WARNING: no image: ghcr.io/yu-min3/${image}: in $file, skipping"
  fi
done

if [ "$UPDATED" -eq 0 ]; then
  echo "ERROR: no image tags updated"
  git checkout "$ORIGINAL_BRANCH"
  git branch -D "$BRANCH"
  exit 1
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
- \`ghcr.io/yu-min3/kensan-dagster:${TAG}\`

## Deploy
Merge this PR -> Argo CD auto-sync"

PR_URL=$(gh pr create --title "$TITLE" --body "$BODY" 2>&1)

# --- Return to original branch ---
echo ""
echo "Switching back to $ORIGINAL_BRANCH..."
git checkout "$ORIGINAL_BRANCH"
echo ""
echo "PR created: $PR_URL"
