# V0 contracts

共通API・保存record・adapterのversion 1契約。`openapi.json`はOpenAPI 3.1で、record型は`schemas/`を参照する。Goの`Validate(name, decodedJSON)`は同じschemaをembedし、外部schemaの取得を禁止する。

## HTTP契約

| 操作 | 契約 |
|---|---|
| capture | multipartの`metadata`（JSON）と`source`（原文bytes）。SHA-256をサーバー側で検算。capture_keyはapp scopeの冪等key |
| job作成 | Idempotency-Key header必須、app×role scope。bodyはtext profile、online／eval、prompt、input、output_schema |
| 一覧・取得 | app×roleで所有者を検査。cursorも認可済み範囲へ結び付ける |
| cancel／retry | headerの冪等keyでaction再送を保護。waitingは同attempt再開、failedは新attempt |
| feedback | event_idで冪等。captureまたはjobが必須、全参照先の所有権を検査 |
| artifact | IDからobjectを解決。任意URL／pathを拒否。原文HTMLはattachment／nosniffで返す |

未認証401、操作権限不足403、他appのresource ID404、同key異内容409、サイズ超過413、予算超過429、依存停止503。secretや本文をerrorへ出さない。

`app_id`は要求で省略可能、一致確認専用。ownerはverified clientからサーバーが設定する。owner・s3_prefix・下流clientのbody上書きはschemaが拒否する。認証・所有者・cross-record整合性はJSON Schemaとは別にserviceが強制する。

client-mapping.example.jsonは資格を含まない例。issuer／JWKS／下流秘密はoperator設定で固定する。API向けtokenをcatalogやruntimeへ転用しない。

## サイズ・整合性

JSON SchemaのmaxLengthは文字数。HTTP／下流送信前には別途byte数を検査する。初期上限はRaw 8 MiB、metadata 16 KiB、adapter message 2 MiB、runtime request 512 KiB、prompt／schema各32 KiB。multipartのパート数・総量も制限し、超過を黙って切り詰めない。

日時はRFC3339、hashはlowercase SHA-256、IDは英数字・underscore・hyphenで最大128文字。IDにS3 pathを使わない。JSON decodeではtrailing JSON、重複key、総量の取り扱いも実装する。

Raw bytesとのhash／size一致、参照先app、source groupのsplit、snapshotのpin、状態に応じたoptional field、応答request IDの一致はservice側の検証が必要。

## Adapter protocol

stdinへUTF-8 JSON objectを1つ、stdoutへJSON objectを1つ返す。stderrは資格・本文を含まないdiagnosticだけ。workerは実行timeout、2 MiBの入出力上限、trailing JSONの拒否を強制する。binaryはoperatorのallowlistからshellを介さず起動する。

```json
{"protocol_version":"1","request_id":"req-1","operation":"prepare","domain_schema_version":"sample-v1","payload":{"document":"example"}}
```

成功はversion／request ID／result、失敗はversion／request ID／error。error codeはinvalid_input、unsupported_version、processing_failed。成功とerrorの同居を拒否する。

prepareのpayload／resultはdomain schemaが定める。gradeは期待条件と実出力を受け、根拠付き評価またはneeds_reviewを返す。共通protocolへ料理型を埋め込まない。adapterはネットワーク取得・provider呼出・正式保存を行わない。

## 状態と公開

外向きjob stateはqueued／running／waiting／succeeded／failed／cancelled。送信前・再照合は内部台帳で扱う。model／usage未取得はnullで保存し、0にしない。

dataset releaseはpublishedだけが評価入口。draftは公開prefixへ置かず、全snapshot／Rawの検査とpin後に最後のmanifestを公開する。latestやglobを評価入力にしない。

record schema、adapter protocol、domain schema、prompt、evaluator、code、policyは別の版として記録する。

## 検証

go test ./...は正負fixtureを同じschemaで検証する。owner／path注入、未知profile、gold化、latest snapshot、任意shell、不正protocol、負のusageを拒否し、OpenAPIの参照先と各operationのsecurity宣言を確認する。実Keycloak／Polaris／S3認可は後続integrationの検証対象。
