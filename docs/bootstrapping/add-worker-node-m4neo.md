# Adding a Bosgame M4 Neo Worker Node

## Overview

Procedure for adding a Bosgame M4 Neo (AMD64) as a worker node to the existing Raspberry Pi 5 (ARM64) cluster.
To avoid instability from the amdgpu kernel module, perform a clean install of Ubuntu Server and operate as a headless worker.

Since this creates a multi-architecture cluster (ARM64 + AMD64), the process is organized in three stages: Phase 1 (node join) -> Phase 2 (workload optimization) -> Phase 3 (multi-architecture image support).

## Node Configuration

| Node | Hostname | IP | Hardware | Architecture | Role |
|--------|----------|----|-------------|--------------|------|
| Master | master | 192.168.1.107 | Raspberry Pi 5 (8GB) | ARM64 | control-plane |
| Worker1 | worker1 | 192.168.1.108 | Raspberry Pi 5 (8GB) | ARM64 | worker |
| Worker2 | worker2 | 192.168.1.109 | Raspberry Pi 5 (8GB) | ARM64 | worker |
| **M4 Neo** | **m4neo** | **192.168.1.110** | **Bosgame M4 Neo (Ryzen 7 7840HS, 32GB DDR5)** | **AMD64** | **worker** |

---

## Phase 1: Node Setup and Cluster Join

> **Setup Script**: Phase 1 steps are consolidated in `temp/setup-m4neo.sh`.
> Run it on the M4 Neo.

### 1.1 Install Ubuntu Server 24.04 LTS

Install Ubuntu Server 24.04 LTS in minimal configuration from a USB boot media.

- **Profile**: minimal (minimal installation)
- **Additional packages**: Select only OpenSSH Server
- **Username**: Your choice (referred to as `yu` hereafter)
- **Disk**: Use the entire internal NVMe (LVM recommended)

> **Note**: Use the Server edition, not Desktop. This fundamentally avoids amdgpu driver instability.

### 1.2 Disable the amdgpu Kernel Module

The amdgpu module may still be loaded on Ubuntu Server. Completely disable it for headless operation.

```bash
# Create /etc/modprobe.d/blacklist-amdgpu.conf
sudo tee /etc/modprobe.d/blacklist-amdgpu.conf <<'EOF'
# Disable amdgpu for headless worker node stability
blacklist amdgpu
blacklist drm_kms_helper
EOF

sudo update-initramfs -u
sudo reboot
```

Verify after reboot:

```bash
lsmod | grep amdgpu
# No output = success
```

### 1.3 Network Configuration

Configure a static IP with netplan. The M4 Neo uses WiFi (`wlp3s0`) (wired ports `eno1`/`enp4s0` are disconnected).

```yaml
# /etc/netplan/01-static.yaml
network:
  version: 2
  wifis:
    wlp3s0:        # Adjust to your actual interface name (check with ip a)
      addresses:
        - 192.168.1.110/24
      routes:
        - to: default
          via: 192.168.1.1
      nameservers:
        addresses:
          - 192.168.1.1
          - 8.8.8.8
      dhcp4: false
      access-points:
        "YOUR_SSID":
          password: "YOUR_PASSWORD"
```

```bash
sudo netplan apply
```

> **Interface name check**: Use `ip a` to verify the actual interface name (`wlp3s0`, etc.) and reflect it in netplan and subsequent Cilium configuration. RPi nodes use `wlan0`, while M4 Neo uses `wlp3s0`.

Set the hostname:

```bash
sudo hostnamectl set-hostname m4neo
```

### 1.4 Kernel Parameters

Configure the kernel modules and parameters required by Kubernetes.

```bash
# Persist kernel modules
cat <<'EOF' | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# sysctl parameters
cat <<'EOF' | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system
```

Disable swap:

```bash
sudo swapoff -a
# Remove or comment out the swap entry in /etc/fstab
sudo sed -i '/\sswap\s/s/^/#/' /etc/fstab
```

### 1.5 Install CRI-O

Use the same CRI-O version as the existing cluster. Check with `crio --version` on the master before installing.

```bash
# Add CRI-O repository (v1.31 series example)
CRIO_VERSION="v1.31"

curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/stable:/$CRIO_VERSION/deb/Release.key | \
  sudo gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/stable:/$CRIO_VERSION/deb/ /" | \
  sudo tee /etc/apt/sources.list.d/cri-o.list

sudo apt-get update
sudo apt-get install -y cri-o

sudo systemctl enable --now crio
```

### 1.6 Install kubeadm / kubelet / kubectl

Use the same Kubernetes version as the existing cluster. Check with `kubectl version --short` on the master.

```bash
KUBE_VERSION="v1.31"

curl -fsSL https://pkgs.k8s.io/core:/stable:/$KUBE_VERSION/deb/Release.key | \
  sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/$KUBE_VERSION/deb/ /" | \
  sudo tee /etc/apt/sources.list.d/kubernetes.list

sudo apt-get update
sudo apt-get install -y kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl

sudo systemctl enable --now kubelet
```

### 1.7 kubeadm join

Generate a join token **on the Master node**:

```bash
# Run on Master
kubeadm token create --print-join-command
```

Run the output command **on the M4 Neo**:

```bash
# Run on M4 Neo (paste the output command as-is)
sudo kubeadm join 192.168.1.107:6443 --token <token> --discovery-token-ca-cert-hash sha256:<hash>
```

### 1.8 Cilium Configuration Changes (Important)

The existing Cilium configuration targets only `wlan0` (the Raspberry Pi WiFi interface).
Since the M4 Neo uses WiFi `wlp3s0`, **the cluster-wide** Cilium configuration must be changed to match both interface names.

#### Changes to `infrastructure/network/cilium/values.yaml`

```yaml
# Before:
devices: wlan0

# After (change to auto-detect to automatically recognize all node interfaces):
devices: ""   # auto-detect (automatically recognizes wlan0, wlp3s0, etc.)
```

> An empty string (auto-detect) causes both RPi's `wlan0` and M4 Neo's `wlp3s0` to be automatically recognized.
> Cilium's `devices` also accepts Linux device name wildcards, but auto-detect is recommended.

#### Changes to `infrastructure/network/cilium/resources/lb-ippool.yaml`

```yaml
# Before:
spec:
  interfaces:
  - wlan0

# After (regex pattern):
spec:
  interfaces:
  - "^wlan.*"
  - "^wlp.*"
```

> CiliumL2AnnouncementPolicy's `interfaces` field accepts regular expressions.
> `^wlan.*` matches RPi's `wlan0`, and `^wlp.*` matches M4 Neo's `wlp3s0`.

#### Applying Changes

```bash
# After changing values.yaml, push to Git and Argo CD will auto-sync
git add infrastructure/network/cilium/values.yaml
git add infrastructure/network/cilium/resources/lb-ippool.yaml
git commit -m "Cilium: multi-interface support (wlan + wlp)"
git push

# Verify Cilium agent restart
kubectl -n kube-system rollout status daemonset/cilium
```

### 1.9 Join Verification

```bash
# Verify the node is Ready
kubectl get nodes -o wide

# Verify Cilium agent is Running on all nodes
kubectl -n kube-system get pods -l k8s-app=cilium -o wide

# Check architectures
kubectl get nodes -o custom-columns=NAME:.metadata.name,ARCH:.status.nodeInfo.architecture,OS:.status.nodeInfo.operatingSystem

# Verify L2 Announcement (LoadBalancer IPs should respond)
kubectl get svc -A | grep LoadBalancer
```

---

## Phase 2: Workload Optimization (nodeAffinity)

The M4 Neo significantly exceeds the RPi 5 in both CPU and memory, so resource-intensive workloads should be preferentially scheduled there.

### 2.1 Node Label Design

Assign custom labels to the M4 Neo:

```bash
kubectl label node m4neo hardware-class=high-performance
```

Also label the existing RPi nodes (for explicit classification):

```bash
kubectl label node worker1 hardware-class=raspberry-pi
kubectl label node worker2 hardware-class=raspberry-pi
```

### 2.2 nodeAffinity Strategy

Use **preferredDuringSchedulingIgnoredDuringExecution** (preferred scheduling).

- **Preferentially** schedule to M4 Neo when available
- Falls back to RPi nodes if M4 Neo is down (maintaining availability)
- Avoid using `required` as it would prevent workloads from starting during node failure

<!-- TODO(human): Determine weight values and component groupings for nodeAffinity -->

```yaml
affinity:
  nodeAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: __WEIGHT__
        preference:
          matchExpressions:
            - key: hardware-class
              operator: In
              values:
                - high-performance
```

### 2.3 Adding affinity to Each Component's values.yaml

Components to add nodeAffinity to:

| Component | File | How to Add |
|--------------|---------|---------|
| Prometheus | `infrastructure/observability/prometheus/values.yaml` | Add to `prometheus.prometheusSpec.affinity` |
| Grafana | `infrastructure/observability/grafana/values.yaml` | Add top-level `affinity:` block |
| Tempo | `infrastructure/observability/tempo/values.yaml` | Replace existing `affinity: {}` |
| Loki | `infrastructure/observability/loki/values.yaml` | Add to `singleBinary.affinity` |
| OTel Collector | `infrastructure/observability/otel-collector/values.yaml` | Replace existing `affinity: {}` |

#### Prometheus

```yaml
# infrastructure/observability/prometheus/values.yaml
# Add to prometheus.prometheusSpec:
prometheus:
  prometheusSpec:
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

#### Grafana

```yaml
# infrastructure/observability/grafana/values.yaml
# Add at top level:
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

#### Tempo / OTel Collector

```yaml
# infrastructure/observability/tempo/values.yaml
# infrastructure/observability/otel-collector/values.yaml
# Replace existing affinity: {} with:
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

#### Loki

```yaml
# infrastructure/observability/loki/values.yaml
# Add to singleBinary section:
singleBinary:
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

### 2.4 Patching Manifest-Managed Components

For components managed via flat manifests (no kustomize overlay), edit the `affinity` block directly in the Deployment / StatefulSet manifest.

#### Keycloak

Edit `affinity` in `infrastructure/security/keycloak/keycloak-deployment.yaml`:

```yaml
spec:
  template:
    spec:
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

> A similar block can be added to `postgresql-statefulset.yaml`, but for databases it may be safer to pin to the current node from a data locality perspective. Make this decision in conjunction with PV placement.

#### Backstage

Edit the `affinity` block directly in `backstage/manifests/backstage-deployment.yaml`.

### 2.5 Applying Changes and Verification

```bash
# Commit & push all changes (Argo CD auto-syncs)
git add -A
git commit -m "nodeAffinity: prefer scheduling Observability workloads to M4 Neo"
git push

# Check Pod placement
kubectl get pods -n monitoring -o wide

# Check scheduling events for a specific Pod
kubectl describe pod <pod-name> -n monitoring | grep -A5 "Events:"
```

---

## Phase 3: Multi-Architecture Image Build Support

### 3.1 Background

The existing cluster was ARM64-only (Raspberry Pi 5), so all custom images were built for ARM64.
With the M4 Neo (AMD64) joining, ARM64 images **cannot run** on AMD64 nodes (`exec format error`).

```
standard_init_linux.go: exec user process caused: exec format error
```

Two solutions:

1. **Multi-platform images**: A single image tag supports both architectures (recommended)
2. **nodeSelector pinning**: Schedule only to specific architecture nodes (workaround)

Phase 3 recommends multi-platform image creation.

### 3.2 Custom Image Inventory

| Image | Base Image | Language | Dockerfile |
|---------|-------------|------|-----------|
| kensan-user | golang -> alpine | Go | `apps/kensan/backend/services/user/Dockerfile` |
| kensan-task | golang -> alpine | Go | `apps/kensan/backend/services/task/Dockerfile` |
| kensan-timeblock | golang -> alpine | Go | `apps/kensan/backend/services/timeblock/Dockerfile` |
| kensan-analytics | golang -> alpine | Go | `apps/kensan/backend/services/analytics/Dockerfile` |
| kensan-memo | golang -> alpine | Go | `apps/kensan/backend/services/memo/Dockerfile` |
| kensan-note | golang -> alpine | Go | `apps/kensan/backend/services/note/Dockerfile` |
| kensan-frontend | node:22-alpine | Node.js | `apps/kensan/frontend/Dockerfile` |
| kensan-ai | python:3.12-slim | Python | `apps/kensan/kensan-ai/Dockerfile` |
| backstage | node:22-bookworm-slim | Node.js | `backstage/app/packages/backend/Dockerfile` |

### 3.3 Go Service Dockerfile Changes

Go has cross-compilation built in as a standard feature, making it the easiest to adapt.

**Before:**

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./services/user/cmd

FROM alpine:3.19
COPY --from=builder /app/server .
```

**After:**

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETARCH

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-w -s" -o /app/server ./services/user/cmd

FROM alpine:3.19
COPY --from=builder /app/server .
```

Key points:
- `--platform=$BUILDPLATFORM`: Build stage runs on the host architecture (fast)
- `ARG TARGETARCH`: Target architecture is automatically injected (`amd64` or `arm64`)
- `GOARCH=$TARGETARCH`: Uses Go's cross-compilation

### 3.4 Node.js / Python App Strategy

Node.js and Python are interpreted languages, so they work as long as the base image architecture matches.
No special Dockerfile changes are needed -- just specify `--platform` at build time.

However, if **native extensions** (modules built with `node-gyp`, Python C extensions, etc.) are included, build stages must be executed on each architecture.

- **kensan-frontend**: Pure Node.js app. No changes needed.
- **kensan-ai**: Python package. Watch for native extensions with `uv pip install`.
- **backstage**: Has `node-gyp` dependency (`isolated-vm`). `npm install` must run on each architecture.

### 3.5 Podman Multi-Platform Build Procedure

This platform uses Podman for image builds.

#### Prerequisites: QEMU Emulation

Building ARM64 images on an AMD64 machine (or vice versa) requires QEMU user-mode emulation.

```bash
# Install on Ubuntu
sudo apt-get install -y qemu-user-static

# Verify
ls /proc/sys/fs/binfmt_misc/qemu-*
```

#### Multi-Platform Build

```bash
# Create manifest list and build
podman build --platform linux/amd64,linux/arm64 \
  --manifest ghcr.io/<your-git-org>/kensan-user:v0.1.0 \
  -f apps/kensan/backend/services/user/Dockerfile \
  apps/kensan/backend/

# Push the manifest list
podman manifest push ghcr.io/<your-git-org>/kensan-user:v0.1.0 \
  docker://ghcr.io/<your-git-org>/kensan-user:v0.1.0
```

#### Batch Build Example for All Services

```bash
REGISTRY="ghcr.io/<your-git-org>"
TAG="v0.1.0"
PLATFORMS="linux/amd64,linux/arm64"

# Go services
for svc in user task timeblock analytics memo note; do
  podman build --platform $PLATFORMS \
    --manifest $REGISTRY/kensan-$svc:$TAG \
    -f apps/kensan/backend/services/$svc/Dockerfile \
    apps/kensan/backend/
  podman manifest push $REGISTRY/kensan-$svc:$TAG \
    docker://$REGISTRY/kensan-$svc:$TAG
done

# Frontend
podman build --platform $PLATFORMS \
  --manifest $REGISTRY/kensan-frontend:$TAG \
  -f apps/kensan/frontend/Dockerfile \
  apps/kensan/frontend/

podman manifest push $REGISTRY/kensan-frontend:$TAG \
  docker://$REGISTRY/kensan-frontend:$TAG
```

### 3.6 GitHub Actions Sample (Future Reference)

Example GitHub Actions for multi-platform builds in CI/CD:

```yaml
# .github/workflows/build-multiarch.yaml
name: Build Multi-Arch Image

on:
  push:
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-qemu-action@v3

      - uses: docker/setup-buildx-action@v3

      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - uses: docker/build-push-action@v6
        with:
          context: apps/kensan/backend/
          file: apps/kensan/backend/services/user/Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ghcr.io/${{ github.repository_owner }}/kensan-user:${{ github.ref_name }}
```

### 3.7 Migration Checklist

- [ ] Add `--platform=$BUILDPLATFORM` / `TARGETARCH` to Go service (6) Dockerfiles
- [ ] Test multi-platform build for kensan-frontend
- [ ] Test multi-platform build for kensan-ai (check for native extensions)
- [ ] Test multi-platform build for backstage (check `isolated-vm`)
- [ ] Rebuild and push all images as multi-platform
- [ ] Verify Pods are scheduled to AMD64 nodes on the cluster
- [ ] Verify no `exec format error` occurs with `kubectl describe pod`

---

## Troubleshooting

### Node Stuck in NotReady

```bash
# Check kubelet logs
sudo journalctl -u kubelet -f --no-pager

# Check Cilium agent status
kubectl -n kube-system get pods -l k8s-app=cilium -o wide
kubectl -n kube-system logs -l k8s-app=cilium --tail=50
```

Common causes:
- CRI-O not running -> `sudo systemctl status crio`
- Cilium agent crashing -> Cilium's `devices` setting doesn't match M4 Neo's WiFi interface name (`wlp3s0`)
- Swap enabled -> Disable with `sudo swapoff -a`

### Cilium Agent Startup Failure

```bash
# Check Cilium status
kubectl -n kube-system exec -it ds/cilium -- cilium status

# Check device recognition
kubectl -n kube-system exec -it ds/cilium -- cilium status --verbose | grep "Devices"
```

If the `devices` setting doesn't match M4 Neo's WiFi interface name (`wlp3s0`), the Cilium agent will fail to start.
Check the actual interface name with `ip a` and fix the `devices` pattern in `values.yaml`.

### exec format error

```
standard_init_linux.go: exec user process caused: exec format error
```

Occurs when an ARM64 image runs on an AMD64 node.

```bash
# Check image architecture
podman inspect <image> | grep Architecture

# Check which node the Pod is running on
kubectl get pod <pod-name> -o wide

# Check the manifest list
podman manifest inspect ghcr.io/<your-git-org>/<image>:<tag>
```

Fixes:
1. Complete the Phase 3 multi-platform image build
2. As a temporary workaround, pin to ARM64 nodes with `nodeSelector`:

```yaml
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/arch: arm64
```

### L2 Announcement Not Working on M4 Neo

```bash
# Check L2 Policy status
kubectl get ciliuml2announcementpolicies -o yaml

# Check Cilium leases for the node
kubectl get lease -n kube-system | grep cilium
```

Verify that the `CiliumL2AnnouncementPolicy`'s `interfaces` includes M4 Neo's WiFi interface name (`wlp3s0`).
Confirm the regex pattern (`^wlan.*`, `^wlp.*`) was applied as described in Phase 1.8.

### PersistentVolume Issues

`local-path-provisioner` creates volumes locally on nodes, so Pods cannot access PVs when migrating between nodes.

```bash
# Check PV node affinity
kubectl get pv -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeAffinity.required.nodeSelectorTerms[0].matchExpressions[0].values[0]
```

Fixes:
- StatefulSets (Prometheus, Loki, Tempo, PostgreSQL) are automatically scheduled to the node with their PV
- If nodeAffinity moves workloads to M4 Neo, existing PVs remain on the RPi. If data migration is needed, delete and recreate the PVC (be aware of data loss)
