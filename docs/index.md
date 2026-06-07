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

- **Gateway** — Cloudflare Tunnel (internet) and Cilium L2 LB (LAN) route traffic through Istio Gateway via Gateway API; oauth2-proxy enforces OIDC at the Gateway via ext_authz
- **Applications** — workloads deployed to `app-{name}` namespaces (ADR-006) via Argo CD
- **Internal Developer Platform** — Backstage provides service catalog, TechDocs, and Golden Path scaffolding
- **Observability** — apps emit telemetry to OTel Collector → Prometheus / Loki / Tempo, visualized in Grafana
- **Secrets** — Vault + External Secrets Operator for dynamic / static creds, Sealed Secrets for Vault-independent bootstrap, Reloader for rollout on rotation
- **Security** — Cilium + Istio NetworkPolicy, cert-manager (Let's Encrypt), Pod Security Standards
- **GitOps** — Argo CD splits into `platform-project` (infra) and `app-project` (applications)

## How to use this site

This site is the **single source of truth** for the current state of kensan-lab. Articles on Zenn / dev.to are point-in-time deep dives — they capture the journey but may go stale. Always cross-check against the docs here.

| If you want to… | Start here |
|---|---|
| Understand the platform | [Architecture](#architecture) above, then [ADRs](adr/index.md) |
| Bring up a similar cluster | [Installation](getting-started/installation.md) → [Configuration](getting-started/configuration.md) → [Bootstrapping](bootstrapping/index.md) |
| Operate it day-to-day | [Secret Management](secret-management/index.md), [Runbooks](runbooks/index.md) |
| Read the back-stories | [Articles](articles.md) — Zenn / dev.to deep-dive index |
| See what broke and why | [Incidents](incidents/index.md) |

## Tech Stack & Hardware

Tech stack table, hardware inventory, and feature highlights are in the [Top README](https://github.com/yu-min3/kensan-lab/blob/main/README.md) — the single source of truth.

## License

[Apache-2.0](https://github.com/yu-min3/kensan-lab/blob/main/LICENSE)
