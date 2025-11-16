# 実装ロードマップ

本ドキュメントはK8s GitOpsプラットフォームの実装進捗を追跡します。

## 概要

プラットフォームは5つのフェーズで構築され、クラスター初期化からアプリケーション開発者の完全なセルフサービス機能まで進めています。

**現在のステータス**: Phase 3完了 ✅ - Phase 4進行中（開発者エクスペリエンス）

---

## Phase 1: クラスター初期化 ✅ 完了

ベアメタルKubernetesクラスターの基盤セットアップ。

| タスクID | タスク | ステータス | 備考 |
|---------|------|--------|-------|
| I-1/I-2 | クラスターのバックアップと初期化 | ✅ 完了 | kubeadmでMasterノード初期化、kubeconfigを開発マシンにコピー |
| I-4/I-3 | Cilium CNI with LoadBalancer | ✅ 完了 | L2アナウンス有効化のCNIをデプロイ、Cilium LoadBalancerでMetalLBを置き換え |
| I-6/I-7 | GHCR設定とSealed Secrets | ✅ 完了 | Sealed Secretsコントローラーをデプロイ、prod/dev用GHCR pull secretsを暗号化 |

### 達成事項

- ベアメタルハードウェア上でKubernetesクラスターが稼働
- CRI-Oコンテナランタイムを設定
- kube-proxy置き換えを有効にしたCilium CNI
- Cilium LoadBalancer with L2アナウンス
- シークレット管理用Sealed Secretsコントローラーをインストール
- GHCR認証シークレットを暗号化し使用可能な状態に

---

## Phase 2: GitOps基盤構築 ✅ 完了

Argo CDをGitOpsエンジンとして確立し、インフラ管理を整備。

| タスクID | タスク | ステータス | 備考 |
|---------|------|--------|-------|
| I-8 | Argo CDブートストラッピング | ✅ 完了 | Argo CDをインストール、LoadBalancer経由でアクセス可能 |
| I-9 | Argo CD Project作成 | ✅ 完了 | `platform-project`、`app-project-prod`、`app-project-dev`を作成、GitOps管理下に |
| I-11 | インフラのGitOps化 | ✅ 完了 | すべてのインフラコンポーネントをGitOps管理下に移行 |
| I-12 | NamespaceとImagePullSecretのデプロイ | ✅ 完了 | app-prod/app-dev NamespaceをServiceAccount設定と共にデプロイ |

### 達成事項

- Argo CDが稼働しLoadBalancer経由でアクセス可能
- プラットフォーム、本番、開発用のGitOps Projectsを作成
- Root Applicationsによる「App of Apps」パターンを確立
- すべてのインフラコンポーネント設定をGitOps管理下に
- Namespace labelingの統一設計を導入（`goldship.platform/*`）

### 技術的改善

- **CRD分離パターン導入**: Prometheus, Argo CD, Cert-ManagerでCRDを`00-crds.yaml`に分離
  - Git Diffの可読性向上（最大96%のファイルサイズ削減）
  - Argo CDでのデプロイ順序制御の改善
  - 再生成スクリプト（`scripts/split_crds.py`）の追加

---

## Phase 3: サービスメッシュと認証 ✅ 完了

セキュリティとアクセス制御のコアコンポーネントをデプロイ。

| タスクID | タスク | 責務 | ステータス | 詳細 |
|---------|------|---------------|--------|---------|
| I-10 | Istio & Keycloakデプロイ | PE | ✅ 完了 | Istio Control Plane、Keycloakインスタンス（Prod/Dev）、Istio Gatewaysをデプロイ |
| I-11 | Prometheus/Backstageデプロイ | PE | ✅ 完了 | GitOps管理下でモニタリングと開発者ポータルをデプロイ |
| I-11b | Cert-Managerデプロイ | PE | ✅ 完了 | Let's Encrypt統合でTLS証明書自動発行を実現 |

### 達成事項

#### Istioサービスメッシュ
- `istio-system` NamespaceにIstio Control Planeをデプロイ
- 環境別Gateways（`gateway-platform`, `gateway-prod`, `gateway-dev`）を作成
- HTTPRouteによるトラフィックルーティング設定
- mTLSによるPod間通信の暗号化

#### Keycloak認証基盤
- Prod環境用Keycloakインスタンスをデプロイ（`platform-auth-prod`）
- Dev環境用Keycloakインスタンスをデプロイ（`platform-auth-dev`）
- Kustomizeベース構成（base + overlays）による環境分離
- PostgreSQLバックエンドとSealed Secretsによるシークレット管理

#### Cert-Manager
- DNS-01チャレンジ対応（AWS Route53統合）
- Let's Encryptによるワイルドカード証明書自動発行
- 証明書自動更新機能

#### Prometheusモニタリング
- `monitoring` NamespaceにPrometheus Stackをデプロイ
- Grafana統合による可視化
- ServiceMonitor CRDsによる自動メトリクス収集
- Istioとアプリケーションからのメトリクス収集

#### Backstage開発者ポータル
- 専用Namespaceにカスタムビルドしたbackstageをデプロイ
- PostgreSQLバックエンド構成
- GHCRからのプライベートイメージ取得（Sealed Secrets管理）
- Kustomize構成（base + overlays/prod）

### アーキテクチャ決定記録（ADR）

Phase 3の実装で以下のADRを文書化：

- **[ADR-001: TLS終端パターン](./adr/001-tls-termination-pattern.md)**
  - Istio GatewayでのTLS証明書管理
  - Cert-Manager + Let's Encrypt統合

- **[ADR-002: 認証・認可アーキテクチャ](./adr/002-authentication-authorization-architecture.md)**
  - Keycloak + Istio RequestAuthenticationによるJWT検証
  - 外部トラフィック認証フロー

---

## Phase 4: 開発者エクスペリエンス 🚧 進行中

アプリケーション開発者向けセルフサービス機能の構築。

| タスクID | タスク | 責務 | ステータス | 詳細 |
|---------|------|---------------|--------|---------|
| I-13 | Backstageテンプレート作成 | PE | ✅ 完了 | backstage-app/templates/にKustomizeベーステンプレートを作成、Argo CD Application CRs自動生成機能を含む |
| I-14 | Backstageカタログ登録 | PE | ⏳ 保留中 | テンプレートが発見可能で、アプリケーション開発者が使用可能であることを確認 |

### 達成事項

- **テンプレート構造**: Kustomizeベーステンプレート（FastAPI例）を作成
- **自動GitOps統合**: テンプレートがArgo CD Application CRsを自動生成する機能を実装
- **デュアル環境サポート**: Prod/Dev両環境への自動デプロイ対応

### テンプレート機能

作成済みのテンプレートは以下の機能を提供：

- Kustomizeベース構造（base + overlays/dev + overlays/prod）
- デュアル環境サポート（ProdとDev）
- Istio HTTPRoute統合
- Prometheus ServiceMonitor包含
- Keycloak AuthorizationPolicyテンプレート
- TechDocsスケルトンドキュメント
- GitHub ActionsによるCI/CD設定

### 次のステップ

- Backstage UIでのテンプレート可視化確認
- テンプレートのドキュメント改善
- 複数のテンプレート追加（他の言語・フレームワーク）

---

## Phase 5: アプリケーション検証 ⏳ 未着手

実際のアプリケーションデプロイでプラットフォームをエンドツーエンドテスト。

| タスクID | タスク | 責務 | ステータス | 詳細 |
|---------|------|---------------|--------|---------|
| I-15 | 新規アプリケーション作成（Dev） | AD | ⏳ 保留中 | Backstageスキャフォールディングをテスト、goldship-platformへ自動コミット |
| I-16 | Devデプロイ検証 | PE/AD | ⏳ 保留中 | Argo CDの自動デプロイ、HTTPRouteルーティングが機能することを確認 |
| I-17 | 新規アプリケーション作成（Prod） | AD | ⏳ 保留中 | 本番環境スキャフォールディングをテスト |
| I-18 | Prodデプロイ検証 | PE/AD | ⏳ 保留中 | Istio + Keycloak JWT認証が機能することを確認 |
| I-19 | コード変更検証 | AD | ⏳ 保留中 | イメージタグ更新がGitOps再デプロイをトリガーすることをテスト |
| I-20 | TechDocs検証 | AD | ⏳ 保留中 | ドキュメントがBackstageで正しくレンダリングされることを確認 |

### 検証基準

- ✅ ADがPEの介入なしでBackstage経由で新規アプリケーションを作成可能
- ✅ Application CRsがgoldship-platformに自動コミット
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

- **GitOpsカバレッジ**: インフラとアプリケーション100%がGit管理 ✅
- **デプロイ自動化**: デプロイに手動kubectlコマンド不要 ✅
- **セキュリティ**: すべての外部エンドポイントでIstio + Keycloak認証 🚧
- **セルフサービス**: ADが5分以内にアプリケーションを作成・デプロイ可能 ⏳
- **可観測性**: すべてのアプリケーションがPrometheusで自動監視 ✅

### 運用指標

- **デプロイまでの平均時間**: Gitコミットから本番デプロイまで2分以内 ✅
- **環境パリティ**: DevとProdが同一のベースマニフェストを使用 ✅
- **ドキュメントカバレッジ**: アプリケーション100%がTechDocsを持つ ⏳
- **監査証跡**: すべてのインフラとアプリケーション変更の完全なGit履歴 ✅

---

## 技術的改善と設計変更

### CRD分離パターン（Phase 2で導入）

大規模なHelm Chart出力（Prometheus, Argo CD, Cert-Manager）でCustomResourceDefinitionsを分離：

**Before:**
```
prometheus/prometheus-stack.yaml  (4.7MB, 76,248行)
```

**After:**
```
prometheus/
├── 00-crds.yaml          (4.1MB, 10 CRDs)
├── prometheus-stack.yaml (626KB, 134 resources)
```

**メリット:**
- Git Diffの可読性向上（設定変更時、CRDの巨大Diffを回避）
- Argo CDでのデプロイ順序制御（00-crds.yamlが最初に適用）
- 再生成の容易化（scripts/split_crds.py）

### Namespace Label統一設計（Phase 2で導入）

すべてのNamespaceに一貫したラベルスキーマを適用：

```yaml
metadata:
  labels:
    app.kubernetes.io/managed-by: argocd
    goldship.platform/environment: production|development|infrastructure
    goldship.platform/tier: platform|application
    goldship.platform/component: <component-name>  # platformのみ
    istio-injection: enabled  # 必要に応じて
```

**活用例:**
- NetworkPolicyでのNamespace選択
- RBACポリシーでの権限制御
- kubectl フィルタリング（`kubectl get ns -l goldship.platform/tier=platform`）

詳細は[Namespaceラベル設計ドキュメント](./namespace-label-design.md)を参照。

### Kustomize使用ガイドライン（Phase 2で導入）

コンポーネントの性質に応じたKustomize使用基準を明確化：

- **Tier 1（必須）**: 環境分離 + 頻繁な更新（Keycloak）
- **Tier 2（推奨）**: 頻繁な更新のみ（Backstage）
- **Tier 3（フラットYAML）**: 低頻度更新（Istio, Prometheus等）

詳細は[Kustomize使用ガイドライン](./kustomize-guidelines.md)を参照。

---

## 将来の拡張

Phase 5以降の潜在的な改善項目：

### セキュリティ・コンプライアンス
- **ポリシー強制**: セキュリティとコンプライアンス用OPA/Gatekeeper
- **脆弱性スキャン**: Trivyによるコンテナイメージスキャン
- **SBOM生成**: ソフトウェア部品表の自動生成

### 開発者エクスペリエンス
- **マルチテナンシー**: リソースクォータによるチーム単位のNamespace分離
- **CI/CD統合**: イメージビルド用GitHub Actions統合
- **プログレッシブデリバリー**: Istioによるカナリアデプロイとトラフィック分割
- **Backstageリポジトリ初期化時のシークレット設定**: テンプレートからリポジトリ初期化時の自動シークレット設定

### 運用改善
- **自動テスト**: Prodプロモーション前のDev環境での統合テスト
- **災害復旧**: 自動バックアップとリストア手順（Velero等）
- **高度なモニタリング**: Grafanaダッシュボードとアラートルール
- **コスト最適化**: リソース使用量追跡と推奨事項

---

## タイムライン

| フェーズ | ステータス | 完了時期 |
|-------|--------|---------|
| Phase 1: クラスター初期化 | ✅ 完了 | 2024 Q3 |
| Phase 2: GitOps基盤 | ✅ 完了 | 2024 Q4 |
| Phase 3: サービスメッシュと認証 | ✅ 完了 | 2024 Q4 |
| Phase 4: 開発者エクスペリエンス | 🚧 進行中 | 2025 Q1 (予定) |
| Phase 5: アプリケーション検証 | ⏳ 計画中 | 2025 Q1-Q2 (予定) |

---

## 関連ドキュメント

### アーキテクチャ・設計
- [プラットフォームアーキテクチャ設計](./architecture/design.md)
- [リポジトリ構成](./architecture/repository-structure.md)
- [ADR-001: TLS終端パターン](./adr/001-tls-termination-pattern.md)
- [ADR-002: 認証・認可アーキテクチャ](./adr/002-authentication-authorization-architecture.md)

### 運用ガイド
- [ブートストラッピング手順](./bootstrapping.md)
- [環境固有の設定変更ガイド](./configuration.md)

### 開発ガイドライン
- [Kustomize使用ガイドライン](./kustomize-guidelines.md)
- [Namespaceラベル設計](./namespace-label-design.md)
