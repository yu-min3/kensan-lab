---
description: GitOps principles, forbidden operations, and workflow conventions
---

# GitOps Workflow Rules

## Core Principle

ALL infrastructure changes MUST go through Git → Argo CD sync. 例外は initial bootstrapping と下記「Verification Exception」（push 前の一時適用による動作確認）のみ。

## Forbidden Operations

- `kubectl apply` / `kubectl delete` on infrastructure namespaces (except initial bootstrapping。`kubectl apply` のみ下記 Verification Exception でも可。`kubectl delete` は検証リソースの後片付けに限る)
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

## Prune Protection

- `argocd.argoproj.io/sync-options: Prune=false` は **守りたいリソース個別の manifest に付ける**。Application メタデータに付けても子リソースは守られない（PR #366 で kensan-data PVC が prune された実証）。Application 側の annotation は「root-app が Application CR 自体を prune するのを防ぐ」効果のみ
- 個別 annotation 済み: clusterwide CCNP 4 本、longhorn の SC/RecurringJob/NFS Service、app-kensan の workspace PVC

## Change Workflow

1. Worktree を切って branch を作る (上記)
2. Edit `values.yaml` or `resources/` files
3. 動作確認が必要な変更は push 前に検証する (下記 Verification Exception。raw manifest は一時適用、`values.yaml` はローカル render 確認)
4. Commit and push to Git
5. Argo CD auto-syncs (or manual sync via UI/CLI)

## Verification Exception（push 前の一時適用による動作確認）

K8s 変更で動作確認が必要な場合に限り、**push 前の検証**を許可する。「push してから壊れる」を防ぐための例外であり、恒久的な手動運用を許すものではない。

### 対象別の検証方法

| 変更対象 | 検証方法 |
|---|---|
| raw manifest（`resources/` 等） | `kubectl apply` で一時適用して実機確認（下記手順） |
| `values.yaml` | `kubectl apply` では検証不可。ローカルで `helm template` を実行して render 結果を確認する（**検証目的のローカル実行は可、出力の commit は引き続き禁止**）。実機確認が必要なら render 結果から該当 manifest を抜き出して一時適用する |

### 手順

1. worktree 上で編集
2. `kubectl apply` で一時適用して動作確認（pod 起動・ログ・疎通など）
3. 確認 OK → commit → push → PR。Argo CD sync で正式反映され、一時適用分は Git と一致する
4. 確認 NG / push を取りやめる場合 → **Git の状態に必ず巻き戻す**。ただし sync は万能ではない:
   - 既存リソースへの変更 → Argo CD sync（または selfHeal）で戻る
   - 検証で**新規作成**したリソース → sync では削除されない（特に `prune: false` の app）。`kubectl delete` で自分で片付ける（Forbidden Operations の delete 例外はこの後片付けのみ）

**禁止**: 一時適用したまま push せず放置すること。一時適用は「直後に push する」か「Git 状態へ巻き戻す（新規分は delete で後片付け）」かの 2 択で必ず収束させる。

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

When presenting shell commands for the user to run, write them to a script file in `temp/` directory (e.g., `temp/fix-xyz.sh`) instead of inline text, and make it executable (`chmod +x`) so the user can run it directly (`./temp/fix-xyz.sh`). This prevents line-break corruption in the terminal.

## Infrastructure Dependencies (Deploy Order)

1. Cilium (CNI) — must be first
2. Gateway API CRDs
3. Istio (base → istiod → resources)
4. cert-manager → Certificate resources
5. Sealed Secrets controller → SealedSecret resources
6. Everything else
