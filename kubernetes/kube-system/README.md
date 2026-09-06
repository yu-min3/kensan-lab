# kube-system

Labels, Pod Security level, and shared resources for the `kube-system` namespace.

kubeadm creates this namespace at install time, so nothing here creates it. This
is the place to declare, through Argo CD, the labels that need to be on it — the
selector for Gateway API's `allowedRoutes`, for instance — and its Pod Security
level.

Pod Security enforcement is **consolidated on Kyverno** (ADR-012 v2). A
namespace declares its PSS level through the Kyverno-only label
`kensan-lab.platform/pss-level`: unset means the `pss-baseline` floor,
`privileged` means excluded from that floor and is permitted only for
`tier=platform`, and `restricted` is opt-in. `kube-system` carries
`pss-level: privileged` because it hosts control-plane components.

The PSA labels (`pod-security.kubernetes.io/*`) still sit alongside it during the
migration. They are transitional and are removed in the same PR that promotes
Kyverno to Enforce (Phase 3), so there is no window without enforcement — an
atomic swap. Do not add new dependencies on the PSA labels.

## Layout

- `namespace.yaml` — adopts `kube-system`'s `metadata.labels` through Argo CD
  (`argocd.argoproj.io/sync-options: ServerSideApply=true,Replace=false`). It
  carries `pss-level: privileged` and the transitional PSA labels

## Related

- Policy enforcement design: [ADR-012](../../docs/adr/012-policy-enforcement-kyverno.md) / [`docs/architecture/policy-enforcement.md`](../../docs/architecture/policy-enforcement.md)
- Namespace label design: [`docs/architecture/namespace-label-design.md`](../../docs/architecture/namespace-label-design.md)
- Naming convention: [ADR-006](../../docs/adr/006-namespace-naming.md)
