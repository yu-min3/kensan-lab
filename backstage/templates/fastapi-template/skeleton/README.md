# ${{ values.name }}

${{ values.description }}

Generated from the Backstage FastAPI template: a FastAPI backend, React frontend, CI workflows, shared-chart deployment configuration and TechDocs sources.

## Local Development

With Python 3.11, [uv](https://docs.astral.sh/uv/) and Make installed:

```bash
make sync
make run-local
```

The backend listens on port 8000. API docs: [Swagger UI](http://localhost:8000/docs) and [ReDoc](http://localhost:8000/redoc).

For the frontend, install Node.js and npm, then run in a second terminal:

```bash
cd frontend
npm install
npm run dev
```

Open the URL printed by Vite. It proxies `/api` to the backend; `make run-local` alone does not build the React page. The Dockerfile builds and serves both together.

An optional VS Code Dev Container is included. Its configuration uses Podman-specific options; inspect `.devcontainer/devcontainer.json` before choosing a runtime.

## Checks

```bash
make format
make lint
make test
```

These targets operate on the Python backend. Run `npm run build` from `frontend/` after installing its dependencies to verify the frontend build. `make help` lists the other targets; the GHCR publishing helpers are not the Explore delivery workflow.

## Deployment

The platform owns `charts/app-base`. This service owns `deploy/values.yaml` and `deploy/resources/` (namespace and ServiceMonitor). Backstage creates a platform configuration PR that registers an Argo CD Application and the OAuth2 cross-namespace ReferenceGrant.

Review and merge that platform PR to add the service. The active Git host determines the image workflow:

- **Explore / Gitea:** `.gitea/workflows/explore-build.yaml` tests, builds and pushes the image to the in-cluster registry, then records the source SHA tag in `deploy/values.yaml`. Backstage waits for the first build before opening the platform PR.
- **GitHub:** `.github/workflows/build-and-push.yaml` lints, tests and builds ARM64/AMD64 images. Non-PR runs publish to GHCR; commit the chosen published tag in `deploy/values.yaml` to deploy it.

Runtime configuration such as replicas, theme and greeting is also in `deploy/values.yaml`. In Explore, changes confined to `deploy/**` skip the image build while still reaching Argo CD.

## Files and Documentation

- `app/`, `frontend/`, `tests/`: application source and backend tests
- `Dockerfile`: frontend build, backend tests and runtime image
- `pyproject.toml`, `uv.lock`: Python dependencies
- `deploy/values.yaml`: values consumed by the shared platform chart
- `deploy/resources/`: namespace and ServiceMonitor manifests
- `catalog-info.yaml`: Backstage Component, API and TechDocs registration
- `docs/index.md`, `mkdocs.yml`: documentation source and site configuration

Read [the service documentation](docs/index.md) for development, delivery, authentication, monitoring and troubleshooting. The same content is available in the service's Docs tab in Backstage.

## Links

- [Argo CD](${{ values.argocdUrl }})
- [Application](https://${{ values.hostname }})

## Owner

${{ values.owner }}
