# 実装ロードマップ

本ドキュメントはK8s GitOpsプラットフォームの実装進捗を追跡します。

## 概要

プラットフォームは5つのフェーズで構築され、クラスター初期化からアプリケーション開発者の完全なセルフサービス機能まで進めています。

**現在のステータス**: Phase 2完了 ✅ - Phase 3へ移行（サービスメッシュと認証）

---

## Phase 1: クラスター初期化 ✅ 完了

ベアメタルKubernetesクラスターの基盤セットアップ。

| タスクID | タスク | ステータス | 備考 |
|---------|------|--------|-------|
| I-1/I-2 | クラスターのバックアップと初期化 | ✅ 完了 | kubeadmでMasterノード初期化、kubeconfigを開発マシンにコピー |
| I-4/I-3 | Cilium CNI with LoadBalancer | ✅ 完了 | L2アナウンス有効化のCNIをデプロイ、Cilium LoadBalancerでMetalLBを置き換え（IPプール: 192.168.0.240-249） |
| I-6/I-7 | GHCR設定とSealed Secrets | ✅ 完了 | Sealed Secretsコントローラーをデプロイ、prod/dev用GHCR pull secretsを暗号化 |

### 達成事項

- Raspberry Piハードウェア上でKubernetesクラスターが稼働
- CRI-Oコンテナランタイムを設定
- kube-proxy置き換えを有効にしたCilium CNI
- Cilium LoadBalancer with L2アナウンス（インターフェース: wlan0）
- シークレット管理用Sealed Secretsコントローラーをインストール
- GHCR認証シークレットを暗号化し使用可能な状態に

---

## Phase 2: GitOps基盤構築 ✅ 完了

Argo CDをGitOpsエンジンとして確立し、インフラ管理を整備。

| タスクID | タスク | ステータス | 備考 |
|---------|------|--------|-------|
| I-8 | Argo CDブートストラッピング | ✅ 完了 | Argo CDをインストール、LoadBalancer経由でアクセス可能（192.168.0.240） |
| I-9 | Argo CD Project作成 | ✅ 完了 | `platform-project`、`app-project-prod`、`app-project-dev`を作成、GitOps管理下に |
| I-11 | インフラのGitOps化 | 🔄 部分完了 | Cilium LoadBalancerをGitOps管理下に。Prometheus/BackstageはPhase 3で |
| I-12 | NamespaceとImagePullSecretのデプロイ | ⏳ 保留中 | app-prod/app-dev NamespaceをServiceAccount設定と共にデプロイ |

### 達成事項

- Argo CDが稼働し http://192.168.0.240 でアクセス可能
- プラットフォーム、本番、開発用のGitOps Projectsを作成
- Root Applicationsによる「App of Apps」パターンを確立
- Cilium設定をGitOps管理下に
- インフラコンポーネントデプロイの準備完了

### 次のステップ

- app-prodとapp-dev Namespaceをデプロイ
- GHCR imagePullSecretsを持つServiceAccountsを設定
- Phase 3のインフラデプロイメント開始

---

## Phase 3: サービスメッシュと認証 ⏳ 進行中

セキュリティとアクセス制御のコアコンポーネントをデプロイ。

| タスクID | タスク | 責務 | ステータス | 詳細 |
|---------|------|---------------|--------|---------|
| I-10 | Istio & Keycloakデプロイ | PE | ⏳ 保留中 | Istio Control Plane、Keycloakインスタンス、Istio Gateways（Prod/Dev）をデプロイ |
| I-11 | Prometheus/Backstageデプロイ | PE | ⏳ 保留中 | GitOps管理下でモニタリングと開発者ポータルをデプロイ |

### 目標

- **Istioサービスメッシュ**: トラフィック管理、セキュリティ、可観測性を提供
- **Keycloak認証**: 外部エンドポイント用JWT認証
- **環境分離**: ProdとDev環境用に独立したGateways
- **モニタリング基盤**: メトリクス収集用Prometheus
- **開発者ポータル**: セルフサービスアプリケーション作成用Backstage

### 実装手順

1. **Istioインストール**
   - `istio-system` NamespaceにIstio Control Planeをデプロイ
   - Prod（`gateway-prod.yaml`）とDev（`gateway-dev.yaml`）用Gatewayリソースを作成
   - Keycloak JWT検証用にIstioを設定

2. **Keycloakデプロイ**
   - Prod環境用Keycloakインスタンスをデプロイ
   - Dev環境用Keycloakインスタンスをデプロイ
   - Realm、クライアント、JWT設定を構成

3. **Prometheusセットアップ**
   - `monitoring` NamespaceにPrometheusをデプロイ
   - 自動検出用ServiceMonitor CRDsを設定
   - Istioとアプリケーションからのメトリクス収集を設定

4. **Backstageデプロイ**
   - 専用NamespaceにBackstageをデプロイ
   - カタログ統合を設定
   - テンプレート統合の準備（Phase 4）

---

## Phase 4: 開発者エクスペリエンス ⏳ 保留中

アプリケーション開発者向けセルフサービス機能の構築。

| タスクID | タスク | 責務 | ステータス | 詳細 |
|---------|------|---------------|--------|---------|
| I-13 | Backstageテンプレート作成 | PE | ✅ 完了 | backstage-app/templates/にKustomizeベーステンプレートを作成、Argo CD Application CRs自動生成機能を含む |
| I-14 | Backstageカタログ登録 | PE | ⏳ 保留中 | テンプレートが発見可能で、アプリケーション開発者が使用可能であることを確認 |

### 目標

- **テンプレート作成**: PEが再利用可能なアプリケーションスキャフォールディングテンプレートを作成
- **自動GitOps**: テンプレートが自動的にArgo CD Application CRsを生成
- **カタログ統合**: テンプレートがADセルフサービス用にBackstageに表示
- **ドキュメント**: アプリケーションドキュメント用TechDocs統合

### テンプレート機能

- Kustomizeベース構造（base + overlays）
- デュアル環境サポート（ProdとDev）
- Istio HTTPRoute統合
- Prometheus ServiceMonitor包含
- Keycloak AuthorizationPolicyテンプレート
- TechDocsスケルトンドキュメント

---

## Phase 5: アプリケーション検証 ⏳ 保留中

実際のアプリケーションデプロイでプラットフォームをエンドツーエンドテスト。

| タスクID | タスク | 責務 | ステータス | 詳細 |
|---------|------|---------------|--------|---------|
| I-15 | 新規アプリケーション作成（Dev） | AD | ⏳ 保留中 | Backstageスキャフォールディングをテスト、platform-configへ自動コミット |
| I-16 | Devデプロイ検証 | PE/AD | ⏳ 保留中 | Argo CDの自動デプロイ、HTTPRouteルーティングが機能することを確認 |
| I-17 | 新規アプリケーション作成（Prod） | AD | ⏳ 保留中 | 本番環境スキャフォールディングをテスト |
| I-18 | Prodデプロイ検証 | PE/AD | ⏳ 保留中 | Istio + Keycloak JWT認証が機能することを確認 |
| I-19 | コード変更検証 | AD | ⏳ 保留中 | イメージタグ更新がGitOps再デプロイをトリガーすることをテスト |
| I-20 | TechDocs検証 | AD | ⏳ 保留中 | ドキュメントがBackstageで正しくレンダリングされることを確認 |

### 検証基準

- ✅ ADがPEの介入なしでBackstage経由で新規アプリケーションを作成可能
- ✅ Application CRsがplatform-configに自動コミット
- ✅ Argo CDが変更を検出し両環境にデプロイ
- ✅ HTTPRouteがIstio Gateway経由でトラフィックを正しくルーティング
- ✅ Keycloak JWT認証がProdエンドポイントで機能
- ✅ Prometheusが自動的にアプリケーションメトリクスを収集
- ✅ TechDocsがBackstageで正しくレンダリング
- ✅ イメージタグ変更が自動再デプロイをトリガー
- ✅ 環境分離（Prod/Dev）が強制される
- ✅ ADがインフラコンポーネントにアクセスまたは変更不可

---

## 成功指標

### 技術的指標

- **GitOpsカバレッジ**: インフラとアプリケーション100%がGit管理
- **デプロイ自動化**: デプロイに手動kubectlコマンド不要
- **セキュリティ**: すべての外部エンドポイントでIstio + Keycloak認証
- **セルフサービス**: ADが5分以内にアプリケーションを作成・デプロイ可能
- **可観測性**: すべてのアプリケーションがPrometheusで自動監視

### 運用指標

- **デプロイまでの平均時間**: Gitコミットから本番デプロイまで2分以内
- **環境パリティ**: DevとProdが同一のベースマニフェストを使用
- **ドキュメントカバレッジ**: アプリケーション100%がTechDocsを持つ
- **監査証跡**: すべてのインフラとアプリケーション変更の完全なGit履歴

---

## 将来の拡張

Phase 5以降の潜在的な改善項目：

- **マルチテナンシー**: リソースクォータによるチーム単位のNamespace分離
- **自動テスト**: Prodプロモーション前のDev環境での統合テスト
- **災害復旧**: 自動バックアップとリストア手順
- **高度なモニタリング**: Grafanaダッシュボードとアラートルール
- **コスト最適化**: リソース使用量追跡と推奨事項
- **CI/CD統合**: イメージビルド用GitHub Actions統合
- **プログレッシブデリバリー**: Istioによるカナリアデプロイとトラフィック分割
- **ポリシー強制**: セキュリティとコンプライアンス用OPA/Gatekeeper
- **Backstageリポジトリ初期化時のシークレット設定**: 現在はカスタムアクションが必要なため見送っているが、将来的にBackstageのテンプレートからリポジトリを初期化する際に、自動的にシークレットを設定できるようにする。

---

## タイムライン

| フェーズ | 完了目標 | ステータス |
|-------|------------------|--------|
| Phase 1: クラスター初期化 | ✅ 完了 | 完了 |
| Phase 2: GitOps基盤 | ✅ 完了 | 完了 |
| Phase 3: サービスメッシュと認証 | 🎯 進行中 | アクティブ |
| Phase 4: 開発者エクスペリエンス | 📅 次 | 計画中 |
| Phase 5: アプリケーション検証 | 📅 将来 | 計画中 |

---

## 関連ドキュメント

- [プラットフォームアーキテクチャ設計](./architecture/design.md)
- [リポジトリ構成](./architecture/repository-structure.md)
- [クイックスタートガイド](./guides/quickstart.md) *（準備中）*
- [CLAUDE.md](../CLAUDE.md) - Claude Code用指示書（git-ignored）
