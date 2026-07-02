---
description: Cilium L2, LoadBalancer IPs, Istio Gateways, domain architecture, and certificate mapping
globs: "kubernetes/network/**, **/httproute*, **/gateway*"
---

# Network & Ingress

## Edge Architecture

外部からクラスタへの入口は 2 経路。両方とも最終的に Istio Gateway に集約される。

| 経路 | 用途 | 入口 |
|---|---|---|
| **Cloudflare Tunnel** | インターネット公開（Cloudflare edge 経由、Public DNS） | `cloudflared` Deployment（`cloudflare-tunnel` ns）→ Istio Gateway |
| **Cilium L2 LoadBalancer** | LAN 内アクセス / Cloudflare 経由したくない管理系 | LB VIP（192.168.0.242/.243）→ Istio Gateway |

Cloudflare Tunnel の token は ESO 経由（`external-secret.yaml`、Vault path `secret/cloudflare-tunnel`）で配布。Tunnel は外向きにしか接続しないので NodePort や FW 開放は不要。

## Cilium LoadBalancer

- **L2 Announcements** enabled on wired + WiFi fallback: regex `^eth.*`, `^en.*`, `^wlan.*`, `^wlp.*`
- **IP Pool**: 192.168.0.240–249 (must not overlap DHCP)
- **Devices**: auto-detect (covers wired `eth0` / `eno1` / `enp4s0` and WiFi fallback `wlan0` / `wlp3s0`)
- kube-proxy replacement enabled
- 2026-05-07 以降は有線メイン運用（有線 metric 100、WiFi metric 600 で fallback）

## LoadBalancer IP Assignments

| Gateway | IP |
|---------|----|
| gateway-platform | 192.168.0.242 |
| gateway-prod | 192.168.0.243 |
| syncthing-sync (app-kensan) | 192.168.0.245 |

## Gateway Architecture

- `gateway-platform` (192.168.0.242) → `*.platform.yu-min3.com` / `wildcard-platform-tls` — platform UI (Backstage, ArgoCD, Grafana, Vault 等)
- `gateway-prod` (192.168.0.243) → `*.app.yu-min3.com` / `wildcard-apps-tls` — user app

**注意 — Gateway 外の入口**: `syncthing-sync`（.245、`app-kensan` ns）は workspace（日記含む生活データ）を Mac ⇄ クラスタで双方向同期する LB VIP。TCP 22000 のみ公開し、Syncthing 自前の TLS + device 認証で保護される（旧 NFS/AUTH_SYS の裸入口は廃止）。global discovery / relay / NAT traversal は init container の config-guard で起動毎に無効化され、LAN-only 不変条件を Git 側から強制する。GUI/REST(8384) は NetworkPolicy `syncthing-guard` でクラスタ内から遮断（設定操作は `kubectl exec` のみ）。Gateway 層 OIDC（ADR-010）の外にあるが、device ペアリング済みのホストのみ接続できる。事実: `kubernetes/apps/app-kensan/resources/syncthing.yaml` / `networkpolicy-syncthing.yaml`。

ホスト一覧と全体図: [`kubernetes/network/README.md`](../../kubernetes/network/README.md) / [`docs/architecture/network.md`](../../docs/architecture/network.md)

## Gateway-level Authentication

Istio Gateway は `oauth2-proxy`（`auth-system` ns）を **ext_authz** として参照し、Gateway 層で OIDC 認証を完了させる（実装方針は ADR-010）。アプリ側は認証済みヘッダを信頼してよい。

## HTTPRoute Convention

Each component's HTTPRoute is placed in its `resources/` directory and attaches to the appropriate Gateway via `parentRefs`.

## Known Issue

Cilium L2 lease renewal can timeout on WiFi (`context deadline exceeded`), causing LoadBalancer IPs to become unreachable. Fix: restart Cilium daemonset.
