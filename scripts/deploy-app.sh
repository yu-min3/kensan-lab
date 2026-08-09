#!/usr/bin/env bash
# アプリ 1発デプロイ（共通） — GitOps (Git → ArgoCD) を 1 コマンドに畳む。
# apps/kensan/deploy.sh を app 非依存に一般化したもの。
#
#   test → values.yaml tag bump → commit → multi-arch image build/push → push → PR (→ merge)
#   merge 後は ArgoCD(app-<app>) が main を auto-sync して反映する。
#
# 前提とする規約（新しいアプリはこの形に乗せるだけで deploy が手に入る）:
#   apps/<app>/Makefile                     — test / k8s-build TAG=... の 2 ターゲット
#   kubernetes/apps/app-<app>/values.yaml   — image.tag が唯一のバージョン記述
#   ghcr.io/yu-min3/<app>                   — image リポジトリ
#   ns app-<app> / deploy <app>             — 反映確認先
#
# 使い方:
#   scripts/deploy-app.sh konro                # 現タグから patch を +1 して出荷
#   scripts/deploy-app.sh konro v0.2.0         # バージョン明示
#   scripts/deploy-app.sh konro v0.2.0 --merge # PR を squash merge まで実行
#   --yes: 確認プロンプトを省略（エージェント・CI 用）
#
# 前提: docker login ghcr.io 済 / gh CLI 認証済 / アプリのコード変更は commit 済（clean tree）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"      # scripts/
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"

die() { echo "✗ $*" >&2; exit 1; }
step() { echo "▶ $*"; }

# --- args ---
APP="${1:-}"
[[ -n "$APP" && -d "$REPO_ROOT/apps/$APP" ]] || die "usage: deploy-app.sh <app> [vX.Y.Z] [--merge] [--yes]（apps/<app> が存在すること）"
shift

APP_DIR="$REPO_ROOT/apps/$APP"
VALUES="$REPO_ROOT/kubernetes/apps/app-$APP/values.yaml"
IMAGE="ghcr.io/yu-min3/$APP"

VERSION=""
AUTO_MERGE=false
ASSUME_YES=false
for a in "$@"; do
  case "$a" in
    --merge) AUTO_MERGE=true ;;
    --yes) ASSUME_YES=true ;;
    v[0-9]*) VERSION="$a" ;;
    *) die "usage: deploy-app.sh <app> [vX.Y.Z] [--merge] [--yes]" ;;
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
    || die "現タグ '$CUR' から自動 bump 不可。バージョンを明示して: deploy-app.sh $APP vX.Y.Z"
  VERSION="v${MA}.${MI}.$((PA + 1))"
fi
[[ "$VERSION" == "$CUR" ]] && die "新バージョン($VERSION)が現タグと同じ"

echo "─────────────────────────────────────"
echo "  app    : $APP"
echo "  branch : $BRANCH"
echo "  image  : $IMAGE:$CUR  →  $IMAGE:$VERSION"
echo "  merge  : $AUTO_MERGE"
echo "─────────────────────────────────────"
if ! $ASSUME_YES; then
  read -r -p "この内容で出荷する？ [y/N] " ans
  [[ "$ans" == "y" || "$ans" == "Y" ]] || die "中止"
fi

# 1. tests
step "make test"
make -C "$APP_DIR" test

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
git -C "$REPO_ROOT" commit -m "deploy($APP): $VERSION"

# 4. build + push multi-arch (commit 後 = working tree が HEAD と一致した状態で)
step "build & push $IMAGE:$VERSION (linux/amd64,arm64)"
make -C "$APP_DIR" k8s-build TAG="$VERSION"

# 5. push + PR
step "git push + PR"
git -C "$REPO_ROOT" push -u origin "$BRANCH"
gh pr create --base main --head "$BRANCH" --fill 2>/dev/null \
  || echo "  (PR は既存 — そのまま利用)"

# 6. merge (optional) → ArgoCD auto-sync
if $AUTO_MERGE; then
  step "gh pr merge --squash"
  gh pr merge "$BRANCH" --squash --delete-branch=false
  echo "✓ merge 完了。ArgoCD(app-$APP) が main を sync して反映します"
  echo "  確認: kubectl -n app-$APP rollout status deploy/$APP"
else
  echo "✓ PR 作成済み。レビュー → merge で ArgoCD が自動反映"
fi
