# GitOps Repository Structure

This document describes the architecture that enables complete separation between Platform Engineers (PE) and Application Developers (AD). The central piece is this `kensan-lab` repository.

## 1. Platform Configuration Repository (kensan-lab)

**Responsibility**: Cluster infrastructure components, security configuration, GitOps control structures, Backstage application source and templates
**Manager**: Platform Engineers (PE)

### Directory Structure

```
kensan-lab/
├── infrastructure/                # Kubernetes infrastructure manifests
│   ├── gitops/argocd/            # Argo CD (with CRDs split)
│   │   ├── 00-crds.yaml         # Argo CD CRDs
│   │   ├── argocd-install.yaml  # Argo CD main installation
│   │   ├── projects/            # AppProject definitions (platform, app-dev, app-prod)
│   │   ├── root-apps/           # Root Application (App of Apps)
│   │   └── applications/        # Platform Application CRs
│   ├── network/cilium/          # Cilium CNI + LoadBalancer configuration
│   ├── network/istio/           # Istio Control Plane and Gateways
│   ├── security/keycloak/       # Keycloak (Kustomize: base + overlays/dev + overlays/prod)
│   ├── observability/prometheus/ # Prometheus Stack (with CRDs split)
│   ├── security/cert-manager/   # Cert-Manager (with CRDs split)
│   ├── security/sealed-secret/  # Sealed Secrets controller
│   ├── network/gateway-api/     # Gateway API CRDs
│   ├── storage/local-path-provisioner/ # Dynamic PV provisioner
│   ├── environments/app-dev/    # Development environment Namespace definition
│   ├── environments/app-prod/   # Production environment Namespace definition
│   └── environments/kube-system/ # kube-system Namespace definition
├── backstage/                    # Backstage application source
│   ├── app/
│   │   ├── packages/
│   │   │   ├── app/             # Frontend React app
│   │   │   └── backend/         # Backend Express app + Dockerfile
│   │   ├── templates/           # Application scaffolding templates
│   │   │   └── fastapi-template/ # Example FastAPI Kustomize template
│   │   ├── plugins/             # Custom Backstage plugins
│   │   ├── app-config.yaml      # Configuration for local development
│   │   ├── app-config.kubernetes.yaml # Configuration for Kubernetes production
│   │   ├── Makefile             # Build automation
│   │   └── yarn.sh             # Yarn wrapper script
├── docs/                        # Documentation
│   ├── architecture/            # Architecture and design documents
│   │   └── repository-structure.md
│   ├── adr/                     # Architecture Decision Records
│   ├── bootstrapping/           # Guide to building from scratch
│   ├── configuration.md         # Guide to environment-specific configuration
│   ├── kustomize-guidelines.md  # Guidelines for using Kustomize
│   ├── namespace-label-design.md # Namespace label design
│   └── roadmap.md               # Implementation roadmap
├── temp/                        # Temporary files (git-ignored)
│   └── ghcr-secret-raw.yaml    # Unencrypted secret (DO NOT COMMIT)
├── .gitignore
├── Makefile                     # For building container images
├── README.md                    # This file
└── LICENSE                      # License file
```

### Key Features

- **Infrastructure as Code**: All cluster components defined declaratively
- **GitOps Control**: Argo CD Projects and Root Applications manage the platform
- **Secret Safety**: Sealed Secrets encrypt credentials before Git commit
- **Environment Separation**: Clear separation of Prod and Dev resources
- **Automatic TLS Certificate Management**: cert-manager automatically obtains and renews wildcard certificates from Let's Encrypt
- **Kustomize-Based Design**: Keycloak/Backstage use Kustomize base + overlays structure for environment-specific configuration
- **Developer Portal**: Backstage application deployed as a custom image

> **CRD Splitting Pattern**: For large Helm Charts (Prometheus, Argo CD, Cert-Manager),
> CustomResourceDefinitions are split into a `00-crds.yaml` file.
> This improves the readability of Git diffs and controls the deployment order in Argo CD.
> See the `README-CRD-SPLIT.md` in each component's directory for details.

---
*Note: Information about Backstage templates and the generated application repositories has been moved to `backstage/app/README.md`.*
