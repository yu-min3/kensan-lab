# 永続job台帳

`Open(dir)`は0700の専用ディレクトリを作り、0600のwriter.lockに非blocking flockを取る。二つ目のwriterは起動しない。snapshotはversion・checksum・record整合を検査し、破損を自動修復しない。

## APIの利用順

1. 認証済みのOwner（App／Role／Subject）とoperator設定のRuntimeClient／RuntimeTarget、要求、冪等keyをAdmissionへ渡す。
2. `Admit(admission, dailyLimit)`で入力と予算予約を同じsnapshotに保存する。同owner・key・内容の再送は同ID、異内容はErrConflict。予算はapp×online/eval×UTC日単位。同要求再送は予算を増やさない。
3. `BeginDispatch`の永続化に成功した後で初めてruntimeへ送る。保存されたPayload、RuntimeClient／Target、DownstreamKeyをそのまま使う。
4. 応答を`Link`で保存する。既存のruntime IDを別IDに差し替えられない。
5. 再起動時はuncertainをreconcilingへ移し、`Pending`で未送信または応答不明のrecordを取る。reconcilingは新keyで再実行せず、同Createを再送して既存jobを照合する。

送信前cancelは`CancelPrepared`だけを使う。uncertainはruntimeに受理された可能性があるため、先に照合する。failed／interruptedの自動再実行はしない。waitingの再開、failedの新attempt、公開jobとattemptの対応付けは上位APIの責務。

`Get`／`List`はOwnerの全要素を照合し、他ownerはErrNotFound／空一覧になる。`Pending`は復旧用の内部APIであり、HTTP一覧として公開しない。Recordにはprivate入力や下流identityが含まれるので、そのまま外向きJSONにしない。返却Payloadはcopyされる。

## 保存と失敗

一時fileへwrite→fsync→close→rename→directory fsyncしてからmemoryを更新する。保存失敗はErrUnavailableにし、以降そのStoreはfail closed。rename後のdirectory fsync失敗では保存成否が不明になり得るので、プロセスを止めて再Openし、disk上のsnapshotから照合する。成功扱いのnetwork dispatchは行わない。

全recordを1snapshotに保存するV0。上限64 MiBで、履歴の自動削除はしない。recordは冪等性と日次予算の根拠なので、artifact保持期間に合わせて直接削除しない。将来compactするならkey tombstoneと当日予算を別途保持する必要がある。quotaは受付回数を数え、cancel／failedでも払い戻さない。

公開APIの署名検証、action認可、JSON duplicate key／サイズ検査、実行順・優先度、runtime呼出はこのpackageの外で行う。

## 検証

race試験は同時30受付の上限、owner／role分離、予算の再起動保持・UTC日替わり、応答喪失から同key復帰、runtime IDの差替拒否、terminal再実行拒否、保存ENOSPC時のmemory／disk非更新、破損／checksum／不整合snapshot拒否、送信前cancelを確認する。
