# K8s GitOps Platform

A modern GitOps-based Kubernetes platform running on bare-metal hardware, designed to achieve a complete separation of responsibilities between Platform Engineers (PE) and Application Developers (AD).

## Table of Contents

- [Key Features](#key-features)
- [Tech Stack](#tech-stack)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
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

## Quick Start

For detailed prerequisites and installation steps, please refer to the [Installation Guide](./docs/installation.md).

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

## Architecture

### Multi-Repository GitOps Strategy

The platform consists of multiple Git repositories to separate responsibilities:

1.  **`goldship-platform`** (This repository): Infrastructure, security, Argo CD control structure, Backstage templates (managed by PE).
2.  **`app-<name>`**: Application code and deployment settings (managed by AD, one repository per app).

For a detailed breakdown of the repository structure and design decisions, please see the [architecture documentation](./docs/architecture/).

### Environment Isolation and Namespace Design

| Tier | Namespaces | Labels | Administrator | Argo CD Project |
|---|---|---|---|---|
| **Infrastructure** | `kube-system`, `istio-system`, `argocd`, `monitoring` | `goldship.platform/tier: platform`<br>`goldship.platform/environment: infrastructure` | PE | `platform-project` |
| **Authentication** | `platform-auth-prod`, `platform-auth-dev`, `backstage` | `goldship.platform/tier: platform`<br>`goldship.platform/environment: production\|development` | PE | `platform-project` |
| **Application** | `app-prod`, `app-dev` | `goldship.platform/tier: application`<br>`goldship.platform/environment: production\|development` | AD | `app-project-prod`, `app-project-dev` |

## Development Workflow

### Platform Engineer (PE)

1.  Modify infrastructure settings in this repository (`goldship-platform`).
2.  Commit and push the changes to Git.
3.  Argo CD automatically syncs the changes to the cluster.
4.  Check the deployment status in the Argo CD UI.

### Application Developer (AD)

1.  Create a new app from a template in the Backstage UI.
2.  Backstage automatically creates a new `app-<name>` repository and commits the Argo CD Application CR to the `goldship-platform` repository.
3.  Develop code in the generated `app-<name>` repository.
4.  Update the image tag in `overlays/dev/kustomization.yaml`.
5.  Argo CD detects the changes and automatically redeploys the application.

## Documentation

This project includes detailed documentation in the `docs/` directory, primarily in Japanese.

### Getting Started
- **[Installation Guide](./docs/installation.md)**: Prerequisites and setup for the platform.
- **[Configuration Guide](./docs/configuration.md)**: How to modify settings for your own environment.
- **[Bootstrapping Guide](./docs/bootstrapping/index.md)**: Reference for generating manifests from scratch.
- **[Secret Management Guide](./docs/secret-management/index.md)**: How to create and manage secrets.

### Architecture
- **[Platform Architecture](./docs/architecture/design.md)**: Core design principles, tech stack, and security model.
- **[Repository Structure](./docs/architecture/repository-structure.md)**: The multi-repository GitOps strategy and file layout.
- **[Namespace Label Design](./docs/namespace-label-design.md)**: The unified namespace labeling strategy.
- **[Architecture Decision Records (ADR)](./docs/adr/)**: Key design decisions.

### Development
- **[Kustomize Usage Guidelines](./docs/kustomize-guidelines.md)**: When and how to use Kustomize in `base-infra/`.
- **[Implementation Roadmap](./docs/roadmap.md)**: Project phases and current progress.

## Security

- **Encrypted Secrets**: Sealed Secrets encrypts credentials before committing to Git.
- **RBAC**: Kubernetes RBAC enforces least-privilege access.
- **Network Policies**: Cilium network policies control traffic between Pods.
- **Service Mesh**: Istio provides mTLS and traffic encryption.
- **JWT Authentication**: Keycloak validates all external requests.
- **GitOps Audit**: A complete Git history of all infrastructure changes.

> **Important**: Never commit unencrypted secrets.

## License

This project is licensed under the Apache-2.0 License. See the [LICENSE](./LICENSE) file for details.