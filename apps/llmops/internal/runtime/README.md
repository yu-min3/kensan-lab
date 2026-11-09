# agent-runtime client

`New(Config)`でowner（app／role／subject）、runtime target、base URL、OAuth client IDとsecret fileを固定する。`Create / Get / Action`は毎回owner一致を確認し、呼出側からendpoint・資格を上書きできない。TargetIDとClientIDはjob台帳へ保存し、再起動時に同じ設定へ解決する。

OAuth Client CredentialsはHTTPS token endpointを使い、Basicのclient ID／secretをform encodingして送る。HTTPはliteral loopbackだけを開発用に許す。DNS名localhostやremote HTTPは拒否する。secret fileはprivate regular file（0400／0600）を使い、refreshごとに読み直す。tokenは期限前に更新し、401を受けたら次回呼出前に再取得する。同一Clientを複数appへ使い回さない。

runtimeとtoken取得の両方で、TLS 1.2以上、system CA＋明示CA、redirect拒否、有限timeoutを使う。TLS検証を無効化する設定は持たない。

## Requestとresponse

text profile、非空prompt、JSON input、objectのoutput schema、非空idempotency keyを受け付ける。prompt／schemaは各32 KiB、serialized requestは512 KiBまで。Createで使った同じkey・同じ入力を照合再送できる。client自身は自動で新keyを発行せず、失敗時に別targetへfallbackしない。

GET／actionは返却job IDと要求IDの一致も検査する。responseは1 MiBまで、既知の6状態だけを受け付ける。waitingとfailedは有効なjob状態でありtransport errorへ変換しない。model／usageが下流に存在しなければnullのまま保持する。

`Action(retry)`はwaiting jobの再開に使う。failedの再実行は上位APIが新attempt／新keyでCreateする。cancel／retry以外の任意actionは拒否する。

`Error.Kind`と`Status`は依存停止、認証拒否、404、409、429を区別する。上流body・URL・token・secretをerror textへ転記しない。429は受付上限であり、既存jobのwaiting状態と区別する。Createのtransport失敗は受理有無が不明なので、job台帳の同keyで照合する。

## 検証

HTTP試験で同Create再送、応答喪失、owner／入力の送信前拒否、既知状態、過大／不正response、認証失敗、quota、redirectとtimeoutを確認する。self-signed CAは未信頼時に失敗し、CA追加後はtoken/runtime双方で成功する。secret rotationと期限更新も検査する。実agentdのOIDC所有者境界は別のintegration taskで検証する。
