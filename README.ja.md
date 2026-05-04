<div align="center">

<a href="README.md">English</a> | <a href="README.ja.md">日本語</a>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/kensan-logo-dark.svg" width="120">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/kensan-logo-light.svg" width="120">
  <img alt="kensan-lab logo" src="docs/assets/kensan-logo-dark.svg" width="120">
</picture>

# kensan-lab

**エンタープライズレベルの Kubernetes をベアメタルで — プラットフォームエンジニアリングのリファレンスアーキテクチャ。**

[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.32-326CE5?style=flat-square&logo=kubernetes&logoColor=white)](https://kubernetes.io/)
[![Argo CD](https://img.shields.io/badge/Argo_CD-v2.14-EF7B4D?style=flat-square&logo=argo&logoColor=white)](https://argoproj.github.io/cd/)
[![Istio](https://img.shields.io/badge/Istio-v1.27-466BB0?style=flat-square&logo=istio&logoColor=white)](https://istio.io/)
[![Cilium](https://img.shields.io/badge/Cilium-v1.18-F8C517?style=flat-square&logo=cilium&logoColor=black)](https://cilium.io/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue?style=flat-square)](./LICENSE)

</div>

---

エンタープライズのプラットフォームエンジニアリングで代表的な技術スタックを用いて構築したベアメタル Kubernetes ホームラボです。Argo CD による GitOps、Istio によるサービスメッシュ、Backstage による開発者セルフサービス、Prometheus / Grafana / Loki / Tempo によるオブザーバビリティ。すべて Raspberry Pi とミニ PC の上で稼働しています。

> これは**リファレンスアーキテクチャ**であり、そのまま使えるテンプレートではありません。Bootstrap 自動化（Ansible + Makefile）は今後公開予定です。学習リソースおよび技術記事の補足資料として公開しています。シークレット、ドメイン、IP レンジは各自の環境に合わせてください。詳細は [Configuration Guide](./docs/configuration.md) を参照。

## 目的

[Golden Kubestronaut](https://www.cncf.io/training/kubestronaut/) が全資格の知識を実際に動くシステムに落とし込むために構築したプラットフォームです。

このホームラボでは、**サービスメッシュ、ゼロトラストネットワークポリシー、Internal Developer Platform（IDP）による横断的なプラットフォーム関心事**に焦点を当てています — Istio による mTLS とトラフィック管理、Backstage による Golden Path テンプレートとサービスカタログを、Argo CD で管理されたベアメタルハードウェア上に構築しています。

**Golden Kubestronaut の実践的な学習環境**としても機能します。このプラットフォームは必要な16資格のうち12資格の技術領域をカバーしています（CKA, CKAD, CKS, KCNA, KCSA, PCA, ICA, CCA, CAPA, CGOA, CBA, OTCA）。これらの資格を学習中の方やプラットフォームエンジニアとして働いている方に向けています。

## アーキテクチャ

<div align="center">
<img src="docs/assets/request-flow.png" alt="プラットフォームアーキテクチャ — リクエストフロー＆コンポーネント連携" width="800">
<br>
</div>


- **Gateway** — Cloudflare Tunnel（インターネット）と Cilium L2 LB（LAN）が、Gateway API を使って Istio Gateway にトラフィックをルーティング
- **Applications** — Argo CD 経由で prod/dev namespace にデプロイされるワークロード、および kensan アプリ（専用 namespace）
- **Internal Developer Platform** — Backstage がサービスカタログ（catalog-info.yaml）、TechDocs（MkDocs）、Golden Path スキャフォールディングテンプレートを提供
- **Observability** — アプリケーションが OTel Collector にテレメトリを送信し、Prometheus（メトリクス）、Loki（ログ）、Tempo（トレース）に分配。Grafana で可視化。AlertManager が Slack にアラート送信
- **Security & Internal Network** — Sealed Secrets による Git 暗号化クレデンシャル、Cilium + Istio の NetworkPolicy、cert-manager による TLS 自動化、Pod Security Standards
- **Argo CD** — GitOps で全ゾーンを管理。`platform-project`（インフラ）と `app-project`（アプリケーション）に分離

<details>
<summary><b>インターネット公開</b></summary>

ローカルネットワークアクセスには Cilium LoadBalancer の L2 Announcement を使用。インターネット公開には Cloudflare Tunnel によるゼロトラストアクセスを採用（ホーム IP を露出しない）。構築手順は[こちらの記事](https://zenn.dev/yuu7751/articles/9df7ce4f1f4830)を参照。

</details>

**特徴:**

- **Argo CD + Helm マルチソース** — App of Apps + ApplicationSet によるスケーラブルな GitOps
- **Istio + Gateway API** — 単なる Ingress Controller ではなく、mTLS 付きのフルサービスメッシュ
- **Backstage** — サービスカタログ、TechDocs、スキャフォールディングテンプレート付きの開発者ポータル
- **マルチアーキテクチャ（ARM64 + AMD64）** — 実際のスケジューリング制約がある、均一でないクラスタ
- **WiFi でも動作** — 有線 LAN なしでも構築可能（有線 LAN 推奨）

## 技術スタック

|                                                             | 名前                                                                                                | 説明                                                                      |
| :---------------------------------------------------------: | --------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
|   <img src="docs/assets/logos/kubernetes.svg" width="32">   | [Kubernetes](https://kubernetes.io/)                                                                | コンテナオーケストレーション（kubeadm、ベアメタル）                       |
|     <img src="docs/assets/logos/cilium.svg" width="32">     | [Cilium](https://cilium.io/)                                                                        | eBPF ベースの CNI、kube-proxy 代替、L2 LB、Hubble                         |
|     <img src="docs/assets/logos/istio.svg" width="32">      | [Istio](https://istio.io/)                                                                          | サービスメッシュ — mTLS、Gateway API、トラフィック管理                    |
|      <img src="docs/assets/logos/argo.svg" width="32">      | [Argo CD](https://argoproj.github.io/cd/)                                                           | GitOps 継続的デリバリー（Helm マルチソース、App of Apps、ApplicationSet） |
|   <img src="docs/assets/logos/backstage.svg" width="32">    | [Backstage](https://backstage.io/)                                                                  | 開発者ポータル — サービスカタログ、TechDocs、テンプレート                 |
|    <img src="docs/assets/logos/keycloak.svg" width="32">    | [Keycloak](https://www.keycloak.org/)                                                               | ID・アクセス管理（IAM / SSO）                                             |
|   <img src="docs/assets/logos/prometheus.svg" width="32">   | [Prometheus](https://prometheus.io/)                                                                | メトリクス収集・アラート                                                  |
|    <img src="docs/assets/logos/grafana.svg" width="32">     | [Grafana](https://grafana.com/)                                                                     | オブザーバビリティダッシュボード                                          |
|      <img src="docs/assets/logos/loki.svg" width="32">      | [Loki](https://grafana.com/oss/loki/)                                                               | ログ集約                                                                  |
|     <img src="docs/assets/logos/tempo.svg" width="32">      | [Tempo](https://grafana.com/oss/tempo/)                                                             | 分散トレーシング                                                          |
| <img src="docs/assets/logos/opentelemetry.svg" width="32">  | [OpenTelemetry](https://opentelemetry.io/)                                                          | テレメトリ収集（OTel Collector）                                          |
|  <img src="docs/assets/logos/cert-manager.svg" width="32">  | [cert-manager](https://cert-manager.io/)                                                            | TLS 証明書の自動管理（Let's Encrypt）                                     |
| <img src="docs/assets/logos/sealed-secrets.png" width="32"> | [Sealed Secrets](https://sealed-secrets.netlify.app/)                                               | Git 上で暗号化されたシークレット                                          |
|                                                             | [Longhorn](https://longhorn.io/)                                                                    | S3 互換バックアップ機能を持つ分散ブロックストレージ（CNCF Incubating）    |
|   <img src="docs/assets/logos/cloudflare.svg" width="32">   | [Cloudflare Tunnel / R2](https://www.cloudflare.com/developer-platform/products/r2/)                | ゼロトラストなインターネット公開（Tunnel）+ オフサイトバックアップ先（R2）|

## ストレージ

耐久性の異なる 2 つの StorageClass を使い分ける:

- **`local-path`**（デフォルト）— ノードローカルな hostPath。再生成可能な ephemeral ワークロード（Argo CD / Backstage のメタデータ、キャッシュ等）に使う。PV は作成されたノードに固定される。
- **`longhorn`** — レプリケーションと S3 互換バックアップを持つ分散ブロックストレージ。`hardware-class=high-performance` のノードのみで稼働（現状は NVMe を持つ M4 Neo のみ）。Phase 1 は `replica=1`（HA なし、検証フェーズ）。SSD 追加後の Phase 3 で 3 ノード `replica=3` に拡張。

バックアップは S3 互換 API 経由で **Cloudflare R2** に置く。Egress 無料なので、月次リストア検証ドリルが安く回せる（本番想定容量でも月 $2 程度）。

```mermaid
flowchart LR
    app[Stateful Pod]
    app -- "PVC<br/>longhorn" --> lh
    app -- "PVC<br/>local-path" --> lp[("local-path<br/>ノード固定<br/>hostPath")]
    subgraph m4neo["m4neo (AMD64, NVMe)"]
      lh["Longhorn Manager"] --- data[("/opt/longhorn<br/>データパス")]
    end
    lh -. "スナップショット<br/>+ バックアップ" .-> r2[("Cloudflare R2<br/>egress: $0")]
    r2 -. "リストア" .-> lh
```

詳細なレイアウト・フェーズ計画・runbook 参照先は [`infrastructure/storage/README.md`](./infrastructure/storage/README.md) を参照。

## ハードウェア

| デバイス       | 台数 | アーキテクチャ | RAM   | 役割                                 |
| -------------- | ---- | -------------- | ----- | ------------------------------------ |
| Raspberry Pi 5 | 3    | ARM64          | 8 GB  | コントロールプレーン + ワーカー      |
| Bosgame M4 Neo | 1    | AMD64          | 32 GB | ワーカー（I/O ヘビーなワークロード） |

4ノード、マルチアーキテクチャ。kubeadm + CRI-O で管理。

<details>
<summary><b>スケジューリング戦略</b></summary>

| ワークロード | 戦略                                                        | 例                                |
| ------------ | ----------------------------------------------------------- | --------------------------------- |
| I/O ヘビー   | `requiredDuringScheduling: hardware-class=high-performance` | Prometheus, Loki, Tempo, Keycloak |
| 中程度       | `preferredDuringScheduling: high-performance` (weight: 80)  | OTel Collector                    |
| 軽量         | アフィニティなし                                            | Grafana, Hubble UI                |
| AMD64 専用   | `required: kubernetes.io/arch=amd64`                        | Backstage                         |

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
apps/                             # プラットフォーム上で稼働するアプリケーション
docs/                             # ADR、アーキテクチャ、ブートストラップガイド
```

## ドキュメント

| カテゴリ           | リンク                                                                                                                                                                                                |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **はじめに**       | [インストール](./docs/installation.md) / [設定](./docs/configuration.md) / [ブートストラップ](./docs/bootstrapping/index.md) _(準備中)_ / [シークレット管理](./docs/secret-management/index.md)       |
| **アーキテクチャ** | [Namespace ラベル設計](./docs/namespace-label-design.md) / [ADR](./docs/adr/) / [Kustomize ガイドライン](./docs/kustomize-guidelines.md)                                                             |
| **日本語詳細**     | [日本語ドキュメント](./docs/ja/)                                                                                                                                                                      |

## アプリケーション: kensan

`apps/kensan/` ディレクトリには、このプラットフォーム上で実際に稼働するフルスタックアプリケーションが含まれています。React フロントエンド、Go マイクロサービス、Python AI エージェント、Iceberg データレイクハウス（Dagster + Polaris）で構成された個人向けプロダクティビティツールです。マルチサービスデプロイ、データベース管理、Argo CD による CI/CD に加え、OpenTelemetry による全サービスのオブザーバビリティ統合も実装しています。

## 謝辞

[Home Operations](https://discord.gg/home-operations) コミュニティをはじめ、Kubernetes エコシステムのホームラボリポジトリを参考にしています。

## ライセンス

[Apache-2.0](./LICENSE)
