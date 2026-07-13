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

Each domain has an architecture page — design thesis, component map, diagrams, and the rationale behind the choices:

| Domain | One-liner |
|---|---|
| [Argo CD](architecture/argocd.md) | One root App-of-Apps, Git as the only actor; ApplicationSet only where structure is uniform |
| [Network](architecture/network.md) · [Cloudflare Tunnel](architecture/cloudflare-tunnel.md) | Cilium CNI + L2 LB, Istio mesh, Gateway API edge; Zero Trust internet exposure |
| [Auth](architecture/auth.md) | Keycloak OIDC, enforced once at the Gateway (oauth2-proxy ext_authz) |
| [Secrets](architecture/secrets.md) | Vault at the core, four delivery methods, SealedSecret bootstrap lane |
| [Observability](architecture/observability.md) | Three pillars through one OTel pipe; the monitoring is itself monitored |
| [Storage](architecture/storage.md) | Longhorn only; Retain + Prune=false + off-cluster R2 backups |
| [Backstage](architecture/backstage.md) | IDP front door — golden path templates, catalog, TechDocs |

Cross-cutting overviews: [Infrastructure](architecture/infrastructure.md) and [Cluster health monitoring](architecture/cluster-health-monitoring.md).

## Showcase

The repository describes a live platform. Start with the [Showcase](showcase.md)
to see the visual proof points: Argo CD app health, Grafana cluster health, and
Hubble network visibility (Backstage and kensan views coming).

## How to use this site

This site is the **single source of truth** for the current state of kensan-lab. Articles on Zenn / dev.to are point-in-time deep dives — they capture the journey but may go stale. Always cross-check against the docs here.

| If you want to… | Start here |
|---|---|
| Understand the platform | [Architecture](#architecture) above, then [ADRs](adr/index.md) |
| See the running system | [Showcase](showcase.md) |
| Bring up a similar cluster | [Installation](getting-started/installation.md) → [Configuration](getting-started/configuration.md) → [Bootstrapping](bootstrapping/index.md) |
| Operate it day-to-day | [Secret Management](secret-management/index.md), [Runbooks](runbooks/index.md) |
| Read the back-stories | [Articles](articles.md) — Zenn / dev.to deep-dive index |
| See what broke and why | [Incidents](incidents/index.md) |

## Tech Stack & Hardware

Tech stack table, hardware inventory, and feature highlights are in the [Top README](https://github.com/yu-min3/kensan-lab/blob/main/README.md) — the single source of truth.

## License

[Apache-2.0](https://github.com/yu-min3/kensan-lab/blob/main/LICENSE)
