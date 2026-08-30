# Application Templates

This directory contains Backstage templates for creating new applications with GitOps deployment patterns.

## Architecture

Part of the kensan-lab GitOps architecture:
- **kensan-lab**: Platform infrastructure and Backstage application (managed by Platform Engineers)
  - `backstage/templates/`: Application templates (this directory)
- **app-\<name\>**: Generated application repositories (managed by Application Developers)

## Available Templates

### FastAPI Template

Location: `fastapi-template/`

A production-ready FastAPI application template with:
- **Modern Python tooling**: uv (fast package manager) + ruff (linter/formatter)
- **Deployment through `charts/app-base`** — the platform's chart, driven by one
  values file, rather than hand-written Deployment / Service / HTTPRoute
- Per-app namespace with the ADR-006 label contract
- Prometheus metrics endpoint and a ServiceMonitor
- Health check endpoints
- Automatic Argo CD Application generation
- GitHub Actions CI/CD pipeline (lint, test, build)
- Backstage TechDocs support
- Sample tests with pytest

## Using Templates

### 1. Register Template in Backstage

Templates are automatically loaded from this directory:

- Production and local development load `template.yaml` (GitHub only).
- Explore overrides the catalog location with `template-explore.yaml` (Gitea only).
- Both entry files render the same `skeleton/`; application source and platform
  manifests are not duplicated.

```yaml
# In app-config.yaml (local development)
catalog:
  locations:
    - type: file
      target: ../../templates/fastapi-template/template.yaml
      rules:
        - allow: [Template]

# In app-config.kubernetes.yaml (Docker image)
catalog:
  locations:
    - type: file
      target: ./templates/fastapi-template/template.yaml
      rules:
        - allow: [Template]
```

### 2. Create New Application

1. Open Backstage UI
2. Navigate to "Create" → "Choose a template"
3. Select "FastAPI Application"
4. Fill in the form:
   - Application Name (e.g., `my-api`)
   - Description
   - Owner
   - Repository location
5. Click "Create"

### 3. Automatic GitOps Setup

Backstage will automatically:
- Create a new GitHub repository
- Generate `deploy/values.yaml` for `charts/app-base` and the namespace it needs
- Open a pull request against `kensan-lab` adding the Argo CD Application
- Register the app in the Backstage catalog

### 4. Deploy Your Application

Argo CD deploys the application into its own namespace, `app-<name>`, once the
platform pull request is merged (ADR-006: one namespace per application).

## Template Structure

```
fastapi-template/
├── template.yaml              # Production template (GitHub)
├── template-explore.yaml      # Explore template (cluster-local Gitea)
├── catalog-info.yaml          # Backstage catalog entry
└── skeleton/                  # Template files
    ├── app/                   # Application code
    ├── deploy/
    │   ├── values.yaml        # charts/app-base の values（唯一の設定面）
    │   └── resources/         # namespace / ServiceMonitor（app が所有）
    ├── docs/                  # TechDocs
    ├── .backstage/            # platform へ PR される内容
    │   └── kubernetes/       #   Application + oauth2 の ReferenceGrant
    ├── .github/workflows/     # CI/CD
    ├── Dockerfile
    ├── pyproject.toml
    └── catalog-info.yaml
```

**手書きの Deployment / Service / HTTPRoute はありません。** それらは
`charts/app-base` が `deploy/values.yaml` から生成します。アプリが所有するのは
「自分の namespace」と「自分が何を公開するか」だけです。

## Development Workflow

### For Platform Engineers

1. Create new templates in this directory
2. Test templates locally using Backstage
3. Templates are automatically registered via app-config
4. Document template usage

### For Application Developers

1. Use Backstage UI to create new apps from templates
2. Develop application code in generated repository
3. Push changes to trigger CI/CD
4. Update image tags in overlays for deployment
5. Argo CD automatically syncs changes

## Key Features

- **GitOps Native**: All deployments managed via Git
- **Environment Separation**: Isolated Dev and Prod configurations
- **Automated CI/CD**: GitHub Actions builds and pushes images
- **Self-Service**: Developers create apps without PE intervention
- **Security**: Istio AuthorizationPolicy, non-root containers
- **Monitoring**: Prometheus metrics and ServiceMonitor
- **Documentation**: TechDocs integration

## Links

- [Platform Repository](https://github.com/yu-min3/kensan-lab)
- [Architecture Documentation](../../docs/architecture/)
- [Backstage Documentation](../README.md)

## Contributing

To add a new template:

1. Create a new directory: `<framework>-template/`
2. Add `template.yaml` with Backstage template definition
3. Create `skeleton/` directory with template files
4. Include `.backstage/` with Argo CD Application CRs
5. Add `catalog-info.yaml` for Backstage registration
6. Test the template in Backstage
7. Submit a pull request

## License

MIT
