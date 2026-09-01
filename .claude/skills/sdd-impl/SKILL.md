---
name: sdd-impl
description: SDD phase 4 — 仕様 (specs/NNN-<slug>) を選んで worktree で自律実装し、必須の検証フェーズ（静的ゲート + ライブ検証）を緑にしてから draft PR を作成する
argument-hint: [feature-name]
disable-model-invocation: true
allowed-tools: Bash, Read, Edit, Write, Grep, Glob, Skill
---

# sdd-impl — 自律実装 + 必須検証 + draft PR (infra)

選択した仕様を worktree 内で自律実装し、**検証フェーズを緑にしてから** draft PR を作成して引き渡す。SDD パイプラインの phase 4。

参照: [`specs/README.md`](../../../specs/README.md) / [`gitops-workflow.md`](../../rules/gitops-workflow.md) / [`security-secrets.md`](../../rules/security-secrets.md)

> **絶対ルール**: 検証フェーズの静的ゲートが全緑になるまで「完了」と宣言しない。検証していない項目を「検証済み」と書かない。緑になっても作るのは **draft PR**（ready 化・merge は人間）。

> **「完了の定義 (DoD)」は spec が持つ**: このスキルが「止まる前に必ず満たすゴール」として参照するのは、選んだ **`spec.md` の `## Acceptance criteria` ＋ `tasks.md` の `## Verification`**。Step 1 でこれを DoD として明示的に取り込み、全項目を「満たした / cluster 未到達で merge 後確認（理由付き）」のどちらかに仕分けるまで「完了」と宣言しない。せっかく作った spec がそのまま停止ゲートの中身になる。

> **組み込み `/goal` との関係**: Claude Code 組み込みの `/goal`（"set a goal Claude checks before stopping"）は同じ「停止前確認」を harness 層で行う別レイヤー。ただし **スキルから組み込み `/goal` をプログラム的にセットすることはできない**（ユーザーが手で打つ UI 機能）。そこでこのスキルは、Step 1 で組み立てた DoD から**そのまま貼れる `/goal` 文字列を提示**する（貼るかはユーザー任意）。貼れば harness 層の番人も立つし、貼らなくても DoD ゲート（Step 4-D / 5）で同じ規律を守る。

---

## Step 1 — 仕様を選択 + 完了の定義 (DoD) を組み立てる

- `$ARGUMENTS[0]` があれば `specs/NNN-<slug>/` を部分一致で特定。
- 無ければ `tasks.md` を持つ `specs/*/` を一覧提示し、ユーザーに選ばせる。
- `tasks.md` が無いディレクトリは**拒否**（「先に `/sdd-spec` → `/sdd-plan` → `/sdd-tasks` を」と案内）。
- `spec.md` / `plan.md` / `tasks.md` を読み込み、特に `plan.md` の `## Affected paths` を把握。
- **DoD を組み立てる**: `spec.md` の `## Acceptance criteria` の各項目 ＋ `tasks.md` の `## Verification` を合わせて、このランで満たすべきチェックリストにする。これが「止まる前に必ず確認するゴール」。実装中・検証中つねにこの DoD に立ち返る。
- **`/goal` 文字列を提示**: DoD を 1 行に要約した、ユーザーがそのまま貼れる組み込み `/goal` 用の文字列を出す。例:
  ```
  /goal <NNN-slug>: 静的ゲート全緑 + <spec の受入基準を列挙> + draft PR 作成
  ```
  「組み込み `/goal` にこれを貼ると harness 側でも停止前確認が効きます（任意）」と一言添える。

## Step 2 — worktree を作成

[`gitops-workflow.md`](../../rules/gitops-workflow.md) の必須制約を遵守。**シェルの `git worktree add` を使う**（harness の EnterWorktree は使わない。`~/` 直下 = workspace 同期外に置く）。

```bash
git fetch origin
git worktree add ~/kensan-lab.worktrees/<NNN-slug> -b feat/<NNN-slug> origin/main
```

以後はこの worktree を**絶対パス**で操作する（メイン working tree は触らない）。

## Step 3 — tasks.md を実装

`tasks.md` の `## Implementation` を**上から順に**実装し、完了タスクの `- [ ]` を `- [x]` に更新していく。

- 3-file パターン（`helm-multisource.md`）・既存コンポーネントの慣習に従う。
- **secrets**（`security-secrets.md` 準拠、self-contained に実施）:
  - SealedSecret: raw を `temp/<name>-raw.yaml`（git-ignored）に作る → `kubeseal --format=yaml < temp/<name>-raw.yaml > .../resources/<name>-sealed.yaml`。**raw は絶対 commit しない**。
  - ExternalSecret: `ExternalSecret` CR を `resources/` に配置（`ClusterSecretStore: vault-backend` 参照）。
- 論理的な区切りで commit（worktree 内）。コミットメッセージは日本語・1 行目 50 字以内。

## Step 4 — 検証フェーズ（MANDATORY）

`plan.md` の `## Affected paths` ∪ `git -C ~/kensan-lab.worktrees/<NNN-slug> diff --name-only origin/main...HEAD`（+ 未ステージ）で対象ファイルを確定し、以下を順に実行。失敗したら **Step 4-C** のループへ。

### 4-A. 静的ゲート（cluster 不要・常時実行）

1. **yamllint** — 触った YAML に対して。未インストールなら best-effort で `pip install yamllint` か `brew install yamllint` を試行。それも不可なら警告を出して継続（このゲートは skip 扱いと明記）。
   ```bash
   yamllint <変更した .yaml ファイル群>
   ```
2. **helm template（render dry-run）** — Pattern A の場合、chart が values で正しくレンダリングされるか確認。**出力は stdout / `temp/` のみ。レンダリング結果は絶対 commit しない**（`gitops-workflow.md`）。
   ```bash
   helm repo add <name> <chart-repo> >/dev/null 2>&1; helm repo update >/dev/null
   helm template <release> <name>/<chart> --version <ver> \
     -f kubernetes/<cat>/<comp>/values.yaml > temp/<comp>-rendered.yaml
   ```
3. **kubeconform（schema 検証）** — render 出力 + `resources/*.yaml` + `app.yaml` を検証。
   ```bash
   kubeconform -ignore-missing-schemas -kubernetes-version <K8s ver> \
     temp/<comp>-rendered.yaml kubernetes/<cat>/<comp>/resources/*.yaml
   ```
4. **Application CR の dry-run** — Argo CD Application の形式検証。
   ```bash
   kubectl apply --dry-run=client -f kubernetes/argocd/applications/<cat>/<comp>/app.yaml
   ```
5. **安全チェック**:
   - 生 Secret 混入検出: staged YAML に `kind: Secret`（`SealedSecret`/`ExternalSecret` 以外）や `temp/*-raw.yaml`・`.env`・`*credentials*` の生ファイルが無いこと。
   - **version 固定**: `app.yaml` の `targetRevision`（chart 側）が `main`/`latest`/`HEAD` でないこと。

### 4-B. ライブ検証（cluster 到達時のみ）

`kubectl cluster-info` で到達確認。到達するなら（ユーザー許可済み）:

6. **server dry-run** — 実 API サーバ + CRD + admission に対する非破壊検証（最強の静的ゲート）。
   ```bash
   kubectl apply --dry-run=server -f kubernetes/<cat>/<comp>/resources/
   ```
7. （必要時のみ・既定では行わず提案）dry-run で拾えない検証が要るなら、scratch namespace へ一時 apply → 確認 → **即 delete**。

到達**しない**場合: ライブ検証を**黙ってスキップせず**「cluster 未到達のため 4-B は未実行」と明記し、Step 5 の runbook に委ねる。

### 4-C. fix-until-green ループ

ゲートが失敗したらエラーを解析し**ソースを修正**して当該ゲートを再実行する。**回避目的でテスト/検証を握り潰さない**。同一ゲートで 3 回直しても緑にできなければ、状況を整理して人間レビューにフラグを立てる（勝手に「完了」にしない）。

### 4-D. 完了ゲート（DoD に対して）

Step 1 で組み立てた **DoD を 1 項目ずつ照合**する。先へ進めるのは次を満たしたときだけ:

- 静的ゲート（4-A）が**全緑**。
- `spec.md` の `## Acceptance criteria` の各項目が「ローカル/ライブ検証で満たした」か「cluster 未到達等で merge 後に確認（理由付き）」のどちらかに**明確に仕分け**られている。未仕分け・未確認のまま「完了」にしない。

`tasks.md` の `## Verification` チェックを更新し、受入基準を 1 項目ずつ状態付きで控える（Step 5 の PR body に転記する）。

## Step 5 — draft PR を作成して引き渡し

静的ゲート（4-A）が全緑になったら、worktree から push して **draft PR** を作る:

```bash
cd ~/kensan-lab.worktrees/<NNN-slug>
git push -u origin feat/<NNN-slug>
gh pr create --draft \
  --title "feat(<comp>): <spec のタイトル>" \
  --body "$(cat <<'EOF'
## Spec
specs/<NNN-slug>/ (spec.md / plan.md / tasks.md)

## 検証
- 静的ゲート: yamllint / helm template / kubeconform / kubectl dry-run=client / secret・version check → 結果を記載
- ライブ検証: kubectl dry-run=server → 実行有無を記載
- 未検証で merge 後に確認が要る Acceptance criteria を列挙

## merge 後 runbook
- /argocd-sync <component>   # Synced + Healthy
- /cluster-status            # pod / gateway / cert
EOF
)"
```

- **draft** で作る（`--draft`）。ready 化・merge は人間が判断する。
- PR body には検証結果サマリ・`spec.md` の Acceptance criteria（ローカル確認済み / merge 後確認）・merge 後 runbook を必ず含める。
- 作成後、**PR の URL を提示**して停止する。`git diff --stat origin/main` も併せて見せる。
- spec.md の `status` を `implemented` に更新してよい。
- worktree はレビュー中残す（merge 後に `git worktree remove ~/kensan-lab.worktrees/<NNN-slug>` で片付け）。

> push / draft PR まではこのスキルが行うが、**ready 化と merge は人間**。検証が緑にできなかった場合は PR を作らず、状況を整理して人間にフラグを立てる。
