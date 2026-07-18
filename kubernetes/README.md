# Infrastructure

The Git source for every platform component synced by Argo CD.

## Directory layout

| Directory | Purpose |
|---|---|
| `argocd/` | Argo CD itself (root-app, projects, applications, AppProject, etc.) |
| `network/` | CNI (Cilium), Service Mesh (Istio), Gateway API, CoreDNS, Cloudflare Tunnel, NetworkPolicy |
| `apps/` | App components under platform management (e.g. `apps/app-kensan` = the new kensan's namespace/PVC/syncthing, etc.) |
| `observability/` | Prometheus, Grafana, Loki, Tempo, OTel Collector |
| `auth/` | Keycloak (OIDC IdP), oauth2-proxy (Gateway ext_authz), Vault OIDC auth |
| `secrets/` | Vault + Vault Config Operator + Vault Database/Transit engines, External Secrets, Sealed Secrets, cert-manager, Reloader |
| `storage/` | Longhorn |
| `policy/` | Kyverno (policy engine + the ClusterPolicy / PolicyException set) |
| `backstage/` | Deploy definitions for Backstage (developer portal) — Pattern B. Source lives at the repo top-level `backstage/` (ADR-018 / ADR-020) |
| `kube-system/` | Label / PSA / shared-resource management for the `kube-system` namespace |

## Component layout rules

Each component directory's file layout branches into two shapes depending on **whether a `values.yaml` is present**. The deciding question: does the Argo CD app render a Helm chart?

### Pattern A: Helm-chart-based (`values.yaml` + `resources/`)

Components where an Argo CD multi-source app combines an **upstream Helm chart + values.yaml + additional raw YAML**.

```
<component>/
├── values.yaml           # Helm values
└── resources/            # Raw YAML outside the chart (namespace, ServiceMonitor, HTTPRoute, SealedSecret, etc.)
    └── *.yaml
```

Examples: `secrets/vault/`, `network/cilium/`, `observability/grafana/`, `observability/prometheus/`, `secrets/cert-manager/`, `secrets/external-secrets/`, `secrets/vault-config-operator/`, `observability/otel-collector/`, `observability/loki/`, `observability/tempo/`

`resources/` exists to **separate out the YAML that Helm doesn't render**. It only makes sense alongside a `values.yaml`, so don't create it for components that have none.

### Pattern B: raw YAML only (flat)

Components with no Helm chart, where the Argo CD app reads raw manifests directly.

```
<component>/
└── *.yaml
```

Examples: `network/network-policy/`, `network/cloudflare-tunnel/`, `network/gateway-api/`, `secrets/sealed-secrets/`, and the Gateway / PeerAuthentication / namespace, etc. directly under `network/istio/`

Place `.yaml` files directly at the top level. No `resources/` indirection.

## Exceptional structures

- **`istio/`**: a multi-component directory bundling several charts (`base/`, `cni/`, `istiod/`). Each subchart follows Pattern A with its own `<subchart>/values.yaml`. Raw YAML that belongs to Istio as a whole but sits outside any chart (Gateway, PeerAuthentication, the `istio-system` namespace, etc.) is placed flat directly under `istio/` (Pattern B).
- **`network-policy/`**: a special component that aggregates NetworkPolicy / CiliumClusterwideNetworkPolicy across namespaces. To keep PE-exclusive resources in one place, these are split out from each namespace-owning app and consolidated into the `network-policy` app.
- **`observability/`**: has no `app.yaml`. The ApplicationSet at `argocd/applications/observability/applicationset.yaml` uses a git generator to pick up each `observability/*/config.json` (which holds the chartVersion etc.) and generates one Application per component. Each `config.json` is the source of truth for that component's chart version.
- **`secrets/vault-database-engine/`, `secrets/vault-transit-engine/`**: a three-part structure of a homegrown chart (`chart/`) + raw YAML shared across all instances (`shared/`) + per-instance values (`platform-values/`). Configured as a pair — `argocd/applications/secrets/<engine>/app-shared.yaml` (syncs `shared/`) and `applicationset-instances.yaml` (an ApplicationSet that generates one instance per file under `platform-values/`) — with no single `app.yaml`.

## Namespace lifecycle management

Most components carry `resources/namespace.yaml` (Pattern A) or a flat `namespace.yaml` (Pattern B) inside their own Application and self-contain namespace creation + labeling via `CreateNamespace=true` (e.g. `secrets/cert-manager/`, `secrets/external-secrets/`, `secrets/reloader/`, `backstage/`, `network/cloudflare-tunnel/`).

A dedicated ns-lifecycle app under `kubernetes/argocd/applications/namespaces/` only exists when one of these three patterns applies (ADR-020):

| Pattern | Reason | Example |
|---|---|---|
| **No owning component** | No component in this repo corresponds to the namespace (e.g. one created by kubeadm outside GitOps, adopted here purely to attach labels) | `kube-system` |
| **Multiple components share the namespace** | No single owning Application can be designated (`policy/kyverno` + `policy/kyverno-policies` share `policy/namespace.yaml`; the 6 `observability/*` components share `observability/namespace.yaml`) | `kyverno`, `monitoring` |
| **Deliberate lifecycle isolation** | There is a single owner, but the app is deliberately split off for protection (e.g. `sealed-secrets` — no finalizer + `Prune=false` to protect the sealing key) | `sealed-secrets` |

Cases that are "single-owner but split off" without matching one of the above are consolidation candidates (`reloader` was consolidated in 2026-07, per ADR-020).

## Argo CD Application placement

Each component's Argo CD Application CR lives at `argocd/applications/<category>/<component>/app.yaml`. `platform-root` (the App-of-Apps) recurses `argocd/applications/` to discover and manage every child app.

## Related documents

- `.claude/rules/helm-multisource.md`: details of Pattern A (the 3-file structure for Argo CD multi-source)
- `.claude/rules/gitops-workflow.md`: GitOps operating rules
- `docs/`: ADRs, architecture diagrams
