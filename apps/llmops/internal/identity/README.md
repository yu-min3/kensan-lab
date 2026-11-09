# Service identity

Platform API向けのRS256 access tokenを検証し、署名済み`azp`をoperator管理のclient mappingへ対応付ける。app／role／runtime clientはtoken本文やHTTP bodyの任意フィールドから取得しない。resourceの操作権限・所有者判定はpolicy層が担う。

## 呼出契約

`New(Config)`でissuer・audience・固定JWKS URL・mapping JSONを設定する。mappingは共通`client-mapping` schemaで検査し、各clientのaudienceが設定と一致することを要求する。`Verify(ctx, token)`はBearer接頭辞を除いたtokenを受け取り、検証済み`Principal`を返す。失敗は本文やtokenを含まない`ErrUnauthorized`に統一する。

署名は`golang-jwt/jwt/v5` v5.3.1で検証し、許可algorithmはRS256のみ。issuer完全一致、audience（配列なら対象を含む）、必須expiration、存在するnbf、非空subject、必須azp、登録済みclientを確認する。client_idも存在する場合はazpとの一致を要求する。JWTのjku／x5uを取得先に使用しない。

`Principal`はClientID、AppID、Role、Subject、RuntimeClient、PolicyVersionを持つ。RuntimeClientはmappingの固定値であり、呼出元からdownstream資格を選ばせない。

## JWKSと更新

既定TTLは5分。期限内の既知kidはcacheを使い、未知kidまたは期限切れでは固定URLから再取得する。未知kidを大量送信しても毎回外部取得しないよう、再取得間隔は既定1秒。鍵ローテーション直後の未知kidはこの間隔内では一時拒否になり、間隔後の再送で更新できる。

取得は5秒の既定timeout、1 MiBのbody上限、100 key上限を持つ。redirectは追わない。RSA signing keyのみ利用し、重複kid、2048 bit未満／8192 bit超のmodulus、不正exponent、空の有効key集合を拒否する。更新に失敗したときはcacheを破棄し、期限切れ・取得失敗の鍵へfallbackしない。

issuerとJWKS URLは通常HTTPS。開発用`AllowLoopbackHTTP`を明示した場合に限り、文字列で指定したloopback IPのHTTPを許す。DNS名localhostはこの例外に含めない。通常のTLS検証は有効であり、独立したlocal CAが必要な場合は正しくtrustしたHTTPClientを供給する。

## 検証

`go test -race ./internal/identity`は標準crypto/rsaで独立生成した署名とTLS JWKS serverを使う。issuer／audience／expiration／nbf／azp／subject／clientの差替え、別署名key、algorithm混同、鍵ローテーション、cache期限・更新失敗・取得回数、重複kid、弱いkey、redirect、設定schemaを検証する。

これはJWT verifierの単体検証。実Keycloakのclient設定とPolaris／S3の権限を通す統合確認は、サービス構築後の分離試験で実施する。
