---
name: sdd-tasks
description: Full SDDのplanを依存順の実装・検証タスクへ分解する
argument-hint: <feature-name>
---

# Full SDD task breakdown

1. `spec.md`、`plan.md`、`reviews/codex-pre-impl.md`を読む。
2. pre-implementation reviewに未解決P0、未裁定finding、未承認fallbackがあれば停止する。
3. `specs/_templates/tasks.md`から`tasks.md`を作る。planのaffected pathsとresource変化を全て小さな依存順タスクで覆う。
4. Verificationは削除せず、追加検証が必要なら追記する。
   Acceptance criterionのstateやevidenceは複製せず、`spec.md`の表だけを更新する。
5. `python3 scripts/sdd_gate.py validate specs/NNN-<slug> --stage implement`を実行する。失敗を修正せずに進めない。
6. `/sdd-impl`へ案内する。
