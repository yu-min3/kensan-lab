# NetworkPolicy 運用ガイド

> **現行設計の SoT**: CCNP 集約後のネットワークポリシー設計は [ADR-009](../adr/009-shared-allow-istio-network-policy.md) と `kubernetes/network/README.md` が Single Source of Truth。本ページはその運用ガイド（書き方・仕組み）。設計の背景は [ADR-004](../adr/004-network-policy-design.md)。

旧 3-ns 構成（`kensan-prod` / `kensan-dev` / `kensan-data`、`platform-auth-dev`）を前提とした per-ns default-deny の手書き運用は廃止した。dev/prod 分離廃止 + Phase 3a で `kensan-prod` + `kensan-data` を `kensan` に統合し、ベースラインは CiliumClusterwideNetworkPolicy (CCNP) に集約済み。

## ベースライン: CCNP 4 種 (mesh-ns 共通)

`istio-injection: enabled` ラベルが付いた **全 namespace** に対し、4 本の CCNP が横断的に効く。新しい mesh ns を作ると追加作業なしで自動カバーされる（後述）。

| CCNP (`kubernetes/network/network-policy/`) | 効果 |
|---|---|
| `clusterwide-default-deny-for-mesh-ns` | ingress + egress を default-deny に切替（`reserved:none` で endpoint を select しつつ何も allow しない）|
| `clusterwide-allow-dns-for-mesh-ns` | kube-system の DNS への egress を解禁 |
| `clusterwide-allow-istio-for-mesh-ns` | istio-system への egress（sidecar xDS）を解禁 |
| `clusterwide-allow-prometheus-scrape-for-mesh-ns` | monitoring からの scrape ingress を解禁 |

Cilium のセマンティクス上、これらは **additive**: default-deny が床を作り、allow 系 CCNP が各方向を個別に解禁する。4 本とも `argocd.argoproj.io/sync-options: Prune=false` でリソース個別に prune 防御済み。

対象外（`istio-injection` 無しなので per-ns NetworkPolicy のまま運用）: `cert-manager`、`cloudflare-tunnel`、`vault`、`external-secrets`、`vault-config-operator` 等。これらは `kubernetes/network/network-policy/<ns>.yaml` に個別 NP を置く。

## per-ns 追加ポリシーの書き方

CCNP が DNS / Istio / scrape の床を張るので、各 ns は **そこから足りない通信だけ** を per-ns NetworkPolicy で allow する。配置は PE 専管の `kubernetes/network/network-policy/<ns>.yaml`（app 自己完結の netpol は例外的に app 側 `resources/` に置く。例: `app-kensan` の `syncthing-guard`）。

実例 — `kubernetes/network/network-policy/kensan.yaml`:

- `allow-intra-namespace` — ns 内の全 pod 間通信（microservice ↔ PostgreSQL ↔ MinIO ↔ Polaris ↔ Dagster を統合 ns 内で許可）
- `allow-otel-egress` — monitoring:4318（OTel Collector）への egress
- `allow-external-ai-egress` — `app: kensan-ai` の pod だけ external:443（AI API）
- `allow-vault-egress` — `app: user-service` の pod だけ vault:8200（Transit）
- `allow-dagster-external-egress` — Dagster 系 pod だけ external:443（Slack / Gmail / weather API 等）

ポイント:

- **外部 egress は `podSelector` で対象 pod を限定する**（ns 全体に external:443 を開けない）
- **cross-ns 通信は両側に必要**: 送信元 ns に egress、送信先 ns に ingress
- DNS / Istio / Prometheus scrape は CCNP 済みなので per-ns で書かない

```yaml
# 例: 特定 pod のみ外部 API への egress を許可
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-external-api
  namespace: <ns>
spec:
  podSelector:
    matchLabels:
      app: my-service      # 対象 pod を限定
  policyTypes:
    - Egress
  egress:
    - ports:
        - protocol: TCP
          port: 443
```

## 新 namespace が自動カバーされる仕組み

CCNP の `endpointSelector` が `k8s:io.cilium.k8s.namespace.labels.istio-injection: enabled` を見ているため、**namespace に `istio-injection: enabled` ラベルを付けるだけ** で default-deny + DNS/Istio/scrape allow の床が即座に効く。per-ns で default-deny / allow-dns / allow-istio / allow-prometheus-scrape を書く必要はない。

```bash
# 新 mesh ns 作成時（label を付けると CCNP が自動適用）
kubectl label namespace my-new-ns istio-injection=enabled
```

追加で必要なのは「その ns 固有の egress（外部 API、cross-ns DB 等）」だけ。

## トラブルシュート

### 通信がブロックされている場合

```bash
# per-ns NetworkPolicy を一覧
kubectl get networkpolicy -n <namespace>

# clusterwide CCNP を一覧
kubectl get ciliumclusterwidenetworkpolicy

# Cilium のポリシー適用状況
kubectl -n kube-system exec -it <cilium-pod> -- cilium policy get

# drop をリアルタイム観測
kubectl -n kube-system exec -it <cilium-pod> -- cilium monitor --type drop

# Hubble で flow 確認
hubble observe --namespace <namespace> --verdict DROPPED
```

### よくある問題

| 症状 | 原因 | 対処 |
|------|------|------|
| 新 ns の pod が全通信できない | `istio-injection: enabled` ラベル未付与で CCNP の allow 系が効いていない | ns に label を付与 |
| Pod が DNS 解決できない | mesh 対象外 ns（CCNP 非適用）で per-ns の DNS egress が無い | per-ns NP に DNS egress を追加 |
| 外部 API に接続できない | per-ns の egress 443 が未許可 | `podSelector` 付き egress を追加 |
| cross-ns 通信ができない | 片側のみのポリシー | 送信元に egress / 送信先に ingress の両方を追加 |

### 緊急時: ベースラインの一時無効化

CCNP が原因と確定した場合に限り、該当 CCNP を一時的に外す（**全 mesh ns に波及するので慎重に**）:

```bash
kubectl delete ciliumclusterwidenetworkpolicy default-deny-for-mesh-ns
```

これは一時対処。原因特定後は Git からの sync で復元すること（CCNP は `Prune=false` なので git から消しても ArgoCD は自動削除しない点に注意）。

## 関連

- [ADR-004](../adr/004-network-policy-design.md) — NetworkPolicy 設計
- [ADR-009](../adr/009-shared-allow-istio-network-policy.md) — allow-istio 共有 + CCNP 集約
- `kubernetes/network/README.md` — network ディレクトリ全体図
