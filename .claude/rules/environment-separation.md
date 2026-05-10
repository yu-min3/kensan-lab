---
description: Three-layer separation model, multi-repo strategy, PE/AD responsibilities, and Backstage scaffolding
globs: "infrastructure/environments/**, **/projects/**"
---

# Environment Separation

dev/prod 分離は廃止済み。homelab で 2 環境運用は重く、必要なら別クラスタを立てる方針。

## Three-Layer Model

| Layer | Namespaces | Owner | Argo CD Project |
|-------|-----------|-------|-----------------|
| Infrastructure | `istio-system`, `monitoring`, `argocd`, `cert-manager`, `vault`, etc. | PE only | `platform-project` |
| Environment | `app-prod` | PE manages ns; AD deploys apps | `app-project-prod` |
| Application | `app-prod-<name>` (将来 per-app ns) | Per-app isolation | Scoped to app namespace |

## Multi-Repository Strategy

| Repository | Owner | Content |
|-----------|-------|---------|
| **kensan-lab** (this repo) | Platform Engineers | Cluster infra, Helm values, Argo CD Projects/Apps |
| **backstage/app/templates/** | Platform Engineers | Scaffolding template (flat YAML, no kustomize) |
| **kensan-lab-apps-\<name\>** (per app) | App Developers | App code, Dockerfile, `manifests/` |

## New App Flow (via Backstage)

1. AD creates app through Backstage template
2. Backstage creates `kensan-lab-apps-<name>` repository with flat `manifests/`
3. Backstage auto-commits Application CR to `infrastructure/gitops/argocd/applications/apps/<name>/`
4. Argo CD detects new CR → deploys to `app-prod`

## Argo CD Projects

| Project | Scope | Allowed Namespaces |
|---------|-------|--------------------|
| `platform-project` | Infrastructure components | `kube-system`, `istio-system`, `monitoring`, `argocd`, etc. |
| `app-project-prod` | App applications | `app-prod` |

## Common Namespaces

```
kube-system, istio-system, argocd, monitoring, cert-manager,
backstage, platform-auth-prod, vault, app-prod
```
