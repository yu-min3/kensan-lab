# Application Templates

This repository contains Backstage templates for creating new applications with GitOps deployment patterns.

## Architecture

Part of a 3-repository GitOps architecture:
- **platform-config**: Platform infrastructure (managed by Platform Engineers)
- **app-templates**: Application templates (this repository)
- **app-\<name\>**: Generated application repositories (managed by Application Developers)

## Available Templates

### FastAPI Template

Location: `fastapi-template/`

A production-ready FastAPI application template with:
- **Modern Python tooling**: uv (fast package manager) + ruff (linter/formatter)
- Kustomize-based deployment (base + dev/prod overlays)
- Prometheus metrics endpoint
- Health check endpoints
- Istio integration (HTTPRoute, AuthorizationPolicy)
- Automatic Argo CD Application CR generation
- GitHub Actions CI/CD pipeline (lint, test, build)
- Backstage TechDocs support
- Sample tests with pytest

## Using Templates

### 1. Register Template in Backstage

Add the template location to Backstage:

```yaml
# In platform-config backstage configuration
catalog:
  locations:
    - type: url
      target: https://github.com/yu-min3/app-templates/blob/main/fastapi-template/catalog-info.yaml
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
- Create a new GitHub repository (`app-<name>`)
- Generate Kustomize manifests (base + dev/prod overlays)
- Create Argo CD Application CRs for both environments
- Commit Application CRs to `platform-config` repository
- Register the app in Backstage catalog

### 4. Deploy Your Application

The application will be automatically deployed by Argo CD:
- **Dev environment**: `app-dev` namespace
- **Prod environment**: `app-prod` namespace

## Template Structure

```
fastapi-template/
├── template.yaml              # Backstage template definition
├── catalog-info.yaml          # Backstage catalog entry
└── skeleton/                  # Template files
    ├── app/                   # Application code
    ├── base/                  # Kustomize base manifests
    ├── overlays/              # Environment-specific configs
    │   ├── dev/
    │   └── prod/
    ├── docs/                  # TechDocs
    ├── .backstage/            # Argo CD Application CRs
    ├── .github/workflows/     # CI/CD pipelines
    ├── Dockerfile
    ├── requirements.txt
    └── catalog-info.yaml
```

## Development Workflow

### For Platform Engineers

1. Create new templates in this repository
2. Test templates locally using Backstage
3. Register templates in Backstage catalog
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

- [Architecture Documentation](./mydocs/architecture/)
- [Design Documentation](./mydocs/design.md)
- [Tasks](./mydocs/tasks.md)
- [Platform Config Repository](https://github.com/yu-min3/platform-config)

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
