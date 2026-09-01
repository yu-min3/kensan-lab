---
name: sdd-plan
description: Full SDDの技術計画を作り、Codex read-onlyレビューを記録する
argument-hint: <feature-name>
---

# Full SDD plan and pre-implementation review

1. 対象`spec.md`を解決し、`mode: full`、placeholderなし、clarificationなしを確認する。Liteはこのskillを使わない。
2. 現行コード、`CLAUDE.md`、該当`.claude/rules/`を読み、`specs/_templates/plan.md`から`plan.md`を作る。
3. resourceのadd/change/delete（なしも明記）、policy、storage/`Prune=false`、公開host三点セット、affected paths、merge behavior、rollbackを具体化する。
4. `AGENTS.md -> CLAUDE.md` symlinkを確認する。欠落時はCodexを呼ばず停止する。
5. Codexを`codex exec -s read-only`で呼び、P0/P1/P2、不可逆性、到達経路、受入基準、rollbackを反証させる。出力は`temp/`へ置き、内容を検査してからClaudeが`reviews/codex-pre-impl.md`へ転記する。
6. 各findingをAdopt / Reject / Holdに分類し理由を書く。未解決P0があれば停止する。

Codexが失敗、timeout、空出力の場合は自動続行しない。review記録に失敗を残し、Yuが明示承認したhuman fallbackだけを許可する。
