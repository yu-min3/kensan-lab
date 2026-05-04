# Infrastructure

Argo CD で同期される全 platform component の Git source。

## ディレクトリ構成

| ディレクトリ | 用途 |
|---|---|
| `gitops/argocd/` | Argo CD 自身 (root-app, projects, applications, AppProject 等) |
| `network/` | CNI (Cilium), Service Mesh (Istio), Gateway API, Cloudflare Tunnel, NetworkPolicy |
| `observability/` | Prometheus, Grafana, Loki, Tempo, OTel Collector |
| `security/` | cert-manager, Sealed Secrets, Vault, Vault Config Operator, External Secrets, Keycloak |
| `storage/` | local-path-provisioner |
| `environments/` | 複数 component が同居する shared namespace の bootstrap (app-dev, app-prod, kensan-{data,dev,prod}, system-infra) |

## コンポーネントのレイアウト規則

各 component dir のファイル配置は **`values.yaml` の有無** で 2 つに分岐する。判定基準は「Argo CD app が Helm chart を render するかどうか」。

### Pattern A: Helm chart 系 (`values.yaml` + `resources/`)

Argo CD multi-source app で **upstream Helm chart + values.yaml + 追加生 YAML** を組み合わせるコンポーネント。

```
<component>/
├── values.yaml           # Helm values
└── resources/            # chart 外の生 YAML (namespace, ServiceMonitor, HTTPRoute, SealedSecret 等)
    └── *.yaml
```

例: `vault/`, `cilium/`, `grafana/`, `prometheus/`, `cert-manager/`, `external-secrets/`, `vault-config-operator/`, `otel-collector/`, `loki/`, `tempo/`

`resources/` は **「Helm が render しない方の YAML」を分離する**役割。`values.yaml` と並ぶときだけ意味があるので、`values.yaml` が無いコンポーネントには作らない。

### Pattern B: 生 YAML のみ (フラット)

Helm chart を使わず、raw manifest だけを Argo CD app が読むコンポーネント。

```
<component>/
└── *.yaml
```

例: `network-policy/`, `cloudflare-tunnel/`, `gateway-api/`, `sealed-secrets/`, `storage/local-path-provisioner/`, `istio/` 直下の Gateway / PeerAuthentication / namespace 等

トップレベルにそのまま .yaml を置く。`resources/` で 1 段挟まない。

## 例外的な構成

- **`istio/`**: 複数 chart (`base/`, `cni/`, `istiod/`) を内包する多 component dir。各 subchart は Pattern A に従い `<subchart>/values.yaml` を持つ。istio 全体に紐付く chart 外の生 YAML (Gateway, PeerAuthentication, istio-system namespace 等) は `istio/` 直下にフラットに置く (Pattern B)。
- **`network-policy/`**: NetworkPolicy / CiliumClusterwideNetworkPolicy を ns 横断で集約した特殊コンポーネント。PE の専管リソースを 1 ヶ所に集めるため、各 ns owning app からは分離して network-policy app に集約している。

## Argo CD Application 配置

各 component の Argo CD Application CR は `gitops/argocd/applications/<category>/<component>/app.yaml` に置く。`platform-root` (App-of-Apps) が `gitops/argocd/applications/` を recurse して全 child app を発見・管理する。

## 関連ドキュメント

- `.claude/rules/helm-multisource.md`: Pattern A の詳細 (Argo CD multi-source の 3 ファイル構成)
- `.claude/rules/gitops-workflow.md`: GitOps 運用ルール
- `docs/`: ADR、アーキテクチャ図
