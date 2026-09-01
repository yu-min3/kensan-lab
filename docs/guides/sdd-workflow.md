# Spec-driven development workflow

kensan-lab の変更は、影響と不可逆性に応じて Full SDD、Lite、SDD 不要に分類する。目的は文書を増やすことではなく、実装前に受入基準を固定し、独立レビューと検証の停止条件に使うことにある。

## Operating model

1. Claude Code が SDD 適用判定、spec、plan、実装、検証、指摘の裁定を所有する。
2. Codex は read-only で plan と diff を独立レビューする。指摘は自動適用しない。
3. Claude Code は draft PR までを準備し、ready 化と merge は人間が判断する。

要求、計画、進捗、レビュー結果は `specs/NNN-<slug>/` に保存する。会話履歴やターミナル出力は状態の正としない。

Acceptance criteriaは`spec.md`の表を唯一の正とする。各行は一意な`AC-N`、観測可能な条件、`pending` / `verified` / `deferred` / `failed`、証跡またはdefer理由を持つ。実装中は状態を直接更新し、`tasks.md`へ別のstatus表を作らない。handoffでは`pending` / `failed`を許可せず、`verified`は証跡、`deferred`は同じセルに`Reason: ...; Next: ...`を要求する。

## Classification

| Class | Apply when | Artifacts |
|---|---|---|
| Full | 不可逆変更、新規 component / namespace / public host、stateful、secrets 方式変更 | `spec.md`, `plan.md`, `tasks.md`, `reviews/` |
| Lite | 既存設定や policy の挙動変更で、固有の受入基準がある | `spec.md`, `reviews/` |
| None | version bump、typo、docs、原因と修正が 1 対 1 の bug | 従来フロー |

不可逆変更は規模に関係なく Full とする。受入基準が `Synced` / `Healthy` だけなら SDD は適用しない。

## Gates

| Gate | Owner | Exit condition |
|---|---|---|
| Spec | Claude | `[NEEDS CLARIFICATION]` がなく、一意なIDを持つ観測可能な受入基準が`pending`で定義されている |
| Pre-implementation review | Codex → Claude | Full は plan、Lite は spec をレビューし、各指摘を採用・却下・保留に分類 |
| Implementation | Claude | 既存 CI と同じ静的検査が成功し、未実行項目に理由がある |
| Diff review | Codex → Claude | P0 解消。P1 は未決事項として明示 |
| Handoff | Human | 全受入基準が証跡付き`verified`または理由付き`deferred`で、draft PR の内容を確認して ready / merge を判断 |

同じ検証 gate を3回修正しても通らなければ停止する。Codex が利用不能な場合は自動続行せず、理由を記録して Yu の明示承認による人間レビューへ切り替える。承認がなければ停止する。

## Verification

静的検査は `scripts/validate-manifests.sh` を正とし、`.github/workflows/manifest-ci.yml` と `/sdd-impl` の双方から呼ぶ。クラスタへ到達できる場合は `kubectl apply --dry-run=server` で admission を確認する。未到達の場合は merge 後に確認する項目と理由を handoff に残す。

## Draft PR projection

draft PR の本文は既存の pull request templateを保ち、`spec.md`、`plan.md`、検証結果から変更目的、レビュー入口、resourceの追加・変更・削除、到達経路、検証、merge時挙動、rollback、未解決事項を投影する。
