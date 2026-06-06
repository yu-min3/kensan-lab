# Adding a Bosgame M4 Neo Worker Node

> **2026-05-07 update**: This document was originally written when the cluster ran on WiFi (wlp3s0 on m4neo). The cluster has since migrated to wired LAN (eno1 on m4neo) with WiFi fallback. The bootstrapping procedure below is **still valid** — only the network interface step differs (use `eno1` instead of `wlp3s0`). The IP table below is also updated to the actual `192.168.0.x` segment.

## Overview

Procedure for adding a Bosgame M4 Neo (AMD64) as a worker node to the existing Raspberry Pi 5 (ARM64) cluster.
To avoid instability from the amdgpu kernel module, perform a clean install of Ubuntu Server and operate as a headless worker.

Since this creates a multi-architecture cluster (ARM64 + AMD64), the process is organized in three stages: Phase 1 (node join) -> Phase 2 (workload optimization) -> Phase 3 (multi-architecture image support).

## Node Configuration

Refer to the canonical node inventory in [`.claude/rules/kubernetes-cluster.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/kubernetes-cluster.md) (wired IP / WiFi fallback / hardware-class).

---

## Phase 1: Node Setup and Cluster Join

> **Setup Script**: All Phase 1 steps (amdgpu blacklist, netplan, kernel params, CRI-O install, kubeadm install) are consolidated in `temp/setup-m4neo.sh`.
> Read the script for the exact commands. Below are the **conceptual** steps and the few decisions that need human input.

### 1.1 Install Ubuntu Server 24.04 LTS

Install Ubuntu Server 24.04 LTS (minimal profile, OpenSSH only) from a USB boot media. Use the entire internal NVMe (LVM recommended).

> **Note**: Use the Server edition, not Desktop. This fundamentally avoids amdgpu driver instability.

### 1.2 Disable the amdgpu Kernel Module

Headless workers must blacklist `amdgpu` and `drm_kms_helper` (`/etc/modprobe.d/blacklist-amdgpu.conf`) and rebuild initramfs. See `temp/setup-m4neo.sh` for the exact command sequence.

### 1.3 Network Configuration

Configure a static IP with netplan on the wired interface `eno1` (`enp4s0` is reserved, `wlp3s0` is WiFi fallback). Set hostname to `m4neo`.

> **Interface name check**: Use `ip a` to verify the actual interface name and reflect it in netplan. The cluster Cilium config is already set to auto-detect, so no per-node Cilium change is needed (see [`.claude/rules/kubernetes-cluster.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/kubernetes-cluster.md) "Adding New Nodes").

### 1.4 Kernel Parameters

Standard Kubernetes pre-flight (`overlay` / `br_netfilter` modules, `net.ipv4.ip_forward`, swap off). Refer to `temp/setup-m4neo.sh`.

### 1.5 / 1.6 Install CRI-O and kubeadm / kubelet / kubectl

Use the **same versions** as the existing cluster:

```bash
# On master
crio --version
kubectl version --short
```

Then install matching versions on M4 Neo. Standard `pkgs.k8s.io` repository setup — see `temp/setup-m4neo.sh`.

### 1.7 kubeadm join

Generate a join token **on the Master node**:

```bash
kubeadm token create --print-join-command
```

Run the printed command on the M4 Neo.

### 1.8 Cilium Configuration (No Change Required)

The cluster Cilium config has used **auto-detect** for device discovery and a regex pattern (`^eth.*`, `^en.*`, `^wlan.*`, `^wlp.*`) for L2 announcements since the 2026-05-07 wired migration. No change is needed when adding a new node.

Details: [`.claude/rules/network-ingress.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/network-ingress.md) (Cilium LoadBalancer section).

### 1.9 Join Verification

```bash
kubectl get nodes -o wide
kubectl -n kube-system get pods -l k8s-app=cilium -o wide
kubectl get nodes -o custom-columns=NAME:.metadata.name,ARCH:.status.nodeInfo.architecture
kubectl get svc -A | grep LoadBalancer
```

---

## Phase 2: Workload Optimization (nodeAffinity)

The M4 Neo significantly exceeds the RPi 5 in both CPU and memory, so resource-intensive workloads should be preferentially scheduled there.

### 2.1 Node Label Design

```bash
kubectl label node m4neo hardware-class=high-performance
kubectl label node worker1 hardware-class=raspberry-pi
kubectl label node worker2 hardware-class=raspberry-pi
```

### 2.2 nodeAffinity Strategy

Use **preferredDuringSchedulingIgnoredDuringExecution** (preferred scheduling), not `required`, to keep fallback to RPi nodes if M4 Neo is down. Per-workload strategy table (I/O Heavy / Medium / Light / AMD64-only) is in [`.claude/rules/kubernetes-cluster.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/kubernetes-cluster.md) (Scheduling Rules).

### 2.3 Affinity Targets per Component

| Component | values.yaml | Affinity Key |
|--------------|---------|---------|
| Prometheus | `kubernetes/observability/prometheus/values.yaml` | `prometheus.prometheusSpec.affinity` |
| Grafana | `kubernetes/observability/grafana/values.yaml` | top-level `affinity:` |
| Tempo | `kubernetes/observability/tempo/values.yaml` | top-level `affinity:` |
| Loki | `kubernetes/observability/loki/values.yaml` | `singleBinary.affinity` |
| OTel Collector | `kubernetes/observability/otel-collector/values.yaml` | top-level `affinity:` |
| Keycloak | `kubernetes/auth/keycloak/keycloak-deployment.yaml` | direct `spec.template.spec.affinity` |
| Backstage | `backstage/manifests/backstage-deployment.yaml` | direct `spec.template.spec.affinity` |

Standard affinity block (weight 80, `hardware-class=high-performance`):

```yaml
affinity:
  nodeAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 80
        preference:
          matchExpressions:
            - key: hardware-class
              operator: In
              values:
                - high-performance
```

### 2.4 Apply and Verify

Standard GitOps flow — commit, push, Argo CD auto-syncs:

```bash
kubectl get pods -n monitoring -o wide
kubectl describe pod <pod-name> -n monitoring | grep -A5 "Events:"
```

---

## Phase 3: Multi-Architecture Image Build

### 3.1 Background

ARM64 images **cannot run** on AMD64 nodes (`exec format error`). Two solutions:

1. **Multi-platform images** (recommended): single tag, both arches via `docker buildx --platform=linux/amd64,linux/arm64 --push`
2. **nodeSelector pinning**: schedule to specific arch only (workaround)

### 3.2 Build via Makefile

All `apps/kensan-legacy/` images are built via the `apps/kensan-legacy/Makefile` `k8s-*` targets, which already produce multi-arch manifest lists:

```bash
cd apps/kensan-legacy
make k8s-release-dev TAG=v0.1.0      # build + push + PR (dev)
make k8s-release-prod TAG=v0.1.0     # build + push + PR (prod)
make k8s-release TAG=v0.1.0          # build + push + PR (both)
```

See `make help` for the full target list. Backstage uses its own `backstage/app/Makefile` (`make all TAG=...`).

### 3.3 Dockerfile Requirements for Multi-Arch

Go services use cross-compilation via `--platform=$BUILDPLATFORM` + `GOARCH=$TARGETARCH`. The Dockerfiles under `apps/kensan-legacy/backend/services/*/Dockerfile` already follow this pattern. Reference:

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETARCH
RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build ...
```

Node.js / Python apps work as long as the base image supports both arches. Watch out for native extensions (e.g., backstage's `isolated-vm`) — these may require per-arch build stages.

### 3.4 Prerequisites for the Build Host

```bash
# Linux only — Docker Desktop bundles QEMU
sudo apt-get install -y qemu-user-static

# First time only
docker buildx create --use
```

### 3.5 GitHub Actions

Not currently wired up. When CI is added, use `docker/setup-qemu-action` + `docker/setup-buildx-action` + `docker/build-push-action` with `platforms: linux/amd64,linux/arm64`. See `apps/kensan-legacy/Makefile` `k8s-build-*` for the equivalent local commands.

---

## Troubleshooting

### Node Stuck in NotReady

```bash
sudo journalctl -u kubelet -f --no-pager
kubectl -n kube-system get pods -l k8s-app=cilium -o wide
kubectl -n kube-system logs -l k8s-app=cilium --tail=50
```

Common causes:
- CRI-O not running → `sudo systemctl status crio`
- Swap enabled → `sudo swapoff -a`

### Cilium Agent Startup Failure

Cluster Cilium config uses auto-detect, so per-node interface mismatch should not occur. If it does:

```bash
kubectl -n kube-system exec -it ds/cilium -- cilium status --verbose | grep "Devices"
```

### exec format error

ARM64 image running on AMD64 node. Rebuild with multi-arch (Phase 3.2) or pin temporarily with `nodeSelector: kubernetes.io/arch: arm64`.

```bash
docker buildx imagetools inspect ghcr.io/<org>/<image>:<tag>   # verify manifest list
```

### L2 Announcement Not Working on a Node

Check the regex covers the node's interface:

```bash
kubectl get ciliuml2announcementpolicies -o yaml
kubectl get lease -n kube-system | grep cilium
```

### PersistentVolume Issues (legacy local-path)

`local-path` PVCs are node-local — moving a Pod recreates the PV and loses data. New PVCs use Longhorn (replicated, node-independent). Details: [`.claude/rules/kubernetes-cluster.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/kubernetes-cluster.md) (Storage section).
