# ADR-004: NetworkPolicy 設計

## Status

Accepted

## Date

2026-02-25

## Context

goldship クラスタは Cilium を CNI として使用しており、NetworkPolicy の enforce 機能が利用可能である。しかし、これまで NetworkPolicy は 1 つも定義されておらず、全 namespace 間の通信が無制限に許可されていた。

gitops-evaluation-report.md で「P1 (Critical): NetworkPolicy が存在しない」と指摘されており、セキュリティ強化の一環として導入する。

## Decision

### 1. Default-Deny + Allow パターン

各対象 namespace に default-deny（ingress + egress 両方）を配置し、必要な通信のみを明示的に許可する。

### 2. 信頼インフラ層（NetworkPolicy 対象外）

以下の 3 namespace は NetworkPolicy を適用しない：

| Namespace | 理由 |
|-----------|------|
| **kube-system** | Cilium（CNI）自身が動作する。default-deny を適用すると NetworkPolicy の enforce 機構自体が壊れる可能性がある |
| **istio-system** | istiod は全 sidecar 注入 namespace に xDS config を push する。対象 namespace が増えるたびにポリシー更新が必要になり運用コストが高い |
| **monitoring** | Prometheus は全 namespace をスクレイプし、OTel Collector は全 namespace からデータを受信する。monitoring の egress を制限すると新しい namespace 追加のたびにポリシー更新が必要 |

これらは PSS で `privileged`（kube-system）または `baseline`（istio-system, monitoring）が適用されており、別のレイヤーで保護されている。

### 3. 標準 Kubernetes NetworkPolicy を使用

CiliumNetworkPolicy（L7 フィルタリング、DNS-based policy 等）ではなく、標準の `networking.k8s.io/v1` NetworkPolicy を使用する。

理由：
- ポータビリティ（Cilium 以外の CNI でも動作）
- Kubernetes ネイティブのリソースであり、学習コストが低い
- L3/L4 レベルの制御で現時点では十分

将来的に L7 レベルの制御（HTTP パスベース、DNS ベース）が必要になった場合に CiliumNetworkPolicy への移行を検討する。

### 4. 共通ポリシーパターン

全対象 namespace に以下の共通ポリシーを配置：

| ポリシー名 | 種別 | 内容 |
|-----------|------|------|
| `default-deny-all` | Ingress + Egress | 全通信を拒否（ベースライン） |
| `allow-dns` | Egress | kube-system:53/UDP,TCP を許可 |
| `allow-intra-namespace` | Ingress + Egress | 同一 namespace 内の Pod 間通信を許可 |
| `allow-prometheus-scrape` | Ingress | monitoring namespace からのスクレイプを許可 |

Istio sidecar 注入 namespace には追加で：

| ポリシー名 | 種別 | 内容 |
|-----------|------|------|
| `allow-istio` | Ingress + Egress | istio-system との双方向通信を許可（xDS + Gateway ルーティング） |

### 5. ファイル配置

| Namespace | 配置先 | 管理方式 |
|-----------|--------|---------|
| kensan-prod | `infrastructure/environments/kensan-prod/network-policy.yaml` | ApplicationSet (Git directory) |
| kensan-dev | `infrastructure/environments/kensan-dev/network-policy.yaml` | ApplicationSet (Git directory) |
| kensan-data | `infrastructure/environments/kensan-data/network-policy.yaml` | ApplicationSet (Git directory) |
| argocd | `infrastructure/gitops/argocd/resources/network-policy.yaml` | Argo CD resources source |
| backstage | `backstage/manifests/base/network-policy.yaml` | Kustomize |
| platform-auth-* | `infrastructure/security/keycloak/base/network-policy.yaml` | Kustomize (base で共有) |
| cert-manager | `infrastructure/security/cert-manager/resources/network-policy.yaml` | Argo CD resources source |

## Consequences

### Positive

- namespace 間の不正な通信がブロックされる
- 通信パターンがコードとして宣言的に管理される
- 新しい namespace/サービス追加時に通信要件の明示化が強制される

### Negative

- 新しい通信パターンを追加する際に NetworkPolicy の更新が必要（意図的なトレードオフ）
- デバッグ時に通信がブロックされている原因の特定に時間がかかる可能性がある

### Risks

- 既存の通信パターンの見落としにより、デプロイ直後にサービスがダウンする可能性がある
- 対策: マージ後に各 Application の sync 状況と Pod の通信を確認する
