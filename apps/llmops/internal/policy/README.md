# Platform API authorization

認証済みの`identity.Principal`と、信頼するstoreのresource metadataを使って認可する。HTTP bodyが指定したowner・app・runtime資格・object prefixを認可情報として保存しない。

## APIの組み込み方

1. identity verifierの成功結果だけをPrincipalとして渡す。
2. createはJSON requestをdecodeして`AuthorizeCreate(ctx, principal, action, body)`を呼ぶ。戻ったScopeからownerを保存する。
3. 個別resourceは`AuthorizeResource(ctx, principal, action, id)`を呼ぶ。metadataはEngine内部でResolverから取得する。
4. 一覧は`ListScope(principal, JobList)`の範囲をstore queryへ先に適用し、その後で並べ替え・件数・paginationを計算する。

`Resolver.Lookup(ctx, principal, type, id)`はAPI内部のstore adapter。job storeなら`jobs.Owner{App: principal.AppID, Role: principal.Role, Subject: principal.Subject}`でGetし、保存済みAdmissionからmetadataを返す。captureやartifactも保存済みの所有者を使う。bodyの`app_id`やobject refにあるownerをそのままResourceに変換する実装は不可。

| 呼出 | 成功時の結果 |
|---|---|
| `AuthorizeCreate` | 許可されたAppID・Role・Subject、jobならKindを持つScope |
| `AuthorizeResource` | storeから取得して所有範囲を確認したResource |
| `ListScope` | queryのapp・role・subject・kindを固定したScope |
| `Cursor`／`ReadCursor` | 署名済みpagination位置、または署名とscopeを検証したafter ID |

`AuthorizeCreate`は共通JSON Schemaでunknown field等も検査する。APIで400を返すための構文・schema検査は認可の前に行える。policy自体は不正createを許可しないため、schema不適合もErrForbiddenとして扱う。

## 最小権限

| 役割 | 許可 |
|---|---|
| app | capture作成、自分のcapture読取、online job作成・一覧・取得・取消・再開、自分のonline artifact参照、所有確認した親へのfeedback |
| eval | eval job作成・一覧・取得・取消・再開、自分のeval artifact参照、承認済みdataset参照元のcapture読取 |
| transform | 自appのcapture読取 |

jobとartifactの所有範囲はapp＋role＋subject＋online/eval種別で確認する。transformはjobへアクセスできない。evalは任意のRawをAPIから読めず、storeが保存した`EvalReadable`のある同app captureに限る。このflagはdataset公開処理が出自を検査して設定する情報で、request bodyから設定できない。

feedbackではcapture/jobの全parentを確認し、before/after objectも保存metadataで所有者を照合する。object refのapp_idが自appでも、実体が別appなら拒否する。goldへの昇格やdataset更新の権限は付与しない。

V0はtext profileのみ。appはonline、evalはevalに固定し、job作成にはmappingで決まったRuntimeClientが必要。新規role・操作・profileは明示追加しない限り拒否する。Polarisのtable権限とS3のobject権限は各サービス側で別途強制する。

## エラーと列挙の防止

| エラー | HTTP分類 | 例 |
|---|---|---|
| identity.ErrUnauthorized | 401 | token不正。認証層で処理 |
| ErrForbidden | 403 | 禁止操作・role・profile、createのapp偽装、資格override |
| ErrNotFound | 404 | 存在しないID、別app・別owner・別種別のID、改変・期限切れ・別scope cursor |
| ErrUnavailable | 503 | metadata storeの障害。内部エラー詳細を返さない |

未知IDと所有範囲外IDを同じ分類にする。許可されない操作はresource検索前に拒否する。store adapterは未検出をpolicy.ErrNotFoundへ対応付け、I/Oエラーを未検出に偽装しない。

## 一覧とcursor

cursorはHMAC-SHA256でscope・after ID・期限を署名する。既定15分、keyは32 byte以上で設定する。署名keyは秘密として管理し、再起動を跨いでcursorを有効にする場合は同じkeyを復元する。key変更は旧cursorを無効化する。

cursorは暗号化されておらず、内容を読むと自分のscopeが見える。他scopeのcursorは署名が有効でも拒否する。read結果のafter IDだけを、先にscopeを適用したqueryへ渡す。全appのcursor・総件数・順位を共通にしない。

## 検証範囲

`go test -race ./internal/policy`で、get/cancel/retryの同app・別app・別owner・別kind、artifact、approved Raw、createのbody偽装と未知資格field、feedback parent、scope固定、他role・他app cursor、改変と期限切れcursorを検査する。store障害の内部詳細も公開しない。

このtaskはpolicyのunit検証。実HTTP handler、Polaris grants、S3 policyを同時に通す分離確認はT025で行う。
