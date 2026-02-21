---
description: Helm multi-source Application pattern for Argo CD managed components
globs: "**/values.yaml, **/app.yaml"
---

# Helm Multi-Source Application Pattern

## Three-File Structure

Each infrastructure component consists of:

| File | Location | Purpose |
|------|----------|---------|
| **Application CR** | `infrastructure/gitops/argocd/applications/<category>/<component>/app.yaml` | Chart repo, version (`targetRevision`), source references |
| **values.yaml** | `infrastructure/<category>/<component>/values.yaml` | Helm values customization |
| **resources/** | `infrastructure/<category>/<component>/resources/` | Additional raw manifests (HTTPRoutes, SealedSecrets, etc.) |

## Common Operations

### Change Configuration
Edit `values.yaml` → commit → push. Argo CD renders and syncs.

### Upgrade Chart Version
Edit `spec.sources[0].targetRevision` in the Application CR → commit → push.

### Add Custom Resources
Place YAML files in `resources/` directory → commit → push. Deployed as a separate Argo CD source.

## Important Rules

- Argo CD renders Helm charts directly — never commit `helm template` output
- Chart versions are pinned in Application CRs as the single source of truth
- `values.yaml` is the primary file to edit for configuration changes
- `resources/` files are deployed alongside the chart as a separate source in the multi-source Application

## Directory Layout Example

```
infrastructure/observability/grafana/
├── values.yaml          # Helm chart values
└── resources/
    ├── httproute.yaml   # Gateway routing
    ├── datasources.yaml # Grafana datasources
    └── sealed-secret.yaml
```

Corresponding Application CR:
```
infrastructure/gitops/argocd/applications/observability/grafana/app.yaml
```
