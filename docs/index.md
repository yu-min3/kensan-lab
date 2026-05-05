---
hide:
  - navigation
---

# kensan-lab

**Enterprise-grade Kubernetes on bare-metal — a reference architecture for platform engineering.**

A bare-metal Kubernetes homelab built with technologies typical of enterprise platform engineering — Argo CD for GitOps, Istio for service mesh, Backstage for developer self-service, and observability with Prometheus, Grafana, Loki, and Tempo. All running on Raspberry Pis and a mini PC.

!!! note "Reference architecture, not a turnkey solution"
    Bootstrap automation (Ansible + Makefile) is planned. Adapt secrets, domains, and IP ranges to your environment — see the [Configuration Guide](getting-started/configuration.md).

## Architecture

<figure markdown>
  ![Platform Architecture](assets/request-flow.png){ width="800" }
  <figcaption>How traffic flows through the platform and how components interact</figcaption>
</figure>

- **Gateway** — Cloudflare Tunnel (internet) and Cilium L2 LB (LAN) route traffic through Istio Gateway via Gateway API
- **Applications** — workloads deployed to prod/dev namespaces via Argo CD
- **Internal Developer Platform** — Backstage provides service catalog, TechDocs, and Golden Path scaffolding
- **Observability** — apps emit telemetry to OTel Collector → Prometheus / Loki / Tempo, visualized in Grafana
- **Security** — Sealed Secrets, Cilium + Istio NetworkPolicy, cert-manager, Pod Security Standards
- **GitOps** — Argo CD splits into `platform-project` (infra) and `app-project` (applications)

## How to use this site

This site is the **single source of truth** for the current state of kensan-lab. Articles on Zenn / dev.to are point-in-time deep dives — they capture the journey but may go stale. Always cross-check against the docs here.

| If you want to… | Start here |
|---|---|
| Understand the platform | [Architecture](#architecture) above, then [ADRs](adr/index.md) |
| Bring up a similar cluster | [Installation](getting-started/installation.md) → [Configuration](getting-started/configuration.md) → [Bootstrapping](bootstrapping/index.md) |
| Operate it day-to-day | [Secret Management](secret-management/index.md), [Runbooks](runbooks/longhorn-restore-test.md) |
| Read the back-stories | [Articles](articles.md) — Zenn / dev.to deep-dive index |
| See what broke and why | [Incidents](incidents/2026-03-02-system-infra-sync-blocked.md) |

## Tech Stack

| Layer | Technology |
|---|---|
| Kubernetes | bare-metal kubeadm (RPi 5 ARM64 ×3 + Bosgame M4 Neo AMD64 ×1) |
| Runtime | CRI-O (cluster), Podman (image builds) |
| CNI / LB | [Cilium](https://cilium.io/) — kube-proxy replacement, L2 LoadBalancer |
| Service Mesh | [Istio](https://istio.io/) + Gateway API |
| GitOps | [Argo CD](https://argo-cd.readthedocs.io/) — Helm multi-source pattern |
| Secrets | [Sealed Secrets](https://sealed-secrets.netlify.app/) |
| Certificates | [cert-manager](https://cert-manager.io/) + Let's Encrypt |
| Auth | [Keycloak](https://www.keycloak.org/) |
| Observability | Prometheus, Grafana, Loki, Tempo, OpenTelemetry Collector |
| Developer Portal | [Backstage](https://backstage.io/) |

## Hardware

| Device | Qty | Arch | RAM | Role |
|---|---|---|---|---|
| Raspberry Pi 5 | 3 | ARM64 | 8 GB | Control plane + workers |
| Bosgame M4 Neo | 1 | AMD64 | 32 GB | Worker (I/O-heavy workloads) |

4 nodes, multi-architecture. Managed by kubeadm with CRI-O runtime.

## License

[Apache-2.0](https://github.com/yu-min3/kensan-lab/blob/main/LICENSE)
