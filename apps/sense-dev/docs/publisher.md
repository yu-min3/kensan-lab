---
type: note
title: sense v0 の独立 Gate と分離 publisher
status: active
tags: [sense, security, gitops]
created: 2026-10-04
updated: 2026-10-04
---

## 結論

App が開発し、Platform の独立 Gate を通した操作だけを別プロセスの publisher が実行する。controller は台帳の唯一の writer。publisher daemon は Store を開かず、モデルには GitHub・cluster・registry credential を渡さない。**この候補はまだ sense へ導入していない。**

## 通常経路

1. operator-owned `-release-plan` が対象 mission・private-canary・影響・rollback・PR要約を固定する。
2. App/Platform の change が独立4工程と credentialless verifier を完了すると、controller が SHA・操作ごとに scan と Platform Gate を生成する。
3. `branch_push` → `pr_create` → `merge` は、それぞれ別 Gate。承認は操作間で転用しない。
4. 認証/secret/公開/統制/破壊に関わる分類は Yu 承認後にも fresh Gate が必要。秘密実値、公開の有効化、所有範囲外は承認でも許可しない。
5. controller が `RunPublish` の直前検査を行い、認証付き private Unix socket で publisher に渡す。GitHub と Yu の早い方の期限を publisher でも確認する。
6. `merge` / `deploy` は GitOps の PR merge API に固定。直接 cluster を変更しない。実 merge commit の二親が reviewed base / head と一致するか確認する。
7. publisher 側の GET-only observer が Argo・private CI artifact・GHCR index/architecture digest・Pod・利用者経路を照合し、controller が system artifact と receipt を記録する。配備証拠のない App 受入は進まない。

## プロセス境界

| 所有者 | 保持するもの | 許可する操作 |
|---|---|---|
| controller | state、限定 bridge auth、release plan | Gate配車、固定policy検査、台帳更新 |
| model worker | 自team context、task checkout、当該providerの購読認証 | 開発・レビュー。publisher socket/管理tokenへのアクセス不可 |
| verifier | task checkout、operator固定test plan | credentialなしの隔離検証 |
| publisher daemon | repo限定GitHub token、専用askpass、bridge auth | 固定repoへの許可済みpush/PR/merge |
| host observer（daemon内） | GET-only kubeconfig/GitHub/GHCR token、固定観測plan | private canaryの観測 |

Unix socket は原則0600。別host user間では専用groupの0660を明示し、親directoryはgroup/world writableにしない。bridge secretは両processへそれぞれ0600のfileで配置し、modelのmountから除外する。daemonは一般web listenerを作らない。既存socketが残る場合は所有者を照合し、稼働daemonのsocketを自動で削除しない。

## GitHub の必須検査

- repo は `yu-min3/kensan-lab`、feature branchのみ。private-canaryのPRはready、その他はdraft。
- merge先mainのbase、PRのhead/ref/repo、非draft、mergeableを再確認。
- `canary (locked tests and runtime image)` と main の全required checks が同SHAでsuccess。GitHub Actionsのapp identityも照合する。
- strict required checksとadmin enforcementが取得・確認できなければ停止。実際のrepo設定は未検証。
- 操作直前の期限切れ、CI待ち、別SHA、保護設定不足は外部操作を行わない。
- `sending` / `unknown` は照合のみ。結果がない場合も自動再送しない。

## 起動設定の組

controllerは `-isolated-worker -release-plan <operator-json> -publisher-socket <private-socket> -publisher-auth-file <controller-copy>`。mock/offモードからpublisherは起動しない。有限実験の `-model-attempt-limit` とrelease driverは現時点で併用不可。共有追加使用OFF監視、購読provider枠、JST inference windowは維持する。

daemonは `-serve-socket <socket> -controller-auth-file <publisher-copy> -repo <trusted-repo> -token-file <repo-token> -askpass <trusted-program>`。観測を有効にする場合は `-observer-plan` / `-observer-kubeconfig` / `-observer-token-file` をすべて指定する。daemonモードで `-data` / `-decision` は指定しない。

従来の単発CLIはcontroller停止中のみに使う。稼働controllerと同じstateを開けばsingle-writer lockで拒否される。通常経路はdaemon bridgeを使う。

## 未完の接続

| タスク | 残り | 現在の挙動 |
|---|---|---|
| T057 | Gate固定image発行→digestをvaluesへ固定→再Gate | 手動workflow_dispatchが必要。mergeだけでimage成功と扱わない |
| T058 | reviewed commit graph移送、verified mergeのtrusted source反映 | publisher repoに作者commitがなければpush拒否。改善baseのmergedcommitがなければcheckout拒否 |
| T037 / T027 / T052 | 正規Backstage生成、private配布、Argo実配備、一巡の実測 | 実証なし。local fixtureを代用しない |

現在のchartはtag形式のため、immutable digestを要求するobserverの成功条件にはまだ到達しない。image source、reviewed App head、GitOps merge revisionを混同せず、次のimage経路ではApp treeの同一性まで検証する。
