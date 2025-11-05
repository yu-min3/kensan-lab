# インフラストラクチャ ブートストラッピング手順

このドキュメントは、ゼロからクラスターを構築する際の参考資料です。通常の運用では、すでに生成された YAML ファイルが `base-infra/` ディレクトリに存在するため、これらの手順を実行する必要はありません。

## 概要

このプラットフォームは GitOps で管理されており、すべてのリソースは Git リポジトリに保存されています。以下の手順は、Helm チャートから Kubernetes マニフェストを生成するために使用されたコマンドの記録です。

## 前提条件

- Kubernetes クラスター（kubeadm で初期化済み）
- kubectl がクラスターに接続可能
- Helm 3.x インストール済み
- kubeseal インストール済み（Sealed Secrets 用）

---

## 1. Cilium CNI + LoadBalancer のデプロイ

Cilium は CNI プラグインとして機能し、kube-proxy を置き換えます。また、L2 アナウンスメントを使用した LoadBalancer 機能も提供します。

```bash
# Helm リポジトリ追加
helm repo add cilium https://helm.cilium.io/
helm repo update

# マニフェスト生成
helm template cilium cilium/cilium \
  --namespace kube-system \
  --set kubeProxyReplacement=true \
  --set k8sClientRateLimit.qps=10 \
  --set k8sClientRateLimit.burst=20 \
  --set k8s.cluster.cidr=10.244.0.0/16 \
  --set ipam.mode=kubernetes \
  --set l2announcements.enabled=true \
  --set externalIPs.enabled=true \
  --set devices=wlan0 \
  > base-infra/cilium/cilium.yaml

# ステータス確認
kubectl get pods -n kube-system -l k8s-app=cilium
kubectl get ciliumloadbalancerippools
```

**設定のポイント:**
- `kubeProxyReplacement=true`: kube-proxy を完全に置き換え
- `l2announcements.enabled=true`: L2 レイヤーで LoadBalancer IP をアナウンス
- `devices=wlan0`: 物理ネットワークインターフェース（環境に応じて変更）
- IP Pool: 192.168.0.240-249（`base-infra/cilium/lb-ippool.yaml` で定義）

---

## 2. Argo CD のデプロイ

Argo CD は GitOps コントローラーとして、Git リポジトリの変更を自動的にクラスターに同期します。

```bash
# Helm リポジトリ追加
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update

# マニフェスト生成（values.yaml を使用）
helm template argocd argo/argo-cd \
  --namespace argocd \
  --values base-infra/argocd/values.yaml \
  > base-infra/argocd/argocd.yaml

# 初期パスワード取得
kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 -d
```

**アクセス:**
- URL: http://192.168.0.240（Cilium LoadBalancer 経由）
- ユーザー名: admin
- パスワード: 上記コマンドで取得

---

## 3. Kubernetes Gateway API CRDs のデプロイ

Istio が Gateway API を使用するため、標準の Gateway API CRDs が必要です。

```bash
# Gateway API CRDs をダウンロード
curl -sL https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml \
  -o base-infra/gateway-api/gateway-api-crds.yaml

# Argo CD 経由でデプロイ（推奨）
# または手動適用:
kubectl apply -f base-infra/gateway-api/gateway-api-crds.yaml
```

---

## 4. Istio Service Mesh のデプロイ

Istio はサービスメッシュとして、トラフィック管理、セキュリティ、可観測性を提供します。

```bash
# Istio Helm リポジトリ追加
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update

# istio-base (CRDs) を生成
helm template istio-base istio/base \
  --namespace istio-system \
  --version 1.27.3 \
  > base-infra/istio/01-istio-base.yaml

# istiod (Control Plane) を生成（Gateway API サポート有効化）
helm template istiod istio/istiod \
  --namespace istio-system \
  --version 1.27.3 \
  --set pilot.env.PILOT_ENABLE_GATEWAY_API=true \
  --set pilot.env.PILOT_ENABLE_GATEWAY_API_STATUS=true \
  --set pilot.env.PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER=true \
  > base-infra/istio/02-istiod.yaml
```

**デプロイ順序（ファイル名順で自動制御）:**
1. `00-namespace.yaml` - istio-system namespace
2. `01-istio-base.yaml` - CRDs
3. `02-istiod.yaml` - Control Plane
4. `gateway-dev.yaml` - Dev Gateway + Service
5. `gateway-prod.yaml` - Prod Gateway + Service

**ステータス確認:**
```bash
kubectl get pods -n istio-system
kubectl get gateway -n istio-system
kubectl get svc -n istio-system -l istio=gateway
```

### ⚠️ 重要: Istio 再インストール時の注意

クラスタの完全な再構築や Istio の再インストールを行う場合、ValidatingWebhook のブートストラップ問題を回避するため、以下のいずれかの方法を使用してください：

**オプション 1: failurePolicy を一時的に変更（推奨）**
```bash
# 1. base-infra/istio/02-istiod.yaml の failurePolicy: Fail を一時的に Ignore に変更
# 2. デプロイ完了後、Fail に戻してコミット
```

**オプション 2: Webhook を事前削除**
```bash
kubectl delete validatingwebhookconfiguration istio-validator-istio-system istiod-default-validator
```

**オプション 3: ファイル名順序制御に依存**
現在の命名規則（00-namespace → 01-base → 02-istiod → gateway）により、通常はブートストラップ問題は発生しません。

---

## 5. GHCR 用 Sealed Secrets の作成

GitHub Container Registry（GHCR）からプライベートイメージをプルするための認証情報を暗号化します。

```bash
# 生シークレット作成（temp/ ディレクトリに保存 - git-ignored）
kubectl create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=<github-username> \
  --docker-password=<PAT> \
  --docker-email=<github-email> \
  --namespace=<namespace> \
  --dry-run=client -o yaml > temp/ghcr-secret-raw.yaml

# シークレットを暗号化
kubeseal --format=yaml < temp/ghcr-secret-raw.yaml \
  > base-infra/sealed-secret/ghcr-pull-secret-prod.yaml

# Sealed Secret を適用
kubectl apply -f base-infra/sealed-secret/ghcr-pull-secret-prod.yaml
```

**セキュリティ注意:**
- `temp/` ディレクトリのファイルは Git に追跡されません（.gitignore で除外）
- 暗号化された Sealed Secret のみを Git にコミットしてください
- Sealed Secrets コントローラーがクラスター内で自動的に復号化します

---

## 6. Prometheus + Grafana 監視スタックのデプロイ

Prometheus と Grafana を使用して、クラスターとアプリケーションのメトリクスを収集・可視化する。

### 6.1. Grafana 管理者パスワードの Sealed Secret 作成

```bash
# ランダムパスワード生成
GRAFANA_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
echo "Generated password: $GRAFANA_PASSWORD"

# 生 Secret 作成（temp/ ディレクトリに保存 - git-ignored）
kubectl create secret generic prometheus-grafana \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="$GRAFANA_PASSWORD" \
  --namespace=monitoring \
  --dry-run=client -o yaml > temp/grafana-secret-raw.yaml

# シークレットを暗号化
kubeseal --format=yaml \
  --controller-name=sealed-secrets \
  --controller-namespace=kube-system \
  < temp/grafana-secret-raw.yaml \
  > base-infra/prometheus/grafana-sealed-secret.yaml

# Sealed Secret を Git にコミット
git add base-infra/prometheus/grafana-sealed-secret.yaml
```

**セキュリティ注意:**
- `temp/` ディレクトリのファイルは Git に追跡されない（.gitignore で除外）
- 暗号化された Sealed Secret のみを Git にコミットする
- Sealed Secrets コントローラーがクラスター内で自動的に復号化する

### 6.2. Prometheus マニフェスト生成

```bash
# Prometheus Helm リポジトリ追加
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# kube-prometheus-stack マニフェスト生成
# prometheus-values.yaml で admin.existingSecret を指定しているため、
# Grafana は上記で作成した Sealed Secret を参照する
# --include-crds: Prometheus Operator の CRD を含める（必須）
helm template prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --include-crds \
  --values docs/prometheus-values.yaml \
  > base-infra/prometheus/prometheus-stack.yaml
```

**生成されるリソース:**
- **CRD（CustomResourceDefinitions）**: PrometheusRule, ServiceMonitor, PodMonitor など（10個）
- **Prometheus Operator**: CRD 管理
- **Prometheus Server**: メトリクス収集・保存
- **Alertmanager**: アラート管理
- **Grafana**: ダッシュボード・可視化
- **Node Exporter**: ノードレベルメトリクス
- **Kube State Metrics**: Kubernetes オブジェクト状態

**重要**: `--include-crds` フラグを指定しないと、PrometheusRule などのカスタムリソースがデプロイ時にエラーになる

### 6.3. Platform Gateway と HTTPRoute の作成

インフラ系サービス（Grafana、Prometheus UI）への外部アクセスを提供する。

```bash
# Platform Gateway の作成（base-infra/istio/gateway-platform.yaml）
# - ホスト名: *.platform.yu-min3.com
# - HTTP/HTTPS リスナー
# - Cilium LoadBalancer で外部 IP 割り当て

# HTTPRoute の作成
# - base-infra/prometheus/httproute-grafana.yaml
#   → https://grafana.platform.yu-min3.com
# - base-infra/prometheus/httproute-prometheus.yaml
#   → https://prometheus.platform.yu-min3.com
```

### 6.4. デプロイとアクセス

**ステータス確認:**
```bash
kubectl get pods -n monitoring
kubectl get sealedsecret -n monitoring
kubectl get secret prometheus-grafana -n monitoring
```

**Grafana 管理者パスワードの取得:**
```bash
kubectl get secret prometheus-grafana -n monitoring \
  -o jsonpath="{.data.admin-password}" | base64 -d
echo  # 改行
```

**アクセス方法:**

Port-forward 経由（開発用）:
```bash
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
# ブラウザで http://localhost:3000 にアクセス
# ユーザー名: admin
# パスワード: 上記コマンドで取得
```

HTTPRoute 経由（本番用）:
- Grafana: `https://grafana.platform.yu-min3.com`
- Prometheus UI: `https://prometheus.platform.yu-min3.com`

**利用可能なデフォルトダッシュボード:**
- Kubernetes / Compute Resources / Cluster
- Kubernetes / Compute Resources / Namespace (Pods)
- Kubernetes / Compute Resources / Node (Pods)
- Node Exporter / Nodes（Raspberry Pi ハードウェアメトリクス）
- Prometheus / Overview

### 監視コンポーネントの役割

| コンポーネント | 役割 | 監視対象 |
|--------------|------|---------|
| **Kube State Metrics** | Kubernetes オブジェクト状態 | Pod 状態、Deployment、Node 状態など |
| **Node Exporter** | OS レベルメトリクス | CPU、メモリ、ディスク I/O、ネットワーク |
| **Prometheus** | メトリクス収集・保存 | 全コンポーネントからメトリクスを収集 |
| **Grafana** | 可視化・ダッシュボード | メトリクスを美しいグラフで表示 |
| **Alertmanager** | アラート管理 | 異常検知時の通知 |

**リソース設計:**
- Raspberry Pi 向けに最適化された軽量設定
- Prometheus データ保持期間: 7 日
- ストレージ: local-path-provisioner 使用
- タイムゾーン: Asia/Tokyo

---

## 7. Keycloak 認証基盤のデプロイ

Keycloak は JWT ベースの認証を提供し、Istio Gateway と統合されます。

```bash
# Keycloak Helm リポジトリ追加
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Prod 環境マニフェスト生成
helm template keycloak-prod bitnami/keycloak \
  --namespace platform-auth \
  --values base-infra/keycloak/overlays/prod/values.yaml \
  > base-infra/keycloak/overlays/prod/keycloak.yaml

# Dev 環境マニフェスト生成
helm template keycloak-dev bitnami/keycloak \
  --namespace platform-auth \
  --values base-infra/keycloak/overlays/dev/values.yaml \
  > base-infra/keycloak/overlays/dev/keycloak.yaml
```

**アクセス:**
- Prod: https://keycloak-prod.your-domain.com
- Dev: https://keycloak-dev.your-domain.com
- Admin Console: `/admin` エンドポイント

---

## GitOps ワークフロー

上記の手順で生成されたマニフェストは、すべて `base-infra/` ディレクトリに保存され、Argo CD によって管理されます。

### デプロイフロー

1. **マニフェスト生成**: Helm template コマンドで YAML を生成
2. **Git コミット**: 生成されたマニフェストを Git にコミット
3. **Argo CD 検知**: Argo CD が変更を自動的に検知
4. **自動同期**: 変更がクラスターに自動的に適用される

### 手動同期が必要な場合

```bash
# Argo CD CLI で同期
argocd app sync <app-name>

# または Argo CD UI で "Sync" ボタンをクリック
```

---

## トラブルシューティング

### Argo CD Application が Progressing のまま

```bash
# Application のステータス確認
kubectl get application -n argocd <app-name> -o yaml

# リソースの詳細確認
kubectl describe <resource-type> <resource-name> -n <namespace>
```

### Istio Gateway が Degraded 状態

```bash
# Gateway のステータス確認
kubectl get gateway -n istio-system <gateway-name> -o yaml

# Istio Proxy ログ確認
kubectl logs -n istio-system <gateway-pod-name>
```

### Sealed Secrets が復号化されない

```bash
# Sealed Secrets Controller のログ確認
kubectl logs -n kube-system -l name=sealed-secrets-controller

# Secret が作成されているか確認
kubectl get secret <secret-name> -n <namespace>
```

---

## 参考リンク

- [Cilium Documentation](https://docs.cilium.io/)
- [Argo CD Documentation](https://argo-cd.readthedocs.io/)
- [Istio Documentation](https://istio.io/latest/docs/)
- [Prometheus Operator](https://prometheus-operator.dev/)
- [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
- [Keycloak Documentation](https://www.keycloak.org/documentation)
