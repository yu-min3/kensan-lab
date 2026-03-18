# Project Roadmap

This document outlines the current status, short-term goals, and long-term vision for the project.

## Current Status

**Automated application deployment platform via Backstage is complete.**

The platform's core capabilities (GitOps, service mesh, monitoring) are running stably. The **Backstage FastAPI template is complete, enabling developers to auto-generate FastAPI applications with just a few clicks from the UI and automatically deploy them to the Kubernetes cluster (Dev/Prod)**.

## Next Up

Development is underway to expand platform capabilities and support more production-grade use cases.

- **Platform Generalization and Feature Enhancements**
  - [ ] **x86 Node Support**: Add validation and support for running on general x86 servers, in addition to the current ARM-based (Raspberry Pi, etc.) setup.
  - [ ] **Streamlit Template**: Develop a new Backstage template for Streamlit applications to support data science use cases.

- **Security and Authentication Enhancements**
  - [ ] **Keycloak Authentication at Istio Gateway**: Require Keycloak JWT authentication at the Istio Gateway layer for all external traffic to applications.
  - [ ] **Thorough Service Mesh Authentication**: Apply strict mTLS authentication for workload-to-workload communication and establish authorization policies (AuthorizationPolicy).

- **Observability Improvements**
  - [ ] **OpenTelemetry Adoption**: Implement distributed tracing to facilitate performance bottleneck identification and error tracking in applications.

- **Developer Experience Improvements**
  - [ ] Improve the Backstage UI (catalog registration, documentation enhancements).
  - [ ] Expand end-to-end testing to further increase platform reliability.

## Future Goals

The following features are extension candidates after core features have been fully validated.

- **Security & Compliance**
  - Policy enforcement with OPA/Gatekeeper
  - Container vulnerability scanning with Trivy, etc.
  - Automatic SBOM (Software Bill of Materials) generation

- **Developer Experience Improvements**
  - Multi-tenancy through resource quotas
  - Support for advanced deployment strategies such as canary releases
  - Automation of initial secret setup in Backstage

- **Operations & Observability Enhancements**
  - Establishing disaster recovery (DR) procedures with Velero, etc.

---

## Completed Milestones

<details>
<summary>Click here for details on previously completed phases</summary>

### Phase 1: Cluster Initialization -- Completed

Set up the bare-metal Kubernetes cluster foundation.

- **Achievements**:
  - Kubernetes cluster running on bare-metal hardware
  - CRI-O container runtime configured
  - Cilium CNI with LoadBalancer introduced with kube-proxy replacement enabled
  - Sealed Secrets controller installed and GHCR credentials encrypted

### Phase 2: GitOps Foundation -- Completed

Established Argo CD as the GitOps engine and organized infrastructure management.

- **Achievements**:
  - Argo CD operational with Platform/App GitOps Projects established
  - Full GitOps management of infrastructure via "App of Apps" pattern
  - Unified namespace label design introduced
  - CRD splitting pattern introduced, improving Git diff readability and deployment order

### Phase 3: Service Mesh and Authentication -- Completed

Deployed core security and access control components.

- **Achievements**:
  - **Istio**: Control Plane and environment-specific Gateways deployed, mTLS enabled
  - **Keycloak**: Keycloak instances for Prod/Dev environments built with Kustomize
  - **Cert-Manager**: Automatic wildcard certificate issuance and renewal with Let's Encrypt integration
  - **Prometheus**: Monitoring stack deployed with automatic collection via ServiceMonitor
  - **Backstage**: Custom-built developer portal deployed

</details>
