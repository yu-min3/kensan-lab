# Prerequisites

## Hardware

- **Minimum**: 1 control plane + 1 worker (4 GB RAM, 50 GB disk each)
- **Recommended**: 1 control plane + 2+ workers (8 GB RAM, 100 GB disk each)
- L2-capable LAN with a free static IP range for Cilium LoadBalancer

The reference cluster's exact node inventory (RPi 5 + M4 Neo, hardware-class labels) is in [`.claude/rules/kubernetes-cluster.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/kubernetes-cluster.md).

## Cluster

- Kubernetes 1.27 or higher (kubeadm)
- kubectl configured to talk to the cluster
- Helm 3.x — only needed for the **initial** Cilium / Argo CD bootstrap. Everything afterwards is managed by Argo CD.

## Build / Dev Machine

- `kubectl`, `helm`, `kubeseal`
- `docker` (default; multi-arch via `docker buildx`) — `make ... CONTAINER_RUNTIME=podman` to switch
- `make` — every component uses `make <target>` as the entry point. See `make help` in `apps/kensan/` and `backstage/app/`.
- Node.js 22.x + Yarn — only for Backstage frontend development (Yarn 4 is bundled in `.yarn/releases/`, no global install needed)

## Accounts & Credentials

- GitHub account + Personal Access Token with `packages:write` (GHCR push)
- Domain name + DNS provider for cert-manager DNS-01 (the reference cluster uses AWS Route 53; see [Configuration Guide](./configuration.md) §6 for swapping)
- Free IP range on your LAN for Cilium LoadBalancer (must not overlap DHCP)

> **Next**: replace author-specific values per the [Configuration Guide](./configuration.md), then follow the [Bootstrapping Guide](../bootstrapping/index.md).
