# Tasks: kind demo PVC persistence dogfood

> Plan: [plan.md](./plan.md)

## Implementation

- [x] [chart] `charts/app-base/templates/pvc.yaml`の生成PVCへ`Prune=false` annotationを追加する。
- [x] [values] `environments/kind/values/demo-app.yaml`で`demo-data` PVC（128Mi / longhorn / `/data`）を有効化する。
- [x] [ci] `.github/workflows/explore-ci.yml`へPVC Bound、marker write/read、Pod UID交代、marker一致、gateway HTTP 200 retryを追加する。
- [x] [ci] failure diagnosticsへapp-demo Pod/PVC情報を追加する。

## Verification

- [x] Existing CI-equivalent static checks executed successfully
- [x] `kubectl apply --dry-run=server` executed against isolated `sdd-pvc-dogfood` kind cluster on 2026-08-12.
- [x] Stateful resources checked for `Prune=false`
- [x] Every acceptance criterion updated in spec.md; no pending / failed state remains
- [x] Codex implementation review recorded and all findings adjudicated
- [x] Draft PR handoff prepared; ready / merge left to the human

## Live verification notes

Acceptance criteriaの状態と証跡は[`spec.md`](./spec.md)を正とする。

- Cluster: temporary `kind-sdd-pvc-dogfood` using an explicit isolated kubeconfig.
- Server dry-run and apply succeeded for ServiceAccount, PVC, Deployment, and Service.
- PVC reached `Bound`; marker `sdd-live-2026-08-12` survived Pod replacement.
- Replacement Pod became Ready with a different UID and returned HTTP 200 through the Service.
- The temporary cluster and nodes were deleted after verification.
- Gateway routing was not installed in the minimal cluster; the existing Explore CI retains that assertion.
