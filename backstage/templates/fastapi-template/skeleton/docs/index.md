# ${{ values.name }}

${{ values.description }}

## Overview

This application was generated from the FastAPI template using Backstage. It includes a complete GitOps deployment setup with Kustomize for Dev and Prod environments.

## Quick Start

### Development with Dev Container (Recommended)

The fastest way to get started is using VS Code Dev Containers:

1. Install prerequisites:
   - Docker or Podman
   - VS Code
   - Dev Containers extension

2. Open in VS Code and reopen in container:
   - Press `F1` → "Dev Containers: Reopen in Container"

3. Start developing:
   - Press `F5` to run and debug the application
   - Save files to auto-format with ruff
   - Use VS Code tasks for common operations

The Dev Container includes Python 3.11, uv, ruff, kubectl, and Docker CLI - everything pre-configured!

### Local Development (Alternative)

**Prerequisites:**

Install [uv](https://docs.astral.sh/uv/) - a fast Python package manager:

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

**Steps:**

1. Sync dependencies (creates .venv automatically):
   ```bash
   uv sync
   ```

2. Run the application:
   ```bash
   uv run python app/main.py
   ```

3. Access the application at `http://localhost:8000`

### Code Quality Tools

The project uses modern Python tooling:

- **uv**: Fast package manager and dependency resolver
- **ruff**: Fast linter and formatter (replaces flake8, black, isort)

```bash
# Format code
make format

# Run linter
make lint

# Run tests
make test
```

### API Documentation

The FastAPI application automatically generates OpenAPI documentation:
- Swagger UI: `http://localhost:8000/docs`
- ReDoc: `http://localhost:8000/redoc`

## Available Endpoints

- `GET /` - Root endpoint
- `GET /health` - Health check endpoint (used by Kubernetes probes)
- `GET /metrics` - Prometheus metrics endpoint
- `GET /api/v1/example` - Example API endpoint

## Building and Deploying

### Quick Start with Makefile

```bash
make deploy TAG=v1.0.0
```

The Makefile automates:
- Docker image build
- Push to GHCR
- Updating the image tag in `manifests/deployment.yaml`

Commit and push the change. Argo CD will automatically deploy to the `app-prod` namespace.

### Manual Build and Deploy

```bash
docker build -t ghcr.io/yu-min3/${{ values.name }}:v1.0.0 .
docker push ghcr.io/yu-min3/${{ values.name }}:v1.0.0
```

Update the image tag in `manifests/deployment.yaml`:

```yaml
image: ghcr.io/yu-min3/${{ values.name }}:v1.0.0  # Update this tag
```

Commit and push the change. Argo CD will automatically deploy.

## Architecture

### Kubernetes Resources

- **Deployment**: Runs the FastAPI application
- **Service**: ClusterIP service exposing port 8000
- **HTTPRoute**: Gateway API route for external access via Istio
- **AuthorizationPolicy**: Istio security policy
- **ServiceMonitor**: Prometheus metrics collection

### Environment Configuration

| Resource | Dev | Prod |
|----------|-----|------|
| Replicas | 1 | 3 |
| CPU Request | 100m | 200m |
| CPU Limit | 500m | 1000m |
| Memory Request | 128Mi | 256Mi |
| Memory Limit | 512Mi | 1Gi |
| Gateway | gateway-dev | gateway-prod |

## Monitoring

The application exposes Prometheus metrics at `/metrics` endpoint. Metrics include:
- `http_requests_total` - Total HTTP requests counter
- `http_request_duration_seconds` - HTTP request latency histogram

These metrics are automatically scraped by Prometheus via the ServiceMonitor resource.

## Security

- Application runs as non-root user (UID 1000)
- Read-only root filesystem capability
- All privileges dropped
- Istio AuthorizationPolicy enforces access control
- HTTPS/TLS termination at Istio Gateway

## Troubleshooting

### Check Application Logs

```bash
kubectl logs -n app-dev deployment/${{ values.name }} -f
```

### Check Argo CD Sync Status

Visit Argo CD UI:
- Dev: https://argocd.yu-min3.com/applications/app-dev-${{ values.name }}
- Prod: https://argocd.yu-min3.com/applications/app-prod-${{ values.name }}

### Test Health Endpoint

```bash
curl https://${{ values.name }}.yu-min3.com/health
```

## Owner

Owner: ${{ values.owner }}

## Links

- [GitHub Repository](https://github.com/yu-min3/${{ values.name }})
- [Argo CD Dev](https://argocd.yu-min3.com/applications/app-dev-${{ values.name }})
- [Argo CD Prod](https://argocd.yu-min3.com/applications/app-prod-${{ values.name }})
