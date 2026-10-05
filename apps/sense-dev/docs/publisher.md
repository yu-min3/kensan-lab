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

1. operator-owned `-release-plan` がmission、private-canary、影響、rollback、PR要約とimage workflowの固定tag ref・commit W・file SHA-256を指定する。live publisherではworkflow指定必須。
2. App source Aが独立4工程＋credentialless verifierを完了。`branch_push` と `image_publish` は別のPlatform Gateを通す。
3. branch push直前にcontrollerが作者checkoutのobjectだけをcredentiallessに読み、base..Aの固定ref bundleを作る。hash・size・base・headを照合してpublisher trusted repoへ取り込み、worker Git config/hooksをcredential processで実行しない。
4. publisherは既存private GHCR package、unused tag、remote A/tree、workflow W/ref/hashを確認して固定workflowをdispatch。曖昧な結果はdispatch IDで照合し、再送しない。
5. CI artifactに加え、registry index・両CPUのmanifest/config実体とsource/workflow labelsを照合してdigest Dを記録する。dispatch受理だけでは配備へ進まない。
6. controllerがAをseedに、original main baseを保持するvalues-only App子task Bを作る。新しい5工程でdigestだけを固定し、Bの `branch_push` → `pr_create` → `merge` は各fresh Gateを通す。
7. GitOps merge Cはreviewed base/Bの二親と照合。host observerが App tree(A)=tree(B)=tree(C)、values(C).digest=D、Argo revision=C、実Pod image/runtime=D、`/api/release`を確認する。
8. App受入passでBと元のsource Aを完了する。failはCをbaseとする新source A2へ戻し、imageの再発行と再Gateを行う。元の依頼全体で修正は最大2回。
9. verified merge graphをbundleでcontroller trusted sourceへ返す。Platform改善と次のApp依頼はその配備済みCをbaseにし、source HEADを手動で進める必要はない。Platform改善でApp treeとDが不変なら証拠を固定してimageを再利用する。

認証/secret/公開/統制/破壊の分類はYu承認後もfresh Gateが必要。秘密実値・公開有効化・所有範囲外は承認でもdeny。`image_publish`にはsource A/tree・workflow ref/SHA/hash・一意tag/dispatch IDを固定し、BへのAの承認転用は許さない。

## プロセス境界

| 所有者 | 保持するもの | 許可する操作 |
|---|---|---|
| controller | state、限定 bridge auth、release plan | Gate配車、固定policy検査、台帳更新 |
| model worker | 自team context、task checkout、当該providerの購読認証 | 開発・レビュー。publisher socket/管理tokenへのアクセス不可 |
| verifier | task checkout、operator固定test plan | credentialなしの隔離検証 |
| publisher daemon | repo限定GitHub token、専用askpass、bridge auth | 固定repoへの許可済みpush/PR/merge、固定private image dispatch |
| host observer（daemon内） | GET-only kubeconfig/GitHub/GHCR token、固定観測plan | private canaryの観測 |

Unix socket は原則0600。別host user間では専用groupの0660を明示し、親directoryはgroup/world writableにしない。bridge secretは両processへそれぞれ0600のfileで配置し、modelのmountから除外する。daemonは一般web listenerを作らない。既存socketが残る場合は所有者を照合し、稼働daemonのsocketを自動で削除しない。

## GitHub の必須検査

- repo は `yu-min3/kensan-lab`、feature branchのみ。private-canaryのPRはready、その他はdraft。
- merge先mainのbase、PRのhead/ref/repo、非draft、mergeableを再確認。
- `canary (locked tests and runtime image)` と main の全required checks が同SHAでsuccess。GitHub Actionsのapp identityも照合する。
- strict required checksとadmin enforcementが取得・確認できなければ停止。実際のrepo設定は未検証。
- 操作直前の期限切れ、CI待ち、別SHA、保護設定不足は外部操作を行わない。
- `sending` / `unknown` は照合のみ。結果がない場合も自動再送しない。
- 送信直前の許可時刻 `authorized_at` を固定する。host観測とunknown照合は推論時間帯・billing admissionの外で継続し、期限後も当時の許可と固定入力を照合する。新しい操作には引き続きlive期限が必要。旧stateのsent intentに許可時刻が無い場合はreceiptを拒否する。

## 起動設定の組

controllerは `-isolated-worker -release-plan <operator-json> -publisher-socket <private-socket> -publisher-auth-file <controller-copy>`。mock/offモードからpublisherは起動しない。有限実験の `-model-attempt-limit` とrelease driverは現時点で併用不可。共有追加使用OFF監視、購読provider枠、JST inference windowは維持する。

daemonは `-serve-socket <socket> -controller-auth-file <publisher-copy> -repo <trusted-repo> -token-file <repo-token> -askpass <trusted-program>`。観測を有効にする場合は `-observer-plan` / `-observer-kubeconfig` / `-observer-token-file` をすべて指定する。daemonモードで `-data` / `-decision` は指定しない。

従来の単発CLIはcontroller停止中のみに使う。稼働controllerと同じstateを開けばsingle-writer lockで拒否される。通常経路はdaemon bridgeを使う。

## 観測先と導入前提

`-observer-plan` の固定 `probe_ip` は従来互換。自律rolloutではoperatorがprivate Pod subnetの `probe_cidr` を固定し、`probe_ip` と排他で使う。hostはnamespace・image@digest・runtime digestをすべて照合済みのready PodだけからCIDR内IPを選ぶ。固定port/path/release marker、proxy禁止、redirect禁止は維持する。CIDRはIPv4 RFC1918内に完全包含し、public・0/0・曖昧prefix・IPv6を拒否する。CIDR境界の実設定は導入審査対象。

### API proxy観測の候補（導入認可ではない）

`probe_transport` は未指定/`direct` が従来のPod IP直接GET、`kubernetes-api` が明示選択したAPI経由のGET。自動fallbackは行わない。API modeはport `8000`、path `/api/release` に固定し、hostがCIDR/IP・namespace・ready・image/runtime digestを検証したPodのname/UIDを内部だけで引き渡す。直前のPod GETでname/UID/IP/image/runtimeを再照合してから、`/api/v1/namespaces/app-canary/pods/<podname>:8000/proxy/api/release` を読む。PodName/URLはplanやworker入力で指定できない。

stock `kubectl get --raw` のredirect追跡を避け、API modeのArgo/Pod/proxy観測は専用HTTPS clientを使う。専用mode0600 kubeconfigは単一context/cluster/user、numeric RFC1918 HTTPS server、CA dataとtokenだけを受け付ける。exec/auth-provider、proxy URL、TLS server override/insecure、token/client cert/keyのfile参照を拒否する。TLS検証、proxy禁止、redirect禁止、10秒timeout、response上限を適用する。管理者kubeconfigのコピーは使用しない。credentialはhost observerだけが保持し、workerにmountしない。

#### API serverでの認証ヘッダー削除の前提

Kubernetes [v1.33.5の標準認証filter](https://github.com/kubernetes/kubernetes/blob/v1.33.5/staging/src/k8s.io/apiserver/pkg/endpoints/filters/authentication.go#L44-L113) は、認証成功後、下流handlerを呼ぶ前に `Authorization` と認証用front proxy headersを削除する。Bearer tokenをApp Podへ渡さない根拠はこのfilterであり、Pod proxy自体が任意の機密headerを除去する保証ではない。専用readerはCookieを設定せず、cookie jarも保持せず、Proxy-Authorizationも送らない。workerは専用kubeconfig/tokenを読み取れない隔離を維持する。

導入前のread-only実測として、親審査でAPI version `v1.33.5`、`kube-apiserver-master` のimage `registry.k8s.io/kube-apiserver:v1.33.5`、`--authorization-mode=Node,RBAC`、`--client-ca-file=/etc/kubernetes/pki/ca.crt`、`--requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt`、`--service-account-issuer=https://kubernetes.default.svc.cluster.local` が報告された。独自の `--authentication-config` / `--anonymous-auth` flagは観測されていない。この実測と固定版ソースから、標準の認証filterを通るAPI serverを採用前提とする。API server認証を無効化・改変するactorはApp/workerの任務外であり、その管理境界を維持する必要がある。認証filterが無効なら削除は迂回されるため、この前提を満たせない環境ではBearer方式を導入しない。

実測はimage tag/metadataとの対応までであり、稼働imageのregistry digestや実binaryと公式ソースの同一性は証明していない。実PodへのAPI proxy GETとPod側でのheader非受信も未実測である。コード候補とソース確認を実環境の配備・到達・非漏洩試験成功と扱わない。

追加RBACの草案は以下。**実権限の導入とcredentialの発行はYuの権限判断が必要**で、このコード候補はどちらも変更しない。

| Namespace | Resource | Verbs | 制限 |
|---|---|---|---|
| `app-canary` | `pods` | `get`, `list` | 固定canary selectorで観測 |
| `app-canary` | `pods/proxy` | `get` | host readerが上記port/pathに固定 |
| `argocd` | `applications.argoproj.io` | `get` | `resourceNames: [app-canary]` |

RBAC自体はproxyのport/pathを限定できないため、専用tokenと固定host readerが追加の境界になる。API proxy成功はこの経路のrelease marker確認であり、senseからPodへの直接到達、user path全体、インターネット非到達を証明しない。direct TCP timeoutの実環境事象はこの候補だけでは解消・再検証していない。

| 必要な前提 | 欠けたとき |
|---|---|
| 実canary source/values・locked test plan | checkout / verifierが進まない |
| main上のcanary check、strict required checks/admin enforcement | mergeを拒否 |
| protected tagのworkflow Wとfile hash、既存private GHCR package | image dispatchを拒否。初回package作成を自動で補わない |
| daemon `-commit-transfer`、private bridge、限定GitHub credential | branch push / merge返却を拒否 |
| GET-only observer credential、実Argo/cluster、固定CIDR/marker | host receiptとApp受入を待つ |
| モデル追加使用OFFのfresh確認、JST inference window | 推論開始を待つ |

この接続はlocalの実Git・fake外部transport・hash付きfixtureで検証する。実GitHub image公開、実Argo配備、実モデルGateの証拠はT037/T027/T052で記録し、local成功を代用しない。
