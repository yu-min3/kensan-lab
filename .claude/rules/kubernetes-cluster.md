---
description: Cluster topology, node labels, scheduling rules, and storage constraints
globs: "kubernetes/**"
---

# Kubernetes Cluster

## Node Inventory

各ノードは **有線メイン + WiFi fallback** の 2 系統構成（routing metric ベース、有線=100 / WiFi=600）。

| Node | Role | Arch | hardware-class | Storage | Wired (metric 100) | WiFi fallback (metric 600) |
|------|------|------|---------------|---------|--------------------|----------------------------|
| master | control-plane | arm64 | — | microSD | eth0 (192.168.0.107) | wlan0 (192.168.0.207) |
| worker1 | worker | arm64 | raspberry-pi | microSD | eth0 (192.168.0.108) | wlan0 (192.168.0.208) |
| worker2 | worker | arm64 | raspberry-pi | microSD | eth0 (192.168.0.109) | wlan0 (192.168.0.209) |
| m4neo | worker | amd64 | high-performance | NVMe PCIe 4.0 | eno1 (192.168.0.110) | wlp3s0 (192.168.0.210) |

- **Runtime**: CRI-O via kubeadm
- **Master IP**: 192.168.0.107, **Pod CIDR**: 10.244.0.0/16
- **Switch**: TP-Link TL-SG116E (16-port 1GbE Easy Smart, fanless)
- **Cabling**: 2026-05-07 wired migration. Static IP via netplan (not DHCP reservation). m4neo の `enp4s0` は予備で未設定。

## Label Axes

| Label | Purpose | Values |
|-------|---------|--------|
| `kubernetes.io/arch` | Architecture (auto-assigned) | `amd64` / `arm64` |
| `hardware-class` | I/O performance (manual) | `high-performance` / `raspberry-pi` |

## Scheduling Rules

| Category | Strategy | Workloads |
|----------|----------|-----------|
| I/O Heavy | `requiredDuringScheduling: hardware-class=high-performance` | Prometheus, Loki, Tempo, Keycloak+PostgreSQL |
| Medium | `preferredDuringScheduling: hardware-class=high-performance` (weight: 80) | OTel Collector |
| Light | No affinity | Grafana, Hubble UI |
| AMD64-only | `required: kubernetes.io/arch=amd64` | kensan, Backstage |

## Storage

| Class | Provisioner | 用途 |
|---|---|---|
| `longhorn`（default） | Longhorn | 通常の StatefulSet / アプリ PVC。レプリケート（複数ノードに block レベルで複製）するため、ノード障害でデータ消失しない |
| `local-path` | local-path-provisioner | レガシー / Longhorn 移行未完の PVC のみ。**node-local** で、StatefulSet を別ノードに移すと PVC 再作成 + データ消失 |

新規ワークロードは原則 `longhorn` を使う（Longhorn 移行は commit 45a6a61 で完了）。`local-path` は段階的に廃止予定。

## Adding New Nodes

1. Install CRI-O + kubeadm
2. Join cluster: `kubeadm join`
3. Assign label: `kubectl label node <name> hardware-class=<value>`
4. Cilium auto-detects network interface (no explicit `devices` config needed)
