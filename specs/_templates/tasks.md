# Tasks: <機能名>

> Plan: [./plan.md](./plan.md)
> 依存順に並べる。各タスクは小さく検証可能に。`/sdd-impl` がこの順で実装し、チェックを付けていく。

## Implementation

- [ ] [layout] `kubernetes/<category>/<component>/` を作成（values.yaml + resources/）
- [ ] [chart] `values.yaml` を記述（plan の Chart & version に従う）
- [ ] [resources] resources/ に必要な raw マニフェストを配置（HTTPRoute / ServiceMonitor / namespace 等）
- [ ] [secrets] secrets を作成（SealedSecret は `temp/<name>-raw.yaml` → `kubeseal` → `resources/<name>-sealed.yaml`。ExternalSecret は CR を配置）
- [ ] [argocd] `kubernetes/argocd/applications/<category>/<component>/app.yaml` を作成（version 固定・project・namespace）

## Verification (MANDATORY — 緑になるまで done としない)

- [ ] 静的ゲート: `yamllint` → `helm template`(render, commit しない) → `kubeconform -ignore-missing-schemas` → `kubectl apply --dry-run=client`(app.yaml) → secret 漏れ / version 固定チェック
- [ ] ライブ検証（cluster 到達時）: `kubectl apply --dry-run=server`（実 API + CRD + admission に対する非破壊検証）
- [ ] spec.md の Acceptance criteria を 1 つずつ確認（cluster 未到達分は PR body / merge 後 runbook に委ねる旨を明記）
- [ ] 緑になったら draft PR を作成（`/sdd-impl` が実施。ready 化・merge は人間）
