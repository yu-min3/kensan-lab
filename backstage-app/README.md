# Backstage Developer Portal

A production-ready Backstage application for the goldship-platform GitOps Kubernetes platform, with integrated application scaffolding templates and optimized for bare-metal deployment.

## ✨ Features

- 🚀 **Integrated Templates**: Application scaffolding templates built-in (`templates/`)
- 📦 **Zero-Install Ready**: Yarn 4 bundled (no global installation needed)
- 🐳 **Container-Ready**: Optimized Docker build with multi-stage support
- 🔐 **Production Configured**: PostgreSQL backend, Sealed Secrets integration
- 📚 **TechDocs Enabled**: Built-in documentation generation
- 🎨 **Customizable**: Easy plugin integration and theming

## 📋 Table of Contents

- [Quick Start](#-quick-start)
- [Prerequisites](#-prerequisites)
- [Configuration](#-configuration)
- [Development](#-development)
- [Building & Deployment](#-building--deployment)
- [Architecture](#-architecture)
- [Troubleshooting](#-troubleshooting)

---

## 🚀 Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/yu-min3/goldship-platform.git
cd goldship-platform/backstage-app
```

### 2. Configure Environment Variables

**IMPORTANT**: Create `.env` file in the repository root (`../`) before starting:

```bash
GITHUB_USER=your-github-username
GITHUB_GHCR_PAT=ghp_your_personal_access_token_here
```

The `app-config.local.yaml` is already committed and references these environment variables:

```yaml
integrations:
  github:
    - host: github.com
      token: ${GITHUB_GHCR_PAT}  # Loaded from .env file
```

### 3. Install Dependencies

```bash
# Using Makefile (recommended)
make install

# Or using the bundled Yarn wrapper
./yarn.sh install

# Or directly
node .yarn/releases/yarn-4.4.1.cjs install
```

**Note**: Installation requires 2GB+ memory. If you encounter issues:
```bash
export NODE_OPTIONS="--max-old-space-size=4096"
make install
```

### 4. Start Development Server

```bash
make dev
```

Access Backstage at **http://localhost:7007** 🎉

---

## 📦 Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| **Node.js** | 22.x | Use nvm for version management |
| **Podman** | Latest | For building container images (recommended) |
| **GitHub PAT** | Read packages | For GHCR and GitHub integration |
| **Memory** | 2GB+ available | For dependency installation |

**Notes:**
- **Yarn is NOT required** - This project bundles Yarn 4 in `.yarn/releases/`.
- **Node.js & Podman are pre-installed** if you use the devcontainer in the repository root.
- **Docker compatibility**: Docker may work but is not officially supported. This project is developed and tested with Podman.

---

## ⚙️ Configuration

### Configuration Files Overview

```
backstage-app/
├── app-config.yaml                  # Base configuration (committed)
├── app-config.kubernetes.yaml       # Kubernetes environment (committed)
└── app-config.local.yaml            # Local overrides (committed, uses env vars)
```

**Note**: All configuration files are committed to the repository. Sensitive values are referenced as environment variables.

### Local Development Configuration

**IMPORTANT**: You must set environment variables before running Backstage.

**Option 1**: Create `.env` file in repository root (`../`)
```bash
GITHUB_USER=your-username
GITHUB_GHCR_PAT=ghp_xxxxxxxxxxxxx
```

**Option 2**: Export directly in your shell
```bash
export GITHUB_USER=your-username
export GITHUB_GHCR_PAT=ghp_xxxxxxxxxxxxx
```

---

## 💻 Development　& Deployment

### Available Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start development server (port 7007) |
| `make install` | Install dependencies with bundled Yarn 4 |
| `make build` | Build Docker image |
| `make push` | Push image to GHCR |
| `make all` | Build and push in one command |
| `make tsc` | TypeScript type checking |
| `make lint` | Run ESLint |
| `make test` | Run unit tests |
| `make test:e2e` | Run Playwright E2E tests |
| `make clean` | Remove local Docker image |

### Development Workflows

#### 1. Configuration Changes (Most Common)

Edit YAML configuration files and see changes immediately:

```bash
# Start development server
make dev

# Edit configuration files (server auto-reloads on changes):
# - app-config.yaml              # Backstage configuration
# - app-config.local.yaml        # Local overrides
# - templates/*/template.yaml    # Scaffolder templates
# - catalog/*.yaml               # Catalog entities

# Changes are reflected immediately at http://localhost:7007
```

**Common configuration changes:**
- Add/modify catalog locations
- Configure integrations (GitHub, GitLab, etc.)
- Update authentication providers
- Customize app title and branding
- Edit scaffolder templates

#### 2. Adding Plugins

When adding new Backstage plugins:

```bash
# 1. Install plugin packages
./yarn.sh add @backstage/plugin-kubernetes --cwd packages/app
./yarn.sh add @backstage/plugin-kubernetes-backend --cwd packages/backend

# 2. Register plugin in packages/app/src/App.tsx
# (Edit the file to import and add routes)

# 3. Configure in app-config.yaml
# (Add plugin-specific configuration)

# 4. Restart development server
make dev
```

#### 3. Container Image Testing

Test the production Docker image locally:

```bash
# Build and run in one command (build is automatic)
make run

# This performs:
# 1. Backend build (yarn workspace backend build)
# 2. Docker image build (podman build)
# 3. Run container on port 7007

# Access at http://localhost:7007
```

#### 4. Production Deployment

Deploy to goldship-platform cluster:

```bash
# Full GitOps deployment
make deploy TAG=v0.0.6

# This command performs the following steps:
# 1. Build backend (yarn workspace backend build)
# 2. Build container image (podman build)
# 3. Push to GHCR (podman push ghcr.io/yu-min3/backstage:v0.0.6)
# 4. Update kustomization.yaml with new image tag
# 5. Git commit with message "Deploy Backstage v0.0.6"
# 6. Git push to remote repository
# 7. Trigger Argo CD sync (if argocd CLI is available)
```

**What happens after `make deploy`:**
- Argo CD detects the kustomization.yaml change
- Automatically syncs the new image to the Kubernetes cluster
- Backstage pods are updated with the new version

**Manual Deployment (Not Recommended):**

⚠️ **Warning**: If Argo CD is configured with auto-sync and self-healing enabled, manual changes may be automatically reverted to match the Git repository state. This behavior depends on Argo CD's sync policy configuration.

If you need to deploy manually without GitOps:

```bash
# Apply manifests directly
kubectl apply -f ../base-infra/backstage/

# Check deployment
kubectl get pods -n backstage
kubectl logs -n backstage deployment/backstage
```

**Note**: To prevent Argo CD from reverting manual changes, you may need to:
- Disable auto-sync: `argocd app set backstage --sync-policy none`
- Or pause the application temporarily

---

## 🏗️ Architecture

### Directory Structure

```
backstage-app/
├── .yarn/
│   └── releases/
│       └── yarn-4.4.1.cjs       # Bundled Yarn 4 (2.7MB)
├── packages/
│   ├── app/                     # Frontend (React)
│   │   ├── src/                 # Source code
│   │   └── e2e-tests/           # Playwright E2E tests
│   └── backend/                 # Backend (Express.js)
│       ├── src/                 # Source code
│       └── Dockerfile           # Production build
├── plugins/                     # Custom Backstage plugins
├── templates/                   # 🌟 Scaffolder templates (integrated)
│   ├── fastapi-template/        # FastAPI app template
│   └── README.md
├── catalog/                     # Production catalog data
│   ├── domains/
│   └── organizations/
├── examples/                    # Local development samples
├── app-config.yaml              # Base configuration
├── app-config.kubernetes.yaml   # Kubernetes config
├── app-config.local.yaml        # Local overrides (gitignored)
├── app-config.local.yaml.example # Template
├── backstage.json               # Backstage version (1.44.0)
├── playwright.config.ts         # E2E test configuration
├── Makefile                     # Build automation
├── yarn.sh                      # Yarn wrapper script
└── README.md                    # This file
```

### Key Design Decisions

#### 🎯 Integrated Templates
- **Location**: `templates/` (not a separate repository)
- **Why**: Simplifies deployment, easier version control
- **Usage**: Automatically loaded in Docker image

#### 🗄️ Database Strategy
- **Local**: In-memory SQLite (fast, ephemeral)
- **Production**: PostgreSQL (persistent, reliable)
- **Config**: Switched via environment variables

#### 🔐 Secret Management
- **Local**: Environment variables
- **Production**: Sealed Secrets in Kubernetes

### Container Image Layers

```dockerfile
FROM node:22-bookworm-slim

# Layer 1: System dependencies (Python, SQLite, mkdocs)
RUN apt-get update && apt-get install -y python3 ...

# Layer 2: Yarn files and workspace setup
COPY backstage-app/.yarn ./.yarn
COPY backstage-app/yarn.lock backstage-app/package.json ...

# Layer 3: Dependencies installation
RUN yarn workspaces focus --all --production

# Layer 4: Production data
COPY backstage-app/catalog ./catalog
COPY backstage-app/templates ./templates    # 🌟 Bundled templates

# Layer 5: Application bundle
COPY backstage-app/packages/backend/dist/bundle.tar.gz ...

CMD ["node", "packages/backend", "--config", "app-config.yaml", "--config", "app-config.kubernetes.yaml"]
```

---

## 🐛 Troubleshooting

### Memory Issues During Installation

**Symptom**: `JavaScript heap out of memory`

**Solution**:
```bash
export NODE_OPTIONS="--max-old-space-size=4096"
make install
```

### Yarn Command Not Found

**Symptom**: `yarn: command not found`

**Solution**: Use bundled Yarn:
```bash
# Use helper script
./yarn.sh install

# Or directly
node .yarn/releases/yarn-4.4.1.cjs install
```

### Template Not Loading

**Symptom**: Templates don't appear in Backstage UI

**Solution**: Check configuration in `app-config.yaml`:
```yaml
catalog:
  locations:
    - type: file
      target: ../../templates/fastapi-template/template.yaml
      rules:
        - allow: [Template]
```

### Database Connection Error

**Symptom**: `connect ECONNREFUSED` or `password authentication failed`

**Solution**:
```bash
# Check environment variables
kubectl exec -n backstage deployment/backstage -- env | grep POSTGRES

# Check PostgreSQL is running
kubectl get pods -n backstage
kubectl logs -n backstage statefulset/postgresql
```

### Image Pull Error in Kubernetes

**Symptom**: `ErrImagePull` or `ImagePullBackOff`

**Solution**:
```bash
# Check GHCR credentials
kubectl get secret backstage-secret -n backstage -o yaml

# Verify secret contains GITHUB_TOKEN
kubectl get secret backstage-secret -n backstage -o jsonpath='{.data.GITHUB_TOKEN}' | base64 -d
```

---

## 🧪 Testing

### Unit Tests

```bash
# Run all tests
make test

# Run with coverage
yarn test:all
```

### E2E Tests (Playwright)

```bash
# Run E2E tests
make test:e2e

# Or directly
yarn test:e2e

# View test report
npx playwright show-report e2e-test-report
```

**What's tested**:
- ✅ App renders welcome page
- ✅ Enter button is clickable
- ✅ Catalog page loads after login

---

## 📚 Additional Resources

- **Backstage Documentation**: https://backstage.io/docs
- **Yarn 4 Documentation**: https://yarnpkg.com
- **Platform Repository**: https://github.com/yu-min3/goldship-platform
- **CLAUDE.md**: See repository root for AI assistant instructions

---

## 📝 License

This project follows the Backstage license (Apache 2.0) for framework code.

---

## 🤝 Contributing

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

---

## 🔖 Version

- **Backstage Version**: 1.44.0 (see `backstage.json`)
- **Node.js**: 22.x
- **Yarn**: 4.4.1 (bundled)

---

**Built with ❤️ for the goldship-platform GitOps ecosystem**
