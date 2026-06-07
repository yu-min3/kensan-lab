---
name: codex
description: OpenAI Codex に作業を委譲する。コードレビュー、セカンドオピニオン、別解の生成、調査・分析を Codex にやらせたいとき（「Codex にレビューさせて」「Codex に聞いて」「Codex の意見も欲しい」等）に使う。
disable-model-invocation: false
---

# Codex への委譲

`codex` CLI（非対話モードの `codex exec`）で OpenAI Codex にタスクを委譲する。認証は ChatGPT ログイン済みの前提（未認証なら `codex login status` で確認し、ユーザーに `! codex` での認証を促す）。

## 基本形

```bash
codex exec -s read-only -o /tmp/codex-out.md "<依頼内容>"
```

- 最終回答は `-o` で指定したファイルに書かれる。実行後にそのファイルを Read して結果を使う（stdout は進捗ログで冗長）
- **sandbox は原則 `read-only`**。Codex に直接ファイルを書かせず、提案を受け取って自分で適用する（この repo の worktree / GitOps ルールに従うため）
- 対象 repo がカレントでない場合は `-C <dir>` を付ける
- 依頼プロンプトには十分なコンテキスト（対象ファイルパス、背景、期待する出力形式）を含める。Codex は会話の文脈を知らない

## コードレビュー

組み込みの review サブコマンドを使う:

```bash
codex exec review --uncommitted                        # working tree の変更をレビュー
git fetch origin && codex exec review --base origin/main   # branch diff をレビュー
codex exec review --commit <sha>                       # 特定 commit をレビュー
codex exec review --base origin/main "<観点の指示>"    # 観点を指定（日本語可）
```

- base branch は **fetch してから `origin/main`（remote-tracking）を指定**する。stale な base だと diff の認識がズレる
- review は最寄りの `AGENTS.md` を自動で読んでレビュー観点に反映する。repo にレビュー基準を持たせたい場合は `AGENTS.md` に書く
- 巨大 diff は context 超過で truncate され得る。大きい PR は観点や対象を分割して複数回レビューさせる

## 会話の継続

直前のセッションに追加で依頼する場合:

```bash
codex exec resume --last -o /tmp/codex-out.md "<フォローアップ>"
```

## 実行時の注意

- 数分かかることがある。長くなりそうなタスクは Bash の `run_in_background: true` で実行し、完了通知を待つ
- 書き込みを伴うタスクをどうしても任せる場合のみ `-s workspace-write`。`--dangerously-bypass-approvals-and-sandbox` は使わない
- 結果を鵜呑みにしない。Codex の提案はセカンドオピニオンとして扱い、適用前に自分で妥当性を検証する
- 構造化出力が必要なら `--output-schema <schema.json>` で JSON Schema を指定できる
