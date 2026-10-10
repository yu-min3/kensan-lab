---
type: note
title: "工房の宅内DNS・HTTPS・本人認証"
status: active
tags: [network, dns, authentication]
created: 2026-10-10
updated: 2026-10-10
---

## 結論

宅内の工房入口は `https://koubou.platform.yu-min3.com`。Cilium LBの既存Gateway `192.168.0.242` でHTTPS・Keycloak本人認証を行い、senseの独立adapterへTLSで中継する。Cloudflare Access/Tunnelや外部公開設定は変更しない。工房の管理tokenはhost内だけで扱う。

この差分は配備準備で、まだ実運用の合格を示さない。sense到達、GatewayからのID token転送、本人以外の拒否、スマホDNSと操作を配備後に実測する。

## 管理の境界

| 所有者 | 対象 | 契約 |
|---|---|---|
| Platform / kensan-lab | LAN DNS、Gateway routeとSSO許可、backend CA | HTTPSと署名付き本人IDをsenseへ渡す。工房の実装・台帳・モデル資格は置かない |
| 工房 / private koubou | controller、UI/API、本人認証adapter、TLS秘密鍵、管理token | 固定本人sub、platform-admin、署名・issuer・aud・exp/nbfを独立検証し内部sessionを作る。UIへ認証基盤を埋め込まない |
| operator | Keycloak clientへの工房callback登録、本人subの確認 | 全realm bootstrapを実行せず、既存callbackを保ったまま工房のURLだけ追加 |
| Yu | PR merge判断、宅内ルーターのDHCP DNS設定 | DNSと認証経路の実測後に切替。スマホへ管理tokenを渡さない |

## 構成

1. スマホは宅内DNS `192.168.0.247:53` で工房と `auth.yu-mins.com` をLAN Gatewayへ解決。
2. Gatewayは既存のTLS証明書とoauth2-proxyによるKeycloak本人認証を使う。工房はplatform-adminのみに許可。
3. Gatewayからselectorless Service／EndpointSliceを通してsense `192.168.0.113:8790` へ中継。DestinationRuleでTLSを始め、専用CAと `koubou-sense.internal` のSANを検証。
4. sense adapterはAuthorizationのID tokenを独立検証し、固定本人subを照合。coreには工房のsession cookieだけを渡し、ID tokenを渡さない。
5. controllerは `127.0.0.1:8787` のまま。JSON API/core/台帳はUIを使わず操作できる。

現Istio 1.27.3にBackendTLSPolicy CRDがないため、既存Istio DestinationRuleを使う。公開CA証明書だけをSealedSecretへ封入し、秘密鍵はGitへ入れない。TLSが成立しない場合のHTTP fallbackは用意しない。

## 宅内DNS

`lan-dns` は独立したCoreDNS 2 Pod。UDP/TCP 53をLBから非特権の5353へ転送する。工房・Keycloak・oauth2-proxyの固定名だけをLANへ向け、他の名前は外部resolverへ転送する。clusterのkube-dnsやMac hostsは変更しない。

DNSの通信制限は、Namespace -2 → NetworkPolicy -1 → Deployment/Service 0の順で、同じPlatform管理Applicationに置く。新namespaceを別Applicationのpolicyが先に参照する競合と、Pod起動時のpolicy未適用を避けるため、通常のnetwork-policy集約からこのresolverだけを分けた。

ルーターへの変更はArcher A10のDHCP ServerのDNS配布。WAN側DNS、DHCPアドレス範囲・gateway・lease・予約は変えない。DNS稼働と入口の合格後にPrimary DNSを192.168.0.247へ変更する。Public DNSをSecondaryへ混ぜるとprivate名が解決しない場合があるため混ぜない。切替前の設定を控える。

## 導入順と確認

1. senseへTLS adapterと固定本人subを設定する。模型workerやPublisherを有効にする操作ではない。
2. operatorは `bootstrap/keycloak/register-koubou-host.py` のread-only確認を実行する。管理資格はmemoryで扱い、ログ・argv・Gitへ出さない。
3. PR merge後、同operatorの `--apply` で工房のcallback/web originだけ追加する。既存URL・user・password・realm設定は維持する。
4. CA Secret Ready → DestinationRule → 工房routeを確認。DNSの2 Pod、LB 192.168.0.247、UDP/TCP名前解決も確認する。
5. DNSを一時指定した検証端末でTLS・本人ログイン・登録・停止を確認。未認証/別人/期限切れ/CSRFの拒否も確認する。
6. YuがDHCP DNSを切替し、スマホWi-Fi再接続で反映。Safariで操作して合格記録を残す。

## 壊れうるもの・戻し方

| 箇所 | 失敗時の体験 | 戻し方 |
|---|---|---|
| callback・SSO許可の登録漏れ | 工房だけログイン時にエラー/403。既存サービスへの追加許可は変更しない | 工房hostの追加をrevert。追加callbackだけ削除し、既存callbackは残す |
| sense/TLS/固定subが未準備 | 工房は503または401で使えない。認証を迂回して開かない | routeをrevertしてhost導入を修復。TLS検証を外さない |
| DNS Pod/上流resolver停止 | DHCP切替後の端末で名前解決できない | ArcherのDHCP DNSを控えた旧設定へ戻し、Wi-Fi再接続 |
| 新Appを削除 | finalizer/pruneで新Namespace内のPod/Serviceが消え、宅内DNSと工房入口が停止 | 先にDHCP DNSを復旧してから削除。新AppはPVCなし。既存canary/Keycloak/PVCを削除対象に含めない |
| backend証明書期限 | Gateway→sense TLSが失敗し工房503 | server証明書更新とhostservice再起動。CA変更時はSealedSecret更新を先行。初期server証明書は180日、更新の自動化は別作業 |

## 実施済み／未実施

実施済み: Goの署名・固定本人sub・groups・期限・peer拒否試験、Linux build、YAML lint/schema、実clusterへのserver dry-run、SealedSecretのcontroller検証、Argo Application静的検証、管理APIで本人IDのread-only確認。

server dry-runは未作成のlan-dns/koubou-entry namespaceだけdefaultへ置換して実施した。実際の新namespaceのdatapath・SSO連携を合格とみなさない。

未実施: このPRのmerge/正式sync、Keycloak callback更新、sense service導入、ルーター変更、スマホ実機。senseは準備時点でSSH到達不能だった。
