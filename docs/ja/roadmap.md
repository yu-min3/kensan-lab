# プロジェクトロードマップ (Project Roadmap)

このドキュメントは、プロジェクトの現在のステータス、短期的な目標、そして長期的なビジョンを示します。

## 現在のステータス (Current Status)

✅ **Backstageによるアプリケーション自動デプロイ基盤の完成**

プラットフォームのコア機能（GitOps, サービスメッシュ, 監視）は安定稼働しています。
現在、**BackstageのFastAPIテンプレートが完成しており、開発者はUIから数クリックでFastAPIアプリケーションを自動生成し、Kubernetesクラスタ（Dev/Prod）へ自動的にデプロイ可能**です。

## 次の目標 (Next Up)

プラットフォームの機能を拡充し、より本番グレードのユースケースに対応するための開発を進めています。

- **🚧 プラットフォームの汎用化と機能強化**
  - [ ] **x86系ノードへの対応**: 現在のARMベース（Raspberry Pi等）に加え、一般的なx86サーバーでの動作検証とサポートを追加する。
  - [ ] **Streamlitテンプレートの作成**: データサイエンス系ユースケースのため、新たにStreamlitアプリケーション用のBackstageテンプレートを開発する。

- **🔐 セキュリティと認証の強化**
  - [ ] **Istio GatewayでのKeycloak認証**: アプリケーションへの全外部トラフィックに対し、Istio GatewayレイヤーでKeycloakによるJWT認証を必須にする。
  - [ ] **サービスメッシュ認証の徹底**: Workload-to-Workload通信に厳格なmTLS認証を適用し、認可ポリシー(AuthorizationPolicy)を整備する。

- **🔭 可観測性の向上**
  - [ ] **OpenTelemetryの導入**: 分散トレーシングを実現し、アプリケーションのパフォーマンスボトルネックやエラー追跡を容易にする。

- **✨ 開発者エクスペリエンスの向上**
  - [ ] Backstage UIの改善（カタログ登録、ドキュメント拡充）。
  - [ ] エンドツーエンドのテストを拡充し、プラットフォームの信頼性をさらに高める。

## 将来的な構想 (Future Goals)

以下の機能は、コア機能が完全に検証された後の拡張候補です。

- **セキュリティ & コンプライアンス**
  - OPA/Gatekeeperによるポリシー強制
  - Trivyなどによるコンテナ脆弱性スキャン
  - SBOM（ソフトウェア部品表）の自動生成

- **開発者エクスペリエンスの向上**
  - リソースクォータによるマルチテナンシーの実現
  - カナリアリリースなど、より高度なデプロイ戦略のサポート
  - Backstageでのシークレット初期設定の自動化

- **運用 & 可観測性の強化**
  - Veleroなどによる災害復旧（DR）手順の確立

---

## 完了したマイルストーン (Completed Milestones)

<details>
<summary>過去に完了したフェーズの詳細はこちらをクリック</summary>

### Phase 1: クラスター初期化 ✅ 完了

ベアメタルKubernetesクラスターの基盤をセットアップしました。

- **達成事項**:
  - ベアメタルハードウェア上でKubernetesクラスターが稼働
  - CRI-Oコンテナランタイムを設定
  - kube-proxy置き換えを有効にしたCilium CNIとLoadBalancerを導入
  - Sealed Secretsコントローラーをインストールし、GHCR認証情報を暗号化

### Phase 2: GitOps基盤構築 ✅ 完了

Argo CDをGitOpsエンジンとして確立し、インフラ管理を整備しました。

- **達成事項**:
  - Argo CDが稼働し、Platform/App用のGitOps Projectsを確立
  - "App of Apps" パターンによるインフラの完全GitOps化
  - Namespaceラベルの統一設計を導入
  - CRD分離パターンを導入し、Git Diffの可読性とデプロイ順序を改善

### Phase 3: サービスメッシュと認証 ✅ 完了

セキュリティとアクセス制御のコアコンポーネントをデプロイしました。

- **達成事項**:
  - **Istio**: Control Planeと環境別Gatewayをデプロイし、mTLSを有効化
  - **Keycloak**: Prod/Dev環境用のKeycloakインスタンスをKustomizeベースで構築
  - **Cert-Manager**: Let's Encryptと連携し、ワイルドカード証明書の自動発行・更新を実現
  - **Prometheus**: 監視スタックをデプロイし、ServiceMonitorによる自動収集を確立
  - **Backstage**: カスタムビルドした開発者ポータルをデプロイ

</details>
