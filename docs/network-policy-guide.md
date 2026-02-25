# NetworkPolicy 運用ガイド

## 設計概要

本クラスタでは **default-deny + allow** パターンで NetworkPolicy を運用する。
設計の詳細は [ADR-004](adr/004-network-policy-design.md) を参照。

## Namespace 分類

| 分類 | Namespace | NetworkPolicy | 理由 |
|------|-----------|---------------|------|
| 信頼インフラ | kube-system | なし | CNI (Cilium) が動作、制限すると壊れる |
| 信頼インフラ | istio-system | なし | 全 sidecar への xDS push が必要 |
| 信頼インフラ | monitoring | なし | 全 namespace のスクレイプ/データ受信が必要 |
| プラットフォーム | argocd | あり | API server + GitHub のみ |
| プラットフォーム | backstage | あり | PostgreSQL + GitHub + Argo CD |
| プラットフォーム | platform-auth-prod | あり | PostgreSQL のみ |
| プラットフォーム | platform-auth-dev | あり | PostgreSQL のみ |
| プラットフォーム | cert-manager | あり | API server + Let's Encrypt |
| アプリケーション | kensan-prod | あり | kensan-data + OTel + AI APIs |
| アプリケーション | kensan-dev | あり | kensan-data + OTel + AI APIs |
| アプリケーション | kensan-data | あり | kensan-prod/dev からの ingress のみ |

## 通信マトリクス

### アプリケーション層 (kensan-prod / kensan-dev)

| 方向 | 相手 | ポート | 用途 |
|------|------|--------|------|
| Egress | kube-system | 53/UDP,TCP | DNS |
| Egress | istio-system | * | Sidecar xDS |
| Egress | kensan-data | 5432 | PostgreSQL |
| Egress | kensan-data | 9000 | MinIO |
| Egress | kensan-data | 8181 | Polaris |
| Egress | monitoring | 4318 | OTel Collector |
| Egress | external | 443 | AI APIs (kensan-ai のみ) |
| Ingress | istio-system | * | Gateway ルーティング |
| Ingress | monitoring | * | Prometheus スクレイプ |
| Ingress | 同一 namespace | * | サービス間通信 |

### データ層 (kensan-data)

| 方向 | 相手 | ポート | 用途 |
|------|------|--------|------|
| Egress | kube-system | 53/UDP,TCP | DNS |
| Egress | monitoring | 4318 | OTel Collector |
| Ingress | kensan-prod | 5432, 9000, 8181 | DB/Storage アクセス |
| Ingress | kensan-dev | 5432, 9000, 8181 | DB/Storage アクセス |
| Ingress | monitoring | * | Prometheus スクレイプ |
| Ingress | 同一 namespace | * | 内部通信 |

### プラットフォーム層

| Namespace | Egress 先 | Ingress 元 |
|-----------|----------|-----------|
| argocd | kube-system (DNS, API:6443), external (443) | istio-system, monitoring, 同一 namespace |
| backstage | kube-system (DNS), istio-system, argocd, external (443) | istio-system, monitoring, 同一 namespace |
| platform-auth-* | kube-system (DNS), istio-system | istio-system, monitoring, 同一 namespace |
| cert-manager | kube-system (DNS, API:6443), external (443) | monitoring, webhook (10250), 同一 namespace |

## 運用手順

### 新しい namespace を追加する場合

1. namespace マニフェストを作成（PSS ラベル含む）
2. 以下の NetworkPolicy ファイルを作成:
   - `default-deny-all` — 全通信拒否
   - `allow-dns` — DNS 許可
   - `allow-intra-namespace` — namespace 内通信許可
   - `allow-prometheus-scrape` — Prometheus スクレイプ許可
   - Istio 注入 namespace なら `allow-istio` も追加
3. アプリ固有の通信要件に応じて追加ポリシーを作成
4. `docs/network-policy-guide.md` の通信マトリクスを更新

### 新しい外部 API 通信を追加する場合

1. 対象の namespace の network-policy.yaml に egress ルールを追加
2. 可能な限り podSelector で対象 Pod を限定する

```yaml
# 例: 特定の Pod のみ外部 API へのアクセスを許可
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-external-api
spec:
  podSelector:
    matchLabels:
      app: my-service      # 対象 Pod を限定
  policyTypes:
    - Egress
  egress:
    - ports:
        - protocol: TCP
          port: 443
```

### 新しい namespace 間通信を追加する場合

**両側に** ポリシーが必要：
1. 送信元 namespace に **egress** ルールを追加
2. 送信先 namespace に **ingress** ルールを追加

## トラブルシュート

### 通信がブロックされている場合の確認手順

```bash
# 1. 対象 namespace の NetworkPolicy を一覧
kubectl get networkpolicy -n <namespace>

# 2. 特定の NetworkPolicy の詳細を確認
kubectl describe networkpolicy <policy-name> -n <namespace>

# 3. Cilium のポリシー適用状況を確認
kubectl -n kube-system exec -it <cilium-pod> -- cilium policy get

# 4. Cilium のトラフィックモニタリング（リアルタイム）
kubectl -n kube-system exec -it <cilium-pod> -- cilium monitor --type drop

# 5. Hubble で通信フローを確認（Hubble が有効な場合）
hubble observe --namespace <namespace> --verdict DROPPED
```

### よくある問題

| 症状 | 原因 | 対処 |
|------|------|------|
| Pod が DNS 解決できない | `allow-dns` ポリシーが不足 | DNS egress ルールを追加 |
| Istio sidecar が起動しない | `allow-istio` ポリシーが不足 | istio-system への egress を許可 |
| Prometheus がスクレイプできない | `allow-prometheus-scrape` が不足 | monitoring からの ingress を許可 |
| サービス間通信ができない | intra-namespace or cross-namespace ルールが不足 | 通信マトリクスを確認し該当ルールを追加 |
| 外部 API に接続できない | egress 443 が許可されていない | podSelector 付きの egress ルールを追加 |

### 緊急時: NetworkPolicy の一時無効化

通信障害の原因が NetworkPolicy であることが確定した場合、default-deny を削除して全通信を許可できる：

```bash
# 特定 namespace の default-deny を削除（全通信が許可される）
kubectl delete networkpolicy default-deny-all -n <namespace>
```

**注意**: これは一時的な対処。原因を特定したら適切な allow ルールを追加し、default-deny を復元すること。
