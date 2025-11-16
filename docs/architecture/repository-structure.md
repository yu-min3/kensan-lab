# GitOps リポジトリ構成

このドキュメントは、プラットフォームエンジニア（PE）とアプリケーション開発者（AD）の完全な分離を可能にする3リポジトリアーキテクチャについて説明します。

## 1. プラットフォーム設定リポジトリ（goldship-platform）

**責務**: クラスター基盤コンポーネント、セキュリティ設定、GitOps制御構造
**管理者**: プラットフォームエンジニア（PE）
**現在のリポジトリ**: このリポジトリ

### ディレクトリ構造

```
goldship-platform/
├── base-infra/
│   ├── argocd/
│   │   ├── projects/
│   │   │   ├── platform-project.yaml      # PE管理のインフラプロジェクト
│   │   │   ├── app-project-prod.yaml      # Prod環境App Project
│   │   │   └── app-project-dev.yaml       # Dev環境App Project
│   │   ├── root-apps/
│   │   │   ├── 01-platform-infra-apps.yaml    # 基盤コンポーネント用Root App
│   │   │   └── 02-app-management-app.yaml     # アプリケーションCR管理用Root App（App of Apps）
│   │   └── applications/
│   │       ├── app-prod-<name>.yaml       # BackstageがProdアプリ用に自動コミット
│   │       └── app-dev-<name>.yaml        # BackstageがDevアプリ用に自動コミット
│   ├── cilium/
│   │   ├── cilium.yaml                    # kube-proxy置き換えを有効にしたCilium CNI
│   │   ├── lb-ippool.yaml                 # LoadBalancer IPプール（192.168.0.240-249）
│   │   └── l2-announcement-policy.yaml    # L2アナウンス設定
│   ├── gateway-api/
│   │   └── gateway-api-crds.yaml          # Kubernetes Gateway API CRD定義
│   ├── istio/
│   │   ├── istio-*.yaml                   # Istio Control Planeインストールマニフェスト
│   │   ├── gateway-prod.yaml              # Prod専用Gatewayリソース（HTTPS/TLS）
│   │   └── gateway-dev.yaml               # Dev専用Gatewayリソース（HTTPS/TLS）
│   ├── cert-manager/
│   │   ├── cert-manager.yaml              # cert-managerコントローラー
│   │   ├── cluster-issuer.yaml            # Let's Encrypt ClusterIssuer（Route53 DNS）
│   │   ├── route53-secret.yaml            # AWS Route53認証情報（SealedSecret）
│   │   └── wildcard-certificate.yaml      # ワイルドカード証明書（*.your-org.com）
│   ├── keycloak/
│   │   ├── base/
│   │   │   ├── kustomization.yaml
│   │   │   ├── namespace.yaml             # keycloak Namespace
│   │   │   ├── postgresql-*.yaml          # PostgreSQLデータベース
│   │   │   └── keycloak-*.yaml            # Keycloakベースマニフェスト
│   │   └── overlays/
│   │       ├── prod/
│   │       │   └── kustomization.yaml     # Prod環境向けカスタマイズ
│   │       └── dev/
│   │           └── kustomization.yaml     # Dev環境向けカスタマイズ
│   ├── backstage/
│   │   ├── base/
│   │   │   ├── kustomization.yaml
│   │   │   ├── namespace.yaml             # backstage Namespace
│   │   │   ├── postgresql-*.yaml          # PostgreSQL StatefulSet
│   │   │   ├── backstage-*.yaml           # Backstage Deployment/Service
│   │   │   └── httproute.yaml             # HTTPRoute（Istio Gateway接続）
│   │   └── overlays/
│   │       └── prod/
│   │           ├── kustomization.yaml
│   │           └── sealed-secrets.yaml    # 暗号化シークレット（DB/GitHub）
│   ├── sealed-secret/
│   │   ├── controller.yaml                # Sealed Secretsコントローラー
│   │   ├── ghcr-pull-secret-prod.yaml     # Prod用暗号化GHCR認証情報
│   │   └── ghcr-pull-secret-dev.yaml      # Dev用暗号化GHCR認証情報
│   ├── app-prod/
│   │   └── namespace.yaml                 # 本番アプリケーションNamespace
│   ├── app-dev/
│   │   └── namespace.yaml                 # 開発アプリケーションNamespace
│   ├── kube-system/
│   │   └── *.yaml                         # コアシステムコンポーネント
│   ├── local-path-provisioner/
│   │   └── *.yaml                         # 動的PVプロビジョナー（ベアメタル用）
│   └── prometheus/
│       └── *.yaml                         # Prometheus設定（実装予定）
├── backstage-app/
│   ├── packages/
│   │   ├── app/                           # Backstageフロントエンド
│   │   └── backend/                       # Backstageバックエンド + Dockerfile
│   ├── plugins/                           # カスタムプラグイン
│   ├── app-config.yaml                    # ローカル開発設定
│   ├── app-config.kubernetes.yaml         # Kubernetes本番設定
│   ├── Makefile                           # ビルド自動化
│   └── yarn.sh                            # Yarnラッパースクリプト
├── docs/
│   ├── architecture/                      # アーキテクチャドキュメント
│   ├── adr/                               # Architecture Decision Records
│   ├── roadmap.md                         # 実装ロードマップ
│   ├── bootstrapping.md                   # ブートストラップ手順
│   └── configuration.md                   # 設定ガイド
├── my_docs/                               # 設計資料（git-ignored）
│   ├── tasks.md                           # タスク管理
│   ├── design.md                          # 設計メモ
│   └── repository_design.md               # リポジトリ設計
├── temp/                                  # 一時ファイル（git-ignored）
│   ├── ghcr-secret-raw*.yaml              # 生シークレット（絶対にコミット禁止）
│   ├── backstage-*-secret-raw.yaml        # Backstageシークレット（コミット禁止）
│   └── test-app/                          # テストアプリケーション
└── CLAUDE.md                              # Claude Code用指示書
```

### 主な特徴

- **Infrastructure as Code**: すべてのクラスターコンポーネントを宣言的に定義
- **GitOps制御**: Argo CD ProjectsとRoot Applicationsがプラットフォームを管理
- **自動生成アプリ**: BackstageがApplication CRを`base-infra/argocd/applications/`にコミット（実装予定）
- **シークレットの安全性**: Sealed Secretsが認証情報をGitコミット前に暗号化
- **環境分離**: ProdとDevのリソースを明確に分離
- **TLS証明書自動管理**: cert-managerがLet's Encryptからワイルドカード証明書を自動取得・更新
- **Kustomizeベース設計**: Keycloak/BackstageはKustomize base + overlays構造で環境差分を管理
- **開発者ポータル**: Backstageアプリケーションがカスタムイメージとしてデプロイ済み

---

## 2. アプリケーションテンプレート（backstage-app/templates/）

**責務**: 新規アプリケーション用のBackstageスキャフォールディングテンプレート（Kustomizeベース）
**管理者**: プラットフォームエンジニア（PE）
**場所**: goldship-platform リポジトリ内の `backstage-app/templates/`
**ステータス**: ✅ 実装済み

### ディレクトリ構造

```
backstage-app/templates/
├── fastapi-template/
│   ├── template.yaml                      # Backstageテンプレート実行定義
│   ├── skeleton/
│   │   ├── app/
│   │   │   └── main.py                    # アプリケーションコードの雛形
│   │   ├── Dockerfile                     # Dockerビルドファイル
│   │   ├── docs/
│   │   │   └── index.md                   # Backstage TechDocsソース
│   │   ├── base/                          # Kustomizeベースマニフェスト
│   │   │   ├── kustomization.yaml
│   │   │   ├── deployment.yaml            # ベースDeployment
│   │   │   ├── service.yaml               # ベースService
│   │   │   ├── httproute.yaml             # HTTPRoute（Gatewayにアタッチ）
│   │   │   ├── authz-policy.yaml          # Istio AuthorizationPolicy（アプリ固有）
│   │   │   └── servicemonitor.yaml        # Prometheus ServiceMonitor
│   │   └── overlays/
│   │       ├── dev/
│   │       │   ├── kustomization.yaml     # Devオーバーレイ（baseを参照）
│   │       │   ├── image-patch.yaml       # Devイメージタグ
│   │       │   └── replica-patch.yaml     # Devレプリカ数（例: 1）
│   │       └── prod/
│   │           ├── kustomization.yaml     # Prodオーバーレイ（baseを参照）
│   │           ├── image-patch.yaml       # Prodイメージタグ
│   │           └── replica-patch.yaml     # Prodレプリカ数（例: 5）
│   └── catalog-info.yaml                  # Backstageカタログ登録
└── streamlit-template/
    └── ...                                # 他フレームワーク用の同様構造
```

### テンプレートワークフロー

1. **テンプレート作成**: PEがKustomize構造（base + overlays）のテンプレートを作成
2. **Backstage登録**: テンプレートをBackstageカタログに登録
3. **AD選択**: ADがBackstage UIからテンプレートを選択
4. **リポジトリ生成**: Backstageが新しい`app-<name>`リポジトリをスキャフォールド
5. **GitOps登録**: BackstageがApplication CRを`goldship-platform`に自動コミット
6. **自動デプロイ**: Argo CDが変更を検出し両環境にデプロイ

---

## 3. 生成されたアプリケーションリポジトリ（app-<app-name>）

**責務**: アプリケーションコード、Dockerfile、TechDocs、環境別デプロイ設定
**管理者**: アプリケーション開発者（AD）
**リポジトリ**: Backstageが生成するアプリケーション毎のリポジトリ
**ステータス**: 🚧 実装予定（Phase 5-6で検証）

### 例: app-fastapi-user

```
app-fastapi-user/
├── app/
│   └── main.py                            # アプリケーションコード（ADが実装）
├── Dockerfile                             # コンテナビルド定義
├── docs/
│   └── index.md                           # TechDocsドキュメント
├── catalog-info.yaml                      # Backstageカタログエントリ
├── base/                                  # 環境非依存のベースマニフェスト
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── httproute.yaml
│   ├── authz-policy.yaml
│   └── servicemonitor.yaml
└── overlays/
    ├── dev/
    │   ├── kustomization.yaml             # ../baseを参照
    │   ├── image-patch.yaml               # Devイメージタグ（GitOpsトリガー）
    │   └── replica-patch.yaml             # Devレプリカ数
    └── prod/
        ├── kustomization.yaml             # ../baseを参照
        ├── image-patch.yaml               # Prodイメージタグ（GitOpsトリガー）
        └── replica-patch.yaml             # Prodレプリカ数
```

### ADワークフロー

1. **コード開発**: ADが`app/`ディレクトリ内のコードを変更
2. **イメージビルド**: ADがDockerイメージをビルドしてGHCRにプッシュ
3. **デプロイ更新**: ADが`image-patch.yaml`を新しいイメージタグで更新
4. **GitOps同期**: Argo CDが変更を検出してアプリケーションを再デプロイ
5. **ドキュメント**: ADが`docs/`ディレクトリのTechDocsを更新
6. **環境プロモーション**: ADがDevとProdで異なるイメージタグを使用可能

### 主な機能

- **Kustomizeベース**: ベースマニフェスト + 環境別パッチ
- **GitOpsトリガー**: `overlays/*/image-patch.yaml`のイメージタグ変更で再デプロイ
- **セルフサービス**: ADがアプリケーションライフサイクルを完全に制御
- **セキュリティ**: ADはインフラを変更できず、他のアプリにもアクセス不可
- **可観測性**: ServiceMonitorが自動的にPrometheusスクレイピングを有効化

---

## リポジトリ統合フロー

```
┌──────────────────────────────────────┐
│   goldship-platform                  │  （PE管理）
│   ├── backstage-app/templates/      │
│   │   └── fastapi-template/         │  テンプレート定義
│   └── base-infra/argocd/            │  Application CRs
└──────────────┬───────────────────────┘
               │
               │ Backstageがスキャフォールド＆Application CRを自動コミット
               ▼
┌─────────────────────┐
│  app-<name>         │  （AD管理）
│  （AD管理）         │  生成されたアプリケーションリポジトリ
│                     │      argocd/applications/へ  │                     │
│  - コード           │                              │ - Argo CD Projects  │
│  - Dockerfile       │                              │ - Root Apps         │
│  - Kustomize        │                              │ - Application CRs   │
│  - TechDocs         │                              │ - インフラ          │
└─────────────────────┘                              └──────────┬──────────┘
           │                                                    │
           │                                                    │
           │                  ┌──────────────────┐             │
           └─────────────────▶│   Argo CD        │◀────────────┘
                              │   （Gitを監視）  │
                              └────────┬─────────┘
                                       │
                                       ▼
                              ┌──────────────────┐
                              │  Kubernetes      │
                              │  クラスター      │
                              └──────────────────┘
```

## この構成のメリット

1. **明確な分離**: PEとADが異なるリポジトリと責務を持つ
2. **GitOpsネイティブ**: すべての変更がGitとArgo CDを経由
3. **セルフサービス**: ADがPEの介入なしでアプリを作成・デプロイ可能（実装予定）
4. **セキュリティ**: RBACとArgo CD Projectsがアクセス境界を強制
5. **一貫性**: テンプレートがすべてのアプリでプラットフォーム標準に準拠（実装予定）
6. **監査可能性**: Git履歴が完全な監査証跡を提供
7. **スケーラビリティ**: 新しいアプリケーションと環境の追加が容易

## 現在の実装状況

### ✅ 実装完了
- **goldship-platform リポジトリ**: 基盤インフラのGitOps管理が完全に機能
  - Argo CD Projects/Root Apps構造
  - Istio + cert-manager + Keycloak デプロイ済み
  - Backstage デプロイ済み（PostgreSQL + Deployment + HTTPRoute）
  - Sealed Secrets による安全なシークレット管理
  - **Backstageテンプレート**: `backstage-app/templates/` に統合済み

### 🚧 実装予定
- **app-<name> リポジトリ**: Backstageによる自動生成フロー（Phase 5-6）
- **自動GitOps統合**: Application CR自動コミット機能（Phase 5）

## 関連ドキュメント

- [プラットフォームアーキテクチャ設計](./design.md)
- [実装ロードマップ](../roadmap.md)
