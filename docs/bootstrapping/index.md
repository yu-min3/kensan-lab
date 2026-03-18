# Infrastructure Bootstrapping Guide

This document serves as a reference for building the cluster from scratch.

## Architecture

This platform is managed using Argo CD's **Helm multi-source Application** pattern. Each component's chart version, repository, and values are defined in Application CRs, and Argo CD automatically handles Helm rendering and deployment.

```
infrastructure/gitops/argocd/applications/   <- Application CRs (Source of Truth for chart info)
infrastructure/<category>/<component>/
  ├── values.yaml                            <- Helm values
  └── resources/                             <- Custom resources outside Helm management
```

To check or change chart versions, refer to `spec.sources[0].targetRevision` in each Application CR.

## Prerequisites

- Kubernetes cluster (initialized with kubeadm)
- kubectl connected to the cluster
- Helm 3.x (required only for initial bootstrap)

**Note:** For secret creation and management, see the [Secret Management Guide](../secret-management/index.md).

---

## Deployment Order and sync-wave

Deployment order is controlled by Argo CD sync-waves.

| Wave | Component | Application CR |
|------|----------|----------------|
| -- | Cilium CNI | `applications/network/cilium/app.yaml` |
| -3 | Istio namespace + Gateways | `applications/network/istio-resources/app.yaml` |
| -2 | Istio Base CRDs | `applications/network/istio-base/app.yaml` |
| -1 | Istiod | `applications/network/istiod/app.yaml` |
| 0 | cert-manager | `applications/security/cert-manager/app.yaml` |
| -- | Prometheus | `applications/observability/prometheus/app.yaml` |
| -- | Grafana | `applications/observability/grafana/app.yaml` |
| -- | Loki | `applications/observability/loki/app.yaml` |
| -- | Tempo | `applications/observability/tempo/app.yaml` |
| -- | OTel Collector | `applications/observability/otel-collector/app.yaml` |
| -- | Argo CD (self-managed) | `applications/gitops/argocd/app.yaml` |

---

## 1. Bootstrapping Cilium CNI

Cilium must be deployed before Argo CD (Pods cannot start without a CNI). Install it directly with `helm install` for the first time only; Argo CD manages it thereafter.

```bash
helm repo add cilium https://helm.cilium.io/
helm install cilium cilium/cilium \
  --version 1.18.3 \
  --namespace kube-system \
  --values infrastructure/network/cilium/values.yaml
```

Apply custom resources (LB IP Pool, L2 Announcement Policy) manually.

```bash
kubectl apply -f infrastructure/network/cilium/resources/
```

---

## 2. Gateway API CRDs

Standard Gateway API CRDs are required because Istio uses the Gateway API.

```bash
./scripts/03-get-gateway-api.sh
```

---

## 3. Bootstrapping Argo CD

Argo CD is also installed directly with `helm install` for the first time.

```bash
helm repo add argo https://argoproj.github.io/argo-helm
helm install argocd argo/argo-cd \
  --version 9.1.0 \
  --namespace argocd \
  --create-namespace \
  --values infrastructure/gitops/argocd/values.yaml
```

After installation, apply the AppProject and Root Application to begin self-management.

```bash
kubectl apply -f infrastructure/gitops/argocd/projects/platform-project.yaml
kubectl apply -f infrastructure/gitops/argocd/root-apps/platform-root-app.yaml
```

The Root Application recursively scans `infrastructure/gitops/argocd/applications/`, automatically detecting and deploying all child Application CRs. No further manual operations are needed for subsequent components.

---

## 4. Making Configuration Changes

To change a component's configuration:

1. Edit the corresponding `values.yaml`
2. Commit and push to Git
3. Argo CD automatically detects the diff and deploys

To change a chart version, update the `targetRevision` in the Application CR.

---

## Backstage Developer Portal

Backstage is managed with Kustomize and is not subject to Helm migration.

- Application creation: `./scripts/07a-create-backstage-app.sh`
- Kubernetes configuration examples: See the `manifests/` directory
- Build and deploy: `./scripts/07c-build-deploy-backstage.sh`

Create related secrets following the [Secret Management Guide](../secret-management/index.md).

## Keycloak Authentication Platform

Keycloak is also managed with Kustomize.

```bash
./scripts/08-generate-keycloak.sh
```

## Grafana Dashboards

Script to fetch OTel dashboards from Grafana.com:

```bash
./scripts/13-generate-grafana-dashboards.sh
```

See [Grafana Independent Deployment](./11-grafana-independent.md) for details.

---

## Adding Worker Nodes

- [Adding a Bosgame M4 Neo Worker Node](./add-worker-node-m4neo.md) -- Adding an AMD64 node to the existing ARM64 cluster (multi-architecture support)
