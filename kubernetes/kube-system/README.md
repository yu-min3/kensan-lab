# kube-system

`kube-system` ns の label / PSA / 共通リソース管理。

kubeadm が install 時に作る ns なので作成はしないが、Argo CD で管理したいラベル (Gateway API `allowedRoutes` の selector など) や Pod Security Standards の enforcement を宣言的に当てるための場所。

## 構成

- `namespace.yaml` — `kube-system` の `metadata.labels` を Argo CD 経由で adopt (`argocd.argoproj.io/sync-options: ServerSideApply=true,Replace=false`)

## 関連

- Namespace label の設計: [`docs/concepts/namespace-label-design.md`](../../docs/concepts/namespace-label-design.md)
- 命名規約: [ADR-006](../../docs/adr/006-namespace-naming.md)
