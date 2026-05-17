# ${{ values.name }}

${{ values.description }}

Generated from the Backstage FastAPI template.

## Quick Start

### Option 1: Dev Container (recommended)

Open the folder in VS Code → `F1` → "Dev Containers: Reopen in Container". First boot builds the container (Python 3.11 + uv + ruff + kubectl + Docker CLI + VS Code extensions). Shortcuts inside the container: `F5` run/debug, `Ctrl+Shift+B` sync deps, `Ctrl+Shift+T` test.

Prereqs: [Docker](https://www.docker.com/get-started) (or [Podman](https://podman.io/)), [VS Code](https://code.visualstudio.com/), [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers).

### Option 2: Local

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh   # uv (Python package manager)
make help                                         # see all targets
make sync                                         # create .venv + uv.lock (commit uv.lock!)
make run-local                                    # run on :8000
```

API docs: <http://localhost:8000/docs> (Swagger) · <http://localhost:8000/redoc>.

## Common Commands

```bash
make help                  # full target list
make format / lint / test  # code quality
make build TAG=v1.0        # build image
make deploy TAG=v1.0       # build + push + bump manifests/deployment.yaml
make validate              # kubectl apply --dry-run on manifests/
```

`TAG` is required and must be `vX.Y` (e.g. `v1.0`, `v10.5`). See `make help` for the container runtime switch (`CONTAINER_RUNTIME=podman`) and other knobs.

## Deployment

GitOps via Argo CD. `make deploy TAG=v1.0` builds, pushes to GHCR, and rewrites the image tag in `manifests/deployment.yaml`. **Commit + push** the manifests change to trigger sync:

```bash
git add manifests/deployment.yaml
git commit -m "Deploy v1.0"
git push
```

Argo CD then syncs the app namespace.

## Directory Structure

```
.
├── app/main.py                     # application source
├── manifests/                      # Kubernetes manifests (Argo CD directory source)
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── httproute.yaml
│   ├── authz-policy.yaml
│   └── servicemonitor.yaml
├── docs/index.md                   # TechDocs source
├── .backstage/argocd-apps.yaml     # Argo CD Application template
├── Dockerfile
├── requirements.txt
└── catalog-info.yaml               # Backstage catalog entry
```

## Endpoints

- `GET /` — root
- `GET /health` — health check
- `GET /metrics` — Prometheus metrics (scraped via ServiceMonitor)
- `GET /api/v1/example` — example

## Links

- [Backstage](${{ values.destination.backstageUrl }})
- [Argo CD](https://argocd.yu-min3.com/applications/app-${{ values.name }})
- [Live App](https://${{ values.name }}.yu-min3.com)

## Owner

${{ values.owner }}
