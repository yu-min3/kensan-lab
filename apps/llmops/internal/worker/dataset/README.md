# 固定dataset releaseとreader

Curatedの特定snapshotから注釈付きcaseを切り出し、2表目の`konro.datasets.import_cases`へ保存する。公開済みreleaseはmanifestに固定したsnapshotだけを読み、最新tableやprefix globを評価入力にしない。

## 公開契約

`llmops-worker dataset build -config <operator config>`はstdinのSpecを受ける。source snapshot、case ID、元Curated record ID、既存dataset-case schemaのcase、code/adapter版を指定する。source/target tableはconfigから解決し、要求側の識別子は一致検査にだけ使う。

caseには元Rawのobject ID・hash・size・app、expected_constraints、annotation版・status、source_group、development/holdout splitを保存する。source groupがsplitを跨ぐ要求、重複case、他app参照、不明なannotation statusを拒否する。syntheticは合成注釈であり人の実利用確認済みという意味ではない。

1. schema・役割・appと固定table設定を検証する。
2. 正規化要求を`apps/<app>/releases/<release>/request.json`へIf-None-Match付きで予約する。hash相当の比較対象にはcase・split・注釈・source table/snapshot・code/adapter版を含む。別内容でのID再利用はtable/tag更新前に拒否する。
3. 公開Rawのbytesとhashを検証し、source snapshotを指定してCurated rowと対応付ける。
4. case metadataとprepared inputを2表目へ一括commitする。
5. sourceとdatasetのsnapshotへREST `set-snapshot-ref`でtagを実作成する。UUIDと既存ref不在をcommit要件にし、既存tagを別snapshotへ更新しない。
6. Rawの保持参照を`releases/pins/raw/<object-id>.json`へ不変登録する。
7. 最後にdataset-release schemaに適合する`manifest.json`を公開する。

要求予約後の失敗は未公開。正確に同じ要求で再実行すれば続行できる。途中までtagができた場合はtagのsnapshotでcaseとprepared全内容を比較して再利用し、現在のsnapshotへ付け替えない。公開後の同要求は最初のmanifestとcreated_atを返す。無効な要求の予約を別内容へ上書きせず、新しいrelease IDを使う。

V0は同一host・同一operator設定のTempRootを共有する単一writer。app/transformのfile lockで並行publisherをbusy拒否し、process終了でlockを解放する。分散writerや別TempRootを使う複数配備は対象外。S3の条件付き予約は異なる要求の競合も防ぐが、分散transaction coordinatorではない。

## 再読込と利用方法

`llmops-worker dataset read -config <config> -release <ID>`、SDKの`dataset.Read`で取得する。evalは専用catalog/S3資格で読めるがbuildやmanifest/table変更はできない。

readerはmanifestと予約要求、table識別子、固定snapshot、retention tag、case一覧hash、Raw保持参照、Rawの実bytes/hashを検証する。未公開、tag欠損・付替え、Raw欠損・改変、case改変は失敗させる。戻り値`Loaded`はCase、Prepared、元record IDを分ける。

**モデルへ渡すのはPreparedだけ。Case内のexpected_constraints・split・注釈はgrader専用で、LLM promptへ混ぜない。** Preparedは原文から決定的に抽出したtext/content/output_schema/text_truncatedであり、期待値を参照して生成しない。

## 保持の範囲

Iceberg tagは`release_<release-id>`、`max-ref-age-ms=9223372036854775807`を明示し、通常のsnapshot expirationで参照が短期に失効しないようにする。作成後と読取時に実metadataのtagを照合する。[Iceberg保持仕様](https://iceberg.apache.org/spec/#snapshot-retention-policy)、[REST更新仕様](https://github.com/apache/iceberg/blob/main/open-api/rest-catalog-open-api.yaml)。

Raw pinは**保持参照のmetadata**であり、S3での物理削除や管理者の操作を禁止するWORM機能ではない。`GuardRawRemoval`／`GuardSnapshotRemoval`は参照中の削除を拒否するための維持管理側の入口。V0には自作expiration engine、削除API、unpin機能を追加しない。失敗した公開のpinも保守的に残す。将来のGCはこのguardを通し、削除要求と保持の衝突は明示判断する。

## 実証

`tests/integration/dataset/run.sh`は限定資格で20件の合成原文を公開・加工し、15 development／5 holdoutのdataset Aを作る。注釈を変えたBでtable snapshotを進めた後もAの入力が同一であること、同要求replay、異内容同ID拒否、未公開拒否、evalの読取と実table/manifest更新拒否を確認する。

実manifest書込の直前に一度だけ障害を返すtest backendで、tag作成済み・manifest未公開からのreplayも実証する。保持guardの拒否、Raw破損によるreader拒否を確認する。実モデルを呼ばず、合成注釈を実用精度の測定結果として扱わない。

Raw pinの実削除防止、backup復元、長時間query中のS3 session更新はこのタスクの実証範囲外。
