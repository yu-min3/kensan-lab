---
title: sense canary生成API
status: candidate
---

# sense canary生成API

agentは人間のOIDCログインを使わず、host adapterから固定private-canary生成APIを呼ぶ。生成結果をcheckoutへ取り込む前にレビューし、画像発行・PR・merge・配備は既存の独立Release Gateを通す。生成APIは外部へのpublishを実行しない。

## 固定契約

| 項目 | 境界 |
|---|---|
| API | `POST /api/sense-canary/v1/generate` |
| 認証 | 外部service principal `sense-canary-agent`。tokenは当pluginだけに限定 |
| caller入力 | `description` (1〜200文字)、`theme` (`day`/`night`)、`message` (1〜60文字)のみ。文字/数字と限定句読点だけ。引用符・backslash・制御文字・template式拒否 |
| 固定値 | name `canary`、既存repo `yu-min3/kensan-lab`、private-canary、Platform owner/domain/system |
| 固定source | template.yaml・skeleton・canary-overlay全36ファイルをoperator設定とsource定数のSHA256で二重固定。symlink禁止 |
| 実行 | own serviceからscaffolderへplugin tokenを発行。内部 `127.0.0.1:7007` のdry-runだけに送信。redirect禁止、30秒上限 |
| action | fetch-canary / arrange-canary / remove-canary-publication / canary-storageの4stepのみ。publish/catalog/Keycloak actionなし |
| 出力 | canary source/values/Argo定義配下のfilesとcontent hashのみ。logs/steps/secrets/tokenは返さない |
| 上限 | caller 4KiB、response 6MiB、decoded files 4MiB/100件。パス逸脱・重複・不完全結果拒否 |

## operator設定とhost利用

既存の起動は変更していない。`senseCanary`が未設定/disabled、または限定service credentialの設定が無ければ新APIはinertで、既存サービスは新secretを必要としない。

導入時だけ `app-config.sense-canary.example.yaml` のoperator overlayを追加する。`SENSE_CANARY_SERVICE_TOKEN`は専用の32文字以上のランダムcredentialをsecret管理から渡し、例ファイルへ実値を書かない。Backstageとhost adapterだけに配置し、worker/modelにはtoken・kubeconfig・内部plugin tokenを渡さない。既存OIDCとpermission設定を緩めず、scaffolder全体のexternalAccessを許可しない。Backstage backendのlisten portは7007に固定する。

host側はoperator管理の既存private HTTPS経路、またはhost loopbackへ限定された接続経路を使用する。clientはIPv4 RFC1918のHTTPS（CA検証有効）または `http://127.0.0.1:<operator port>` のみを受け付け、public IP・HTTP private IP・URLに付加したpath/query/userinfo・redirect・環境proxyを拒否する。loopback接続の転送先をBackstageに固定するのはhost operatorの境界であり、workerから変更できない。新しい公開routeや転送権限の導入はこの実装に含まない。

```sh
scripts/sense-canary-generate.py \
  --base-url http://127.0.0.1:7007 \
  --token-file /operator/private/sense-canary-token \
  --output /operator/private/generated-canary \
  --theme day --message 'Hello from the platform'
```

token fileはabsolute canonical regular file・mode0600、出力は未存在のdirectoryだけ。既存checkoutへの上書きはしない。callerは生成API証明とfiles hashを固定し、review/Release Gateへ渡す。ファイルの生成は配備権限やreceiptの代替ではない。

## 検証と残る導入確認

局所型検査とAPI境界試験で、disabled/missing credential、service-only、入力差替え、4固定step、内部token、logs非返却、悪い出力拒否を確認する。policy試験は実source bundleのpinと変更検出、path逸脱・secret/mode差替えを確認する。host client試験はローカルHTTP fixtureで固定route/credential、content証明、redirect、public endpoint、広いfile権限、既存出力拒否を確認する。

API試験はcompiled `senseCanary.js`/`senseCanaryPolicy.js`のdirectoryを `SENSE_CANARY_TEST_BUILD`へ設定し、依存をinstallしたbackend packageから実行する。policy試験はNode22.18+のtype strippingが必要。

```sh
node --test backstage/tests/sense-canary-api.test.cjs
node --experimental-strip-types --test backstage/tests/sense-canary-policy.node-test.ts
python3 scripts/tests/test-sense-canary-generate.py
```

実Backstageでの新image/config反映、機械token受理、実scaffolder dry-run、hostの固定private接続経路は未実測。局所API試験のscaffolder応答はfixtureであり、実生成成功の証拠として扱わない。既存mock/既存サービス、外部credential、route、cluster状態はこの実装で変更していない。
