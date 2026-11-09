# Platform API handlers

## 組み立て

Capture／feedbackは`NewCaptureFeedback(CaptureFeedbackConfig)`へVerifier、policy Engine、app別object Storeを渡し、`Register(mux)`で登録する。Verifierは実identity verifierのVerifyを使う。handlerはBearer tokenを検証してprivate contextへPrincipalを入れる。X-Client-ID／X-App-ID等を信頼する経路は持たない。

後続handlerは`Authenticated(next)`を同じmiddlewareとして使い、`Principal(request)`で検証済みidentityを取得できる。HTTP入口を持つhandlerは必ずmiddlewareの内側へ登録する。app storeのmappingは固定appと一致する場合だけ受け付ける。

policy Engineには`StorageResolver{Stores,Jobs,Next}`を接続する。captureとraw artifactは公開manifestのapp・owner subject／role、jobは永続台帳のownerを照合する。追加dataset／run artifactはNext resolverへ拡張できる。lookup errorは403／404／503へ分類し、body・token・保存pathを出さない。

## capture受付

POST /v1/capturesはmultipartのmetadata（application/json）とsourceだけを受け付ける。metadataは16 KiB、sourceは8 MiB、総requestは8 MiB＋64 KiBまで。未知part・重複part・不足part・不正MIME・不正JSONを拒否する。sourceのfilenameは保存pathの決定に使わない。

metadataはschemaとpolicyの両方を通す。ownerはPrincipalから設定し、capture key／MIME／hash／source URIと結び付ける。原文bytesはobjects Storeへ渡し、公開が完了してから202を返す。request内にapp_idがある場合は本人のappと一致する必要がある。

captureのpublic recordはschema v1を維持し、ownerはprivate publication wrapper v2で保持する。ownerのない古い試作publication v1は自動的に読み替えず、明示移行か新しいcapture keyが必要。既存業務レシピに対する移行ではない。

## feedback受付

POST /v1/feedbackはJSON、最大256 KiB。JSONはUTF-8、重複keyなし、trailing JSONなし、nesting 64段まで。schemaで未知fieldを拒否し、元capture／jobとbefore／after artifactの所有権をpolicyで確認する。byte参照は実体のhash／sizeも検算する。

event IDはapp scopeのimmutable key。verified subject／roleとcanonical requestをhashへ結び付け、serverがcreated_at、app、schema_version、annotation_status=unreviewedを設定する。actor_user_idは監査情報だけであり権限を増やさない。

保存先は`apps/<app>/records/feedback/<event-id>.json`。request hashとdata hashを持つobjectを条件付きで作成する。同要求の再送は最初のtimestampと本文、別内容は409。通常編集や注釈案をgoldへ自動昇格しない。

API用S3資格はこのfeedback prefixへのGet／Putも必要。eval／transformへfeedback更新資格を与える必要はない。署名やIAMによる実資格分離は別のintegration試験で確認する。

## HTTP試験

httptestは実policyとobjects Storeを組み合わせ、認証だけを決定的Verifierに置き換える。成功・再送・conflict、app／role／owner偽装、別app参照、gold上書き、raw参照hash偽装、重複JSON／part、過大sourceを確認する。この成功は実Keycloak／S3接続の実証とは区別する。
