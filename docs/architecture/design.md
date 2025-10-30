# K8s GitOps プラットフォームアーキテクチャ

## 1. プロジェクトのビジョンと技術スタック

本プロジェクトは、プラットフォームエンジニア（PE）とアプリケーション開発者（AD）の責務を完全に分離し、GitOpsを中心としたモダンな開発プラットフォームを構築することを目的としています。プラットフォームは、CRI-Oをコンテナランタイムとして使用し、ベアメタルのRaspberry Piハードウェア上で動作します。

### 技術スタック

| カテゴリ | コンポーネント | 役割 |
|---------|-----------|------|
| オーケストレーション | Kubernetes (`kubeadm`) | ベアメタルインフラ上のコンテナオーケストレーション |
| コンテナランタイム | CRI-O | 軽量なコンテナランタイムインターフェース |
| ネットワーク | Cilium | CNI、ネットワークポリシー、サービスメッシュ統合 |
| ロードバランサ | Cilium LoadBalancer | L2アナウンスベースの外部トラフィックルーティング（MetalLBを置き換え） |
| サービスメッシュ | Istio | 外部トラフィック認証、アプリ間制御、セキュリティ |
| 認証/認可 | Keycloak | ユーザーアカウント管理とJWT発行 |
| GitOps | Argo CD | すべてのアプリケーションとインフラの自動デプロイと追跡 |
| シークレット管理 | Sealed Secrets | Gitリポジトリでのシークレット安全管理（GHCR認証情報） |
| モニタリング | Prometheus | クラスターおよびアプリケーションメトリクスの収集 |
| 開発者ポータル | Backstage | アプリケーションテンプレート展開、ドキュメント、カタログ |


## 2. 環境と権限の分離設計

プラットフォームは3層分離モデルを使用します：

| 層 | Namespace例 | 管理者 | Argo CD Project | 目的と責務 |
|-------|-------------------|---------------|-----------------|------------------------------|
| 1. 基盤層 | `istio-system`, `monitoring`, `argocd`, `platform-auth` | PE | `platform-project` | クラスターのコアコンポーネント管理。PEのみアクセス可。 |
| 2. 環境層 | `app-prod`, `app-dev` | AD | `app-project-prod`, `app-project-dev` | ライフサイクルの分離。ADは自身の環境Namespace内のリソースのみ操作可。 |
| 3. アプリ層 | `app-prod-<app-name>` | AD | N/A | アプリケーションごとのセキュリティ/リソース分離。 |

### セキュリティ境界

- **基盤層**: `platform-project`経由でPEが排他的に管理。Istio、Prometheus、Argo CD、Keycloakを含む。
- **環境層**: ProdとDev用の独立したArgo CD Project。ADは自身のNamespaceリソースのみアクセス可。
- **アプリ層**: アプリケーション単位の分離のための将来的な拡張。

## 3. Gitリポジトリ構成（GitOps）

すべてのKubernetesリソース定義はGitリポジトリで管理されます。プラットフォームは複数リポジトリ戦略を使用します：

| リポジトリ | 責務 | 管理者 | 追跡対象リソース |
|-----------|---------------|---------------|-------------------|
| `platform-config` | クラスター基盤、セキュリティ、Argo CD制御構造（App Project、Root Apps） | PE | Istio、Keycloak、Sealed Secrets、Namespaces、RBAC、Argo CD Applications |
| `app-templates` | Backstage用のアプリケーション雛形（Kustomizeベース） | PE | Backstageテンプレート定義、ベースKubernetesマニフェスト |
| `app-<app-name>` | アプリケーションコード、Dockerfile、TechDocs、環境別デプロイ設定 | AD | アプリケーションコード、イメージタグパッチ、レプリカパッチ |

### GitOpsワークフロー

1. すべてのK8sリソースはGitリポジトリで定義
2. Argo CDが継続的に変更を監視して同期
3. Argo CDはRoot Applicationsによる「App of Apps」パターンを使用
4. アプリケーション開発者はBackstageテンプレート経由で新規アプリを作成
5. ADがBackstage経由で新規アプリを作成すると：
   - Kustomize構造を持つ新しい`app-<name>`リポジトリが作成される
   - BackstageがApplication CRを`platform-config/base-infra/argocd/applications/`に自動コミット
   - Argo CDが新しいApplication CRを検出し、DevとProd両環境にデプロイ

## 4. ネットワークとトラフィック管理

### Cilium LoadBalancer with L2アナウンス

プラットフォームはCiliumの組み込みLoadBalancer機能とL2アナウンスを使用し、MetalLBを置き換えます：

- **IPプール**: `192.168.0.240-192.168.0.249`（`base-infra/cilium/lb-ippool.yaml`で定義）
- **L2アナウンスインターフェース**: `wlan0`(家庭用wifiで機能)
- **Lease管理**: リーダー選出にKubernetes lease APIを使用
- **RBAC**: `coordination.k8s.io/leases`リソースへの適切な権限

### Istioサービスメッシュ統合

- Istio Gatewayリソースは環境ごと（Prod/Dev）に`base-infra/istio/`で定義
- アプリケーションはHTTPRouteリソースを使用して適切なGatewayにアタッチ
- Keycloak JWT検証による外部トラフィック認証
- アプリケーションごとの認可ポリシーによる細かいアクセス制御

## 5. シークレット管理戦略

- 機密シークレット（GHCR認証情報）はSealed Secretsで暗号化
- Sealed SecretはGitに安全にコミット可能
- クラスター内のSealed Secretsコントローラーが通常のK8s Secretsに復号化
- アプリNamespace内のServiceAccountsがプライベートイメージpull用に`ghcr-pull-secret`を参照

## 6. 開発者エクスペリエンス

### Backstage統合

- **テンプレートスキャフォールディング**: ADがBackstageを使用してPE管理のテンプレートから新規アプリを作成
- **自動GitOps**: BackstageがArgo CD Application CRを`platform-config`に自動コミット
- **TechDocs**: アプリケーションドキュメントがBackstage上で直接レンダリング
- **カタログ**: すべてのアプリケーションがBackstageカタログに登録され、発見可能に

### 関心の分離

- **PEの責務**: インフラ、セキュリティ、テンプレート、プラットフォーム設定
- **ADの責務**: アプリケーションコード、環境別設定（イメージタグ、レプリカ数）
- **明確な境界**: ADはインフラを変更できず、他チームのリソースにもアクセス不可

## 7. モニタリングと可観測性

- **Prometheus**: すべてのクラスターコンポーネントとアプリケーションからメトリクス収集
- **ServiceMonitor**: 各アプリケーションにServiceMonitor CRが含まれ、自動的にメトリクス収集
- **Istioテレメトリー**: サービスメッシュが分散トレーシングと可観測性を提供

## 8. 設計原則

1. **GitOps優先**: すべての変更はGitコミットとArgo CD同期を経由
2. **最小権限**: RBACが必要最小限の権限を強制
3. **環境分離**: ProdとDev環境間の厳格な分離
4. **セルフサービス**: ADがPEの介入なしでアプリケーションをデプロイ・管理可能
5. **デフォルトでセキュア**: すべての外部エンドポイントでIstio + Keycloak認証
6. **宣言的設定**: すべてのリソースをYAMLマニフェストで宣言的に定義
7. **イミュータブルインフラ**: クラスターへの直接変更は一切行わない

