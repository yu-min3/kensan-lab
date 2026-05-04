# kensan tenants

kensan project の per-environment namespace bootstrap を集約したディレクトリ。

各 env dir (`kensan-data/`, `kensan-dev/`, `kensan-prod/`) は以下を含む:

- `config.json` — `infrastructure/gitops/argocd/applications/environments/applicationset.yaml` (ApplicationSet `environments`) の generator が読み込む
- `namespace.yaml` — namespace 定義 (PSA labels, istio-injection 等)
- `ghcr-pull-secret.yaml` — GHCR からの image pull 用 SealedSecret
- `serviceaccount.yaml` — default ServiceAccount に `imagePullSecrets` を紐付け

## 関連

- `infrastructure/kube-system/` — kube-system ns の label/PSA 管理 (単発 Application `kube-system`)
- `infrastructure/observability/namespace.yaml` — monitoring ns の管理 (単発 Application `monitoring`)
- `infrastructure/environments/` — `app-dev/`, `app-prod/` のみ残置 (per-app ns 化までの暫定)
