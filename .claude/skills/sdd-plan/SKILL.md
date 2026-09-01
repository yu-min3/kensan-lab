---
name: sdd-plan
description: SDD phase 2 — spec.md から infra の技術設計 (how) を作る。Helm パターン・chart・namespace 等を決め plan.md を生成
argument-hint: <feature-name>
---

# Technical Planning (infra)

`$ARGUMENTS[0]` の `spec.md` を読み、技術設計 `plan.md` を作成する。SDD パイプラインの phase 2。

参照: [`specs/README.md`](../../../specs/README.md) / [`helm-multisource.md`](../../rules/helm-multisource.md)

## 手順

1. **spec を解決**:
   - `specs/` 配下から `$ARGUMENTS[0]` に一致する `NNN-<slug>/spec.md` を探す（slug の部分一致可）。
   - 複数候補なら一覧を出してユーザーに選ばせる。見つからなければ「先に `/sdd-spec` を」と案内。

2. **進行可否チェック**:
   - `spec.md` に未解消の `[NEEDS CLARIFICATION]` が残っていたら**中断**し、`/sdd-spec` で解消するよう差し戻す。

3. **コードベースに接地**:
   - `kubernetes/` を探索し、似た既存コンポーネント（同 category、似た chart、Pattern A/B/ApplicationSet）を読む。
   - [`kubernetes/README.md`](../../../kubernetes/README.md)（Pattern A/B）、[`kubernetes/argocd/`](../../../kubernetes/argocd/) の app.yaml と root-app を参考にする。
   - 既存の値・命名・syncPolicy の慣習を踏襲する。

4. **plan.md を執筆**: [`specs/_templates/plan.md`](../../../specs/_templates/plan.md) を雛形に、`specs/NNN-<slug>/plan.md` を作る。
   - **Constitution check**: 該当する `.claude/rules/` をチェックし、各々「この設計がどう従うか」を一言添える。該当しない行は削除。複製はしない。
   - **配置**: category / Pattern A(Helm multi-source) or B(raw YAML) or ApplicationSet / component path / Application CR path。
   - **Chart & version**: repo・chart・**固定 version**（`main`/`latest`/`HEAD` 禁止）。raw のみなら N/A。
   - **Argo CD project / namespace / syncPolicy / sync wave・依存**。
   - **ネットワーク**: Gateway（platform=242 / prod=243）・host・HTTPRoute。公開しないなら明記。
   - **Secrets 方式**: Vault dynamic / ESO static / Transit / SealedSecret / なし。Vault path・Reloader 要否。
   - **`## Affected paths`**: 触る/作る `kubernetes/<...>` パスと `app.yaml` を列挙（`/sdd-impl` の検証対象シグナル。正確に）。
   - **Risks / rollback**。

5. **出力して案内**: `plan.md` を提示し、「レビュー後 `/sdd-tasks <slug>` でタスク分解へ」と案内。spec.md の frontmatter `status` を `planned` に更新してよい。
