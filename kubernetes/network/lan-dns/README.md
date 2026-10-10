---
type: note
title: "宅内DNS"
status: active
tags: [network, dns]
created: 2026-10-10
updated: 2026-10-10
---

## 結論

LAN端末は `192.168.0.247` のDNSを利用する。Appは共通 `app-base` chartにHTTPRouteのhostを指定すればよく、DNSの個別編集は不要。

| 管理対象 | 所有者 | 登録元 |
|---|---|---|
| LAN DNS・API読み取り権限・登録範囲 | Platform | このディレクトリ |
| Appのhost・HTTPRoute・DNS除外設定 | App | 自分のrepoのchart values |
| App/PlatformのA/AAAAレコード | 自動導出 | HTTPRoute → Gateway.status.addresses |
| 外部SSO名 `auth.yu-mins.com` の宅内向け回答 | Platform | Corefileのhosts |
| スマホへのDNS配布 | LAN管理者 | ルーターのLAN DHCP DNS設定 |

DNSは専用CoreDNS `k8s_gateway` イメージをdigest固定で使用する。クラスタ内部のkube-dnsは変更しない。外部DNS・Cloudflareの公開設定も変更しない。

## Macのhostsから集約する名前

稼働HTTPRouteと照合した。次の名前は既存Routeから自動登録される。

| 回答IP | 名前 |
|---|---|
| 192.168.0.242 | auth.platform.yu-min3.com / argocd.platform.yu-min3.com / backstage.platform.yu-min3.com / grafana.platform.yu-min3.com / prometheus.platform.yu-min3.com / hubble.platform.yu-min3.com / longhorn.platform.yu-min3.com / vault.platform.yu-min3.com |
| 192.168.0.243 | kensan.app.yu-min3.com / konro.app.yu-min3.com |
| 192.168.0.242 | auth.yu-mins.com（固定の宅内SSO別名） |
| 192.168.0.243 | koubou.app.yu-min3.com（AppのRoute配備後に自動登録） |

旧 `.240` の名前、otel.platform、auth-dev.platform、kensan-preview.appには現行HTTPRouteがなく、移行対象に含めない。localhostとDockerのローカル名はMacに残す。Macのhostsは端末DNS切替と照合が終わるまで変更しない。

## 名前の所有境界

ValidatingAdmissionPolicyをAPIサーバーで強制する。App namespace `app-{name}` は `{name}.app.yu-min3.com` のみ登録でき、parentは `istio-system/gateway-prod` に限る。既存 `app-kensan` の `kensan.yu-mins.com` だけ例外として残す。namespace labelを変えてもこの制限を迂回できない。Platform namespaceのRouteはPlatform管理者が管理する。

新たな別名・開発環境Gatewayが必要な場合は、Platformで所有権の契約を拡張する。Appのchartだけで任意の名前を登録することはできない。

DNS PodはHTTPRoute/Gatewayのlist/watchと対象CRDのgetだけを許可する。Secret読み取り、APIの更新、DNSレコードの別ストアへの書き込みは不要。PodはUID65532、capabilities全drop、読み取り専用rootfs、port5353で動作する。API egressはCiliumのkube-apiserver entityに限定する。

## App chartの使い方

```yaml
httproute:
  enabled: true
  hostnames: [example.app.yu-min3.com]
  dns:
    enabled: true
  gateway:
    name: gateway-prod
    namespace: istio-system
```

`dns.enabled` は既定true。falseにすると通常RouteとOAuth2 Routeの両方をDNS対象から除外する。Routeの削除・hostの変更も自動反映する。TTLは30秒で、クライアント側のキャッシュにより切替直後は旧回答が残る。

## 既知の制約と導入順

- DNSはRouteの存在を回答する。pluginはAccepted/ResolvedRefsやアプリの健康状態を検査しないため、配備途中に名前が引けてもHTTP接続が成功するとは限らない。Gatewayの承認・SSO・受入試験は別に行う。
- 導入前の既存RouteはAdmissionで遡及検査されない。現行app-kensan/app-konroの全Routeが所有境界に適合することを確認してから有効化する。
- 最初の通常CoreDNS imageにはNET_BIND_SERVICEのfile capabilityがあり、cap全drop構成でexecがEPERMになった。採用imageは同じ制限で起動できることをDockerとkindで確認する。権限を追加して回避しない。
- DNSはクラスタ依存。2Podを分散するが、クラスタ全停止時には宅内専用名が引けない。まず1端末で試験し、家全体のDHCP配布は最後に変更する。外部DNSを副DNSに並べても宅内名の冗長化にはならない。

1. PRを確認してマージし、ArgoのSync・PodのReadyを確認する。
2. Macから `dig @192.168.0.247` で上表の全host・外部名をUDP/TCPで確認する。
3. スマホ1台のWi-Fi DNSを手動で `.247` にしてHTTPS/SSOを確認する。
4. 問題なければルーターのLAN DHCP DNSを `.247` にする。WAN DNS設定ではない。
5. Macのhostsから確認済みの宅内hostだけを除去する。

切戻しは端末/ルーターのDNSを元に戻してからGitをrevertする。DNS namespaceにはデータ/PVCがない。Admissionの導入で既存外のApp別名や別GatewayへのRoute追加が拒否される場合、契約拡張のPRが必要。

## 検証と一次資料

`python3 scripts/test_lan_dns.py` は専用kind contextでのみ動作する。実image・RBAC・Pod権限制限、所有外hostとGatewayのCREATE/UPDATE拒否、既存kensan別名、UDP/TCP回答、DNS opt-outと削除を検査する。実クラスタCilium/LB・スマホ・DHCPはマージ後の確認対象。

- [k8s_gatewayの仕様と権限](https://github.com/k8s-gateway/k8s_gateway/tree/v1.8.2)
- [Kubernetes ValidatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/)
