<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/kensan-logo-dark.svg" width="120">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/kensan-logo-light.svg" width="120">
  <img alt="kensan-lab logo" src="docs/assets/kensan-logo-dark.svg" width="120">
</picture>

# kensan-lab

**Enterprise-grade Kubernetes on bare-metal — for learning, not for show.**

*kensan (研鑽) — the Japanese discipline of continuous skill refinement, like sharpening a blade on a whetstone.*

[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.32-326CE5?style=flat-square&logo=kubernetes&logoColor=white)](https://kubernetes.io/)
[![Argo CD](https://img.shields.io/badge/Argo_CD-v2.14-EF7B4D?style=flat-square&logo=argo&logoColor=white)](https://argoproj.github.io/cd/)
[![Istio](https://img.shields.io/badge/Istio-v1.27-466BB0?style=flat-square&logo=istio&logoColor=white)](https://istio.io/)
[![Cilium](https://img.shields.io/badge/Cilium-v1.18-F8C517?style=flat-square&logo=cilium&logoColor=black)](https://cilium.io/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue?style=flat-square)](./LICENSE)

</div>

---

A bare-metal Kubernetes homelab that mirrors how enterprise platform teams actually run clusters — Argo CD for GitOps, Istio for service mesh, Backstage for developer self-service, and full observability with Prometheus, Grafana, Loki, and Tempo. All running on Raspberry Pis and a mini PC over WiFi.

> This is a **reference architecture**, not a turnkey solution. Published as a learning resource and companion to the author's technical articles. Adapt secrets, domains, and IP ranges for your environment. See [Configuration Guide](./docs/configuration.md).

## Why This Exists

Most homelab repos use Flux + Talos + minimal networking. This one takes a different path — the **enterprise Kubernetes stack** (ArgoCD, Istio, Backstage, Keycloak) deployed on commodity hardware. If you're studying for CKA/CKAD, working as a platform engineer, or want to understand how production clusters are actually structured, this is for you.

**What makes this different:**

- **Argo CD + Helm multi-source** — not Flux. Real-world enterprise GitOps pattern with App of Apps
- **Istio + Gateway API** — full service mesh with mTLS, not just an ingress controller
- **Backstage** — developer portal with scaffolding templates. Almost no homelab does this
- **Keycloak JWT auth** — every external endpoint is authenticated
- **Multi-arch (ARM64 + AMD64)** — real scheduling constraints, not a uniform cluster
- **WiFi-only networking** — because not everyone has ethernet drops everywhere

## Tech Stack

| Category | Technology | Purpose |
|---|---|---|
| **Orchestration** | Kubernetes (kubeadm) | Bare-metal, no managed K8s |
| **Container Runtime** | CRI-O | Lightweight OCI runtime |
| **CNI / Load Balancer** | Cilium | kube-proxy replacement, L2 LoadBalancer |
| **Service Mesh** | Istio + Gateway API | mTLS, traffic management |
| **Authentication** | Keycloak | JWT-based auth (prod/dev) |
| **GitOps** | Argo CD | Helm multi-source Application pattern |
| **Secrets** | Sealed Secrets | Encrypted secrets in Git |
| **Certificates** | cert-manager + Let's Encrypt | Automated TLS |
| **Observability** | Prometheus, Grafana, Loki, Tempo | Metrics, dashboards, logs, traces |
| **Developer Portal** | Backstage | Self-service templates and catalog |

## Hardware

| Device | Qty | Arch | RAM | Role |
|--------|-----|------|-----|------|
| Raspberry Pi 5 | 3 | ARM64 | 8 GB | Control plane + workers |
| Bosgame M4 Neo | 1 | AMD64 | 16 GB | Worker (I/O-heavy workloads) |

4 nodes, multi-architecture, all on WiFi. Managed by kubeadm with CRI-O runtime.

<details>
<summary><b>Scheduling Strategy</b></summary>

| Workload Type | Strategy | Examples |
|---------------|----------|---------|
| I/O Heavy | `requiredDuringScheduling: hardware-class=high-performance` | Prometheus, Loki, Tempo, Keycloak |
| Medium | `preferredDuringScheduling: high-performance` (weight: 80) | OTel Collector |
| Light | No affinity | Grafana, Hubble UI |
| AMD64-only | `required: kubernetes.io/arch=amd64` | Backstage |

</details>

## Architecture

### Multi-Repository GitOps Strategy

```
Platform Engineer (PE)                    Application Developer (AD)
        |                                          |
        v                                          v
+------------------+    Argo CD Application   +--------------+
|  kensan-lab      |<------- CR ------------->|  app-<name>  |
|  (this repo)     |    auto-created by       |  repository  |
|                  |    Backstage              |              |
| infrastructure/  |                          | overlays/    |
| backstage/       |                          |   dev/       |
| apps/            |                          |   prod/      |
+--------+---------+                          +------+-------+
         |                                          |
         +------------> Argo CD <-------------------+
                           |
                     +-----v-----+
                     | Kubernetes |
                     |  Cluster   |
                     +-----------+
```

### Environment Isolation

| Tier | Namespaces | Owner | Argo CD Project |
|---|---|---|---|
| **Infrastructure** | `istio-system`, `argocd`, `monitoring`, `backstage` | Platform Engineer | `platform-project` |
| **Application** | `app-prod`, `app-dev` | App Developer | `app-project-prod`, `app-project-dev` |

<details>
<summary><b>Development Workflow</b></summary>

**Platform Engineer (PE)**:
1. Modify infrastructure settings in this repository
2. Commit and push → Argo CD automatically syncs

**Application Developer (AD)**:
1. Create a new app from Backstage template
2. Backstage auto-creates `app-<name>` repo + Argo CD Application CR
3. Develop in `app-<name>` repo → Argo CD auto-deploys

</details>

## Repository Structure

```
infrastructure/                    # Core platform (GitOps-managed)
├── gitops/argocd/                # Argo CD: applications/, projects/, root-apps/
├── observability/                # Prometheus, Grafana, Loki, Tempo, OTel Collector
├── network/                      # Cilium, Istio, Gateway API
├── security/                     # cert-manager, Sealed Secrets, Keycloak
├── environments/                 # app-dev, app-prod, observability, system-infra
└── storage/                      # local-path-provisioner
backstage/                        # Developer portal (app/ + manifests/)
apps/                             # Sample applications
docs/                             # ADRs, architecture, bootstrapping guides
```

## Security

| Layer | Implementation |
|-------|---------------|
| **Secrets** | Sealed Secrets — encrypted in Git, decrypted in-cluster |
| **Network** | Cilium network policies + Istio mTLS |
| **Auth** | Keycloak JWT validation on all external traffic |
| **RBAC** | Least-privilege access per namespace |
| **Audit** | Complete Git history of all infrastructure changes |

<details>
<summary><b>Internet Exposure</b></summary>

The platform uses Cilium LoadBalancer with L2 announcements for local network access.

For internet exposure, three options are supported:
1. **Port Forwarding** — Simple home lab setup with Dynamic DNS
2. **Cloudflare Tunnel** — Zero Trust access without exposing home IP (recommended)
3. **VPS Reverse Proxy** — Production setup with WireGuard/Tailscale

See [Configuration Guide](./docs/configuration.md) for details.

</details>

## Documentation

| Category | Links |
|----------|-------|
| **Getting Started** | [Installation](./docs/installation.md) / [Configuration](./docs/configuration.md) / [Bootstrapping](./docs/bootstrapping/index.md) / [Secret Management](./docs/secret-management/index.md) |
| **Architecture** | [Platform Design](./docs/architecture/design.md) / [Repository Structure](./docs/architecture/repository-structure.md) / [Namespace Labels](./docs/namespace-label-design.md) / [ADRs](./docs/adr/) |
| **Development** | [Kustomize Guidelines](./docs/kustomize-guidelines.md) / [Roadmap](./docs/roadmap.md) |
| **日本語** | [Japanese documentation (日本語ドキュメント)](./docs/ja/) |

## Sample Application: kensan

The `apps/kensan/` directory contains a full-stack application running on this platform — a personal productivity tool built with React, Go microservices, Python AI agents, and an Iceberg data lakehouse (Dagster + Polaris). It serves as both a real workload and a showcase of what the platform supports: multi-service deployments, database management, observability integration, and CI/CD via ArgoCD.

## Acknowledgments

Inspired by the [Home Operations](https://discord.gg/home-operations) community and projects like [khuedoan/homelab](https://github.com/khuedoan/homelab) and [onedr0p/home-ops](https://github.com/onedr0p/home-ops).

## License

[Apache-2.0](./LICENSE)
