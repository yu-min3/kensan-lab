---
name: sdd-spec
description: SDD phase 1 — infra 機能の要件仕様 (what/why) を対話的に作成する。specs/NNN-<slug>/spec.md を生成
argument-hint: <feature-name> [概要]
---

# Spec Authoring (infra)

`$ARGUMENTS[0]` の infra 機能について、要件仕様 `spec.md` を**対話的に**作成する。これは SDD パイプラインの phase 1。

参照: [`specs/README.md`](../../../specs/README.md)

## 原則

- **what / why のみ**を書く。chart 名・version・マニフェスト構造などの技術選定は書かない（→ `/sdd-plan`）。
- 要件を**勝手に発明しない**。曖昧な点は質問する。確定できない点は `[NEEDS CLARIFICATION: ...]` として残す。
- コードもマニフェストも worktree も**作らない**。成果物は `spec.md` 1 枚だけ。

## 手順

1. **spec ディレクトリを採番**:
   - `specs/` 直下の既存 `NNN-*` を見て次の連番（ゼロ埋め 3 桁、例 `004`）を決める。最初なら `001`。
   - `$ARGUMENTS[0]` をケバブケース slug 化 → `specs/NNN-<slug>/` を作る。
   ```bash
   ls -d specs/[0-9]* 2>/dev/null | sort | tail -1   # 既存の最大連番を確認
   ```

2. **テンプレを読む**: [`specs/_templates/spec.md`](../../../specs/_templates/spec.md) を雛形にする。

3. **対話的クラリフィケーション**: ドラフトを書く前に、ユーザーに 2〜4 個の的を絞った質問をする。最低限カバーすべき軸:
   - 目的 / why（なぜ今これが必要か）
   - 対象 namespace・公開ドメインの有無・ネットワーク入口（Gateway 経由 / LAN のみ / 非公開）
   - セキュリティ影響（扱う secrets、認証要否）
   - スコープ境界（非ゴール）
   - 受入基準（何が確認できたら「できた」か）
   - 提供された `[概要]`（`$ARGUMENTS` の 2 つ目以降）から埋まる部分は質問を省く。

4. **spec.md を執筆**: テンプレの全セクションを埋める。
   - frontmatter: `id: NNN-<slug>`, `domain: infra`, `status: draft`, `created`/`updated` を実際の日付に。
   - `## Acceptance criteria` は観測可能・確認可能な形で（例: 「Argo CD app が Synced+Healthy」「`https://...` が認証画面へリダイレクト」「Certificate が Ready」）。これが `/sdd-impl` の検証 oracle になる。
   - 残った不明点は `## Open questions` に `[NEEDS CLARIFICATION: ...]` で明記。

5. **出力して案内**:
   - `specs/NNN-<slug>/spec.md` を提示。
   - `[NEEDS CLARIFICATION]` が残っていればそれを列挙し、解消を促す。
   - 「内容をレビューし、問題なければ `/sdd-plan <slug>` で技術設計へ」と案内する。
