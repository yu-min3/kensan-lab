# GitOps リポジトリ構成

このドキュメントは、プラットフォームエンジニア（PE）とアプリケーション開発者（AD）の完全な分離を可能にするアーキテクチャについて説明します。中心となるのはこの`kensan-lab`リポジトリです。

## 1. プラットフォーム設定リポジトリ（kensan-lab）

**責務**: クラスター基盤コンポーネント、セキュリティ設定、GitOps制御構造、Backstageアプリケーションのソースとテンプレート
**管理者**: プラットフォームエンジニア（PE）

### ディレクトリ構造

```
kensan-lab/
├── infrastructure/                # Kubernetes infrastructure manifests
│   ├── argocd/                # Argo CD (with CRDs split)
│   │   ├── 00-crds.yaml       # Argo CD CRDs
│   │   ├── argocd-install.yaml # Argo CD main installation
│   │   ├── projects/          # AppProject definitions (platform, app-dev, app-prod)
│   │   ├── root-apps/         # Root Application (App of Apps)
│   │   └── applications/      # Platform Application CRs
│   ├── cilium/                # Cilium CNI + LoadBalancer configuration
│   ├── istio/                 # Istio Control Plane and Gateways
│   ├── keycloak/              # Keycloak (Kustomize: base + overlays/dev + overlays/prod)
│   ├── prometheus/            # Prometheus Stack (with CRDs split)
│   ├── cert-manager/          # Cert-Manager (with CRDs split)
│   ├── sealed-secret/         # Sealed Secrets controller
│   ├── gateway-api/           # Gateway API CRDs
│   ├── local-path-provisioner/ # Dynamic PV provisioner
│   ├── backstage/             # Backstage (Kustomize: base + overlays/prod)
│   ├── app-dev/               # Development environment Namespace definition
│   ├── app-prod/              # Production environment Namespace definition
│   └── kube-system/           # kube-system Namespace definition
├── backstage-app/             # Backstage application source
│   ├── packages/
│   │   ├── app/               # Frontend React app
│   │   └── backend/           # Backend Express app + Dockerfile
│   ├── templates/             # Application scaffolding templates
│   │   └── fastapi-template/  # Example FastAPI Kustomize template
│   ├── plugins/               # Custom Backstage plugins
│   ├── app-config.yaml        # Configuration for local development
│   ├── app-config.kubernetes.yaml # Configuration for Kubernetes production
│   ├── Makefile               # Build automation
│   └── yarn.sh                # Yarn wrapper script
├── docs/                      # Documentation
│   ├── architecture/          # Architecture and design documents
│   │   ├── design.md
│   │   └── repository-structure.md
│   ├── adr/                   # Architecture Decision Records
│   ├── bootstrapping/         # Guide to building from scratch
│   ├── configuration.md       # Guide to environment-specific configuration
│   ├── kustomize-guidelines.md # Guidelines for using Kustomize
│   ├── namespace-label-design.md # Namespace label design
│   └── roadmap.md             # Implementation roadmap
├── temp/                      # Temporary files (git-ignored)
│   └── ghcr-secret-raw.yaml   # Unencrypted secret (DO NOT COMMIT)
├── .gitignore
├── Makefile                   # For building container images
├── README.md                  # This file
└── LICENSE                    # License file
```

### 主な特徴

- **Infrastructure as Code**: すべてのクラスターコンポーネントを宣言的に定義
- **GitOps制御**: Argo CD ProjectsとRoot Applicationsがプラットフォームを管理
- **シークレットの安全性**: Sealed Secretsが認証情報をGitコミット前に暗号化
- **環境分離**: ProdとDevのリソースを明確に分離
- **TLS証明書自動管理**: cert-managerがLet's Encryptからワイルドカード証明書を自動取得・更新
- **Kustomizeベース設計**: Keycloak/BackstageはKustomize base + overlays構造で環境差分を管理
- **開発者ポータル**: Backstageアプリケーションがカスタムイメージとしてデプロイ済み

> **CRD Splitting Pattern**: For large Helm Charts (Prometheus, Argo CD, Cert-Manager),
> CustomResourceDefinitions are split into a `00-crds.yaml` file.
> This improves the readability of Git diffs and controls the deployment order in Argo CD.
> See the `README-CRD-SPLIT.md` in each component's directory for details.

---
*Note: Information about Backstage templates and the generated application repositories has been moved to `backstage-app/README.md`.*
