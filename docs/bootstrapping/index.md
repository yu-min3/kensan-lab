# インフラストラクチャ ブートストラッピング手順

このドキュメントは、ゼロからクラスターを構築する際の参考資料です。通常の運用では、すでに生成された YAML ファイルが `base-infra/` ディレクトリに存在するため、これらの手順を実行する必要はありません。

## 概要

このプラットフォームは GitOps で管理されており、すべてのリソースは Git リポジトリに保存されています。以下の手順は、Helm チャートから Kubernetes マニフェストを生成するために使用されたコマンドの記録です。

各スクリプトは `docs/bootstrapping/scripts` ディレクトリにあります。

## 前提条件

- Kubernetes クラスター（kubeadm で初期化済み）
- kubectl がクラスターに接続可能
- Helm 3.x インストール済み

**注意:** シークレットの作成と管理については、[シークレット管理ガイド](../secret-management/index.md)を参照してください。

---

## 1. Cilium CNI + LoadBalancer + Hubble UI のデプロイ

Cilium は CNI プラグインとして機能し、kube-proxy を置き換えます。また、L2 アナウンスメントを使用した LoadBalancer 機能と、Hubble によるネットワーク可視化も提供します。

マニフェストを生成するには、以下のスクリプトを実行します。
```bash
./scripts/01-generate-cilium.sh
```

---

## 2. Argo CD のデプロイ

Argo CD は GitOps コントローラーとして、Git リポジトリの変更を自動的にクラスターに同期します。

マニフェストを生成するには、以下のスクリプトを実行します。
```bash
./scripts/02-generate-argo.sh
```
**重要**: スクリプト実行後、`base-infra/argocd/argocd-install.yaml` を手動で編集し、`argocd-cmd-params-cm` に `server.insecure: "true"` を追加する必要があります。

---

## 3. Kubernetes Gateway API CRDs のデプロイ

Istio が Gateway API を使用するため、標準の Gateway API CRDs が必要です。

CRDをダウンロードするには、以下のスクリプトを実行します。
```bash
./scripts/03-get-gateway-api.sh
```

---

## 4. Istio Service Mesh のデプロイ

Istio はサービスメッシュとして、トラフィック管理、セキュリティ、可観測性を提供します。

マニフェストを生成するには、以下のスクリプトを実行します。
```bash
./scripts/04-generate-istio.sh
```

---

## 5. Prometheus + Grafana 監視スタックのデプロイ

PrometheusとGrafanaのスタックをデプロイします。Grafanaの管理者パスワードなど、関連するシークレットは[シークレット管理ガイド](../secret-management/index.md)に従って別途作成してください。

以下のスクリプトを実行して、`kube-prometheus-stack` のマニフェストを生成します。
```bash
./scripts/06b-generate-prometheus.sh
```

---

## 6. Backstage 開発者ポータルのブートストラップ

### 6.1. Backstage アプリケーションの作成

以下のスクリプトを使用して、Backstageアプリケーションのスケルトンを生成します。
```bash
./scripts/07a-create-backstage-app.sh
```

### 6.2. Kubernetes 対応のカスタマイズ

BackstageをKubernetesで実行するために、以下の設定ファイルやDockerfileの例を参照してください。
- `manifests/app-config.kubernetes.yaml`
- `manifests/Dockerfile.backstage`
- `manifests/Makefile.backstage`

### 6.3. ビルドとデプロイ

Backstageアプリケーションをビルドしてデプロイするためのコマンドの例です。関連するシークレットは[シークレット管理ガイド](../secret-management/index.md)に従って別途作成してください。
```bash
./scripts/07c-build-deploy-backstage.sh
```

---

## 7. Keycloak 認証基盤のデプロイ

Keycloak は JWT ベースの認証を提供し、Istio Gateway と統合されます。

マニフェストを生成するには、以下のスクリプトを実行します。
```bash
./scripts/08-generate-keycloak.sh
```

---

## 8. OpenTelemetry Collector のデプロイ

OpenTelemetry Collector は、アプリケーションからの OTLP トラフィック（メトリクス/トレース/ログ）を受信・処理・転送するコンポーネントです。

マニフェストを生成するには、以下のスクリプトを実行します。
```bash
./scripts/09-generate-otel-collector.sh
```

**デプロイされるリソース:**
- ServiceAccount
- ConfigMap（Collector設定）
- Service（OTLP受信ポート: 4317/gRPC, 4318/HTTP, 8888/metrics）
- Deployment（Collector Pod）
- ServiceMonitor（Prometheus Operator統合）

**設定のカスタマイズ:**
`infrastructure/observability/otel-collector/values.yaml` を編集後、スクリプトを再実行してマニフェストを再生成してください。

---

## 9. Grafana Tempo 分散トレーシングバックエンドのデプロイ

Grafana Tempo は、OpenTelemetry Collector から送信されたトレースデータを保存・クエリするための分散トレーシングバックエンドです。

マニフェストを生成するには、以下のスクリプトを実行します。
```bash
./scripts/10-generate-tempo.sh
```

**デプロイされるリソース:**
- ServiceAccount
- ConfigMap（Tempo設定）
- Service（OTLP受信ポート: 4317/gRPC, 4318/HTTP, 3200/HTTP API）
- StatefulSet（Tempo Pod - Single Binary Mode）
- PersistentVolumeClaim（トレースデータ保存用: 10Gi）

**設定のカスタマイズ:**
`infrastructure/observability/tempo/values.yaml` を編集後、スクリプトを再実行してマニフェストを再生成してください。

**⚠️ 重要な注意事項（手動修正が必要）:**

Tempo Helm Chart（v1.24.0）の既知の制限により、生成されたマニフェストの `volumeClaimTemplates` セクションに不備があります。マニフェスト生成後、以下の手動修正が必要です。

`infrastructure/observability/tempo/tempo-manifests.yaml` を開き、`volumeClaimTemplates:` セクションを以下のように修正してください:

```yaml
# ❌ Helmが生成する不完全な構造
volumeClaimTemplates:
  - metadata:
      name: storage
      annotations:
        null  # 無効なYAML
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: "10Gi"
      storageClassName: local-path

# ✅ 修正後の正しい構造
volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: storage
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: "10Gi"
      storageClassName: local-path
      volumeMode: Filesystem
```

**修正内容:**
1. `apiVersion: v1` を追加
2. `kind: PersistentVolumeClaim` を追加
3. 無効な `annotations: null` を削除
4. `volumeMode: Filesystem` を追加

**データフロー:**
```
Application → OTel Collector (4317/4318) → Tempo (4317) → Storage (PVC)
```

---

## 10. Grafana Loki ログ集約バックエンドのデプロイ

Grafana Loki は、OpenTelemetry Collector から送信されたログデータを保存・クエリするためのログ集約システムです。

マニフェストを生成するには、以下のスクリプトを実行します。
```bash
./scripts/11-generate-loki.sh
```

**デプロイされるリソース:**
- ServiceAccount
- ConfigMap（Loki設定）
- Service（HTTP API: 3100、OTLP受信: 3100/otlp/v1/logs）
- StatefulSet（Loki Pod - Monolithic Mode）
- PersistentVolumeClaim（ログデータ保存用: 10Gi）
- ServiceMonitor（Prometheus Operator統合）

**設定のカスタマイズ:**
`infrastructure/observability/loki/values.yaml` を編集後、スクリプトを再実行してマニフェストを再生成してください。

**✓ 注意:**
Loki Helm Chart（v6.46.0）は `volumeClaimTemplates` を正しく生成します。Tempoと異なり、手動修正は不要です。

**データフロー:**
```
Application → OTel Collector (OTLP logs) → Loki (HTTP 3100) → Storage (PVC)
```
