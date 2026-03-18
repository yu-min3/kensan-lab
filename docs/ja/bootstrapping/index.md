# インフラストラクチャ ブートストラッピング手順

このドキュメントは、ゼロからクラスターを構築する際の参考資料です。

## アーキテクチャ

このプラットフォームは Argo CD の **Helm multi-source Application** パターンで管理されています。各コンポーネントのチャートバージョン・リポジトリ・values は Application CR に定義されており、Argo CD が Helm のレンダリングとデプロイを自動で行います。

```
infrastructure/gitops/argocd/applications/   ← Application CR（チャート情報の Source of Truth）
infrastructure/<category>/<component>/
  ├── values.yaml                            ← Helm values
  └── resources/                             ← Helm 管理外のカスタムリソース
```

チャートバージョンの確認・変更は各 Application CR の `spec.sources[0].targetRevision` を参照してください。

## 前提条件

- Kubernetes クラスター（kubeadm で初期化済み）
- kubectl がクラスターに接続可能
- Helm 3.x（初回ブートストラップのみ必要）

**注意:** シークレットの作成と管理については、[シークレット管理ガイド](../secret-management/index.md)を参照してください。

---

## デプロイ順序と sync-wave

Argo CD の sync-wave によりデプロイ順序が制御されます。

| Wave | コンポーネント | Application CR |
|------|--------------|----------------|
| — | Cilium CNI | `applications/network/cilium/app.yaml` |
| -3 | Istio namespace + Gateways | `applications/network/istio-resources/app.yaml` |
| -2 | Istio Base CRDs | `applications/network/istio-base/app.yaml` |
| -1 | Istiod | `applications/network/istiod/app.yaml` |
| 0 | cert-manager | `applications/security/cert-manager/app.yaml` |
| — | Prometheus | `applications/observability/prometheus/app.yaml` |
| — | Grafana | `applications/observability/grafana/app.yaml` |
| — | Loki | `applications/observability/loki/app.yaml` |
| — | Tempo | `applications/observability/tempo/app.yaml` |
| — | OTel Collector | `applications/observability/otel-collector/app.yaml` |
| — | Argo CD (self-managed) | `applications/gitops/argocd/app.yaml` |

---

## 1. Cilium CNI のブートストラップ

Cilium は Argo CD より先にデプロイが必要です（CNI がないと Pod が起動しない）。初回のみ `helm install` で直接インストールし、以降は Argo CD が管理します。

```bash
helm repo add cilium https://helm.cilium.io/
helm install cilium cilium/cilium \
  --version 1.18.3 \
  --namespace kube-system \
  --values infrastructure/network/cilium/values.yaml
```

カスタムリソース（LB IP Pool, L2 Announcement Policy）は手動で適用します。

```bash
kubectl apply -f infrastructure/network/cilium/resources/
```

---

## 2. Gateway API CRDs

Istio が Gateway API を使用するため、標準の Gateway API CRDs が必要です。

```bash
./scripts/03-get-gateway-api.sh
```

---

## 3. Argo CD のブートストラップ

Argo CD も初回は `helm install` で直接インストールします。

```bash
helm repo add argo https://argoproj.github.io/argo-helm
helm install argocd argo/argo-cd \
  --version 9.1.0 \
  --namespace argocd \
  --create-namespace \
  --values infrastructure/gitops/argocd/values.yaml
```

インストール後、AppProject と Root Application を apply して自己管理を開始します。

```bash
kubectl apply -f infrastructure/gitops/argocd/projects/platform-project.yaml
kubectl apply -f infrastructure/gitops/argocd/root-apps/platform-root-app.yaml
```

Root Application が `infrastructure/gitops/argocd/applications/` を再帰スキャンし、全ての子 Application CR を自動検知・デプロイします。以降のコンポーネントは手動操作不要です。

---

## 4. 設定変更の方法

各コンポーネントの設定を変更する場合：

1. 対応する `values.yaml` を編集
2. Git に commit & push
3. Argo CD が自動的に差分を検知してデプロイ

チャートバージョンを変更する場合は、Application CR の `targetRevision` を更新してください。

---

## Backstage 開発者ポータル

Backstage は Kustomize ベースで管理されており、Helm 移行の対象外です。

- アプリケーション作成: `./scripts/07a-create-backstage-app.sh`
- Kubernetes 対応の設定例: `manifests/` ディレクトリ参照
- ビルド・デプロイ: `./scripts/07c-build-deploy-backstage.sh`

関連するシークレットは[シークレット管理ガイド](../secret-management/index.md)に従って作成してください。

## Keycloak 認証基盤

Keycloak も Kustomize ベースで管理されています。

```bash
./scripts/08-generate-keycloak.sh
```

## Grafana ダッシュボード

Grafana.com から OTel ダッシュボードを取得するスクリプトです。

```bash
./scripts/13-generate-grafana-dashboards.sh
```

詳細は [Grafana 独立デプロイ](./11-grafana-independent.md) を参照してください。

---

## ワーカーノード追加

- [Bosgame M4 Neo ワーカーノード追加手順](./add-worker-node-m4neo.md) — AMD64 ノードを既存 ARM64 クラスタに追加（マルチアーキテクチャ対応）
