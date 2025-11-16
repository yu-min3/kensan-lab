# K8s GitOps プラットフォーム

ベアメタルハードウェア上で動作する、プラットフォームエンジニア（PE）とアプリケーション開発者（AD）の完全な責務分離を実現したモダンなGitOpsベースKubernetesプラットフォーム。

## 目次

- [主な機能](#主な機能)
- [技術スタック](#技術スタック)
- [前提条件](#前提条件)
- [クイックスタート](#クイックスタート)
- [アーキテクチャ](#アーキテクチャ)
- [セットアップ](#セットアップ)
- [リポジトリ構造](#リポジトリ構造)
- [開発ワークフロー](#開発ワークフロー)
- [ドキュメント](#ドキュメント)
- [セキュリティ](#セキュリティ)
- [ライセンス](#ライセンス)

## 主な機能

- **GitOps優先**: すべてのリソースをGitとArgo CDで管理
- **セルフサービス**: アプリケーション開発者がプラットフォームエンジニアの介入なしでアプリをデプロイ可能
- **デフォルトでセキュア**: Istioサービスメッシュ + Keycloak JWT認証
- **環境分離**: 本番環境と開発環境の厳格な分離
- **宣言的設定**: Infrastructure as Code によるKubernetesマニフェスト管理
- **自動デプロイ**: Gitの変更が自動的にクラスターに同期
- **開発者ポータル**: テンプレート、ドキュメント、カタログ機能を持つBackstage

## 技術スタック

| カテゴリ | 技術 | 目的 |
|----------|-----------|---------|
| **オーケストレーション** | Kubernetes (kubeadm) | ベアメタル上のコンテナオーケストレーション |
| **コンテナランタイム** | CRI-O | 軽量なコンテナランタイム |
| **ネットワーク** | Cilium CNI | ネットワークポリシーとサービスメッシュ統合 |
| **ロードバランサ** | Cilium LoadBalancer | L2ベースの外部トラフィックルーティング |
| **サービスメッシュ** | Istio | トラフィック管理とセキュリティ |
| **認証** | Keycloak | JWTベース認証 |
| **GitOps** | Argo CD | Gitからの継続的デプロイ |
| **シークレット管理** | Sealed Secrets | Git内の暗号化シークレット |
| **モニタリング** | Prometheus + Grafana | メトリクス収集と可視化 |
| **開発者ポータル** | Backstage | セルフサービステンプレートとドキュメント |

## 前提条件

### ハードウェア

- **最小構成**: Master 1台 + Worker 1台以上
- **推奨構成**: Master 1台 + Worker 2台以上
- **メモリ**: 各ノード最低4GB、推奨8GB以上
- **ストレージ**: 各ノード最低50GB、推奨100GB以上
- **ネットワーク**: L2ネットワークでの通信が可能な環境

### ソフトウェア

**クラスター構築済み環境:**
- Kubernetes 1.27以上（kubeadmで構築）
- kubectl（クラスターバージョンと互換性のあるバージョン）
- kubeconfig設定済み

**開発マシン:**
- kubectl
- helm 3.x
- kubeseal（Sealed Secrets CLI）
- docker または podman（コンテナイメージビルド用）
- make
- Python 3.8以上（CRD分離スクリプト用）

**オプション（Backstage開発時）:**
- Node.js 18.x以上
- Yarn 4.x

### アカウント・認証情報

- GitHub アカウント（コンテナレジストリGHCR用）
- GitHub Personal Access Token（packages:write権限）
- DNS プロバイダー（Cert-Manager + Let's Encrypt用、例: AWS Route53）
- ドメイン名（TLS証明書発行用）

### ネットワーク要件

- Cilium LoadBalancer用のIP範囲（DHCPと重複しない範囲）
- 外部からアクセス可能なIPアドレス（Istio Gateway用）

> **注意**: 詳細な設定項目は[環境固有の設定変更ガイド](./docs/configuration.md)を参照してください。

## クイックスタート

### 1. Argo CD にアクセス

```bash
# Argo CD LoadBalancer IPを取得
kubectl get svc -n argocd argocd-server -o jsonpath='{.status.loadBalancer.ingress[0].ip}'

# 初期パスワード取得
kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 -d
```

ブラウザで `http://<ARGOCD_IP>` にアクセスし、ユーザー名 `admin` とパスワードでログイン。

### 2. デプロイ状況の確認

```bash
# すべての Application を確認
kubectl get applications -n argocd

# 特定の Namespace のリソースを確認
kubectl get all -n monitoring
kubectl get all -n backstage
kubectl get all -n istio-system
```

### 3. Grafana で監視

```bash
# Port-forward でアクセス
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
# ブラウザで http://localhost:3000
# デフォルトユーザー名: admin / パスワード: prom-operator（またはsealedされた値）
```

## 現在のステータス

**実装状況**: Phase 3完了（基本インフラ＋認証基盤デプロイ済み）

- ✅ Phase 1: クラスター初期化
- ✅ Phase 2: GitOps基盤（Argo CD）
- ✅ Phase 3: サービスメッシュ・認証（Istio, Keycloak, Cert-Manager, Prometheus, Backstage）
- 🚧 Phase 4: 開発者エクスペリエンス（Backstageテンプレート作成中）
- ⏳ Phase 5: アプリケーション検証（未着手）

詳細は[実装ロードマップ](./docs/roadmap.md)を参照してください。

## アーキテクチャ

### マルチリポジトリGitOps戦略

プラットフォームは責務分離のため、複数のGitリポジトリで構成されます：

1. **goldship-platform**（このリポジトリ）
   インフラ、セキュリティ、Argo CD制御構造、Backstageテンプレート（PEが管理）
   - `base-infra/`: Kubernetesインフラマニフェスト
   - `backstage-app/`: Backstageアプリケーションソース
   - `backstage-app/templates/`: アプリケーションスキャフォールディングテンプレート

2. **app-\<name\>**
   アプリケーションコードとデプロイ設定（ADが管理、アプリごとに1リポジトリ）
   - `base/`: 共通Kubernetesマニフェスト
   - `overlays/dev/`: 開発環境固有の設定
   - `overlays/prod/`: 本番環境固有の設定

### 環境分離とNamespace設計

| 層 | Namespaces | ラベル | 管理者 | Argo CD Project |
|-------|-----------|--------|--------|----------------|
| **基盤層** | `kube-system`, `istio-system`, `argocd`, `monitoring` | `goldship.platform/tier: platform`<br>`goldship.platform/environment: infrastructure` | PE | `platform-project` |
| **認証層** | `platform-auth-prod`, `platform-auth-dev`, `backstage` | `goldship.platform/tier: platform`<br>`goldship.platform/environment: production\|development` | PE | `platform-project` |
| **アプリ層** | `app-prod`, `app-dev` | `goldship.platform/tier: application`<br>`goldship.platform/environment: production\|development` | AD | `app-project-prod`, `app-project-dev` |

> Namespaceラベル設計の詳細は[Namespaceラベル設計ドキュメント](./docs/namespace-label-design.md)を参照。

### アーキテクチャ決定記録（ADR）

重要な設計決定は以下のADRで文書化されています：

- [ADR-001: TLS終端パターン](./docs/adr/001-tls-termination-pattern.md) - Istio GatewayでのTLS証明書管理
- [ADR-002: 認証・認可アーキテクチャ](./docs/adr/002-authentication-authorization-architecture.md) - Keycloak + Istio RequestAuthenticationによるJWT検証

## セットアップ

このプラットフォームは GitOps で管理されており、すべてのインフラストラクチャリソースは `base-infra/` ディレクトリに YAML マニフェストとして保存されています。

### インフラのブートストラップ

ゼロからクラスターを構築する場合や、マニフェストを再生成する場合は、[ブートストラッピング手順](./docs/bootstrapping.md)を参照してください。

**注意**: 通常の運用では、既存の YAML ファイルが使用されるため、ブートストラッピング手順を実行する必要はありません。

### 環境固有の設定

このリポジトリを自分の環境で使用する場合、以下の設定を変更する必要があります：

- GitHub組織名・ユーザー名（Argo CD Application CR、GHCR認証情報）
- Cilium LoadBalancer IP範囲（ネットワーク環境に応じて）
- ドメイン名（TLS証明書、HTTPRoute）
- AWS Route53認証情報（Cert-Manager DNS-01チャレンジ用）

詳細は[環境固有の設定変更ガイド](./docs/configuration.md)を参照してください。

### 共通操作

**コンテナイメージのビルドとプッシュ（Backstage等）:**

```bash
# .envファイルに環境変数を設定:
# GITHUB_USER=<your-username>
# GITHUB_GHCR_PAT=<your-token>

make all      # イメージのビルドとプッシュ
make build    # ビルドのみ
make push     # GHCRへプッシュ
make clean    # ローカルイメージ削除
```

## リポジトリ構造

```
goldship-platform/
├── base-infra/                # Kubernetesインフラマニフェスト
│   ├── argocd/                # Argo CD（CRD分離済み）
│   │   ├── 00-crds.yaml       # Argo CD CRDs
│   │   ├── argocd-install.yaml # Argo CD本体
│   │   ├── projects/          # AppProject定義（platform, app-dev, app-prod）
│   │   ├── root-apps/         # Root Application（App of Apps）
│   │   └── applications/      # Platform Application CRs
│   ├── cilium/                # Cilium CNI + LoadBalancer設定
│   ├── istio/                 # Istio Control PlaneとGateways
│   ├── keycloak/              # Keycloak（Kustomize: base + overlays/dev + overlays/prod）
│   ├── prometheus/            # Prometheus Stack（CRD分離済み）
│   ├── cert-manager/          # Cert-Manager（CRD分離済み）
│   ├── sealed-secret/         # Sealed Secretsコントローラー
│   ├── gateway-api/           # Gateway API CRDs
│   ├── local-path-provisioner/ # 動的PVプロビジョナー
│   ├── backstage/             # Backstage（Kustomize: base + overlays/prod）
│   ├── app-dev/               # 開発環境Namespace定義
│   ├── app-prod/              # 本番環境Namespace定義
│   └── kube-system/           # kube-system Namespace定義
├── backstage-app/             # Backstageアプリケーションソース
│   ├── packages/
│   │   ├── app/               # Frontend React app
│   │   └── backend/           # Backend Express app + Dockerfile
│   ├── templates/             # アプリケーションスキャフォールディングテンプレート
│   │   └── fastapi-template/  # FastAPI Kustomizeテンプレート例
│   ├── plugins/               # カスタムBackstageプラグイン
│   ├── app-config.yaml        # ローカル開発用設定
│   ├── app-config.kubernetes.yaml # Kubernetes本番用設定
│   ├── Makefile               # ビルド自動化
│   └── yarn.sh                # Yarnラッパースクリプト
├── scripts/                   # 運用スクリプト
│   └── split_crds.py          # Helm出力からCRDを分離
├── docs/                      # ドキュメント
│   ├── architecture/          # アーキテクチャとデザインドキュメント
│   │   ├── design.md
│   │   └── repository-structure.md
│   ├── adr/                   # Architecture Decision Records
│   │   ├── 001-tls-termination-pattern.md
│   │   └── 002-authentication-authorization-architecture.md
│   ├── bootstrapping.md       # ゼロから構築する手順
│   ├── configuration.md       # 環境固有の設定変更ガイド
│   ├── kustomize-guidelines.md # Kustomize使用ガイドライン
│   ├── namespace-label-design.md # Namespaceラベル設計
│   └── roadmap.md             # 実装ロードマップ
├── temp/                      # 一時ファイル（git-ignored）
│   └── ghcr-secret-raw.yaml   # 暗号化前のシークレット（コミット厳禁）
├── .gitignore
├── Makefile                   # コンテナイメージビルド
├── README.md                  # このファイル
└── CLAUDE.md                  # Claude Code AI用のコンテキスト情報

> **CRD分離パターン**: 大きなHelm Chart（Prometheus, Argo CD, Cert-Manager）は、
> CustomResourceDefinitionを`00-crds.yaml`に分離して管理しています。
> これにより、Git Diffの可読性向上とArgo CDでのデプロイ順序制御を実現しています。
> 詳細は各コンポーネントの`README-CRD-SPLIT.md`を参照。
```

## 開発ワークフロー

### プラットフォームエンジニア（PE）

1. このリポジトリ（`goldship-platform`）でインフラ設定を変更
2. Gitに変更をコミット＆プッシュ
3. Argo CDが自動的にクラスターに同期
4. Argo CD UIでデプロイ状況を確認

```bash
# 例: Prometheusの設定変更
vi base-infra/prometheus/prometheus-stack.yaml
git add base-infra/prometheus/
git commit -m "Update Prometheus scrape interval"
git push

# Argo CDで自動同期（または手動Sync）
kubectl get applications -n argocd -w
```

### アプリケーション開発者（AD）

1. Backstage UIからテンプレートを使用して新規アプリを作成
2. Backstageが自動的に：
   - 新しい`app-<name>`リポジトリを作成
   - Argo CD Application CRを`goldship-platform`にコミット
3. 生成された`app-<name>`リポジトリでコードを開発
4. `overlays/dev/kustomization.yaml`でイメージタグを更新
5. Argo CDが変更を検出してアプリケーションを自動再デプロイ

```bash
# 例: アプリケーションのイメージタグ更新
cd app-myapp/overlays/dev
kustomize edit set image ghcr.io/your-org/myapp:v1.2.3
git add kustomization.yaml
git commit -m "Deploy v1.2.3 to dev"
git push  # → Argo CDが自動デプロイ
```

## ドキュメント

### セットアップ・運用ガイド

- **[ブートストラッピング手順](./docs/bootstrapping.md)**: ゼロからクラスターを構築する際の参考手順
- **[環境固有の設定変更ガイド](./docs/configuration.md)**: 別環境で使用する際に変更が必要な設定項目

### アーキテクチャ・設計

- **[プラットフォームアーキテクチャ](./docs/architecture/design.md)**: 設計原則、技術スタック、セキュリティモデル
- **[リポジトリ構成](./docs/architecture/repository-structure.md)**: マルチリポジトリGitOps戦略とワークフロー
- **[実装ロードマップ](./docs/roadmap.md)**: プロジェクトフェーズと現在の進捗
- **[ADR-001: TLS終端パターン](./docs/adr/001-tls-termination-pattern.md)**: TLS証明書管理の設計決定
- **[ADR-002: 認証・認可アーキテクチャ](./docs/adr/002-authentication-authorization-architecture.md)**: Keycloak + Istio認証の設計決定

### 開発ガイドライン

- **[Kustomize使用ガイドライン](./docs/kustomize-guidelines.md)**: base-infra/でKustomizeを使用するタイミングと構造化パターン
- **[Namespaceラベル設計](./docs/namespace-label-design.md)**: 統一されたNamespaceラベリング戦略とセキュリティポリシー活用

### コンポーネント固有ドキュメント

- **[Prometheus CRD分離](./base-infra/prometheus/README-CRD-SPLIT.md)**: Prometheus Stack CRD管理
- **[Argo CD CRD分離](./base-infra/argocd/README-CRD-SPLIT.md)**: Argo CD CRD管理
- **[Cert-Manager CRD分離](./base-infra/cert-manager/README-CRD-SPLIT.md)**: Cert-Manager CRD管理

## セキュリティ

- **暗号化シークレット**: Sealed SecretsがGitコミット前に認証情報を暗号化
- **RBAC**: Kubernetes RBACが最小権限アクセスを強制
- **ネットワークポリシー**: CiliumネットワークポリシーがPod間トラフィックを制御
- **サービスメッシュ**: IstioがmTLSとトラフィック暗号化を提供
- **JWT認証**: Keycloakがすべての外部リクエストを検証
- **GitOps監査**: すべてのインフラ変更の完全なGit履歴
- **イメージPullシークレット**: GHCR認証情報をSealed Secretsで暗号化管理

> **重要**: `temp/`ディレクトリ内のファイル（暗号化前のシークレット）は絶対にコミットしないでください。

## ライセンス

このプロジェクトはMITライセンスの下で公開されています。詳細は[LICENSE](./LICENSE)ファイルを参照してください。

---

**構成技術**: Kubernetes • Cilium • Istio • Argo CD • Keycloak • Backstage • Prometheus • Sealed Secrets
