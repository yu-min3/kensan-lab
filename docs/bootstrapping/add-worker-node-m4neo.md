# Bosgame M4 Neo ワーカーノード追加手順

## 概要

Bosgame M4 Neo（AMD64）を既存の Raspberry Pi 5（ARM64）クラスタにワーカーノードとして追加する手順書。
amdgpu カーネルモジュールの不安定さを回避するため、Ubuntu Server にクリーンインストールしヘッドレスワーカーとして運用する。

ARM64 + AMD64 のマルチアーキテクチャクラスタとなるため、Phase 1（ノード参加）→ Phase 2（ワークロード最適化）→ Phase 3（マルチアーキテクチャイメージ対応）の3段階で構成する。

## ノード構成

| ノード | ホスト名 | IP | ハードウェア | アーキテクチャ | 役割 |
|--------|----------|----|-------------|--------------|------|
| Master | master | 192.168.1.107 | Raspberry Pi 5 (8GB) | ARM64 | control-plane |
| Worker1 | worker1 | 192.168.1.108 | Raspberry Pi 5 (8GB) | ARM64 | worker |
| Worker2 | worker2 | 192.168.1.109 | Raspberry Pi 5 (8GB) | ARM64 | worker |
| **M4 Neo** | **m4neo** | **192.168.1.110** | **Bosgame M4 Neo (Ryzen 7 7840HS, 32GB DDR5)** | **AMD64** | **worker** |

---

## Phase 1: ノードセットアップとクラスター参加

> **セットアップスクリプト**: Phase 1 の手順は `temp/setup-m4neo.sh` にまとめてあります。
> M4 Neo 上で実行してください。

### 1.1 Ubuntu Server 24.04 LTS インストール

USB ブートメディアから Ubuntu Server 24.04 LTS を最小構成でインストールする。

- **Profile**: minimal（最小インストール）
- **追加パッケージ**: OpenSSH Server のみ選択
- **ユーザー名**: 任意（以降 `yu` として記載）
- **ディスク**: 内蔵 NVMe 全体を使用（LVM 推奨）

> **注意**: Desktop 版ではなく Server 版を使用すること。amdgpu ドライバの不安定さを根本的に回避する。

### 1.2 amdgpu カーネルモジュール無効化

Ubuntu Server でも amdgpu モジュールがロードされる場合がある。ヘッドレス運用のため完全に無効化する。

```bash
# /etc/modprobe.d/blacklist-amdgpu.conf を作成
sudo tee /etc/modprobe.d/blacklist-amdgpu.conf <<'EOF'
# Disable amdgpu for headless worker node stability
blacklist amdgpu
blacklist drm_kms_helper
EOF

sudo update-initramfs -u
sudo reboot
```

再起動後に確認:

```bash
lsmod | grep amdgpu
# 出力なし = 成功
```

### 1.3 ネットワーク設定

netplan で静的 IP を設定する。M4 Neo は WiFi（`wlp3s0`）を使用する（有線ポート `eno1`/`enp4s0` は未接続）。

```yaml
# /etc/netplan/01-static.yaml
network:
  version: 2
  wifis:
    wlp3s0:        # 実際のインターフェース名に合わせる（ip a で確認）
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

> **インターフェース名の確認**: `ip a` で実際のインターフェース名（`wlp3s0` 等）を確認し、netplan と後続の Cilium 設定に反映すること。RPi は `wlan0`、M4 Neo は `wlp3s0` を使用している。

ホスト名の設定:

```bash
sudo hostnamectl set-hostname m4neo
```

### 1.4 カーネルパラメータ

Kubernetes が必要とするカーネルモジュールとパラメータを設定する。

```bash
# カーネルモジュールの永続化
cat <<'EOF' | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

# sysctl パラメータ
cat <<'EOF' | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system
```

swap の無効化:

```bash
sudo swapoff -a
# /etc/fstab から swap エントリを削除またはコメントアウト
sudo sed -i '/\sswap\s/s/^/#/' /etc/fstab
```

### 1.5 CRI-O インストール

CRI-O は既存クラスタと同じバージョンを使用する。master で `crio --version` を確認してからインストールすること。

```bash
# CRI-O のリポジトリ追加（v1.31 系の例）
CRIO_VERSION="v1.31"

curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/stable:/$CRIO_VERSION/deb/Release.key | \
  sudo gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/stable:/$CRIO_VERSION/deb/ /" | \
  sudo tee /etc/apt/sources.list.d/cri-o.list

sudo apt-get update
sudo apt-get install -y cri-o

sudo systemctl enable --now crio
```

### 1.6 kubeadm / kubelet / kubectl インストール

既存クラスタと同じ Kubernetes バージョンを使用する。master で `kubectl version --short` を確認すること。

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

**Master ノードで** join トークンを生成する:

```bash
# Master で実行
kubeadm token create --print-join-command
```

出力されたコマンドを **M4 Neo で** 実行する:

```bash
# M4 Neo で実行（出力されたコマンドをそのまま貼り付け）
sudo kubeadm join 192.168.1.107:6443 --token <token> --discovery-token-ca-cert-hash sha256:<hash>
```

### 1.8 Cilium 設定変更（重要）

既存の Cilium 設定は `wlan0`（Raspberry Pi の WiFi インターフェース）のみを対象としている。
M4 Neo は WiFi `wlp3s0` を使用するため、**クラスタ全体**の Cilium 設定を変更して両方のインターフェース名にマッチさせる必要がある。

#### `infrastructure/network/cilium/values.yaml` の変更

```yaml
# 変更前:
devices: wlan0

# 変更後（auto-detect に変更し、全ノードのインターフェースを自動認識）:
devices: ""   # auto-detect（wlan0, wlp3s0 等を自動認識）
```

> 空文字列（auto-detect）により、RPi の `wlan0` と M4 Neo の `wlp3s0` の両方が自動的に認識される。
> Cilium の `devices` は Linux のデバイス名ワイルドカードも受け付けるが、auto-detect が推奨。

#### `infrastructure/network/cilium/resources/lb-ippool.yaml` の変更

```yaml
# 変更前:
spec:
  interfaces:
  - wlan0

# 変更後（正規表現パターン）:
spec:
  interfaces:
  - "^wlan.*"
  - "^wlp.*"
```

> CiliumL2AnnouncementPolicy の `interfaces` フィールドは正規表現を受け付ける。
> `^wlan.*` は RPi の `wlan0` に、`^wlp.*` は M4 Neo の `wlp3s0` にマッチする。

#### 変更の適用

```bash
# values.yaml を変更後、Git に push すれば Argo CD が自動 sync する
git add infrastructure/network/cilium/values.yaml
git add infrastructure/network/cilium/resources/lb-ippool.yaml
git commit -m "Cilium: マルチインターフェース対応（wlan + wlp）"
git push

# Cilium agent の再起動を確認
kubectl -n kube-system rollout status daemonset/cilium
```

### 1.9 参加確認

```bash
# ノードが Ready になっていること
kubectl get nodes -o wide

# Cilium agent が全ノードで Running であること
kubectl -n kube-system get pods -l k8s-app=cilium -o wide

# アーキテクチャの確認
kubectl get nodes -o custom-columns=NAME:.metadata.name,ARCH:.status.nodeInfo.architecture,OS:.status.nodeInfo.operatingSystem

# L2 Announcement の確認（LoadBalancer IP が応答すること）
kubectl get svc -A | grep LoadBalancer
```

---

## Phase 2: ワークロード最適化（nodeAffinity）

M4 Neo は RPi 5 と比較して CPU・メモリ共に大幅に上回るため、リソース消費の大きいワークロードを優先的にスケジュールする。

### 2.1 ノードラベル設計

M4 Neo にカスタムラベルを付与する:

```bash
kubectl label node m4neo hardware-class=high-performance
```

既存の RPi ノードにもラベルを付けておく（明示的な分類のため）:

```bash
kubectl label node worker1 hardware-class=raspberry-pi
kubectl label node worker2 hardware-class=raspberry-pi
```

### 2.2 nodeAffinity 戦略

**preferredDuringSchedulingIgnoredDuringExecution**（推奨スケジューリング）を使用する。

- M4 Neo が利用可能なら**優先的に**スケジュールする
- M4 Neo がダウンしていても RPi にフォールバックする（可用性を維持）
- `required` にするとノード障害時にワークロードが起動できなくなるため避ける

<!-- TODO(human): nodeAffinity の weight 値と対象コンポーネントのグループ分け -->

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

### 2.3 各コンポーネントの values.yaml への affinity 追記

nodeAffinity を追記する対象コンポーネント一覧:

| コンポーネント | ファイル | 追記方法 |
|--------------|---------|---------|
| Prometheus | `infrastructure/observability/prometheus/values.yaml` | `prometheus.prometheusSpec.affinity` に追記 |
| Grafana | `infrastructure/observability/grafana/values.yaml` | トップレベルに `affinity:` ブロック追加 |
| Tempo | `infrastructure/observability/tempo/values.yaml` | 既存の `affinity: {}` を書き換え |
| Loki | `infrastructure/observability/loki/values.yaml` | `singleBinary.affinity` に追記 |
| OTel Collector | `infrastructure/observability/otel-collector/values.yaml` | 既存の `affinity: {}` を書き換え |

#### Prometheus

```yaml
# infrastructure/observability/prometheus/values.yaml
# prometheus.prometheusSpec に追記:
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
# トップレベルに追加:
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
# 既存の affinity: {} を以下に書き換え:
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
# singleBinary セクションに追記:
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

### 2.4 Kustomize 管理コンポーネントのパッチ方針

Keycloak と Backstage は Kustomize で管理されているため、overlay に strategic merge patch を追加する。

#### Keycloak（prod/dev 各 overlay）

`infrastructure/security/keycloak/overlays/prod/affinity-patch.yaml` を作成:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: keycloak
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

`kustomization.yaml` の `patches:` に追加:

```yaml
patches:
  - path: affinity-patch.yaml
```

> PostgreSQL StatefulSet にも同様のパッチを適用可能だが、DB はデータローカリティの観点から現在のノードに固定する方が安全な場合もある。PV の配置と合わせて判断すること。

#### Backstage

Backstage も同様の手法で `backstage/manifests/overlays/prod/` にパッチを追加する。

### 2.5 変更の適用と確認

```bash
# 全変更を commit & push（Argo CD が自動 sync）
git add -A
git commit -m "nodeAffinity: M4 Neo へ Observability ワークロードを優先スケジュール"
git push

# Pod の配置確認
kubectl get pods -n monitoring -o wide

# 特定 Pod のスケジューリングイベント確認
kubectl describe pod <pod-name> -n monitoring | grep -A5 "Events:"
```

---

## Phase 3: マルチアーキテクチャイメージビルド対応

### 3.1 背景

既存クラスタは ARM64（Raspberry Pi 5）のみだったため、カスタムイメージは全て ARM64 用にビルドされている。
M4 Neo（AMD64）が加わると、ARM64 イメージは AMD64 ノードでは **実行できない**（`exec format error`）。

```
standard_init_linux.go: exec user process caused: exec format error
```

解決策は 2 つ:

1. **マルチプラットフォームイメージ**: 1 つのイメージタグで両アーキテクチャに対応（推奨）
2. **nodeSelector でピン止め**: 特定のアーキテクチャノードにのみスケジュール（回避策）

Phase 3 ではマルチプラットフォームイメージ化を推奨する。

### 3.2 対象カスタムイメージ一覧

| イメージ | ベースイメージ | 言語 | Dockerfile |
|---------|-------------|------|-----------|
| kensan-user | golang → alpine | Go | `apps/kensan/backend/services/user/Dockerfile` |
| kensan-task | golang → alpine | Go | `apps/kensan/backend/services/task/Dockerfile` |
| kensan-timeblock | golang → alpine | Go | `apps/kensan/backend/services/timeblock/Dockerfile` |
| kensan-analytics | golang → alpine | Go | `apps/kensan/backend/services/analytics/Dockerfile` |
| kensan-memo | golang → alpine | Go | `apps/kensan/backend/services/memo/Dockerfile` |
| kensan-note | golang → alpine | Go | `apps/kensan/backend/services/note/Dockerfile` |
| kensan-frontend | node:22-alpine | Node.js | `apps/kensan/frontend/Dockerfile` |
| kensan-ai | python:3.12-slim | Python | `apps/kensan/kensan-ai/Dockerfile` |
| backstage | node:22-bookworm-slim | Node.js | `backstage/app/packages/backend/Dockerfile` |

### 3.3 Go サービスの Dockerfile 変更

Go はクロスコンパイルが標準機能として組み込まれているため、最も対応が容易。

**変更前:**

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./services/user/cmd

FROM alpine:3.19
COPY --from=builder /app/server .
```

**変更後:**

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETARCH

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-w -s" -o /app/server ./services/user/cmd

FROM alpine:3.19
COPY --from=builder /app/server .
```

ポイント:
- `--platform=$BUILDPLATFORM`: ビルドステージはホストのアーキテクチャで実行（高速）
- `ARG TARGETARCH`: ターゲットアーキテクチャが自動注入される（`amd64` or `arm64`）
- `GOARCH=$TARGETARCH`: Go のクロスコンパイルを利用

### 3.4 Node.js / Python アプリの対応方針

Node.js と Python はインタプリタ言語のため、ベースイメージのアーキテクチャが一致すれば動作する。
特別な Dockerfile 変更は不要で、ビルド時に `--platform` を指定するだけでよい。

ただし、**ネイティブ拡張**（`node-gyp` でビルドされるモジュール、Python の C 拡張等）を含む場合は、各アーキテクチャでビルドステージを実行する必要がある。

- **kensan-frontend**: 純粋な Node.js アプリ。変更不要。
- **kensan-ai**: Python パッケージ。`uv pip install` でネイティブ拡張がある場合は注意。
- **backstage**: `node-gyp` 依存あり（`isolated-vm`）。各アーキテクチャで `npm install` が必要。

### 3.5 Podman マルチプラットフォームビルド手順

このプラットフォームでは Podman を使用してイメージをビルドする。

#### 事前準備: QEMU エミュレーション

AMD64 マシン上で ARM64 イメージをビルドする（またはその逆）には、QEMU のユーザーモードエミュレーションが必要。

```bash
# Ubuntu にインストール
sudo apt-get install -y qemu-user-static

# 確認
ls /proc/sys/fs/binfmt_misc/qemu-*
```

#### マルチプラットフォームビルド

```bash
# マニフェストリストを作成してビルド
podman build --platform linux/amd64,linux/arm64 \
  --manifest ghcr.io/<your-git-org>/kensan-user:v0.1.0 \
  -f apps/kensan/backend/services/user/Dockerfile \
  apps/kensan/backend/

# マニフェストリストを push
podman manifest push ghcr.io/<your-git-org>/kensan-user:v0.1.0 \
  docker://ghcr.io/<your-git-org>/kensan-user:v0.1.0
```

#### 全サービスの一括ビルド例

```bash
REGISTRY="ghcr.io/<your-git-org>"
TAG="v0.1.0"
PLATFORMS="linux/amd64,linux/arm64"

# Go サービス
for svc in user task timeblock analytics memo note; do
  podman build --platform $PLATFORMS \
    --manifest $REGISTRY/kensan-$svc:$TAG \
    -f apps/kensan/backend/services/$svc/Dockerfile \
    apps/kensan/backend/
  podman manifest push $REGISTRY/kensan-$svc:$TAG \
    docker://$REGISTRY/kensan-$svc:$TAG
done

# フロントエンド
podman build --platform $PLATFORMS \
  --manifest $REGISTRY/kensan-frontend:$TAG \
  -f apps/kensan/frontend/Dockerfile \
  apps/kensan/frontend/

podman manifest push $REGISTRY/kensan-frontend:$TAG \
  docker://$REGISTRY/kensan-frontend:$TAG
```

### 3.6 GitHub Actions サンプル（将来参考）

CI/CD でマルチプラットフォームビルドを行う場合の GitHub Actions 例:

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

### 3.7 移行チェックリスト

- [ ] Go サービス（6 個）の Dockerfile に `--platform=$BUILDPLATFORM` / `TARGETARCH` を追加
- [ ] kensan-frontend のマルチプラットフォームビルドをテスト
- [ ] kensan-ai のマルチプラットフォームビルドをテスト（ネイティブ拡張の確認）
- [ ] backstage のマルチプラットフォームビルドをテスト（`isolated-vm` の確認）
- [ ] 全イメージをマルチプラットフォームで再ビルド & push
- [ ] クラスタ上で AMD64 ノードに Pod がスケジュールされることを確認
- [ ] `kubectl describe pod` で `exec format error` が発生しないことを確認

---

## トラブルシューティング

### ノードが NotReady のまま

```bash
# kubelet のログを確認
sudo journalctl -u kubelet -f --no-pager

# Cilium agent の状態を確認
kubectl -n kube-system get pods -l k8s-app=cilium -o wide
kubectl -n kube-system logs -l k8s-app=cilium --tail=50
```

よくある原因:
- CRI-O が起動していない → `sudo systemctl status crio`
- Cilium agent がクラッシュ → Cilium の `devices` 設定が M4 Neo の WiFi インターフェース名（`wlp3s0`）に合っていない
- swap が有効 → `sudo swapoff -a` で無効化

### Cilium agent 起動失敗

```bash
# Cilium のステータス確認
kubectl -n kube-system exec -it ds/cilium -- cilium status

# デバイス認識の確認
kubectl -n kube-system exec -it ds/cilium -- cilium status --verbose | grep "Devices"
```

`devices` の設定が M4 Neo の WiFi インターフェース名（`wlp3s0`）にマッチしない場合、Cilium agent が起動に失敗する。
`ip a` で実際のインターフェース名を確認し、`values.yaml` の `devices` パターンを修正すること。

### exec format error

```
standard_init_linux.go: exec user process caused: exec format error
```

AMD64 ノード上で ARM64 イメージが実行された場合に発生する。

```bash
# イメージのアーキテクチャ確認
podman inspect <image> | grep Architecture

# Pod がどのノードで動いているか確認
kubectl get pod <pod-name> -o wide

# マニフェストリストの確認
podman manifest inspect ghcr.io/<your-git-org>/<image>:<tag>
```

対処:
1. Phase 3 のマルチプラットフォームイメージビルドを完了させる
2. 暫定対応として `nodeSelector` で ARM64 ノードにピン止めする:

```yaml
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/arch: arm64
```

### L2 Announcement が M4 Neo で機能しない

```bash
# L2 Policy の状態確認
kubectl get ciliuml2announcementpolicies -o yaml

# ノードの Cilium リース確認
kubectl get lease -n kube-system | grep cilium
```

`CiliumL2AnnouncementPolicy` の `interfaces` に M4 Neo の WiFi インターフェース名（`wlp3s0`）が含まれていることを確認する。
Phase 1.8 の手順で正規表現パターン（`^wlan.*`, `^wlp.*`）に変更済みであることを確認。

### PersistentVolume 問題

`local-path-provisioner` はノードローカルにボリュームを作成するため、Pod がノード間を移動すると PV にアクセスできなくなる。

```bash
# PV のノードアフィニティ確認
kubectl get pv -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeAffinity.required.nodeSelectorTerms[0].matchExpressions[0].values[0]
```

対処:
- StatefulSet（Prometheus, Loki, Tempo, PostgreSQL）は PV のあるノードに自動的にスケジュールされる
- nodeAffinity で M4 Neo に移動させた場合、既存の PV は RPi 上に残る。データ移行が必要になる場合は PVC を削除して再作成する（データロス注意）
