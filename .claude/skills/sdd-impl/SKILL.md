---
name: sdd-impl
description: Full/Lite SDDを隔離worktreeで実装・検証・独立レビューしdraft PR手前まで引き渡す
argument-hint: <feature-name>
disable-model-invocation: true
---

# SDD implementation gate

Claude Codeだけがworktree、ファイル変更、task状態、commitを所有する。Codexはread-only reviewerであり、変更やcommitを行わない。

## 1. Preconditions

- `spec.md`のmodeを読む。Fullは`plan.md`、`tasks.md`、`reviews/codex-pre-impl.md`必須。Liteはspecのpre-implementation review必須で、plan/tasksは要求しない。
- placeholder、clarification、未解決P0、未裁定findingがあれば停止する。
- `AGENTS.md -> CLAUDE.md` symlinkを確認する。
- `python3 scripts/sdd_gate.py validate specs/NNN-<slug> --stage implement`が成功しなければ停止する。

## 2. Isolation and implementation

- 最新mainを起点に、workspace同期対象外の`~/kensan-lab.worktrees/<NNN-slug>`または`/private/tmp/`へ専用worktreeを作る。他のworktreeを編集しない。
- Fullはtasksを上から実装してcheckboxを更新する。Liteはspecのacceptance criteriaをDoDとして実装する。
- secret、rendered manifest、`.env`をcommitしない。論理的な区切りでcommitする。

## 3. Verification

- `scripts/validate-manifests.sh all`を実行する。依存commandが無ければインストールで環境を変えず、missing dependencyとして停止またはCI委譲を明示する。
- 変更に外部chartや追加renderがあればplan固有の検証を追加する。
- cluster到達時は対象raw manifestまたは一時renderに`kubectl apply --dry-run=server`を実行する。renderは`temp/`に置きcommitしない。未到達は理由とmerge後checkを記録する。
- 同じgateを3回修正しても成功しなければ停止する。
- 検証のたびに`spec.md`の各acceptance criterionを`pending` / `verified` / `deferred` / `failed`のいずれかへ更新する。`verified`は証跡、`deferred`は`Reason: ...; Next: ...`として理由と次の検証点を同じ行へ記録し、tasksへ状態表を複製しない。

## 4. Independent diff review

- Codexをread-onlyでbase branchとの差分レビューに使い、P0/P1/P2、plan/specとの差、未知の削除、`Prune=false`、secret、multi-arch、GitOps bypass、検証不足を確認する。
- 出力は`temp/`からClaudeが`reviews/codex-impl.md`へ転記し、全findingを裁定する。P0未解決ならhandoffを作らない。P1は未決事項へ載せる。
- Codex不能時は自動続行せず、Yu承認のhuman fallbackを記録する。

## 5. Human handoff

既存pull request templateへspec/plan/検証を投影した本文を`temp/`に作る。追加・変更・削除（なしも明記）、到達経路、検証の実行/skip、acceptance criteria、merge behavior、rollback、未決P1を含める。

`python3 scripts/sdd_gate.py validate specs/NNN-<slug> --stage handoff`を実行する。acceptance criteriaに`pending` / `failed`または証跡のない`verified` / `deferred`があれば停止し、成功後だけhandoffを提示する。

このworkspaceではremote操作を行わない。local commit、diff、handoffをYuへ提示して停止する。ready化・mergeを行う指示はこのskillに含めない。
