---
description: Three-layer separation model, multi-repo strategy, PE/AD responsibilities, and Backstage scaffolding
globs: "infrastructure/environments/**, **/projects/**"
---

# Environment Separation

## Three-Layer Model

| Layer | Namespaces | Owner | Argo CD Project |
|-------|-----------|-------|-----------------|
| Infrastructure | `istio-system`, `monitoring`, `argocd`, `cert-manager` | PE only | `platform-project` |
| Environment | `app-prod`, `app-dev` | PE manages ns; AD deploys apps | `app-project-prod`, `app-project-dev` |
| Application | `app-prod-<name>` | Per-app isolation | Scoped to app namespace |

## Multi-Repository Strategy

| Repository | Owner | Content |
|-----------|-------|---------|
| **kensan-lab** (this repo) | Platform Engineers | Cluster infra, Helm values, Argo CD Projects/Apps |
| **backstage/app/templates/** | Platform Engineers | Scaffolding templates (Kustomize-based) |
| **app-\<name\>** (per app) | App Developers | App code, Dockerfile, `base/` + `overlays/{dev,prod}/` |

## New App Flow (via Backstage)

1. AD creates app through Backstage template
2. Backstage creates `app-<name>` repository with Kustomize structure
3. Backstage auto-commits Application CRs to `infrastructure/gitops/argocd/applications/apps/`
4. Argo CD detects new CRs → deploys to both dev and prod

## Argo CD Projects

| Project | Scope | Allowed Namespaces |
|---------|-------|--------------------|
| `platform-project` | Infrastructure components | `kube-system`, `istio-system`, `monitoring`, `argocd`, etc. |
| `app-project-dev` | Dev applications | `app-dev` |
| `app-project-prod` | Prod applications | `app-prod` |

## Common Namespaces

```
kube-system, istio-system, argocd, monitoring, cert-manager,
backstage, keycloak-prod, keycloak-dev, app-prod, app-dev
```
