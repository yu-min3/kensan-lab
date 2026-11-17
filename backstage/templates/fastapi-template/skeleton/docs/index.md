# ${{ values.name }}

${{ values.description }}

## Overview

This service was generated from Backstage's FastAPI template. It contains a FastAPI backend, a React frontend, CI workflows, deployment values and this documentation.

The platform owns the shared Helm chart, `charts/app-base`. This repository owns `deploy/values.yaml` and the namespace and ServiceMonitor manifests in `deploy/resources/`. Argo CD combines those sources to deploy the service into `app-${{ values.name }}`.

## Local Development

### Backend

Install Python 3.11, [uv](https://docs.astral.sh/uv/) and Make, then run:

```bash
make sync
make run-local
```

The API listens on port 8000. Open [Swagger UI](http://localhost:8000/docs) or [ReDoc](http://localhost:8000/redoc). Running the backend alone does not build or serve the React page.

### Frontend

With Node.js and npm installed, run these commands in a second terminal while the backend is running:

```bash
cd frontend
npm install
npm run dev
```

Open the URL printed by Vite. Its development server proxies `/api` requests to the backend on port 8000. The Dockerfile builds the frontend and packages its static assets with FastAPI for deployment.

### Dev Container

The repository includes `.devcontainer/` for VS Code. With the Dev Containers extension and a compatible container runtime installed, use **Dev Containers: Reopen in Container**. The configuration uses Podman-specific container options; check `.devcontainer/devcontainer.json` before using it with another runtime. The Python, uv and Podman tools are defined in its Dockerfile.

### Code Quality

```bash
make format
make lint
make test
```

These targets format, lint and test the Python backend. To check that the frontend builds:

```bash
cd frontend
npm install
npm run build
```

## Delivery Through Git

Backstage creates a separate platform configuration PR to register the service with Argo CD. Review and merge that PR to add the application. The image referenced by `deploy/values.yaml` must be available before the workload can start.

The repository includes workflows for both Git hosts. The active host determines which workflow runs:

- **Explore / Gitea:** `.gitea/workflows/explore-build.yaml` tests the source, builds and pushes an image to the in-cluster registry, then commits the source SHA as the image tag in `deploy/values.yaml`. Backstage waits for the initial build before opening the platform PR.
- **GitHub:** `.github/workflows/build-and-push.yaml` lints and tests the source, then builds images for ARM64 and AMD64. Non-PR runs publish to GHCR. Set the intended published image tag in `deploy/values.yaml` and commit the change for Argo CD to deploy it.

For runtime settings such as replicas, theme and greeting, edit `deploy/values.yaml` and commit the change. In Explore, a commit that changes only `deploy/**` skips the image build; Argo CD still applies the new configuration. A manual `kubectl scale` does not update Git and can be reverted by Argo CD.

## Repository Structure

```text
app/                         FastAPI backend
frontend/                    React frontend and Vite configuration
tests/                       Backend tests
deploy/values.yaml           Values for the platform's charts/app-base
deploy/resources/            Namespace and ServiceMonitor manifests
docs/index.md                TechDocs content
mkdocs.yml                   TechDocs site configuration
catalog-info.yaml            Backstage Component and API definitions
Dockerfile                   Frontend build, backend tests and runtime image
pyproject.toml / uv.lock      Python dependencies and lockfile
.github/workflows/           GitHub CI
.gitea/workflows/            Explore CI
.devcontainer/               Optional development container
```

The platform PR carries the Argo CD Application and the cross-namespace ReferenceGrant needed by the OAuth2 route. The platform chart renders the Deployment, Service and HTTPRoutes; this repository does not contain separate manifests for those resources.

## Authentication

The platform's Gateway-level Istio AuthorizationPolicy delegates authentication to oauth2-proxy. The application receives identity headers after login instead of implementing an OIDC flow itself.

`auth.gatewayOAuth2.enabled` in the service values adds the `/oauth2` route used for login and callbacks. The Gateway policy, ReferenceGrant and Keycloak redirect-URI registration are platform responsibilities. A local development server on port 8000 is outside that Gateway authentication flow.

## Monitoring and Endpoints

`deploy/resources/servicemonitor.yaml` connects the service's `/metrics` endpoint to Prometheus. The backend exposes `http_requests_total` and `http_request_duration_seconds` for HTTP request counts and latency.

| Endpoint | Purpose |
| --- | --- |
| `/` | React page in the built container |
| `/health` | Kubernetes health probes |
| `/metrics` | Prometheus metrics |
| `/api/config` | Runtime name, theme and greeting |
| `/api/whoami` | Identity headers received from the Gateway |
| `/api/v1/example` | Example API response |
| `/api/load` | Demonstration load generator, enabled only when `DEMO_LOAD_ENABLED` is set |
| `/docs`, `/redoc` | FastAPI API documentation |

## Troubleshooting

Check the application logs with:

```bash
kubectl logs -n app-${{ values.name }} deployment/${{ values.name }} -f
```

Use [Argo CD](${{ values.argocdUrl }}) to inspect sync and health status. Deployment follows Git polling and reconciliation, so a merged change may take several minutes to appear.

Open the [application](https://${{ values.hostname }}) through the Gateway and sign in when prompted. An unauthenticated request to the public `/health` URL may redirect to login; it is not the same request path as Kubernetes' Pod health probe.

## Documentation and Ownership

Edit this file alongside application changes. `mkdocs.yml` defines the site and the `backstage.io/techdocs-ref` annotation in `catalog-info.yaml` connects it to the service's Docs tab in Backstage.

Owner: ${{ values.owner }}
