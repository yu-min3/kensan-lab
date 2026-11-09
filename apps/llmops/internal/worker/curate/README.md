# 公開captureからCurated Icebergへ

`llmops-worker curate`はoperator configのapp・transform role・catalog/tableを固定して、一つ以上の公開captureを加工する。stdinはcapture IDとsource groupの配列。任意のS3 URI、SQL、binary、資格を要求側から指定させない。

## データ経路

1. 同appのobjects.Storeから公開manifestとRawを読み、MIME・サイズ・SHA256を検証する。
2. hash固定したKonro adapterのpure prepareへ元形式の原文を渡す。
3. prepared text、切詰め有無、recipe、schema、warningsを出自付きrowへ変換する。
4. Goから埋込Python bridgeを起動し、秘密をstdinでだけ渡す。子envへ親の資格を継承しない。
5. 固定DuckDB 2.0とextension revisionを確認し、Keycloakのcatalog-transform資格でREST catalogへattachする。
6. typed rowを一時表へbindし、一つのMERGEでbatchをIcebergへcommitする。

SQL値はparameterとしてbindし、table/namespaceは固定configとidentifier allowlistから作る。table構造は埋込のKonro SQL profileに置き、共有worker SDKに材料・工程schemaを持たせない。credentialとtokenはcommand arguments、stdout、通常ログへ出さない。失敗時は固定処理段階と例外classだけを返す。

## 保存するもの

`konro.curated.recipes`はprovenanceのVARCHAR列、prepared text／bytes／truncated、材料・工程のVARCHAR配列を含むrecipe STRUCT、warnings配列を持つ。完全なcontentとoutput schemaもlossless JSON文字列として残す。Iceberg/DuckDBのJSON論理型はこの組合せで書けないためVARCHARにし、typed recipe列と両立させる。

record IDはapp、capture、原文hash、source group、transform release、domain schema、adapter SHA256から決める。同じ加工の再送はMERGEで増やさず、加工版やgroupが変われば別recordとして残す。schema_versionはadapterのdomain schema版。親captureのschema版は公開Raw metadataから辿れる。評価datasetは対象transformを明示して切り出し、履歴全件をそのまま正解集合にしない。

原文は変更しない。HTML抽出でprepared textが切り詰められた場合もtext_truncatedを保存する。prepared textのbyte数が実際と異なる応答は拒否する。Rawに書けないtransform資格で読むため、加工処理から原文を上書きしない。

## 構成と再実行

worker configには既存app/role/storage/adaptersに加えてcatalogを指定する。nameはappと一致、namespaceはcurated、tableはrecipes。client IDは`catalog-<app>-transform`、secret fileは同名`.secret`、CAとPython executableはoperatorが選んだ絶対パス。

repo rootの`apps/llmops/tests/integration/curate/run.sh`が必要なbinary/configをtempへ作り、限定API資格で公開した合成2件を実サービスで加工する。事前にdev/bootstrapを起動しておく。実モデルを呼ばない。Onion/Tomatoの材料・工程と元原文hash、出自、変換版、schema版、切詰めflagを実tableからread backして一致を確認し、同じbatchの再送でもrecordが増えないことをassertする。Tomato原文は140KiB程度にして切詰めの明示を確認する。

catalog側のreadonly/別app拒否は`dev/bootstrap/catalog.py`で実証する。unit試験は原文破損がadapterへ到達しないこと、別app／evalの拒否、不正prepared bytes、重複captureの拒否を確認する。

SDKのadapter実行はOS sandboxではない。DuckDB bridgeも信頼した埋込codeをoperator管理のPythonで動かす。ネットワーク禁止を仮定せず、必要なcatalog/IdP/S3接続を役割資格とservice policyで制限する。
