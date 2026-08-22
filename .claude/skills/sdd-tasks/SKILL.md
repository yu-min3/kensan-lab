---
name: sdd-tasks
description: SDD phase 3 — spec.md + plan.md から依存順のタスクリスト tasks.md を生成する（末尾に必須検証タスク）
argument-hint: <feature-name>
---

# Task Breakdown (infra)

`$ARGUMENTS[0]` の `spec.md` + `plan.md` を読み、依存順のタスクリスト `tasks.md` を作る。SDD パイプラインの phase 3。

参照: [`specs/README.md`](../../../specs/README.md)

## 手順

1. **spec を解決**: `specs/NNN-<slug>/` を `$ARGUMENTS[0]` から特定（部分一致可、曖昧なら一覧）。`plan.md` が無ければ「先に `/sdd-plan` を」と案内。

2. **読む**: `spec.md`（受入基準）と `plan.md`（配置・chart・secrets・Affected paths）を読む。

3. **tasks.md を執筆**: [`specs/_templates/tasks.md`](../../../specs/_templates/tasks.md) を雛形に `specs/NNN-<slug>/tasks.md` を作る。
   - `## Implementation`: 小さく検証可能なチェックボックスタスクを**依存順**に並べる。各タスクに層タグ（`[layout]`/`[chart]`/`[resources]`/`[secrets]`/`[argocd]` 等）。plan の Affected paths を全てカバーする。
   - secrets タスクは方式に合わせる（SealedSecret は raw→kubeseal→sealed.yaml、ExternalSecret は CR 配置。`security-secrets.md` 準拠）。
   - `## Verification (MANDATORY)`: テンプレの固定検証タスクを必ず残す（静的ゲート / ライブ検証 / 受入基準確認）。これは `/sdd-impl` の検証フェーズが実行する。**削除・骨抜きにしない**。

4. **出力して案内**: `tasks.md` を提示し、「レビュー後 `/sdd-impl <slug>` で自律実装へ」と案内。任意で、組み込みの `/goal` に受入基準をセットしておくと「検証が緑になるまで止まらない」番人として効く（下記 README 参照）。
