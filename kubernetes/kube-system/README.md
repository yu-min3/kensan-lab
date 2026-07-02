# kube-system

`kube-system` ns の label / Pod Security level / 共通リソース管理。

kubeadm が install 時に作る ns なので作成はしないが、Argo CD で管理したいラベル (Gateway API `allowedRoutes` の selector など) や Pod Security level を宣言的に当てるための場所。

Pod Security の強制は **Kyverno に一本化済み** (ADR-012 v2)。ns の PSS level は Kyverno 専用 label `kensan-lab.platform/pss-level` で宣言する (無印 = `pss-baseline` の床 / `privileged` = 床から除外、`tier=platform` のみ許可 / `restricted` = opt-in)。`kube-system` は control-plane component を持つため `pss-level: privileged` を付与している。

PSA label (`pod-security.kubernetes.io/*`) も移行期として併存しているが、これは過渡的なもので、Phase 3 (Kyverno Enforce 昇格) と同じ PR で撤去する (強制の空白を作らない atomic swap)。新規に PSA label に依存しないこと。

## 構成

- `namespace.yaml` — `kube-system` の `metadata.labels` を Argo CD 経由で adopt (`argocd.argoproj.io/sync-options: ServerSideApply=true,Replace=false`)。`pss-level: privileged` と過渡期の PSA label を含む

## 関連

- ポリシー強制の設計: [ADR-012](../../docs/adr/012-policy-enforcement-kyverno.md) / [`docs/concepts/policy-enforcement.md`](../../docs/concepts/policy-enforcement.md)
- Namespace label の設計: [`docs/concepts/namespace-label-design.md`](../../docs/concepts/namespace-label-design.md)
- 命名規約: [ADR-006](../../docs/adr/006-namespace-naming.md)
