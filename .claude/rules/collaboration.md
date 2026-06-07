---
description: Agent collaboration rules — PR conventions, design/status reporting format, script output
---

# Collaboration Rules

エージェントが Yu・他エージェントと協働するときの作業規約。GitOps 固有のルールは `gitops-workflow.md` を見る。

## Pull Request Rules

- **PR 作成後に独断でマージしない** — 別エージェントによるレビューがあるため、マージは必ずレビュー完了 + Yu の指示を待つ
- **PR 本文にアーキテクチャの変更をわかりやすく記載する** — 何がどう変わるか（構成図・データフロー・依存関係の変化など）を、diff を読まなくても把握できるレベルで書く
- **レビューエージェントに必要な情報を記載する** — 変更の背景・意図、影響範囲、検証方法（確認済みなら結果も）、関連 ADR / issue / PR へのリンクを PR 本文に含める

## Design & Status Reporting Rule

開発者に設計や状況（アーキテクチャ案、移行計画、調査結果、進捗サマリ等）を伝えるときは、テキストだけで済ませず **HTML ページを作成して図示を交えてわかりやすく可視化し、ブラウザで開く** ところまで行う。

- 配置: `temp/` 配下（例: `temp/design-<topic>.html`、git-ignored）
- 図示: 構成図・データフロー・シーケンス・before/after 比較などを SVG / Mermaid 等で描く。diff やテキストを貼るだけにしない
- 作成後に `open temp/<file>.html` でブラウザ表示まで実行する（macOS 前提。headless / scheduled セッション等でブラウザが開けない場合はファイルパスの提示のみでよい）

## Multi-Agent Setup（Codex 等の他エージェント）

- repo 規約の SoT は `CLAUDE.md`。Codex 等の AGENTS.md 系エージェントには **`AGENTS.md` → `CLAUDE.md` の symlink**（gitignored、ローカルのみ）で同じ内容を読ませる
- 新しい clone / worktree で Codex を使う場合は symlink を再作成する: `ln -s CLAUDE.md AGENTS.md`
- レビュー観点の優先度基準は `CLAUDE.md` の `## Review Guidelines` に定義（`codex exec review` が自動適用）

## Script Output Rule

When presenting shell commands for the user to run, write them to a script file in `temp/` directory (e.g., `temp/fix-xyz.sh`) instead of inline text, and make it executable (`chmod +x`) so the user can run it directly (`./temp/fix-xyz.sh`). This prevents line-break corruption in the terminal.
