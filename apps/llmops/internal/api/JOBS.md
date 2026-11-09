# Jobとartifact APIの実装

`cmd/llmops-api`は共通APIを起動し、実JWT verifier→resource policy→単一writer job台帳→app別runtime資格の順に処理する。capture／feedbackとjob／artifactは同じ認証middleware・policy・trusted storage resolverを使う。

## 起動設定

`config/api.example.json`は秘密を含まない書式例。`/run/llmops/...`とruntime.example.testは置換が必要で、動く接続先を表すものではない。CLIの`-config`または`LLMOPS_CONFIG_FILE`からoperator管理のJSONを読む。unknown field・末尾JSONは拒否する。config／mappingはAPI利用者が書換えられない場所に配置する。

| 設定 | 責務 |
|---|---|
| issuer／audience／jwks_url／mapping_file | 宛先固定のJWT本人確認とapp×roleの解決 |
| ca_file | 開発用CAの明示trust。TLS検証は無効化しない |
| state_directory | 0700の単一writer台帳。別processの同時writerは拒否 |
| cursor_key_file | 32 byte以上、private file。署名cursorを再起動後も使うなら同じkeyを復元 |
| stores | app別のS3接続と限定資格ファイル。root資格を使わない |
| runtimes | downstream client IDをキーにapp・role・targetを固定するroute |
| daily_limits | app×online／eval別のUTC日次受付上限。省略／0は新規受付を拒否 |

未送信・照合中のjobに必要なruntime routeや資格が設定から消えた場合は、起動時に拒否する。既存jobが完了するまで元routeを保持し、別endpointへ黙って付け替えない。

APIはliteral loopbackでのみHTTPを許す。それ以外のlisten先はTLS cert/keyが必要。credential fileはgroup／otherから読めないregular fileでなければ起動・利用を拒否する。S3資格JSONはaccess_key／secret_keyと任意session_tokenで、APIは最大1分のcacheを使って再読込する。Keycloak client secretもruntime clientが更新時にprivate fileから読む。

`GET /healthz`はlivenessだけを返し、app数・job数・資格を公開しない。runtime／S3接続の疎通や実行可能枠を保証するreadinessではない。

## 受付・再送・実行

1. POST /v1/jobsを認可し、owner・canonical payload・downstream client/target・冪等keyと日次予算予約を原子的に保存する。
2. 未送信pendingを、online優先で選ぶ。同一process内のdispatchは逐次。
3. networkへ送る前にuncertainを永続化し、runtimeへ固定の下流冪等keyでCreateする。
4. 成功応答のruntime IDと状態を保存する。応答喪失やAPI再起動後も同じCreateを再送する。

上流Idempotency-Keyは1〜128文字の英数字・`.`・`_`・`:`・`-`。同owner／同keyで内容が違えば409。再送は新しい予算を予約しない。promptはUTF-8で32 KiB、output schemaはserialized JSONで32 KiB、下流request全体512 KiBまでを受付前に検査する。

| 下流応答 | 保存・表示・再開 |
|---|---|
| 400／404／405／409／413／415／422 | 明確な受付拒否としてfailed／runtime_request_rejected。自動再実行しない |
| 401／403 | 未受付のwaiting／runtime_auth_required。同じkeyの明示再送・retryで再開 |
| 429 | 未受付のwaiting／runtime_quota_wait。同じkeyで再開 |
| transport／5xx／結果不明 | uncertainを保持しwaiting／dispatch_reconciliation_requiredを表示。自動試行は最大3回／process、backoff付き。明示replay／retryで同keyの照合を再開 |

同一runtime jobのwaiting再開は同attempt。failedからの明示retryは新attempt／job IDを作り、予算を使う。新attemptにもIdempotency-Keyが必須で、同じretry要求の再送は一つの新attemptへ対応する。preparedの取消はローカルで完了する。応答不明で下流IDがない取消は409とし、生成を実行していないと偽って成功を返さない。

online優先はAPIに残る未送信キューの中だけ。runtimeへ既に送ったjob、既存直結consumer、共有provider枠を横取り・優先制御しない。

## 読取と成果物

Get／cancel／retryは全てpolicyでapp・role・subject・kindを確認する。一覧はstoreのowner scopeとkindで先に分離し、その後でID順とpaginationを計算する。他appの件数・cursor・全体順位を返さない。cursorはscope・位置・期限の署名であり、別scopeから使うと404。

GETは下流状態を照会し、成功・失敗・取消の結果をapp別immutable recordへ保存する。保存済みのterminal結果は以後runtimeへ依存せず再読込でき、同jobの出力が書き換わらない。public jobには下流client／target／payloadを露出しない。model／usageが取得できなければartifactではnullを保持し、推測値を埋めない。

artifact APIは`raw_<capture-id>`と`job_<platform-job-id>`だけを、trusted metadataから解決する。任意URLやbucket/pathのproxyはない。downloadはattachment／nosniff／CSP sandboxで返し、原文HTMLやモデル出力をブラウザ画面として実行しない。job artifactの内部にはruntime ID等を保持するが、公開responseはplatform job ID・output・model・usageだけ。

immutable record storageはruntimeの最大responseに合わせ2 MiB。feedback受付は別途256 KiBのまま維持している。

## 検証

API試験は標準crypto/rsaで独立生成した実署名とTLS JWKS serverを使う。HTTP routing・JWT本人性・policy・実ファイルjob台帳を通し、runtime GatewayとS3 backendだけを決定的fixtureに置き換える。

確認項目は所有者別の全job操作、artifact拒否、bodyのapp／資格偽装、一覧とcursor、予算、byte上限、online優先、応答喪失から台帳再起動後の同key照合、failed新attemptと再送、401／429の待機、400の自動再実行拒否、5xxの不確実状態。

実Keycloak／実runtimeの認可契約は別のopt-in試験、実S3／Polarisもサービス統合試験で確認する。本taskのHTTP fixture成功をそれらの実サービス成功と呼ばない。
