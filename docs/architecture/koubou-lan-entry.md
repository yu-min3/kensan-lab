---
type: note
title: "工房Appの独立配備とPlatformの不足契約"
status: active
tags: [platform, app, koubou]
created: 2026-10-10
updated: 2026-10-10
---

## 結論

工房はprivate `yu-min3/koubou` repoのapp-base用valuesと追加resourcesで配備する。kensan-labにはapp-projectのApplication登録だけを置き、共通Deployment/Service/HTTPRouteを再実装しない。共有DNS・Gateway・SSOの変更をこのPRに混ぜない。配備は手動syncとし、以下の前提が満たされるまで運転開始しない。

旧PR案のnetwork/koubou-entry、platform-project、共有Gateway認可へのhost追加、工房専用DestinationRule/SealedSecret、新LAN DNSは今回のdiffから削除した。旧案は未マージ・未配備であり、既存リソースのpruneは発生しない。

## App／Platformの契約

| 項目 | Appが所有 | Platformが提供 |
|---|---|---|
| 配備 | private repoのchart/valuesと固定commit | Argo CD、app-project、repo参照権限 |
| 通信 | app-koubou内relay/Service/route/NetworkPolicy、senseへのCA/SAN検証 | gateway-prodのHTTPS・既定SSO、許可されたroute接続 |
| 認証 | senseの本人sub/group/署名検証、内部session | Keycloak本人認証、署名済みID token、callback登録 |
| 名前解決 | 必要なhostとGateway IPを要求 | LAN端末のDNS解決。DHCPや共通DNSはApp管理外 |

namespaceはApp resourcesが所有しPrune=false。PVCなし。App内relayからsenseへTLSを張るためEndpointSliceやGateway用DestinationRuleを追加しない。既存SSO契約を使うため共有AuthorizationPolicyに工房hostを追記しない。

候補URL: `https://koubou.app.yu-min3.com`（LAN Gateway 192.168.0.243）。まだ利用不可。App側の仕様・導入手順はprivate repoの `deploy/app-entry.md`。

## Platformに足りない要素・確認事項

| ID | 現状／不足 | 当面の導入条件 | 継続改善の契約 | 担当 |
|---|---|---|---|---|
| P001 | private repoの参照権限は既存org credential templateの仕組みがあるが、工房repoへの実アクセス未検証 | Argo CDが工房の固定commitを読めることを確認。資格値は取得/表示しない | repo登録時に接続確認まで返す | Platform |
| P002 | Keycloakのcallback登録はhostごとのPlatform操作 | 工房App hostのcallback/weborigin登録を別Platform変更として行う | Appがhostを申請し、Platformが承認/登録/検証結果を返す。共有realmをAppが変更しない | Platform、公開/認証変更の判断はYu |
| P003 | OAuth2 routeのReferenceGrantがapp-kensan/app-konroを個別列挙 | app-koubouのHTTPRoute→auth-system/oauth2-proxy参照を別変更で許可 | App登録とcross-namespace認証参照許可を一緒に処理する。全namespaceへの無条件許可にはしない | Platform |
| P004 | LANスマホ用の名前解決がなく、Mac hostsに依存 | 工房host→.243と認証hostのLAN解決を提供 | LAN DNSを共有基盤として設計・独立PRで提供。Appはレコード要求だけを渡す | Platform、DHCP切替はYu |

| P005 | app-baseにcommand/argsと追加volume/mountの指定がない | 共通chartの汎用拡張draft PR #543をレビュー・マージ後に利用 | Appが共通templateを複製せずvaluesで設定できる | Platform |

P001は機能がないと断定せず確認事項。P002/P003は既存の手動登録契約が残っている。P004がスマホ導入の不足。これらを工房Appが直接作る構成にはしない。外部host TLSはAppのConfigMapで設定する。app-baseに必要な汎用機能はP005として別PRに分離する。

## 採用・却下

| 判定 | 案 | 理由 |
|---|---|---|
| 採用 | private app-base用valuesと追加resources＋既存gateway-prod | Appの追加で共有認可のhost列挙を編集しない。App資源はapp-project内に収まる |
| 採用 | App relayがsense TLSを検証 | App固有CAをAppへ閉じ、EndpointSlice権限の拡大や共有DestinationRuleを不要にする |
| 却下 | 工房専用入口をnetwork/platform-projectへ置く | AppがPlatform権限に依存し、repo独立の要件を満たさない |
| 却下 | このApp PRでLAN DNS/DHCPまで導入する | 宅内全体への影響をAppの変更と分離して判断できない |

## 検証と残作業

既存app-baseと工房valuesはHelm lint/render、schema検証、上流imageのamd64/arm64 manifest確認を実施。Platform登録はAppProjectの許可kind/namespaceと整合する。実SSO・sense・Gateway datapath・スマホは未検証。P001〜P005とsense側導入を満たしてから手動syncし、本人ログイン/登録/停止と拒否経路を実測する。

## 壊れうるもの／戻し方

このPRの旧infra案は未配備のため、その削除による既存namespace/PVC/DNSのpruneはない。新Appはauto-sync/finalizerなし、ApplicationとnamespaceにはPrune=false。手動配備後にAppを止める場合はApp内route/relayだけを対象とし、共有Gateway/SSO/DNSは削除しない。

## Yuの未決事項

このdraft PRでは共有Platform変更の承認を求めない。不足要素を別のPlatform改善として具体化してからレビューする。マージ前の本人確認条件を維持する。


## 共通Helm chartの利用

工房側の独自Deployment/Service/HTTPRoute templatesは削除した。Argo CDは既存app-baseとprivate工房values/resourcesをmulti-sourceで組み合わせる。汎用chartの不足（起動指定、追加mount、API token制御、listener指定）は別draft PR #543で扱う。既存kensan/konro/canaryのrenderが旧chartと同じ構造であることを確認した。工房固有のConfigMap・Namespace・NetworkPolicyだけがprivate repoに残る。
