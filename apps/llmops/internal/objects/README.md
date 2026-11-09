# 原形式objectと公開capture

`NewS3(S3Config)`はapp、bucket、region、endpoint、資格providerをoperator設定から固定する。環境変数のdefault credential chainを暗黙に使わない。AWS SDK for Go v2のS3 v1.114.9をpinし、[PutObject IfNoneMatch](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3#PutObjectInput)で条件付き作成を使う。

`NewStore(app, backend)`はbackendのappと一致する場合だけ作成できる。public入力はcapture key・MIME・source URI・SHA-256・原文body。bucket、prefix、object pathを受け取らない。

## 公開の順番

1. 原文を最大8 MiBまで読み、SHA-256・MIME・source URIを検査する。textはUTF-8、画像／PDFは宣言MIMEとmagic bytesを確認する。画像全体の復号やPDF解析は行わない。
2. appとcapture keyから安定capture IDを導出する。keyが同じなら同じ保存先へ行く。
3. `apps/<app>/raw/<capture-id>/source`へIf-None-Match *付きPUTを行う。
4. sourceをGETし、SHA-256・size・MIMEを検算する。先行writerが別内容を置いていたらconflict。
5. `manifest.json`を最後に条件付き作成し、公開済みmetadataを読み戻して返す。

原文は元のbytesでありJSONへ埋め込まない。metadataはmanifestに保持する。ETagをSHA-256として扱わず、S3のディレクトリrenameを仮定しない。

同app・同capture key・同bytes・同metadataは既存のtimestamp／IDを返す。別hash、MIME、source URIはErrConflict。異内容で先行sourceだけが残った場合も上書きしない。manifestのないsourceは未公開として扱い、GetCapture／GetArtifactから読み出せない。

応答喪失は同capture keyで再送する。source PUT後・manifest前の失敗ではsourceが残り得る。再送時は保存済みsourceを検証して公開を続ける。未公開sourceのGCは別の保持処理で扱い、通常再送で消さない。

## 取得と安全なID

GetCaptureはcapture IDから公開manifestを読み、原文の整合も確認する。GetArtifactは`raw_<capture-id>`だけを解決し、同じappの公開manifestを必須にする。外部URIや任意S3 keyは受け付けない。

APIで返すときの認可・attachment／nosniff header・原文本文のログ除外は上位APIの責務。app／roleを跨ぐ直接S3拒否は保存側policyでも強制し、資格を全Lake共通にしない。

## S3 HTTP境界

TLS 1.2以上、system CA＋明示CA、有限timeout、redirect拒否を使う。HTTPはliteral loopbackの開発接続だけを許す。S3 pathはapp固定prefixの下に限定し、dot segment・percent escape・外部URIを拒否する。

conditional PUTの412はErrExists、404はErrNotFound、その他のSDK errorは本文・endpoint・資格を含まないErrUnavailable。409の競合は勝手に条件を外さず、呼出側が同keyで再送する。SDKのretryもIf-None-Matchを維持する。

sourceは8 MiB、manifestは16 KiBを上限にGETする。schema／byte hash／所属app・capture ID／object ID・request hashが不整合ならErrCorruptを返し、不正manifestを公開成功として扱わない。

## 検証範囲

unit試験は原文bytesの保持、再送、別内容／metadataの拒否、同keyに30writerが競合する場合、公開前のartifact拒否、保存途中失敗、corruption、入力／サイズ／MIME検査を確認する。

SDKを通したHTTP試験はSigV4署名、固定bucket／prefix、If-None-Match、412／404、過大GET、他app key拒否、応答bodyのerror漏洩防止を確認する。これは実RustFSのpolicy検証ではない。実資格によるS3分離はintegration taskで確認する。
