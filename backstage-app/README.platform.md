# Platform Backstage App

This directory contains the Backstage application for the platform engineering portal.

## Bootstrapping (Initial Setup)

This Backstage app was created using the official Backstage CLI and is included in the platform-config repository for version control and easy plugin management.

### How This App Was Created

```bash
# From the repository root
npx @backstage/create-app@latest --path backstage-app
# App name: platform-backstage
```

The app was then customized for the Kubernetes platform:
- Added `app-config.kubernetes.yaml` for production configuration
- Modified `packages/backend/Dockerfile` to use Kubernetes config
- Created `Makefile` for build automation
- Updated repository `.gitignore` to exclude build artifacts

### Yarn Command Wrapper

Since this project uses Yarn 4 (bundled in `.yarn/releases/`), you don't need to install yarn globally. Use one of these methods:

**Method 1: Helper script (easiest)**
```bash
./yarn.sh <command>
# Example: ./yarn.sh install
```

**Method 2: Makefile commands**
```bash
make install
make dev
make build
```

**Method 3: Direct execution**
```bash
node .yarn/releases/yarn-4.4.1.cjs <command>
```

## Setup

### Prerequisites

- Node.js 20 or 22
- Docker (for building images)
- GitHub Personal Access Token with GHCR push access

### Environment Variables

Create a `.env` file in the repository root (`../` relative to this directory) with:

```bash
GITHUB_USER=your-github-username
GITHUB_GHCR_PAT=your-github-pat
```

### Installation

```bash
cd backstage-app

# Option 1: Using Makefile
make install

# Option 2: Using yarn.sh wrapper
./yarn.sh install

# Option 3: Direct yarn command
node .yarn/releases/yarn-4.4.1.cjs install
```

This will install all dependencies using the bundled Yarn 4.

**Note**: Installation requires significant memory (2GB+). If you encounter memory issues, try:
```bash
export NODE_OPTIONS="--max-old-space-size=4096"
make install
```

### Local Development

Run Backstage locally for testing:

```bash
make dev
```

This will start Backstage on `http://localhost:3000` with hot reload enabled.

## Building and Deploying

### Build Docker Image

```bash
make build
```

This will:
1. Build the backend package
2. Create a Docker image tagged as `ghcr.io/yu-min3/backstage:latest`

You can specify a custom tag:

```bash
make build TAG=v1.0.0
```

### Push to GHCR

```bash
make push
```

This will login to GHCR and push the image.

### Build and Push (All-in-One)

```bash
make all
```

## Configuration

The application uses multiple configuration files:

- **app-config.yaml**: Base configuration for local development
- **app-config.production.yaml**: Production overrides (database, etc.)
- **app-config.kubernetes.yaml**: Kubernetes-specific configuration (used in Docker image)

The Docker image is configured to use `app-config.yaml` + `app-config.kubernetes.yaml`.

### Kubernetes Deployment

The Kubernetes manifests are located in `../base-infra/backstage/`:

- `backstage-deployment.yaml`: Uses `ghcr.io/yu-min3/backstage:latest`
- `backstage-secret.yaml`: Contains `POSTGRES_USER`, `POSTGRES_PASSWORD`, `GITHUB_TOKEN`
- `postgresql-statefulset.yaml`: PostgreSQL database

Environment variables in Kubernetes:
- `POSTGRES_HOST=postgresql`
- `POSTGRES_PORT=5432`
- `POSTGRES_USER` (from secret)
- `POSTGRES_PASSWORD` (from secret)
- `GITHUB_TOKEN` (from secret)

## Adding Plugins

### 1. Install Plugin Packages

```bash
# Example: Kubernetes plugin
yarn --cwd packages/app add @backstage/plugin-kubernetes
yarn --cwd packages/backend add @backstage/plugin-kubernetes-backend
```

### 2. Integrate Plugin Code

Edit `packages/app/src/App.tsx`:

```typescript
import { KubernetesPage } from '@backstage/plugin-kubernetes';

// Add to routes
<Route path="/kubernetes" element={<KubernetesPage />} />
```

### 3. Configure Plugin

Edit `app-config.kubernetes.yaml`:

```yaml
kubernetes:
  serviceLocatorMethod:
    type: 'multiTenant'
  clusterLocatorMethods:
    - type: 'config'
      clusters:
        - url: https://kubernetes.default.svc
          name: local
          authProvider: 'serviceAccount'
```

### 4. Rebuild and Redeploy

```bash
make all
kubectl rollout restart deployment/backstage -n backstage
```

## Common Plugins for Platform Engineering

### Kubernetes Plugin
Visualize and manage Kubernetes resources.

```bash
yarn --cwd packages/app add @backstage/plugin-kubernetes
yarn --cwd packages/backend add @backstage/plugin-kubernetes-backend
```

### ArgoCD Plugin
Monitor ArgoCD applications.

```bash
yarn --cwd packages/app add @roadiehq/backstage-plugin-argo-cd
```

### TechDocs Plugin
Already included! Configure in `app-config.kubernetes.yaml`:

```yaml
techdocs:
  builder: 'local'
  generator:
    runIn: 'docker'
  publisher:
    type: 'local'
```

### Scaffolder Templates
Templates are loaded from the app-templates repository:

```yaml
catalog:
  locations:
    - type: url
      target: https://github.com/yu-min3/app-templates/blob/main/catalog-info.yaml
      rules:
        - allow: [Template]
```

## Troubleshooting

### Memory Issues During Build

If you encounter memory issues during `make install` or `make build`, try:

```bash
# Increase Node.js memory limit
export NODE_OPTIONS="--max-old-space-size=4096"
make install
```

### Image Pull Errors in Kubernetes

Ensure GHCR credentials are configured:

```bash
kubectl get secret backstage-secret -n backstage -o yaml
```

The secret should contain `GITHUB_TOKEN` with GHCR read access.

### Database Connection Issues

Check PostgreSQL is running:

```bash
kubectl get pods -n backstage
kubectl logs -n backstage deployment/postgresql
```

Verify environment variables:

```bash
kubectl exec -n backstage deployment/backstage -- env | grep POSTGRES
```

## Architecture

```
backstage-app/
├── packages/
│   ├── app/              # Frontend (React + plugins)
│   └── backend/          # Backend (Express + plugins)
├── plugins/              # Custom plugins
├── examples/             # Sample catalog entities
├── app-config.yaml       # Base config
├── app-config.kubernetes.yaml  # K8s config
└── Makefile             # Build automation
```

The application follows the standard Backstage monorepo structure with Yarn workspaces.
