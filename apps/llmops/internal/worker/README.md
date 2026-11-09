# 共通workerの実行境界

`cmd/llmops-worker`はoperatorが用意した設定を読み、登録済みadapterの`prepare`／`grade`をstdin/stdoutのversioned JSONで呼ぶ。料理のschemaを共有runnerへ埋め込まず、sample-text adapterでも同じ入口を使う。

## SDK

| 入口 | 契約 |
|---|---|
| `DecodeConfig`／`New` | app、transform/eval role、private temp、adapter registryを起動時に固定 |
| `Call(ctx,id,Request)` | request schema/domain版を検証し、登録binaryとSHA256が一致するときだけ起動 |
| `WithWorkspace(fn)` | trusted workerの一処理に0700の一時directoryを与え、終了時に削除 |
| `Objects()` | app固定のS3 clientを生成。専用role credential fileを明示し環境からroot資格を拾わない |
| `App()`／`Role()` | 起動時に確定したidentityの読出し。requestから変更不可 |

`Response.Error`はadapterの構造化domain errorであり、transportのerrorとは分ける。呼出側はこれを確認してからResultを処理する。実行失敗・期限・上限・不正応答のerrorにはraw stdout/stderrや入力を含めない。呼出元のcancelはcontext.Canceledとして返す。

Curate／dataset等の後続処理はこのSDKからRawを読み、adapterに原文を渡し、DuckDB側でtableへcommitする。adapter自身へS3資格・provider資格を渡さない。DuckDBの実行方式は後続のcatalog処理へ閉じ込め、共通SDKのdomain protocolへSQLを混ぜない。

## 設定

設定はJSON。adapter entryは絶対パスの`binary`、64桁SHA256、`domain_schema_version`を持つ。workerはリクエストにbinary／args／shellを指定させない。実行前にSHA256を再検証し、symlinkやgroup/otherが書けるbinaryを拒否する。registryは起動時にcopyして後から差し替えられないようにする。

`app`と`role`を要求bodyへ持ち込まず、処理全体で固定する。`storage`を設定する場合、credential_fileは絶対パスかつ`<role>-<app>.json`で、0600のregular fileに限る。中身は`access_key`と`secret_key`。このfilename照合は設定取り違えの防止であり、権限を証明するのは実S3 policyである。

実際のroleファイルはbootstrapが作る`temp/llmops-dev/storage/transform-konro.json`／`eval-konro.json`。全資格を含むdev.envをworkerへ渡さない。ファイルを更新した場合は短命workerを再起動し、ローテーション後の接続を確認する。S3 credential scopeはobject store側のapp×role policyと組み合わせて守る。

`storage`なしでは純粋なadapter実行だけを利用でき、Objectsは拒否する。これはsample demo用で、S3認可の成功を表すものではない。

## Processと資格の境界

子へ継承する環境はLANG、private HOME／TMPDIR／XDG_CACHE_HOMEだけ。親のtoken・AWS資格・HOME・PATHを継承しない。出力はschema、request ID、protocol versionを検証する。stdoutは設定上限、stderrは4 KiBに制限し、途中切れのJSON・余分な応答・request ID違いを拒否する。期限・cancel・上限時はprocess groupを終了する。

一時領域はapp/role別0700で分ける。設定したtemp rootが既存の公開directoryやsymlinkなら拒否する。Macの`/var`等の親aliasは起動時にcanonical pathへ解決する。完了・失敗の両方でattempt directoryを削除する。

これは**信頼したadapter binaryを呼ぶprocess境界であり、OS sandboxではない**。同じOS userのファイルやnetworkへの一般的なアクセスを強制禁止する機能は持たない。任意利用者のbinaryを登録する用途では使わない。adapterのpure function契約はコードと試験で確認し、外部提供binaryや強い実行隔離が必要になれば専用sandboxを導入する。hash検証とexecの間も、operatorが管理するbinaryを変更しないことが前提。

## 実行と検証

repo rootの`apps/llmops/tests/integration/worker/run.sh`はworkerとsample adapterをbuildし、tempにhash固定configを作ってprepare／gradeを試す。exec operationとcommand fieldの拒否もassertする。Docker・provider・S3には接続しない。

unit試験はactual child processで、環境secret除外、workspace削除、timeout、過大stdout/stderr、不完全JSON、別protocol／ID、binary hash違い、role credential取り違え、公開credential fileを検証する。副作用を要求する不正operationがchildへ到達しないことも確認する。強いOS隔離を試験したことにはしない。

Goのworker packageはDarwin/Linuxのprocess groupを使う。Windows対応は初版対象外。
