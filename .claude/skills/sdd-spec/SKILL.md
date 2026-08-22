---
name: sdd-spec
description: Infra変更をFull/Lite/不要に分類し、必要な場合だけspecを作る
argument-hint: <feature-name> [summary]
---

# SDD classification and spec

最初に [`docs/guides/sdd-workflow.md`](../../../docs/guides/sdd-workflow.md) を読み、次の順で分類する。

1. 不可逆、stateful、secrets方式変更か → **Full**
2. 新規component / namespace / public hostか → **Full**
3. `Synced` / `Healthy` 以外の受入基準を書けないか → **不要**
4. 既存設定・policyの挙動変更か → **Lite**。それ以外は**不要**

不要なら理由を伝えてファイルを作らない。Full/Liteなら以下を実行する。

1. `specs/[0-9]*` の最大番号を作成直前に再確認する。同一番号またはslugがあれば停止して再採番する。
2. `specs/_templates/spec.md` から `specs/NNN-<slug>/spec.md` を作り、`mode`を設定する。
3. what / why、scope、non-goals、観測可能なacceptance criteriaをユーザーと確定する。各criterionへ一意な`AC-N`を付け、stateを`pending`で開始する。技術選定を発明しない。
4. placeholderまたは`[NEEDS CLARIFICATION]`が残る間は次gateへ進めない。
5. Fullは`/sdd-plan`へ、LiteはCodexによるspecのpre-implementation reviewへ案内する。

Liteでは`plan.md`と`tasks.md`を作らない。
