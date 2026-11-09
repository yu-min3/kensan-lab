# アプリ別identityとdata accessのbootstrap

`run.sh`はlocal dev stackを起動し、Keycloak client、RustFS user/policy、Polaris principal/role/grantを構築して正負のアクセスを検証する。管理者資格は初期構築スクリプト内だけで使い、実際のtable試験は専用OIDC clientとvended S3 sessionを使う。本番設定へ適用するコマンドではない。

## 構築するもの

| 区分 | client／role | audience・アクセス |
|---|---|---|
| アプリAPI | konro-app、konro-transform、konro-eval、sample-app | llmops-platform。app/role mappingは共通API側で強制 |
| Catalog | catalog-konro-transform、catalog-konro-eval、catalog-sample-app | polaris。専用principalへazpで対応付け |
| Runtime | runtime-konro-online、runtime-konro-eval、runtime-sample-online | agent-runtime。runtime側allowlistとowner判定へ接続 |
| Runtime否定試験 | runtime-expiry-probe、runtime-denied-probe | agent-runtime。前者だけ3秒token、後者はruntime allowlistに入れない |

一つのtokenを全サービスへ流さず、audienceごとに別clientを使う。Keycloakはclient credentialsを発行し、Polarisは署名・issuer・audienceを検証したうえで、azpと一致するprincipal、polaris_roleに対応するprincipal roleのgrantを評価する。内蔵authorizerを使い、OPAは追加しない。

PolarisのOIDC URLとtoken issuerはhost/containerとも`https://id.localhost:19443/realms/llmops`。`quarkus.oidc.tls.tls-configuration-name=oidc`で名前付きPEM truststoreを選び、hostname検証を有効に保つ。内部OAuthは管理bootstrap用に残すmixed構成。

## CatalogとS3を分離する

共有Polarisサービス内にapp別catalog `konro`と`sample`を作る。各catalog内は`curated`と`datasets` namespace。実テーブルのidentifierは`konro.curated.recipes`、`konro.datasets.import_cases`を想定する。DuckDBではcatalogを設定からattachし、namespace/tableも設定から選ぶ。

当初の単一catalogにapp別namespaceを置く案から変えた理由は、catalog全域のNAMESPACE_LISTを渡さず、他appのnamespace列挙まで拒否するため。同じPolarisサーバーを共有する点は変わらない。sample clientは自分のcatalogを使えても、konro catalogのLISTは403/404になる。

秘密を含まない`config/access-model.json`をS3 policyの原本とする。

| S3 role | 読取 | 書込 |
|---|---|---|
| api-konro／api-sample | 自app raw、records/feedback、runs、reports | 同範囲にGet/Put。Delete権限なし |
| transform-konro | 自app raw、records/feedback | warehouseのGet/Put/Delete、releasesのGet/Put |
| eval-konro | 自app raw、warehouse、releases | runs／reportsのGet/Put。warehouse・release書換なし |
| catalog-store | konro／sampleのwarehouseだけ | 同warehouseのGet/Put/Delete。Raw/runアクセスなし |

feedbackの実体は`apps/<app>/records/feedback/<event-id>.json`で、API保存契約に一致させる。transformは注釈案を読めるが書き換えられず、evalはfeedbackを書けない。

catalog-storeは共有catalogサービスの内部資格であり、writer/evalの直接S3資格として配らない。table-scoped sessionはPolarisが発行し、個別tableへ絞る。RustFS beta.8では`sts:AssumeRole`のResource `*`がpolicy parserで拒否されたため、互換表現`arn:aws:s3:::*`をSTS actionにのみ使う。通常のS3 Get/Put/Deleteは引き続きwarehouse prefixに限定している。実vended sessionで他app RawのGETが403になることも検証する。

## 資格の受け渡し

すべてrepo内のGit管理外`temp/llmops-dev/`へ保存する。値をログ・PR・manifestへ出さない。

- `clients/<client-id>.secret`：Keycloak confidential secret。0600。
- `storage/<role>.json`：`access_key`と`secret_key`。0600。API/workerは自分のroleの一ファイルだけを受け取る。
- `storage/<role>-policy.json`：適用したS3 policy。0600。
- `ca.pem`：公開CA。host clientはこれを明示trustする。
- `dev.env`：bootstrap管理資格とcatalog-store資格。一般API/workerへこのファイル全体を渡さない。

Polarisにはdev.envの`CATALOG_S3_KEY`／`CATALOG_S3_SECRET`だけを環境変数で渡す。RustFS root keyを渡さない。APIの役割選択はrequest bodyから行わず、検証済みappから対応するcredential fileを選ぶ。

RustFS IAM操作には固定release・checksum照合済みのMinIO mcをbootstrap専用に使う。binary自体はGitへ入れずtempへ置く。AGPL-3.0の外部管理ツールであり、アプリruntimeの依存ではない。mcのuser addはsecretを引数に要求するため、スクリプトはcommandやstdoutを印字せず、専用local bootstrap内で短時間だけ使用する。

## 冪等性と資格更新

再実行時は既存のprivate secret fileを再利用し、clientとpolicyを同じ定義へ更新し、存在するprincipal/catalog/namespaceは再作成しない。service-accountのclient IDは固定するためowner identityも維持する。既存grantを任意に削る汎用reconcileツールではない。policyやroleを削除する変更は明示的なrevokeと否定試験を追加する。

Keycloakのsecret更新は`clients.py --rotate-client <client-id>`で明示実行できる。旧secretで401、新secretでtoken取得成功をassertし、private fileも更新する。catalog-konro-evalでrotation後の実table readまで確認済み。runtime通常clientを試験のために勝手にrotateしない。発行済みtokenは期限まで有効になり得るためsecret rotationを既存tokenの即時失効とは扱わない。

## 実証したこと

- 12 clientのclient credentials、azp、aud、issuer
- API向けtokenをPolarisへ送ると401
- transform OIDCでtable作成、eval OIDCで同table読取成功、INSERT403
- 別app catalogのnamespace LIST拒否
- API/transform/evalの他app Raw GET403
- evalのdataset release／warehouse PUT403、transformのRaw PUT403
- warehouse限定catalog-storeを使うvended sessionから他app Raw GET403
- 全bootstrap再実行と、専用clientのsecret rotation

一時的なtable `_permission_probe`は試験後に削除する。二つの業務用tableをここで作成したとは扱わない。通常のmodel providerは一度も呼ばない。

API自体の認可、datasetの版固定、長時間query中のS3 session失効後更新、権限取消後のcache扱い、backup復元は別のタスクで検証する。MinIO mcのLinux用checksumは固定しているが、この時点の実行証拠はMac arm64のみ。
