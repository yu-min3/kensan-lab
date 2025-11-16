# ${{ values.name }}

${{ values.description }}

Generated from the Backstage FastAPI template.

## Quick Start

### Development Options

You can develop this application in two ways:
1. **Dev Container** (Recommended): Pre-configured environment with all tools
2. **Local Development**: Install dependencies on your local machine

### Option 1: Dev Container (Recommended)

The easiest way to get started! Open this project in VS Code and use Dev Containers.

**Prerequisites:**
- [Docker](https://www.docker.com/get-started) or [Podman](https://podman.io/)
- [VS Code](https://code.visualstudio.com/)
- [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

**Steps:**
1. Open this folder in VS Code
2. Press `F1` and select "Dev Containers: Reopen in Container"
3. Wait for the container to build (first time only)
4. Start developing!

The Dev Container includes:
- Python 3.11
- uv (package manager)
- ruff (linter/formatter) - auto-format on save
- kubectl, kustomize (for Kubernetes)
- Docker CLI (for building images)
- All VS Code extensions configured

**Quick commands in Dev Container:**
- `F5` - Run and debug FastAPI application
- `Ctrl+Shift+B` - Sync dependencies
- `Ctrl+Shift+T` - Run tests
- VS Code tasks available via `Ctrl+Shift+P` → "Tasks: Run Task"

### Option 2: Local Development

**Prerequisites:**

Install [uv](https://docs.astral.sh/uv/) - a fast Python package manager:

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

**Local Development:**

```bash
# First time: Sync dependencies (creates .venv and uv.lock)
uv sync

# Run the application
uv run python app/main.py

# Or use the Makefile
make sync
make run-local
```

**Important**: After generating this project from the template, run `uv sync` first to create the `uv.lock` file. This file should be committed to Git to ensure reproducible builds.

Access the application at `http://localhost:8000`

API Documentation:
- Swagger UI: `http://localhost:8000/docs`
- ReDoc: `http://localhost:8000/redoc`

### Code Quality

```bash
# Format code
make format

# Run linter
make lint

# Run tests
make test
```

### Docker Build

```bash
# Build image (requires uv.lock file)
docker build -t ghcr.io/yu-min3/${{ values.name }}:dev-latest .

# Run container
docker run -p 8000:8000 ghcr.io/yu-min3/${{ values.name }}:dev-latest

# Or use the Makefile
make build TAG=dev-latest
```

## Deployment

This application uses GitOps with Argo CD for automated deployments.

### Quick Deployment with Makefile

The included Makefile provides convenient commands for building, pushing, and deploying:

```bash
# Show all available commands
make help

# Deploy to Dev environment (any tag format allowed)
make deploy-dev TAG=dev-latest
make deploy-dev TAG=dev-v1.0.0
make deploy-dev TAG=feature-branch

# Deploy to Prod environment (vX.Y format required)
make deploy-prod TAG=v1.0
make deploy-prod TAG=v1.2
make deploy-prod TAG=v10.5

# Build image only
make build TAG=v1.0

# Push image only
make push TAG=v1.0

# Update environment without building
make update-dev TAG=dev-latest
make update-prod TAG=v1.2

# Validate Kustomize manifests
make validate
```

**TAG Format Requirements:**
- **Dev environment**: Any tag format (e.g., `dev-latest`, `dev-v1.0`, `feature-branch`)
- **Prod environment**: Must be `vX.Y` format (e.g., `v1.0`, `v1.2`, `v10.5`, `v12.34`)

The Makefile will:
1. Validate the TAG format (Prod only)
2. Build the Docker image with the specified TAG
3. Push it to GHCR
4. Update the `overlays/{dev,prod}/kustomization.yaml` file with the new tag
5. Prompt you to commit and push the changes

### Manual Deployment

#### Dev Environment

1. Build and push image:
   ```bash
   docker build -t ghcr.io/yu-min3/${{ values.name }}:dev-latest .
   docker push ghcr.io/yu-min3/${{ values.name }}:dev-latest
   ```

2. Update tag in `overlays/dev/kustomization.yaml`:
   ```yaml
   images:
     - name: app-image
       newName: ghcr.io/yu-min3/${{ values.name }}
       newTag: dev-latest  # Update this
   ```

3. Commit and push:
   ```bash
   git add overlays/dev/kustomization.yaml
   git commit -m "Update Dev image to dev-latest"
   git push
   ```

4. Argo CD will automatically sync to the `app-dev` namespace

#### Prod Environment

1. Build and push image with version tag:
   ```bash
   docker build -t ghcr.io/yu-min3/${{ values.name }}:v1.0.0 .
   docker push ghcr.io/yu-min3/${{ values.name }}:v1.0.0
   ```

2. Update tag in `overlays/prod/kustomization.yaml`:
   ```yaml
   images:
     - name: app-image
       newName: ghcr.io/yu-min3/${{ values.name }}
       newTag: v1.0.0  # Update this
   ```

3. Commit and push:
   ```bash
   git add overlays/prod/kustomization.yaml
   git commit -m "Deploy v1.0.0 to Prod"
   git push
   ```

4. Argo CD will automatically sync to the `app-prod` namespace

## Directory Structure

```
.
├── app/                      # Application source code
│   └── main.py
├── base/                     # Kustomize base manifests
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── httproute.yaml
│   ├── authz-policy.yaml
│   └── servicemonitor.yaml
├── overlays/                 # Environment-specific configurations
│   ├── dev/
│   └── prod/
├── docs/                     # TechDocs documentation
│   └── index.md
├── .backstage/              # Argo CD Application templates
│   ├── argocd-app-dev.yaml
│   └── argocd-app-prod.yaml
├── Dockerfile
├── requirements.txt
└── catalog-info.yaml        # Backstage catalog definition
```

## Available Endpoints

- `GET /` - Root endpoint
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics
- `GET /api/v1/example` - Example API endpoint

## Monitoring

Prometheus metrics are exposed at `/metrics` and automatically scraped via ServiceMonitor.

## Links

- [Backstage](${{ values.destination.backstageUrl }})
- [Argo CD Dev](https://argocd.yu-min3.com/applications/app-dev-${{ values.name }})
- [Argo CD Prod](https://argocd.yu-min3.com/applications/app-prod-${{ values.name }})
- [Live App](https://${{ values.name }}.yu-min3.com)

## Owner

${{ values.owner }}
