<div align="center">

<a href="README.md">English</a> | <a href="README.ja.md">日本語</a>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/kensan-logo-dark.svg" width="120">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/kensan-logo-light.svg" width="120">
  <img alt="kensan-lab logo" src="docs/assets/kensan-logo-dark.svg" width="120">
</picture>

# kensan-lab

**エンタープライズレベルの Kubernetes をベアメタルで — 学びのために、見せるためじゃなく。**

*研鑽（けんさん）— 刃物を砥石で磨き続けるように、スキルを磨き続けること。*

[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.32-326CE5?style=flat-square&logo=kubernetes&logoColor=white)](https://kubernetes.io/)
[![Argo CD](https://img.shields.io/badge/Argo_CD-v2.14-EF7B4D?style=flat-square&logo=argo&logoColor=white)](https://argoproj.github.io/cd/)
[![Istio](https://img.shields.io/badge/Istio-v1.27-466BB0?style=flat-square&logo=istio&logoColor=white)](https://istio.io/)
[![Cilium](https://img.shields.io/badge/Cilium-v1.18-F8C517?style=flat-square&logo=cilium&logoColor=black)](https://cilium.io/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue?style=flat-square)](./LICENSE)

</div>

---

Raspberry Pi とミニ PC の上に、エンタープライズのプラットフォームチームが実際に使う技術スタックを構築したホームラボです。Argo CD による GitOps、Istio によるサービスメッシュ、Backstage による開発者セルフサービス、Prometheus / Grafana / Loki / Tempo によるフルオブザーバビリティ。

> これは**リファレンスアーキテクチャ**であり、そのまま使えるテンプレートではありません。学習リソースおよび技術記事の補足資料として公開しています。シークレット、ドメイン、IP レンジは各自の環境に合わせてください。詳細は [Configuration Guide](./docs/configuration.md) を参照。

## なぜこれを作ったのか

このリポジトリは、[Golden Kubestronaut](https://www.cncf.io/training/kubestronaut/) が全資格の知識を実際に動くシステムに落とし込むために構築した、**エンタープライズレベルのクラスタ運用を学ぶための自己研鑽プラットフォーム**です。

多くのホームラボリポジトリはセルフホスティングアプリの運用と軽量なネットワーク構成に焦点を当てています。サービスメッシュやゼロトラストポリシー、開発者セルフサービスまで踏み込んだものはほとんどありません。このリポジトリはそこに踏み込みます — **Istio による mTLS とトラフィック管理、Backstage による Golden Path テンプレート、Keycloak による IAM**を、Argo CD で管理されたベアメタルハードウェア上に構築しています。

**Golden Kubestronaut の実践的な学習環境**としても機能します。このプラットフォームは必要な16資格のうち12資格の技術領域をカバーしています（CKA, CKAD, CKS, KCNA, KCSA, PCA, ICA, CCA, CAPA, CGOA, CBA, OTCA）。これらの資格を学習中の方やプラットフォームエンジニアとして働いている方に向けています。

<div align="center">
<img src="docs/assets/request-flow.png" alt="プラットフォームアーキテクチャ — リクエストフロー＆コンポーネント連携" width="800">
<br>
<sub>トラフィックがプラットフォームをどう流れ、各コンポーネントがどう連携するか</sub>
</div>

**他のホームラボとの違い:**

- **Argo CD + Helm マルチソース** — エンタープライズで実際に使われる GitOps パターン（App of Apps）
- **Istio + Gateway API** — 単なる Ingress Controller ではなく、mTLS 付きのフルサービスメッシュ
- **Backstage** — サービスカタログ、TechDocs、スキャフォールディングテンプレート付きの開発者ポータル
- **Keycloak** — IAM / SSO をデプロイ済み。Gateway レベルの JWT 認証に対応可能
- **マルチアーキテクチャ（ARM64 + AMD64）** — 実際のスケジューリング制約がある、均一でないクラスタ
- **WiFi でも動作** — 有線 LAN なしでも構築可能（有線 LAN 推奨）

## 技術スタック

| ロゴ | 名前 | 説明 |
|------|------|------|
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/kubernetes.svg" width="24"> | [Kubernetes](https://kubernetes.io/) | コンテナオーケストレーション（kubeadm、ベアメタル） |
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/cilium.svg" width="24"> | [Cilium](https://cilium.io/) | eBPF ベースの CNI、kube-proxy 代替、L2 LB、Hubble |
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/istio.svg" width="24"> | [Istio](https://istio.io/) | サービスメッシュ — mTLS、Gateway API、トラフィック管理 |
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/argo.svg" width="24"> | [Argo CD](https://argoproj.github.io/cd/) | GitOps 継続的デリバリー（Helm マルチソース、App of Apps） |
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/backstage.svg" width="24"> | [Backstage](https://backstage.io/) | 開発者ポータル — サービスカタログ、TechDocs、テンプレート |
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/keycloak.svg" width="24"> | [Keycloak](https://www.keycloak.org/) | ID・アクセス管理（IAM / SSO） |
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/prometheus.svg" width="24"> | [Prometheus](https://prometheus.io/) | メトリクス収集・アラート |
| <img src="https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/grafana.svg" width="24"> | [Grafana](https://grafana.com/) | オブザーバビリティダッシュボード |
| | [Loki](https://grafana.com/oss/loki/) | ログ集約 |
| | [Tempo](https://grafana.com/oss/tempo/) | 分散トレーシング |
| | [OpenTelemetry](https://opentelemetry.io/) | テレメトリ収集（OTel Collector） |
| | [cert-manager](https://cert-manager.io/) | TLS 証明書の自動管理（Let's Encrypt） |
| | [Sealed Secrets](https://sealed-secrets.netlify.app/) | Git 上で暗号化されたシークレット |
| | [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/) | ゼロトラストなインターネット公開 |

## ハードウェア

| デバイス | 台数 | アーキテクチャ | RAM | 役割 |
|---------|------|-------------|-----|------|
| Raspberry Pi 5 | 3 | ARM64 | 8 GB | コントロールプレーン + ワーカー |
| Bosgame M4 Neo | 1 | AMD64 | 16 GB | ワーカー（I/O ヘビーなワークロード） |

4ノード、マルチアーキテクチャ。kubeadm + CRI-O で管理。

<details>
<summary><b>スケジューリング戦略</b></summary>

| ワークロード | 戦略 | 例 |
|------------|------|-----|
| I/O ヘビー | `requiredDuringScheduling: hardware-class=high-performance` | Prometheus, Loki, Tempo, Keycloak |
| 中程度 | `preferredDuringScheduling: high-performance` (weight: 80) | OTel Collector |
| 軽量 | アフィニティなし | Grafana, Hubble UI |
| AMD64 専用 | `required: kubernetes.io/arch=amd64` | Backstage |

</details>

## アーキテクチャ & セキュリティ

上のアーキテクチャ図は、プラットフォームがどのようにゾーン分けされているかを示しています:

- **Gateway** — Cloudflare Tunnel（インターネット）と Cilium L2 LB（LAN）が、Gateway API を使って Istio Gateway にトラフィックをルーティング
- **Applications** — Argo CD 経由で prod/dev namespace にデプロイされるワークロード、および kensan アプリ（専用 namespace）
- **Internal Developer Platform** — Backstage がサービスカタログ（catalog-info.yaml）、TechDocs（MkDocs）、Golden Path スキャフォールディングテンプレートを提供
- **Observability** — アプリケーションが OTel Collector にテレメトリを送信し、Prometheus（メトリクス）、Loki（ログ）、Tempo（トレース）に分配。Grafana で可視化。AlertManager が Slack にアラート送信
- **Security & Internal Network** — Sealed Secrets による Git 暗号化クレデンシャル、Cilium + Istio の NetworkPolicy、cert-manager による TLS 自動化、Pod Security Standards
- **Argo CD** — GitOps で全ゾーンを管理。`platform-project`（インフラ）と `app-project`（アプリケーション）に分離

<details>
<summary><b>インターネット公開</b></summary>

ローカルネットワークアクセスには Cilium LoadBalancer の L2 Announcement を使用。

インターネット公開には3つのオプションをサポート:
1. **ポートフォワーディング** — Dynamic DNS を使ったシンプルな構成
2. **Cloudflare Tunnel** — ホーム IP を露出しないゼロトラストアクセス（推奨）
3. **VPS リバースプロキシ** — WireGuard/Tailscale を使った本番構成

詳細は [Configuration Guide](./docs/configuration.md) を参照。

</details>

## リポジトリ構成

```
infrastructure/                    # コアプラットフォーム（GitOps 管理）
├── gitops/argocd/                # Argo CD: applications/, projects/, root-apps/
├── observability/                # Prometheus, Grafana, Loki, Tempo, OTel Collector
├── network/                      # Cilium, Istio, Gateway API
├── security/                     # cert-manager, Sealed Secrets, Keycloak
├── environments/                 # app-dev, app-prod, kensan-dev, kensan-prod, kensan-data, observability, system-infra
└── storage/                      # local-path-provisioner
backstage/                        # 開発者ポータル（app/ + manifests/）
apps/                             # サンプルアプリケーション
docs/                             # ADR、アーキテクチャ、ブートストラップガイド
```

## ドキュメント

| カテゴリ | リンク |
|---------|-------|
| **はじめに** | [インストール](./docs/installation.md) / [設定](./docs/configuration.md) / [ブートストラップ](./docs/bootstrapping/index.md) _(準備中)_ / [シークレット管理](./docs/secret-management/index.md) |
| **アーキテクチャ** | [プラットフォーム設計](./docs/architecture/design.md) / [リポジトリ構成](./docs/architecture/repository-structure.md) / [Namespace ラベル設計](./docs/namespace-label-design.md) / [ADR](./docs/adr/) |
| **開発** | [Kustomize ガイドライン](./docs/kustomize-guidelines.md) / [ロードマップ](./docs/roadmap.md) |
| **日本語詳細** | [日本語ドキュメント](./docs/ja/) |

## サンプルアプリケーション: kensan

`apps/kensan/` ディレクトリには、このプラットフォーム上で動作するフルスタックアプリケーションが含まれています。React フロントエンド、Go マイクロサービス、Python AI エージェント、Iceberg データレイクハウス（Dagster + Polaris）で構成された個人向けプロダクティビティツールです。マルチサービスデプロイ、データベース管理、Argo CD による CI/CD に加え、OpenTelemetry による全サービスのオブザーバビリティ統合も実装しています。

## 謝辞

[Home Operations](https://discord.gg/home-operations) コミュニティをはじめ、Kubernetes エコシステムのホームラボリポジトリを参考にしています。

## ライセンス

[Apache-2.0](./LICENSE)
