# network

クラスタの通信基盤 — CNI・Service Mesh・外部公開・NetworkPolicy・DNS を束ねる。

**設計思想**: データプレーンを 2 つのエンジンで分担する。**Cilium が L3/4（eBPF datapath・到達性の基底）**、**Istio が L7（identity・入口）**。両者を **zero-trust が縦串で束ねる** — 到達性は Cilium の default-deny、identity は Istio の mTLS、入口は Gateway 認証。「層を混ぜない・部品を増やさない・既定は拒否」が貫く原則。

## 構成

| dir | 役割 | 主な ns |
|---|---|---|
| `cilium/` | CNI + kube-proxy replacement (eBPF) + L2 LoadBalancer (`192.168.0.240-249`) | kube-system |
| `istio/` | Service Mesh (base / cni / istiod) + Gateway + PeerAuthentication | istio-system |
| `gateway-api/` | Gateway API CRDs | (cluster) |
| `cloudflare-tunnel/` | 外部公開用 Cloudflare Tunnel（outbound-only、ポート開放不要） | cloudflare-tunnel |
| `coredns/` | cluster DNS + custom domain resolution | kube-system |
| `network-policy/` | NetworkPolicy + CiliumClusterwideNetworkPolicy を ns 横断で集約 | (cluster) |

## エンジンの役割分担（2 エンジン × Zero-Trust）

下段 = Cilium（L3/4）、上段 = Istio（L7）。zero-trust は両層から 3 つのコントロールを束ねる縦串。

```mermaid
flowchart TB
    subgraph ISTIO["L7 ─ Istio mesh ｜ identity と入口"]
        GW["Istio Gateway<br/>TLS 終端 / 入口認証 (ext_authz → oauth2-proxy)"]
        MTLS["sidecar 間 mTLS (PERMISSIVE)"]
    end
    subgraph CILIUM["L3/4 ─ Cilium eBPF ｜ datapath と到達性の基底"]
        KP["kube-proxy 置換 (eBPF)"]
        LB["L2 LoadBalancer (.240-249)"]
        CCNP["CCNP default-deny"]
        HUB["Hubble 可視化"]
    end
    ISTIO === CILIUM

    ZT{{"🛡 Zero-Trust<br/>3 コントロールを縦串で束ねる"}}
    ZT -.->|入口認証| GW
    ZT -.->|identity| MTLS
    ZT -.->|到達性 deny→allow| CCNP

    classDef default fill:#FCFAF6,stroke:#D2C9B5,color:#1A1714
    classDef l7 fill:#E0F1F8,stroke:#0284C7,color:#1A1714
    classDef l34 fill:#E3F0E8,stroke:#377A50,color:#1A1714
    classDef zt fill:#FBE7D2,stroke:#C97516,color:#1A1714
    class GW,MTLS l7
    class KP,LB,CCNP,HUB l34
    class ZT zt
```

## 全体図（リクエストフロー）

入口 2 経路 → Istio Gateway 集約 → App Pod。「どこを通って届くか」だけを示す。

```mermaid
flowchart TB
    Internet([Internet]) -->|HTTPS| CF[Cloudflare Tunnel]
    CF -.->|forward| GW

    LAN([LAN 192.168.0.0/24]) -->|HTTPS| LB[(Cilium L2 LB<br/>192.168.0.242-243)]
    LB --> GW[Istio Gateway<br/>gateway-platform / gateway-prod]

    GW -->|HTTPRoute| App[App Pod<br/>+ Istio sidecar]

    subgraph Cilium
      LB
    end
    subgraph "Istio Mesh"
      GW
      App
    end

    classDef default fill:#FCFAF6,stroke:#D2C9B5,color:#1A1714
    classDef ext fill:#FBE7D2,stroke:#C97516,color:#1A1714
    classDef l7 fill:#E0F1F8,stroke:#0284C7,color:#1A1714
    classDef l34 fill:#E3F0E8,stroke:#377A50,color:#1A1714
    class Internet,LAN,CF ext
    class GW,App l7
    class LB l34
```

## ゼロトラスト 2 層

到達経路とは別に、メッシュ全体へ 2 つの規則がかかる。性質が違う軸なので分けて捉える。

| 規則 | 何の話か | 横断する軸 |
|---|---|---|
| **default-deny** (CCNP) | 到達性（届くか） | **ns 境界** — 横断は許可制 |
| **mTLS PERMISSIVE** | 暗号化 / identity（どう守るか） | **pod 間** — ns に依らず自動 |

### default-deny ― ns 横断は許可制

> 同一 ns は通る / ns 横断は許可しないと通らない。

```mermaid
flowchart LR
    subgraph nsA["ns A"]
        A1[Pod] <==>|"✓ 同一 ns"| A2[Pod]
    end
    subgraph nsB["ns B"]
        B1[Pod]
    end
    A2 -->|"✗ default-deny<br/>許可が要る"| B1

    classDef default fill:#FCFAF6,stroke:#D2C9B5,color:#1A1714
    linkStyle 0 stroke:#377A50,color:#377A50
    linkStyle 1 stroke:#CC3925,color:#CC3925
```

### mTLS PERMISSIVE ― pod 間に自動

> sidecar 同士は自動で mTLS。平文も受理する（＝強制はしない。STRICT なら拒否）。

```mermaid
flowchart LR
    subgraph P1["Pod A（sidecar 注入済）"]
        App1[app] --- SC1[istio-proxy]
    end
    subgraph P2["Pod B（sidecar 注入済）"]
        SC2[istio-proxy] --- App2[app]
    end
    Out["mesh 外 / sidecar 無し"]

    SC1 <==>|"🔒 mTLS を Istio が自動で張る<br/>（設定不要・相互認証）"| SC2
    Out -.->|"平文も受理（PERMISSIVE）<br/>※ STRICT なら拒否"| SC2

    classDef default fill:#FCFAF6,stroke:#D2C9B5,color:#1A1714
    classDef proxy fill:#E0F1F8,stroke:#0284C7,color:#1A1714
    classDef plain fill:#ECE6DA,stroke:#9A9183,color:#6B6358
    class SC1,SC2 proxy
    class Out plain
    linkStyle 2 stroke:#0284C7,color:#0284C7
    linkStyle 3 stroke:#9A9183,color:#9A9183
```

## 設計の要点

**3 つの原則がこの構成を貫く：**

1. **eBPF-native で部品を統合。** Cilium 1 つで kube-proxy 置換・L2 LB・NetworkPolicy・Hubble 可視化を賄い、MetalLB や kube-proxy を足さない。moving parts を最小化する。
2. **L3/4 ↔ L7 で役割を分ける。** 到達性（誰が誰に届くか）は Cilium、identity と入口認証は Istio。層を混ぜず各層の責務を単純に保つ。
3. **既定は拒否（zero-trust 基底）。** CCNP default-deny から始め必要分だけ allow、identity は全 hop で mTLS、入口は Gateway で認証。`istio-injection` label をセレクタにして新 ns を自動編入（ADR-004 / 009）。

具体の選択：

- **入口は 2 経路、出口は 1 つ。** インターネット公開は Cloudflare Tunnel（cloudflared が outbound 接続のみで成立、ポート開放・NodePort 不要）、LAN / 管理系は Cilium L2 LoadBalancer。どちらも最終的に Istio Gateway に集約し、**TLS 終端と認可を 1 箇所に寄せる**（ADR-001）。

- **Gateway は用途で 2 分割。** ドメインと証明書を分け、platform / app の責務境界を入口で可視化する。

  | Gateway | LB IP | 用途 | TLS |
  |---|---|---|---|
  | `gateway-platform` | `.242` | platform UI (Backstage / Grafana / ArgoCD / Keycloak / Vault) | `*.platform.yu-min3.com` |
  | `gateway-prod` | `.243` | user app (kensan 他) | `*.app.yu-min3.com` |

  （VIP pool `192.168.0.240-249` の全割当は network-ingress.md を参照）

- **NetworkPolicy は 2 層で集約。** PE 専管リソースなので `network-policy/` 1 箇所に置き、component ごとの分散管理を避ける（ADR-004 / ADR-009）。

  | 層 | スコープ | 例 |
  |---|---|---|
  | CCNP | 全 istio-injection ns 横断 | default-deny / allow-dns / allow-istio / allow-prometheus-scrape |
  | NetworkPolicy | per-ns | allow-intra-namespace / allow-otel-egress / allow-vault-egress |

- **mTLS は当面 PERMISSIVE。** sidecar 未注入 ns との互換のため。STRICT 移行は将来課題（ADR-007 の前提に関連）。

## 関連

- ADR: [001 TLS 終端パターン](https://github.com/yu-min3/kensan-lab/blob/main/docs/adr/001-tls-termination-pattern.md) / [004 NetworkPolicy 設計](https://github.com/yu-min3/kensan-lab/blob/main/docs/adr/004-network-policy-design.md) / [009 Shared allow-istio NetworkPolicy](https://github.com/yu-min3/kensan-lab/blob/main/docs/adr/009-shared-allow-istio-network-policy.md)
- LB IP 割当・WiFi fallback・既知問題: [`.claude/rules/network-ingress.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/network-ingress.md)
- ノード / インターフェース構成: [`.claude/rules/kubernetes-cluster.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/kubernetes-cluster.md)
