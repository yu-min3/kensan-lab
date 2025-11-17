# K8s GitOps Platform

A modern GitOps-based Kubernetes platform running on bare-metal hardware, designed to achieve a complete separation of responsibilities between Platform Engineers (PE) and Application Developers (AD).

## Table of Contents

- [Key Features](#key-features)
- [Tech Stack](#tech-stack)
- [Who Should Use This](#who-should-use-this)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Development Workflow](#development-workflow)
- [Documentation](#documentation)
- [Security](#security)
- [Internet Exposure](#internet-exposure)
- [License](#license)

## Key Features

- **GitOps + Service Mesh Ready**: Deploy a production-ready GitOps cluster with Istio service mesh integration out of the box.
- **Self-Service Application Deployment**: Application developers can deploy apps via Backstage templates, including automatic HTTPRoute distribution for traffic management.
- **Secure by Default**: Istio service mesh + Keycloak JWT authentication for all external traffic.
- **Environment Isolation**: Strict separation between production and development environments with dedicated Argo CD Projects.
- **Declarative Configuration**: All Kubernetes resources managed through Infrastructure as Code in Git.
- **Automated Deployment**: Changes in Git are automatically synced to the cluster via Argo CD.
- **Developer Portal**: Backstage with scaffolding templates, TechDocs, and service catalog for streamlined development workflows.

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

## Who Should Use This

This platform is ideal for:

### Learning the Golden Kubernetes Stack
If you're studying **Golden Kubernetes** (a curated set of CNCF tools for production-ready clusters), this repository provides hands-on experience with:
- ✅ **Argo CD** for GitOps continuous deployment
- ✅ **Istio** for service mesh and traffic management
- ✅ **Cilium** for CNI and network policies
- ✅ **Prometheus + Grafana** for observability
- ✅ **Backstage** for developer experience

**Note**: The following Golden Kubernetes components are not yet implemented:
- ❌ OpenTelemetry (distributed tracing)
- ❌ Keyverno (policy management)
- ❌ Other Argo products (Argo Workflows, Argo Rollouts, Argo Events)

### Organizations Requiring Secure GitOps Clusters
This platform is designed for organizations that need:
- **Security-first architecture**: mTLS service mesh, JWT authentication, encrypted secrets in Git
- **Compliance and auditability**: Complete Git history of all infrastructure changes
- **Multi-tenancy**: Strict namespace isolation between teams and environments
- **Self-service developer workflows**: Reduce bottlenecks by empowering developers with Backstage templates

If your organization values **infrastructure as code, zero-trust networking, and developer productivity**, this platform provides a solid foundation.

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

1.  **`goldship`** (This repository): Infrastructure, security, Argo CD control structure, Backstage templates (managed by PE).
2.  **`app-<name>`**: Application code and deployment settings (managed by AD, one repository per app).

For a detailed breakdown of the repository structure and design decisions, please see the [architecture documentation](./docs/architecture/).

### Environment Isolation and Namespace Design

| Tier | Namespaces | Labels | Administrator | Argo CD Project |
|---|---|---|---|---|
| **Infrastructure** | `kube-system`, `istio-system`, `argocd`, `monitoring`, `backstage`, `platform-auth-prod`, `platform-auth-dev` | `goldship.platform/tier: platform`<br>`goldship.platform/environment: infrastructure` | PE | `platform-project` |
| **Application** | `app-prod`, `app-dev` | `goldship.platform/tier: application`<br>`goldship.platform/environment: production\|development` | AD | `app-project-prod`, `app-project-dev` |

## Development Workflow

### Platform Engineer (PE)

1.  Modify infrastructure settings in this repository (`goldship`).
2.  Commit and push the changes to Git.
3.  Argo CD automatically syncs the changes to the cluster.
4.  Check the deployment status in the Argo CD UI.

### Application Developer (AD)

1.  Create a new app from a template in the Backstage UI.
2.  Backstage automatically creates a new `app-<name>` repository and commits the Argo CD Application CR to the `goldship` repository.
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

## Internet Exposure

### Current Limitations

This platform currently uses **Cilium LoadBalancer with L2 announcements** for local network access. Services are exposed on the local network (e.g., `192.168.0.240-249`) but are **not accessible from the internet**.

### Exposing Services to the Internet

If you need to expose cluster endpoints to the internet, you'll need to set up one of the following:

#### Option 1: Port Forwarding (Simple Home Lab Setup)
For home labs or testing environments:

1. **Configure your router** to forward ports (80, 443) to the Istio Gateway LoadBalancer IP
2. **Set up Dynamic DNS** (e.g., DuckDNS, No-IP) if you don't have a static public IP
3. **Update DNS records** to point your domain to your public IP
4. **Configure TLS certificates** using Let's Encrypt with DNS-01 or HTTP-01 challenges

**Example using cert-manager**:
```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer for Let's Encrypt
# See docs/internet-exposure.md for detailed configuration
```

#### Option 2: Cloudflare Tunnel (Zero Trust Access)
For secure internet access without exposing your home IP:

1. Set up **Cloudflare Tunnel** (cloudflared)
2. Configure tunnel routes to your Istio Gateway
3. Manage access via Cloudflare's Zero Trust dashboard

This approach provides DDoS protection, automatic TLS, and doesn't require port forwarding.

#### Option 3: VPS Reverse Proxy (Production Setup)
For production deployments:

1. Deploy a **VPS with a public IP** (e.g., DigitalOcean, AWS EC2)
2. Set up **WireGuard VPN** or **Tailscale** between your cluster and the VPS
3. Configure **Nginx/Traefik** on the VPS to proxy traffic to your cluster
4. Manage TLS certificates on the VPS

### Documentation

Detailed guides for each approach will be added to `docs/internet-exposure.md`. The setup is relatively straightforward and typically takes 1-2 hours depending on the chosen method.

**Recommended for beginners**: Start with Cloudflare Tunnel for the easiest setup with built-in security.

## License

This project is licensed under the Apache-2.0 License. See the [LICENSE](./LICENSE) file for details.