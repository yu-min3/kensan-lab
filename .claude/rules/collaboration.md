---
description: Agent collaboration rules — PR conventions, design/status reporting format, script output
---

# Collaboration Rules

エージェントが Yu・他エージェントと協働するときの作業規約。GitOps 固有のルールは `gitops-workflow.md` を見る。

## Pull Request Rules

- **PR 作成後に独断でマージしない** — 別エージェントによるレビューがあるため、マージは必ずレビュー完了 + Yu の指示を待つ
- **本文の構成は `.github/pull_request_template.md` に従う**（見出しの意味と必須条件はテンプレート内のコメントが SoT）
- **日本語で書く** — タイトルは Conventional Commits、本文は日本語

### 本文に何を書くか

読み手は**人間の Yu と レビューエージェントの 2 者**で、どちらも「diff を読まずに判断できるか」を求めている。したがって本文の役割は**要約ではなく判断材料の提供**。

- **変更点を列挙しない。** Files changed に出ているものを散文に直しても情報は増えない
- **分かれ道があったなら、選んだ理由と却下した案を書く。** これが無いと、レビュアーが同じ検討を最初からやり直す
- **検証は「やったこと」と「やっていないこと」の両方を書く。** 未検証を書かないと、レビュアーは検証済みだと解釈する
- **詳細な設計は repo 内の docs に置いてリンクする。** PR 本文に設計文書を丸ごと書くと、マージ後に読まれない場所へ知識が沈む

### 長さ

**目安 2,000 字、上限 4,000 字。** 実績値は 400〜3,300 字（中央値 1,300 字）で、これがこの repo の読める長さ。

長くなる原因はほぼ 2 つで、どちらも本文を削る前に構造で解く:

1. repo 内の docs と重複している → docs へリンクする
2. 変更点を列挙している → 消す

バグ修正は 背景 / 何がどう変わるか / 確認 の 3 見出しで足りる。原因と修正点だけ書けばよく、diff の再記述は不要。

### 「壊れうるもの / 戻し方」を必ず書く変更

過去に本番を壊した経路。この repo では**削除は明示的な操作ではなく、rename や path 変更の副作用として起きる**ため、レビュー時に個別に確認する:

| 触るもの | 典型的な壊れ方 |
|---|---|
| Argo CD の Application / ApplicationSet | rename = 旧 prune + 新 create。destination namespace ごと消える。ApplicationSet の同名 takeover は ownerRef 経由で子ごと cascade prune |
| namespace / PVC / StorageClass / PV | reclaim policy 次第で物理データが消え、復元手段が無くなる。`Prune=false` は**リソース個別**に付ける必要がある |
| Gateway / HTTPRoute / AuthorizationPolicy / RequestAuthentication | host の追加漏れが**無認証の素通し**になる。逆に消し忘れると到達不能になる |
| Keycloak / Vault / Sealed Secrets | 信頼の根。鍵を失うと git 履歴からしか戻せない |

無害なら「prune されるリソースなし」の 1 行でよい。**書かないことと、無害であることは違う。**

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

- **スクリプトを渡すときは概要を必ず説明する**: 何をするか・どこを変更するか・前提条件（要ログイン、要 docker 等）・失敗した場合に何が起きるかを、実行を促す前にチャットで要約する。スクリプト冒頭コメントにも同じ概要を書く
