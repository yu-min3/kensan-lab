# Infrastructure

Argo CD で同期される全 platform component の Git source。

## ディレクトリ構成

| ディレクトリ | 用途 |
|---|---|
| `argocd/` | Argo CD 自身 (root-app, projects, applications, AppProject 等) |
| `network/` | CNI (Cilium), Service Mesh (Istio), Gateway API, CoreDNS, Cloudflare Tunnel, NetworkPolicy |
| `apps/` | platform 管理下の app component (例: `apps/app-kensan` = 新 kensan の ns/PVC/syncthing 等) |
| `observability/` | Prometheus, Grafana, Loki, Tempo, OTel Collector |
| `auth/` | Keycloak (OIDC IdP), oauth2-proxy (Gateway ext_authz), Vault OIDC auth |
| `secrets/` | Vault + Vault Config Operator + Vault Database/Transit engines, External Secrets, Sealed Secrets, cert-manager, Reloader |
| `storage/` | local-path-provisioner, Longhorn |
| `policy/` | Kyverno (policy engine + ClusterPolicy / PolicyException 群) |
| `kube-system/` | kube-system ns の label / PSA / 共通リソース管理 |
| `namespaces/` | namespace ライフサイクル管理の規約置き場（README のみ。shared landing zone は全廃済み）。ns-lifecycle app 群は `argocd/applications/namespaces/` |

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

例: `secrets/vault/`, `network/cilium/`, `observability/grafana/`, `observability/prometheus/`, `secrets/cert-manager/`, `secrets/external-secrets/`, `secrets/vault-config-operator/`, `observability/otel-collector/`, `observability/loki/`, `observability/tempo/`

`resources/` は **「Helm が render しない方の YAML」を分離する**役割。`values.yaml` と並ぶときだけ意味があるので、`values.yaml` が無いコンポーネントには作らない。

### Pattern B: 生 YAML のみ (フラット)

Helm chart を使わず、raw manifest だけを Argo CD app が読むコンポーネント。

```
<component>/
└── *.yaml
```

例: `network/network-policy/`, `network/cloudflare-tunnel/`, `network/gateway-api/`, `secrets/sealed-secrets/`, `storage/local-path-provisioner/`, `network/istio/` 直下の Gateway / PeerAuthentication / namespace 等

トップレベルにそのまま .yaml を置く。`resources/` で 1 段挟まない。

## 例外的な構成

- **`istio/`**: 複数 chart (`base/`, `cni/`, `istiod/`) を内包する多 component dir。各 subchart は Pattern A に従い `<subchart>/values.yaml` を持つ。istio 全体に紐付く chart 外の生 YAML (Gateway, PeerAuthentication, istio-system namespace 等) は `istio/` 直下にフラットに置く (Pattern B)。
- **`network-policy/`**: NetworkPolicy / CiliumClusterwideNetworkPolicy を ns 横断で集約した特殊コンポーネント。PE の専管リソースを 1 ヶ所に集めるため、各 ns owning app からは分離して network-policy app に集約している。
- **`observability/`**: `app.yaml` を持たず、`argocd/applications/observability/applicationset.yaml` の ApplicationSet が各 `observability/*/config.json` (chartVersion 等を保持) を git generator で拾って component ごとの Application を生成する。chart 版数の SoT は各 `config.json`。
- **`secrets/vault-database-engine/`, `secrets/vault-transit-engine/`**: 自作 chart (`chart/`) + 全インスタンス共通の生 YAML (`shared/`) + インスタンス別 values (`platform-values/`) の 3 構成。`argocd/applications/secrets/<engine>/` の `app-shared.yaml` (shared/ を sync) と `applicationset-instances.yaml` (platform-values/ の各ファイルから ApplicationSet がインスタンスを生成) のペアで構成され、単一の `app.yaml` は持たない。

## Argo CD Application 配置

各 component の Argo CD Application CR は `argocd/applications/<category>/<component>/app.yaml` に置く。`platform-root` (App-of-Apps) が `argocd/applications/` を recurse して全 child app を発見・管理する。

## 関連ドキュメント

- `.claude/rules/helm-multisource.md`: Pattern A の詳細 (Argo CD multi-source の 3 ファイル構成)
- `.claude/rules/gitops-workflow.md`: GitOps 運用ルール
- `docs/`: ADR、アーキテクチャ図
