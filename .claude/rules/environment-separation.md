---
description: Three-layer separation model, namespace naming (ADR-006), multi-repo strategy, PE/AD responsibilities, and Backstage scaffolding
globs: "kubernetes/environments/**, **/projects/**"
---

# Environment Separation

dev/prod 分離は廃止済み。homelab で 2 環境運用は重く、必要なら別クラスタを立てる方針。

## Three-Layer Model

| Layer | Namespaces | Owner | Argo CD Project |
|-------|-----------|-------|-----------------|
| Infrastructure | `istio-system`, `argocd`, `cert-manager`, `vault`, `external-secrets`, `sealed-secrets`, `reloader`, `monitoring`, `cilium-secrets`, `kube-system`, `longhorn-system`, `cloudflare-tunnel` | PE only | `platform-project` |
| Platform Services | `platform-auth-prod`, `auth-system`, `backstage` | PE only | `platform-project` |
| Application | `app-{name}`（ADR-006、新規）/ `app-prod`（既存共有 ns） | AD（PE が ns bootstrap） | `app-project` |

## Application Namespace Naming（ADR-006）

採用パターン: **`app-{name}` flat + 3-axis labels**

```
app-{name}            # 例: app-streamlit, app-iceberg-ui
platform-{component}  # 例: platform-auth-prod, platform-keycloak（既存命名は維持）
```

### 3-axis labels

| Label | 例 | 用途 |
|---|---|---|
| `kensan-lab.platform/environment` | `production` / `development` / `infrastructure` | Gateway API allowedRoutes selector |
| `kensan-lab.platform/team` | `team-a` / `platform` | AuthorizationPolicy / NetworkPolicy / Argo CD project scope |
| `kensan-lab.platform/app` | `streamlit` 等 | 識別用（任意） |

旧 `app-{env}-<name>` 命名（`app-prod-foo` 等）は当面 coexistence。新規は `app-{name}` flat で作る。`app-prod` は env-shared landing zone として残置（空になり次第削除候補）。

## Multi-Repository Strategy

| Repository | Owner | Content |
|-----------|-------|---------|
| **kensan-lab** (this repo) | Platform Engineers | Cluster infra, Helm values, Argo CD Projects/Apps |
| **backstage/app/templates/** | Platform Engineers | Scaffolding template (flat YAML, no kustomize) |
| **kensan-lab-apps-\<name\>** (per app) | App Developers | App code, Dockerfile, `manifests/` |

## New App Flow (via Backstage)

1. AD が Backstage Software Template で app を作成
2. Backstage が `kensan-lab-apps-<name>` リポジトリ + flat `manifests/` を生成
3. Backstage が Application CR を `kubernetes/argocd/applications/apps/<name>/` に auto-commit
4. Argo CD が新 CR を検知 → `app-{name}`（または当面 `app-prod`）にデプロイ

## Argo CD Projects

| Project | Scope | Allowed Namespaces |
|---------|-------|--------------------|
| `platform-project` | Infrastructure + platform services | `kube-system`, `istio-system`, `monitoring`, `argocd`, `vault`, `auth-system`, `cert-manager`, `external-secrets`, `sealed-secrets`, `reloader`, `cloudflare-tunnel`, `longhorn-system`, `platform-auth-prod`, `backstage`, etc. |
| `app-project` | App applications | `app-prod`, `app-{name}`（ADR-006 で順次拡張）, `kensan`, `kensan-data` |

## Common Namespaces

```
kube-system, istio-system, argocd, monitoring, cert-manager,
backstage, platform-auth-prod, auth-system,
vault, external-secrets, sealed-secrets, reloader,
cloudflare-tunnel, longhorn-system, app-prod
```
