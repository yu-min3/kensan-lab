# Specs — Spec-Driven Development (SDD)

仕様を書き上げたら AI が worktree 内で自律実装し、**必ず検証フェーズを通してから**引き渡す、という開発フロー。参照: [GitHub spec-kit](https://github.com/github/spec-kit) / [Martin Fowler "SDD with 3 tools"](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html)。

このディレクトリは **infra（GitOps / `kubernetes/`）向け**。app 向けは別フローで [`apps/kensan/`](../apps/kensan/) 配下に同じ骨格で用意する（後述）。

> 📖 **実際の開発手順は [`GUIDE.md`](./GUIDE.md)**（mermaid 図つきのステップ・バイ・ステップ）。設計の背景は [`sdd-overview.html`](./sdd-overview.html)。この README はワークフロー定義（リファレンス）。

## フロー（4 フェーズ）

```
/sdd-spec <name>     要件 (what/why)          → specs/NNN-<slug>/spec.md
      ↓ レビュー
/sdd-plan <name>     技術設計 (how)            → specs/NNN-<slug>/plan.md
      ↓ レビュー
/sdd-tasks <name>    タスク分解                → specs/NNN-<slug>/tasks.md
      ↓ レビュー
/sdd-impl <name>     worktree で自律実装        → ~/kensan-lab.worktrees/<NNN-slug>
                     + 必須検証フェーズ
                     → 緑になったら draft PR を作成（ready 化・merge は人間）
```

各フェーズは独立したレビュー可能な成果物を残す。仕様（chat ではなく `specs/` のファイル）が single source of truth。

> **コマンド名について**: `sdd-` 接頭辞は、Claude Code 組み込みの `/goal`（"set a goal Claude checks before stopping"）や他プラグインとの衝突を避けるため。`/goal` は別物（停止前にゴールを確認する番人）で、`/sdd-impl` と**併用**できる（下記）。

## 「完了の定義」は spec が持つ ＋ 組み込み `/goal`

`/sdd-impl` が「**止まる前に必ず満たすゴール**」として参照するのは、作った **`spec.md` の `## Acceptance criteria` ＋ `tasks.md` の `## Verification`**。これを Definition of Done (DoD) として取り込み、全項目を満たす（or cluster 未到達で merge 後確認、と理由付きで仕分ける）まで完了にしない。**せっかく作った spec がそのまま停止ゲートの中身になる。**

組み込みの `/goal`（"set a goal Claude checks before stopping"）は同じ「停止前確認」を harness 層で行う別レイヤー。ただし **スキルから `/goal` をプログラム的にセットすることはできない**（ユーザーが手で打つ UI 機能）。そこで `/sdd-impl` は DoD から**そのまま貼れる `/goal` 文字列を提示**する:

```
/goal 001-loki-ruler: 静的ゲート全緑 + <spec の受入基準を列挙> + draft PR 作成   ← /sdd-impl が提示。貼るかは任意
/sdd-impl 001
```

貼れば harness 層の番人も立つし、貼らなくても `/sdd-impl` 自身の DoD ゲートで同じ規律を守る。`/sdd-impl` は番人を再発明せず、その下で走る infra 専用の自動化（worktree + helm/kubeconform ゲート + draft PR）に徹する。

## レイアウト

```
specs/
├── README.md
├── _templates/{spec,plan,tasks}.md   # 雛形（各スキルが読み込む）
└── NNN-<slug>/                        # 1 機能 1 ディレクトリ。NNN = ゼロ埋め連番
    ├── spec.md
    ├── plan.md
    └── tasks.md
```

`NNN-<slug>` はそのまま branch / worktree 名（`feat/NNN-<slug>`, `~/kensan-lab.worktrees/NNN-<slug>`）に流用する。

## 憲法 (constitution)

spec-kit の `constitution.md` は**作らない**。このリポジトリの統治ルールは既に存在する:

- [`CLAUDE.md`](../CLAUDE.md) — Mandatory Constraints
- [`.claude/rules/`](../.claude/rules/) — gitops-workflow / helm-multisource / security-secrets / network-ingress / kubernetes-cluster / environment-separation

plan.md の `## Constitution check` でこれらを**リンク参照**する（複製しない）。仕様/計画/タスクは軽量に保ち、恒久ルールは `.claude/` に一本化する。

## 2 段階の検証

infra にはテストスイートが無く、本番検証は merge 後の Argo CD。そこで `/sdd-impl` の検証は 2 段階:

1. **静的ゲート（merge 前・ローカル・常時）**: `yamllint` / `helm template`(render dry-run、出力は commit しない) / `kubeconform -ignore-missing-schemas` / `kubectl apply --dry-run=client` / secret 漏れ・version 固定チェック。
2. **ライブ検証（cluster 到達時）**: `kubectl apply --dry-run=server`（実 API + CRD + admission への非破壊検証）。必要なら scratch namespace への一時 apply → 即 delete。

静的ゲート全緑が必須。cluster 未到達分は黙ってスキップせず明示し、PR body / merge 後 runbook（draft PR → ready → merge → Argo CD sync → `/argocd-sync` + `/cluster-status`）に委ねる。

## フェーズの再実行

要件や設計が変わったら前フェーズのスキルを再実行して成果物を上書きし、再レビューする。下流の成果物（plan/tasks）は上流変更後に作り直す。

## app 向けフロー（後日）

app は `apps/kensan/` 配下で完結させる方針。app 向け SDD は `apps/kensan/.claude/skills/`（同名 `/sdd-*`。cwd スコープで infra 版と衝突しない）+ `apps/kensan/specs/` に同じ骨格で用意する。検証フェーズは静的ゲートではなく `make test` / `npm run build` / Playwright 等の実テストに差し替える。
