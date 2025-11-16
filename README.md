# K8s GitOps Platform

A modern GitOps-based Kubernetes platform running on bare-metal hardware, designed to achieve a complete separation of responsibilities between Platform Engineers (PE) and Application Developers (AD).

## Table of Contents

- [Key Features](#key-features)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Setup](#setup)
- [Repository Structure](#repository-structure)
- [Development Workflow](#development-workflow)
- [Documentation](#documentation)
- [Security](#security)
- [License](#license)

## Key Features

- **GitOps First**: All resources are managed with Git and Argo CD.
- **Self-Service**: Application developers can deploy apps without the intervention of platform engineers.
- **Secure by Default**: Istio service mesh + Keycloak JWT authentication.
- **Environment Isolation**: Strict separation between production and development environments.
- **Declarative Configuration**: Kubernetes manifest management through Infrastructure as Code.
- **Automated Deployment**: Changes in Git are automatically synced to the cluster.
- **Developer Portal**: Backstage with templates, documentation, and a catalog feature.

## Tech Stack

| Category | Technology | Purpose |
|---|---|---|
| **Orchestration** | Kubernetes (kubeadm) | Container orchestration on bare-metal |
| **Container Runtime** | CRI-O | Lightweight container runtime |
| **Network** | Cilium CNI | Network policy and service mesh integration |
| **Load Balancer** | Cilium LoadBalancer | L2-based external traffic routing |
| **Service Mesh** | Istio | Traffic management and security |
| **Authentication** | Keycloak | JWT-based authentication |
| **GitOps** | Argo CD | Continuous deployment from Git |
| **Secret Management** | Sealed Secrets | Encrypted secrets in Git |
| **Monitoring** | Prometheus + Grafana | Metrics collection and visualization |
| **Developer Portal** | Backstage | Self-service templates and documentation |

## Prerequisites

### Hardware

- **Minimum Configuration**: 1 Master + 1 or more Workers
- **Recommended Configuration**: 1 Master + 2 or more Workers
- **Memory**: Minimum 4GB per node, 8GB or more recommended
- **Storage**: Minimum 50GB per node, 100GB or more recommended
- **Network**: An environment where L2 network communication is possible

### Software

**For an existing cluster environment:**
- Kubernetes 1.27 or higher (built with kubeadm)
- kubectl (a version compatible with the cluster version)
- kubeconfig configured

**On a development machine:**
- kubectl
- helm 3.x
- kubeseal (Sealed Secrets CLI)
- docker or podman (for building container images)
- make
- Python 3.8 or higher (for CRD splitting script)

**Optional (for Backstage development):**
- Node.js 18.x or higher
- Yarn 4.x

### Accounts & Credentials

- GitHub Account (for container registry GHCR)
- GitHub Personal Access Token (with `packages:write` permissions)
- DNS Provider (for Cert-Manager + Let's Encrypt, e.g., AWS Route53)
- Domain Name (for issuing TLS certificates)

### Network Requirements

- IP range for Cilium LoadBalancer (a range that does not overlap with DHCP)
- Externally accessible IP address (for Istio Gateway)

> **Note**: For detailed configuration items, please refer to the [Environment-Specific Configuration Guide](./docs/configuration.md).

## Quick Start

### 1. Access Argo CD

```bash
# Get the Argo CD LoadBalancer IP
kubectl get svc -n argocd argocd-server -o jsonpath='{.status.loadBalancer.ingress[0].ip}'

# Get the initial password
kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath="{.data.password}" | base64 -d
```

Access `http://<ARGOCD_IP>` in your browser and log in with the username `admin` and the password.

### 2. Check Deployment Status

```bash
# Check all Applications
kubectl get applications -n argocd

# Check resources in a specific Namespace
kubectl get all -n monitoring
kubectl get all -n backstage
kubectl get all -n istio-system
```

### 3. Monitor with Grafana

```bash
# Access via Port-forward
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
# In your browser: http://localhost:3000
# Default username: admin / password: prom-operator (or the sealed value)
```

## Current Status

**Implementation Status**: Phase 3 complete (base infrastructure + authentication platform deployed)

- ✅ Phase 1: Cluster Initialization
- ✅ Phase 2: GitOps Foundation (Argo CD)
- ✅ Phase 3: Service Mesh & Authentication (Istio, Keycloak, Cert-Manager, Prometheus, Backstage)
- 🚧 Phase 4: Developer Experience (Backstage templates in progress)
- ⏳ Phase 5: Application Validation (Not started)

For details, please see the [Implementation Roadmap](./docs/roadmap.md).

## Architecture

### Multi-Repository GitOps Strategy

The platform consists of multiple Git repositories to separate responsibilities:

1.  **`goldship-platform`** (This repository)
    Infrastructure, security, Argo CD control structure, Backstage templates (managed by PE)
    - `base-infra/`: Kubernetes infrastructure manifests
    - `backstage-app/`: Backstage application source
    - `backstage-app/templates/`: Application scaffolding templates

2.  **`app-<name>`**
    Application code and deployment settings (managed by AD, one repository per app)
    - `base/`: Common Kubernetes manifests
    - `overlays/dev/`: Development environment-specific settings
    - `overlays/prod/`: Production environment-specific settings

### Environment Isolation and Namespace Design

| Tier | Namespaces | Labels | Administrator | Argo CD Project |
|---|---|---|---|---|
| **Infrastructure** | `kube-system`, `istio-system`, `argocd`, `monitoring` | `goldship.platform/tier: platform`<br>`goldship.platform/environment: infrastructure` | PE | `platform-project` |
| **Authentication** | `platform-auth-prod`, `platform-auth-dev`, `backstage` | `goldship.platform/tier: platform`<br>`goldship.platform/environment: production\|development` | PE | `platform-project` |
| **Application** | `app-prod`, `app-dev` | `goldship.platform/tier: application`<br>`goldship.platform/environment: production\|development` | AD | `app-project-prod`, `app-project-dev` |

> For details on namespace label design, see the [Namespace Label Design Document](./docs/namespace-label-design.md).

### Architecture Decision Records (ADR)

Important design decisions are documented in the following ADRs:

- [ADR-001: TLS Termination Pattern](./docs/adr/001-tls-termination-pattern.md) - TLS certificate management at the Istio Gateway
- [ADR-002: Authentication & Authorization Architecture](./docs/adr/002-authentication-authorization-architecture.md) - JWT validation with Keycloak + Istio RequestAuthentication

## Setup

This platform is managed by GitOps, and all infrastructure resources are stored as YAML manifests in the `base-infra/` directory.

### Infrastructure Bootstrap

If you are building a cluster from scratch or need to regenerate manifests, please refer to the [Bootstrapping Guide](./docs/bootstrapping.md).

**Note**: In normal operation, you do not need to run the bootstrapping steps as the existing YAML files are used.

### Environment-Specific Configuration

To use this repository in your own environment, you need to change the following settings:

- GitHub organization/user name (in Argo CD Application CRs, GHCR credentials)
- Cilium LoadBalancer IP range (according to your network environment)
- Domain name (for TLS certificates, HTTPRoutes)
- AWS Route53 credentials (for Cert-Manager DNS-01 challenge)

For details, see the [Environment-Specific Configuration Guide](./docs/configuration.md).

### Common Operations

**Building and Pushing Container Images (e.g., Backstage):**

```bash
# Set environment variables in a .env file:
# GITHUB_USER=<your-username>
# GITHUB_GHCR_PAT=<your-token>

make all      # Build and push the image
make build    # Build only
make push     # Push to GHCR
make clean    # Remove local image
```

## Repository Structure

```
goldship-platform/
├── base-infra/                # Kubernetes infrastructure manifests
│   ├── argocd/                # Argo CD (with CRDs split)
│   │   ├── 00-crds.yaml       # Argo CD CRDs
│   │   ├── argocd-install.yaml # Argo CD main installation
│   │   ├── projects/          # AppProject definitions (platform, app-dev, app-prod)
│   │   ├── root-apps/         # Root Application (App of Apps)
│   │   └── applications/      # Platform Application CRs
│   ├── cilium/                # Cilium CNI + LoadBalancer configuration
│   ├── istio/                 # Istio Control Plane and Gateways
│   ├── keycloak/              # Keycloak (Kustomize: base + overlays/dev + overlays/prod)
│   ├── prometheus/            # Prometheus Stack (with CRDs split)
│   ├── cert-manager/          # Cert-Manager (with CRDs split)
│   ├── sealed-secret/         # Sealed Secrets controller
│   ├── gateway-api/           # Gateway API CRDs
│   ├── local-path-provisioner/ # Dynamic PV provisioner
│   ├── backstage/             # Backstage (Kustomize: base + overlays/prod)
│   ├── app-dev/               # Development environment Namespace definition
│   ├── app-prod/              # Production environment Namespace definition
│   └── kube-system/           # kube-system Namespace definition
├── backstage-app/             # Backstage application source
│   ├── packages/
│   │   ├── app/               # Frontend React app
│   │   └── backend/           # Backend Express app + Dockerfile
│   ├── templates/             # Application scaffolding templates
│   │   └── fastapi-template/  # Example FastAPI Kustomize template
│   ├── plugins/               # Custom Backstage plugins
│   ├── app-config.yaml        # Configuration for local development
│   ├── app-config.kubernetes.yaml # Configuration for Kubernetes production
│   ├── Makefile               # Build automation
│   └── yarn.sh                # Yarn wrapper script
├── scripts/                   # Operational scripts
│   └── split_crds.py          # Splits CRDs from Helm output
├── docs/                      # Documentation
│   ├── architecture/          # Architecture and design documents
│   │   ├── design.md
│   │   └── repository-structure.md
│   ├── adr/                   # Architecture Decision Records
│   │   ├── 001-tls-termination-pattern.md
│   │   └── 002-authentication-authorization-architecture.md
│   ├── bootstrapping/         # Guide to building from scratch
│   ├── configuration.md       # Guide to environment-specific configuration
│   ├── kustomize-guidelines.md # Guidelines for using Kustomize
│   ├── namespace-label-design.md # Namespace label design
│   └── roadmap.md             # Implementation roadmap
├── temp/                      # Temporary files (git-ignored)
│   └── ghcr-secret-raw.yaml   # Unencrypted secret (DO NOT COMMIT)
├── .gitignore
├── Makefile                   # For building container images
├── README.md                  # This file
└── LICENSE                    # License file

> **CRD Splitting Pattern**: For large Helm Charts (Prometheus, Argo CD, Cert-Manager),
> CustomResourceDefinitions are split into a `00-crds.yaml` file.
> This improves the readability of Git diffs and controls the deployment order in Argo CD.
> See the `README-CRD-SPLIT.md` in each component's directory for details.
```

## Development Workflow

### Platform Engineer (PE)

1.  Modify infrastructure settings in this repository (`goldship-platform`).
2.  Commit and push the changes to Git.
3.  Argo CD automatically syncs the changes to the cluster.
4.  Check the deployment status in the Argo CD UI.

```bash
# Example: Changing a Prometheus setting
vi base-infra/prometheus/prometheus-stack.yaml
git add base-infra/prometheus/
git commit -m "Update Prometheus scrape interval"
git push

# Auto-sync in Argo CD (or manual sync)
kubectl get applications -n argocd -w
```

### Application Developer (AD)

1.  Create a new app from a template in the Backstage UI.
2.  Backstage automatically:
    - Creates a new `app-<name>` repository.
    - Commits the Argo CD Application CR to the `goldship-platform` repository.
3.  Develop code in the generated `app-<name>` repository.
4.  Update the image tag in `overlays/dev/kustomization.yaml`.
5.  Argo CD detects the changes and automatically redeploys the application.

```bash
# Example: Updating an application's image tag
cd app-myapp/overlays/dev
kustomize edit set image ghcr.io/your-org/myapp:v1.2.3
git add kustomization.yaml
git commit -m "Deploy v1.2.3 to dev"
git push  # -> Argo CD deploys automatically
```

## Documentation

A detailed Japanese guide can be found in the `docs/` directory.

### Setup and Operations Guides

- **[Bootstrapping Guide](./docs/bootstrapping/index.md)**: Reference guide for building a cluster from scratch.
- **[Secret Management Guide](./docs/secret-management/index.md)**: How to create and manage secrets.
- **[Environment-Specific Configuration Guide](./docs/configuration.md)**: Settings that need to be changed when using in a different environment.

### Architecture and Design

- **[Platform Architecture](./docs/architecture/design.md)**: Design principles, tech stack, security model.
- **[Repository Structure](./docs/architecture/repository-structure.md)**: Multi-repository GitOps strategy and workflow.
- **[Implementation Roadmap](./docs/roadmap.md)**: Project phases and current progress.
- **[ADR-001: TLS Termination Pattern](./docs/adr/001-tls-termination-pattern.md)**: Design decision for TLS certificate management.
- **[ADR-002: Authentication & Authorization Architecture](./docs/adr/002-authentication-authorization-architecture.md)**: Design decision for Keycloak + Istio authentication.

### Development Guidelines

- **[Kustomize Usage Guidelines](./docs/kustomize-guidelines.md)**: When and how to use Kustomize in `base-infra/`.
- **[Namespace Label Design](./docs/namespace-label-design.md)**: Unified namespace labeling strategy and its use in security policies.

### Component-Specific Documentation

- **[Prometheus CRD Splitting](./base-infra/prometheus/README-CRD-SPLIT.md)**: Managing Prometheus Stack CRDs.
- **[Argo CD CRD Splitting](./base-infra/argocd/README-CRD-SPLIT.md)**: Managing Argo CD CRDs.
- **[Cert-Manager CRD Splitting](./base-infra/cert-manager/README-CRD-SPLIT.md)**: Managing Cert-Manager CRDs.

## Security

- **Encrypted Secrets**: Sealed Secrets encrypts credentials before committing to Git.
- **RBAC**: Kubernetes RBAC enforces least-privilege access.
- **Network Policies**: Cilium network policies control traffic between Pods.
- **Service Mesh**: Istio provides mTLS and traffic encryption.
- **JWT Authentication**: Keycloak validates all external requests.
- **GitOps Audit**: A complete Git history of all infrastructure changes.
- **Image Pull Secrets**: GHCR credentials are managed and encrypted with Sealed Secrets.

> **Important**: Never commit files in the `temp/` directory (unencrypted secrets).

## License

This project is licensed under the Apache-2.0 License. See the [LICENSE](./LICENSE) file for details.

---

**Technologies**: Kubernetes • Cilium • Istio • Argo CD • Keycloak • Backstage • Prometheus • Sealed Secrets
