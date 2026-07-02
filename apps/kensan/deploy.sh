#!/usr/bin/env bash
# kensan 1発デプロイ — GitOps (Git → ArgoCD) を 1 コマンドに畳む。
#
#   test → values.yaml tag bump → commit → multi-arch image build/push → push → PR (→ merge)
#   merge 後は ArgoCD(app-kensan) が main を auto-sync して反映する。
#
# 使い方:
#   ./deploy.sh                # 現タグから patch を +1 して出荷
#   ./deploy.sh v0.2.0         # バージョン明示
#   ./deploy.sh v0.2.0 --merge # PR を squash merge まで実行
#
# 前提: docker login ghcr.io 済 / gh CLI 認証済 / アプリのコード変更は commit 済（clean tree）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"      # apps/kensan
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"
VALUES="$REPO_ROOT/kubernetes/apps/app-kensan/values.yaml"
IMAGE="ghcr.io/yu-min3/kensan"

die() { echo "✗ $*" >&2; exit 1; }
step() { echo "▶ $*"; }

# --- args ---
VERSION=""
AUTO_MERGE=false
for a in "$@"; do
  case "$a" in
    --merge) AUTO_MERGE=true ;;
    v[0-9]*) VERSION="$a" ;;
    *) die "usage: deploy.sh [vX.Y.Z] [--merge]" ;;
  esac
done

# --- preflight ---
command -v gh >/dev/null      || die "gh CLI が必要"
docker buildx version >/dev/null 2>&1 || die "docker buildx が必要"
[[ -f "$VALUES" ]]           || die "values.yaml が見つからない: $VALUES"
# 未追跡ファイルも含めて完全 clean を要求（untracked が image に入って git に無い不整合を防ぐ）
[[ -z "$(git -C "$REPO_ROOT" status --porcelain)" ]] \
  || die "working tree が dirty（未追跡含む）。アプリのコード変更を先に全て commit してから実行"

BRANCH="$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)"
CUR="$(grep -E '^[[:space:]]*tag:' "$VALUES" | head -1 | awk '{print $2}')"

# --- derive version (auto patch bump) ---
if [[ -z "$VERSION" ]]; then
  base="${CUR#v}"; IFS='.' read -r MA MI PA <<<"$base"
  [[ "$MA" =~ ^[0-9]+$ && "$MI" =~ ^[0-9]+$ && "$PA" =~ ^[0-9]+$ ]] \
    || die "現タグ '$CUR' から自動 bump 不可。バージョンを明示して: ./deploy.sh vX.Y.Z"
  VERSION="v${MA}.${MI}.$((PA + 1))"
fi
[[ "$VERSION" == "$CUR" ]] && die "新バージョン($VERSION)が現タグと同じ"

echo "─────────────────────────────────────"
echo "  branch : $BRANCH"
echo "  image  : $IMAGE:$CUR  →  $IMAGE:$VERSION"
echo "  merge  : $AUTO_MERGE"
echo "─────────────────────────────────────"
read -r -p "この内容で出荷する？ [y/N] " ans
[[ "$ans" == "y" || "$ans" == "Y" ]] || die "中止"

# 1. tests
step "make test"
make -C "$SCRIPT_DIR" test

# 2. bump image.tag
step "bump values.yaml tag → $VERSION"
if command -v yq >/dev/null; then
  yq -i ".image.tag = \"$VERSION\"" "$VALUES"
else
  sed -i.bak -E "s|^([[:space:]]*tag:[[:space:]]*).*|\1$VERSION|" "$VALUES" && rm -f "$VALUES.bak"
fi
grep -qE "tag:[[:space:]]*$VERSION" "$VALUES" || die "tag の書き換えに失敗"

# 3. commit tag bump (working tree は clean だったので values.yaml の 1 ファイルだけ)
step "git commit"
git -C "$REPO_ROOT" add "$VALUES"
git -C "$REPO_ROOT" commit -m "deploy(kensan): $VERSION"

# 4. build + push multi-arch (commit 後 = working tree が HEAD と一致した状態で)
step "build & push $IMAGE:$VERSION (linux/amd64,arm64)"
make -C "$SCRIPT_DIR" k8s-build TAG="$VERSION"

# 5. push + PR
step "git push + PR"
git -C "$REPO_ROOT" push -u origin "$BRANCH"
gh pr create --base main --head "$BRANCH" --fill 2>/dev/null \
  || echo "  (PR は既存 — そのまま利用)"

# 6. merge (optional) → ArgoCD auto-sync
if $AUTO_MERGE; then
  step "gh pr merge --squash"
  gh pr merge "$BRANCH" --squash --delete-branch=false
  echo "✓ merge 完了。ArgoCD(app-kensan) が main を sync して反映します"
  echo "  確認: kubectl -n app-kensan rollout status deploy/kensan"
else
  echo "✓ PR 作成済み。レビュー → merge で ArgoCD が自動反映"
fi
