# K8s GitOps プラットフォーム

ベアメタルハードウェア上で動作する、プラットフォームエンジニア（PE）とアプリケーション開発者（AD）の完全な責務分離を実現したモダンなGitOpsベースKubernetesプラットフォーム。

### 主な機能

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
| **モニタリング** | Prometheus | メトリクス収集 |
| **開発者ポータル** | Backstage | セルフサービステンプレートとドキュメント |

## 現在のステータス
[実装ロードマップ](./docs/roadmap.md)を参照してください。

## アーキテクチャ

プラットフォームはマルチリポジトリGitOps戦略を使用します：

1. **goldship-platform**（このリポジトリ）: インフラ、セキュリティ、Argo CD制御構造、Backstageテンプレート（PEが管理）
   - `base-infra/`: Kubernetesインフラマニフェスト
   - `backstage-app/`: Backstageアプリケーション
   - `backstage-app/templates/`: アプリケーションスキャフォールディングテンプレート
2. **app-<name>**: アプリケーションコードとデプロイ設定（ADが管理、アプリごとに1つ）

### 環境分離

| 層 | Namespaces | 管理者 | Argo CD Project |
|-------|-----------|---------------|----------------|
| 基盤層 | `istio-system`, `monitoring`, `argocd` | PE | `platform-project` |
| 環境層 | `app-prod`, `app-dev` | AD | `app-project-prod`, `app-project-dev` |
| アプリ層 | `app-prod-<name>` | AD | N/A |

## セットアップ

このプラットフォームは GitOps で管理されており、すべてのインフラストラクチャリソースは `base-infra/` ディレクトリに YAML マニフェストとして保存されています。

### クイックスタート

1. **Argo CD にアクセス**
   ```bash
   # URL: http://192.168.0.240
   # 初期パスワード取得:
   kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 -d
   ```

2. **デプロイ状況の確認**
   ```bash
   # すべての Application を確認
   kubectl get applications -n argocd

   # 特定の Namespace のリソースを確認
   kubectl get all -n <namespace>
   ```

3. **Grafana で監視**
   ```bash
   # Port-forward でアクセス
   kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
   # ブラウザで http://localhost:3000
   # ユーザー名: admin / パスワード: admin
   ```

### インフラのブートストラップ

ゼロからクラスターを構築する場合や、マニフェストを再生成する場合は、[ブートストラッピング手順](./docs/bootstrapping.md)を参照してください。

**注意**: 通常の運用では、既存の YAML ファイルが使用されるため、ブートストラッピング手順を実行する必要はありません。

### 共通操作
**コンテナイメージのビルドとプッシュ:**
```bash
# .envファイルに環境変数を設定:
# GITHUB_USER=<your-username>
# GITHUB_GHCR_PAT=<your-token>

make all      # イメージのビルドとプッシュ
make build    # ビルドのみ
make push     # GHCRへプッシュ
make clean    # ローカルイメージ削除
```

## ドキュメント

### セットアップ・運用ガイド
- **[ブートストラッピング手順](./docs/bootstrapping.md)**: ゼロからクラスターを構築する際の参考手順
- **[環境固有の設定変更ガイド](./docs/configuration.md)**: 別環境で使用する際に変更が必要な設定項目

### アーキテクチャ・設計
- **[プラットフォームアーキテクチャ](./docs/architecture/design.md)**: 設計原則、技術スタック、セキュリティモデル
- **[リポジトリ構成](./docs/architecture/repository-structure.md)**: マルチリポジトリGitOps戦略とワークフロー
- **[実装ロードマップ](./docs/roadmap.md)**: プロジェクトフェーズと現在の進捗

### 開発ガイドライン
- **[Kustomize使用ガイドライン](./docs/kustomize-guidelines.md)**: base-infra/でKustomizeを使用するタイミングと構造化パターン
- **[Namespaceラベル設計](./docs/namespace-label-design.md)**: 統一されたNamespaceラベリング戦略とセキュリティポリシー活用

### その他
- **[CLAUDE.md](./CLAUDE.md)**: Claude Code用指示書（git-ignored）

## リポジトリ構造

```
goldship-platform/
├── base-infra/
│   ├── argocd/              # Argo CD Projects、Root Apps、Application CRs
│   ├── cilium/              # Cilium CNI + LoadBalancer設定
│   ├── istio/               # Istio Control PlaneとGateways
│   ├── keycloak/            # Keycloakインスタンス（Prod/Dev）
│   ├── prometheus/          # Prometheus + Grafana監視スタック
│   ├── sealed-secret/       # Sealed Secretsコントローラーと暗号化シークレット
│   ├── rbac/                # ServiceAccount設定
│   └── backstage/           # Backstage Kubernetesマニフェスト
├── backstage-app/
│   ├── packages/            # Backstage packages（app, backend）
│   ├── templates/           # アプリケーションスキャフォールディングテンプレート
│   └── plugins/             # カスタムBackstageプラグイン
├── docs/
│   ├── architecture/        # アーキテクチャとデザインドキュメント
│   └── guides/              # ユーザーおよび運用ガイド
└── temp/                    # 一時ファイル（git-ignored）
```

## 開発ワークフロー

### プラットフォームエンジニア（PE）

1. `goldship-platform`でインフラ設定を変更
2. Gitに変更をコミット
3. Argo CDが自動的にクラスターに同期
4. Argo CD UIでデプロイを確認

### アプリケーション開発者（AD）

1. Backstageテンプレートから新規アプリを作成
2. BackstageがApplication CRを`goldship-platform`に自動コミット
3. 生成された`app-<name>`リポジトリでコードを開発
4. `overlays/dev/image-patch.yaml`でイメージタグを更新
5. Argo CDが変更を検出してアプリケーションを再デプロイ

## セキュリティ

- **暗号化シークレット**: Sealed SecretsがGitコミット前に認証情報を暗号化
- **RBAC**: Kubernetes RBACが最小権限アクセスを強制
- **ネットワークポリシー**: CiliumネットワークポリシーがPod間トラフィックを制御
- **サービスメッシュ**: IstioがmTLSとトラフィック暗号化を提供
- **JWT認証**: Keycloakがすべての外部リクエストを検証
- **GitOps監査**: すべてのインフラ変更の完全なGit履歴


---

**構成技術**: Kubernetes • Cilium • Istio • Argo CD • Keycloak • Backstage • Prometheus • Sealed Secrets
