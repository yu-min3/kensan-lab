# GitOps リポジトリ構成

このドキュメントは、プラットフォームエンジニア（PE）とアプリケーション開発者（AD）の完全な分離を可能にする3リポジトリアーキテクチャについて説明します。

## 1. プラットフォーム設定リポジトリ（platform-config）

**責務**: クラスター基盤コンポーネント、セキュリティ設定、GitOps制御構造
**管理者**: プラットフォームエンジニア（PE）
**現在のリポジトリ**: このリポジトリ

### ディレクトリ構造

```
platform-config/
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
│   ├── istio/
│   │   ├── install/                       # Istio Control Planeインストール
│   │   ├── gateway-prod.yaml              # Prod専用Gatewayリソース
│   │   └── gateway-dev.yaml               # Dev専用Gatewayリソース
│   ├── keycloak/
│   │   ├── keycloak-prod.yaml             # ProdのKeycloakインスタンス
│   │   └── keycloak-dev.yaml              # DevのKeycloakインスタンス
│   ├── sealed-secret/
│   │   ├── controller.yaml                # Sealed Secretsコントローラー
│   │   ├── ghcr-pull-secret-prod.yaml     # Prod用暗号化GHCR認証情報
│   │   └── ghcr-pull-secret-dev.yaml      # Dev用暗号化GHCR認証情報
│   ├── rbac/
│   │   └── serviceaccount-config.yaml     # imagePullSecretsを含むServiceAccount
│   ├── namespaces/
│   │   ├── app-prod.yaml                  # 本番アプリケーションNamespace
│   │   └── app-dev.yaml                   # 開発アプリケーションNamespace
│   └── monitoring/
│       └── prometheus/                    # Prometheus設定
├── docs/
│   ├── architecture/                      # アーキテクチャドキュメント
│   └── guides/                            # ユーザーおよび運用ガイド
├── temp/                                  # 一時ファイル（git-ignored）
│   └── ghcr-secret-raw*.yaml              # 生シークレット（絶対にコミット禁止）
└── CLAUDE.md                              # Claude Code用指示書（git-ignored）
```

### 主な特徴

- **Infrastructure as Code**: すべてのクラスターコンポーネントを宣言的に定義
- **GitOps制御**: Argo CD ProjectsとRoot Applicationsがプラットフォームを管理
- **自動生成アプリ**: BackstageがApplication CRを`base-infra/argocd/applications/`にコミット
- **シークレットの安全性**: Sealed Secretsが認証情報をGitコミット前に暗号化
- **環境分離**: ProdとDevのリソースを明確に分離

---

## 2. アプリケーションテンプレートリポジトリ（app-templates）

**責務**: 新規アプリケーション用のBackstageスキャフォールディングテンプレート（Kustomizeベース）
**管理者**: プラットフォームエンジニア（PE）
**リポジトリ**: PEが管理する別リポジトリ

### ディレクトリ構造

```
app-templates/
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
5. **GitOps登録**: BackstageがApplication CRを`platform-config`に自動コミット
6. **自動デプロイ**: Argo CDが変更を検出し両環境にデプロイ

---

## 3. 生成されたアプリケーションリポジトリ（app-<app-name>）

**責務**: アプリケーションコード、Dockerfile、TechDocs、環境別デプロイ設定
**管理者**: アプリケーション開発者（AD）
**リポジトリ**: Backstageが生成するアプリケーション毎のリポジトリ

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
┌─────────────────────┐
│   app-templates     │  （PE管理）
│   テンプレート定義   │
└──────────┬──────────┘
           │
           │ Backstageがスキャフォールド
           ▼
┌─────────────────────┐      Application CRを      ┌─────────────────────┐
│  app-<name>         │      自動コミット           │ platform-config     │
│  （AD管理）         │─────────────────────────────▶│ （PE管理）          │
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
3. **セルフサービス**: ADがPEの介入なしでアプリを作成・デプロイ可能
4. **セキュリティ**: RBACとArgo CD Projectsがアクセス境界を強制
5. **一貫性**: テンプレートがすべてのアプリでプラットフォーム標準に準拠
6. **監査可能性**: Git履歴が完全な監査証跡を提供
7. **スケーラビリティ**: 新しいアプリケーションと環境の追加が容易

## 関連ドキュメント

- [プラットフォームアーキテクチャ設計](./design.md)
- [実装ロードマップ](../roadmap.md)
