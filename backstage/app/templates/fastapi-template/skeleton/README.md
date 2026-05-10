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
- kubectl (for Kubernetes)
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

```bash
# Show all available commands
make help

# Build, push, and update image tag in manifests/deployment.yaml (vX.Y format required)
make deploy TAG=v1.0
make deploy TAG=v1.2

# Build only
make build TAG=v1.0

# Push only
make push TAG=v1.0

# Update image tag in manifests without building
make update-image TAG=v1.2

# Validate manifests with kubectl dry-run
make validate
```

**TAG format**: Must be `vX.Y` (e.g., `v1.0`, `v1.2`, `v10.5`).

The `deploy` target builds the image, pushes to GHCR, and updates `manifests/deployment.yaml` with the new tag. Commit and push the manifest change to trigger Argo CD sync.

### Manual Deployment

1. Build and push image with version tag:
   ```bash
   docker build -t ghcr.io/yu-min3/${{ values.name }}:v1.0.0 .
   docker push ghcr.io/yu-min3/${{ values.name }}:v1.0.0
   ```

2. Update image tag in `manifests/deployment.yaml`:
   ```yaml
   image: ghcr.io/yu-min3/${{ values.name }}:v1.0.0  # Update this
   ```

3. Commit and push:
   ```bash
   git add manifests/deployment.yaml
   git commit -m "Deploy v1.0.0"
   git push
   ```

4. Argo CD will automatically sync to the `app-prod` namespace.

## Directory Structure

```
.
├── app/                      # Application source code
│   └── main.py
├── manifests/                # Kubernetes manifests (Argo CD directory source)
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── httproute.yaml
│   ├── authz-policy.yaml
│   └── servicemonitor.yaml
├── docs/                     # TechDocs documentation
│   └── index.md
├── .backstage/               # Argo CD Application template
│   └── argocd-apps.yaml
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
- [Argo CD](https://argocd.yu-min3.com/applications/app-${{ values.name }})
- [Live App](https://${{ values.name }}.yu-min3.com)

## Owner

${{ values.owner }}
