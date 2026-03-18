# K8s GitOps Platform Architecture

## 1. Project Vision and Technology Stack

This project aims to build a modern development platform centered on GitOps, with a complete separation of responsibilities between Platform Engineers (PE) and Application Developers (AD). The platform runs on bare-metal Raspberry Pi hardware using CRI-O as the container runtime.

### Technology Stack

| Category | Component | Role |
|---------|-----------|------|
| Orchestration | Kubernetes (`kubeadm`) | Container orchestration on bare-metal infrastructure |
| Container Runtime | CRI-O | Lightweight container runtime interface |
| Networking | Cilium | CNI, network policies, service mesh integration |
| Load Balancer | Cilium LoadBalancer | L2 announcement-based external traffic routing (replaces MetalLB) |
| Service Mesh | Istio | External traffic authentication, inter-service control, security |
| Authentication/Authorization | Keycloak | User account management and JWT issuance |
| Certificate Management | cert-manager | Automatic wildcard certificate issuance and renewal via Let's Encrypt + Route53 DNS challenge |
| GitOps | Argo CD | Automated deployment and tracking of all applications and infrastructure |
| Secret Management | Sealed Secrets | Secure secret management in Git repositories (GHCR credentials) |
| Monitoring | Prometheus | Cluster and application metrics collection (not yet implemented) |
| Developer Portal | Backstage | Application template deployment, documentation, catalog (deployed) |


## 2. Environment and Permission Separation Design

The platform uses a three-layer separation model:

| Layer | Example Namespaces | Manager | Argo CD Project | Purpose and Responsibilities |
|-------|-------------------|---------------|-----------------|------------------------------|
| 1. Infrastructure | `istio-system`, `monitoring`, `argocd`, `platform-auth` | PE | `platform-project` | Core cluster component management. PE-only access. |
| 2. Environment | `app-prod`, `app-dev` | AD | `app-project-prod`, `app-project-dev` | Lifecycle separation. ADs can only operate on resources within their own environment namespace. |
| 3. Application | `app-prod-<app-name>` | AD | N/A | Per-application security/resource isolation. |

### Security Boundaries

- **Infrastructure Layer**: Exclusively managed by PE via `platform-project`. Includes Istio, Prometheus, Argo CD, and Keycloak.
- **Environment Layer**: Independent Argo CD Projects for Prod and Dev. ADs can only access their own namespace resources.
- **Application Layer**: Future extension for per-application isolation.

## 3. Git Repository Structure (GitOps)

All Kubernetes resource definitions are managed in Git repositories. The platform uses a multi-repository strategy:

| Repository | Responsibility | Manager | Tracked Resources |
|-----------|---------------|---------------|-------------------|
| `kensan-lab` | Cluster infrastructure, security, Argo CD control structures (App Projects, Root Apps) | PE | Istio, Keycloak, Sealed Secrets, Namespaces, RBAC, Argo CD Applications |
| `backstage-app/templates/` | Application scaffolding templates for Backstage (Kustomize-based) | PE | Backstage template definitions, base Kubernetes manifests (in kensan-lab) |
| `app-<app-name>` | Application code, Dockerfile, TechDocs, environment-specific deployment configuration | AD | Application code, image tag patches, replica patches |

### GitOps Workflow

1. All K8s resources are defined in Git repositories
2. Argo CD continuously monitors and syncs changes
3. Argo CD uses the "App of Apps" pattern via Root Applications
4. Application developers create new apps through Backstage templates
5. When an AD creates a new app via Backstage:
   - A new `app-<name>` repository with Kustomize structure is created
   - Backstage auto-commits Application CRs to `kensan-lab/infrastructure/argocd/applications/`
   - Argo CD detects the new Application CRs and deploys to both Dev and Prod environments

## 4. Network and Traffic Management

### Cilium LoadBalancer with L2 Announcements

The platform uses Cilium's built-in LoadBalancer capability with L2 announcements, replacing MetalLB:

- **IP Pool**: `192.168.0.240-192.168.0.249` (defined in `infrastructure/network/cilium/resources/lb-ippool.yaml`)
- **L2 Announcement Interfaces**: `wlan0` (functional on home WiFi)
- **Lease Management**: Uses Kubernetes lease API for leader election
- **RBAC**: Appropriate permissions for `coordination.k8s.io/leases` resources

### Istio Service Mesh Integration

- **Istio Gateway**: Defined per environment (Prod/Dev) in `infrastructure/network/istio/`
  - `gateway-prod`: Production Gateway (HTTPS/TLS termination with cert-manager)
  - `gateway-dev`: Development Gateway (HTTPS/TLS termination with cert-manager)
- **HTTPRoute**: Routing configuration using Kubernetes Gateway API
  - Applications attach to the appropriate Gateway via HTTPRoute resources
  - Supports path-based and host-based routing
- **TLS Certificates**: Automatically obtained from Let's Encrypt by cert-manager
  - Wildcard certificates (`*.your-org.com`) managed via Route53 DNS challenge
  - Certificates placed as `wildcard-tls` Secrets in each environment namespace
- **Authentication/Authorization**: External traffic authentication via Keycloak JWT validation (planned)
- **Authorization Policy**: Fine-grained access control per application (planned)

## 5. Secret Management Strategy

- Sensitive secrets (GHCR credentials) are encrypted with Sealed Secrets
- Sealed Secrets can be safely committed to Git
- The Sealed Secrets controller in the cluster decrypts them into regular K8s Secrets
- ServiceAccounts in app namespaces reference `ghcr-pull-secret` for private image pulls

## 6. Developer Experience

### Backstage Integration

Backstage is deployed in the `backstage` namespace and consists of the following components:

- **PostgreSQL StatefulSet**: Database for Backstage metadata persistence
- **Backstage Deployment**: Uses a custom image (`ghcr.io/your-org/backstage`)
- **HTTPRoute**: External access configuration via Istio Gateway
- **Sealed Secrets**: Securely manages PostgreSQL credentials and GitHub Personal Access Token

#### Key Features

- **Template Scaffolding**: ADs use Backstage to create new apps from PE-managed templates (planned)
- **Automated GitOps**: Backstage auto-commits Argo CD Application CRs to `kensan-lab` (planned)
- **TechDocs**: Application documentation rendered directly on Backstage (planned)
- **Catalog**: All applications registered in the Backstage catalog for discoverability (planned)

### Separation of Concerns

- **PE Responsibilities**: Infrastructure, security, templates, platform configuration
- **AD Responsibilities**: Application code, environment-specific settings (image tags, replica counts)
- **Clear Boundaries**: ADs cannot modify infrastructure or access other teams' resources

## 7. Monitoring and Observability

- **Prometheus**: Collects metrics from all cluster components and applications
- **ServiceMonitor**: Each application includes a ServiceMonitor CR for automatic metrics collection
- **Istio Telemetry**: Service mesh provides distributed tracing and observability

## 8. Design Principles

1. **GitOps First**: All changes go through Git commits and Argo CD sync
2. **Least Privilege**: RBAC enforces minimum required permissions
3. **Environment Isolation**: Strict separation between Prod and Dev environments
4. **Self-Service**: ADs can deploy and manage applications without PE intervention
5. **Secure by Default**: Istio + Keycloak authentication on all external endpoints
6. **Declarative Configuration**: All resources defined declaratively in YAML manifests
7. **Immutable Infrastructure**: No direct changes to the cluster

## 9. Current Implementation Status

### Completed

- **Phase 1: Cluster Initialization**
  - Cluster setup with kubeadm
  - Cilium CNI with LoadBalancer (L2 announcements)
  - Sealed Secrets controller
  - GHCR pull secrets (Prod/Dev environments)

- **Phase 2: GitOps Foundation**
  - Argo CD deployment (LoadBalancer: 192.168.0.240)
  - Argo CD Projects (platform-project, app-project-prod, app-project-dev)
  - App-of-Apps pattern implementation
  - Namespace separation (app-prod, app-dev)

- **Phase 3: Service Mesh and Authentication Foundation**
  - Istio Control Plane (`istio-system`)
  - Istio Gateway (Prod/Dev) with HTTPS/TLS termination
  - cert-manager + Let's Encrypt (Route53 DNS challenge)
  - Wildcard certificate (`*.your-org.com`) automatic management
  - Keycloak (Prod/Dev environments) deployment
  - Backstage deployment (PostgreSQL + Backstage Deployment + HTTPRoute)

### Planned

- **Phase 4: Monitoring**
  - Prometheus deployment
  - ServiceMonitor configuration
  - Grafana dashboards

- **Phase 5: Developer Experience**
  - Backstage template creation (backstage-app/templates/)
  - Argo CD Application CR auto-generation
  - TechDocs integration
  - Backstage Catalog registration

- **Phase 6: Application Validation**
  - New application creation flow validation (Dev/Prod)
  - Istio + Keycloak JWT authentication validation
  - GitOps automated deployment flow validation
