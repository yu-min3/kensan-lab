# sense-dev

sense 上で動く自動開発 controller の実装。現段階はファイル台帳、App/Platform の独立 context、版付き成果物配送、依存付きの fake worker 配車、独立 Release Gate の判定と publish intent、本人用の loopback 管理画面、公式 CLI アダプタまで。隔離 worker の明示的 opt-in 配線はあるが、service unit は模擬運転のみ。sense の rootfs/namespace/認証を検証した実モデル運転、GitHub 操作、日報送信、実機導入は未完了。

## 配置

- `internal/core/`: 単一 writer の台帳、immutable 成果物、team/agent context、message、Release Gate。
- `internal/web/`: loopback 管理画面。Whetstone の `packages/design-tokens/tokens.css` を原本として読む。
- `internal/isolation/`: Linux bubblewrap の worker 起動引数。専用 rootfs を読取専用、worktree と購読認証 home のみ書込可で渡す。controller state/admin token の経路重複を拒否する。実 worker にはまだ未接続。
- `internal/workerwire/`: controller→隔離 worker の版付き JSON 入力と session/result/failure イベントの契約。provider/model の組合せ、入力上限、未知のフィールドを拒否する。
- `internal/workerclient/` と `cmd/sense-dev-worker/`: bubblewrap 内の worker との stdin/stdout IPC。session ID を controller が永続化して ACK を返すまでモデル turn は開始しない。現 service unit からは起動しない。
- `internal/worktree/`: task ID ごとの自己完結した Git 作業ツリーを作成・再照合する。隔離内で元 repo の `.git` を参照しないよう、linked worktree ではなくローカル clone を使い、remote と共有 object を除去する。別 branch・symlink・base SHA の不一致を拒否する。
- `cmd/sense-dev/`: private listener の入口。公開 IP・DNS 名の bind を拒否。既定は推論 off、既存 service unit は `-mock-worker` のみ。隔離実 worker は明示 opt-in と事前検査が必要。
- `deploy/`: host systemd unit の候補と、sense の秘密値を出さない read-only 棚卸しスクリプト。実機未導入。

## 開発時の起動

`go test ./...` と `go vet ./...` をこのディレクトリで実行する。起動には保護された管理トークンファイル、絶対パスの state directory、repo の `packages/design-tokens/tokens.css` のパスが必要。トークンは32文字以上、mode 0600。コマンドに値を直接載せない。

起動例（パスとユーザーは sense の実測後に確定）:

```text
sense-dev -listen 127.0.0.1:8787 -data /var/lib/kensan-dev -admin-token-file /var/lib/kensan-dev/admin-token -tokens-css /opt/kensan-dev/source/packages/design-tokens/tokens.css
```

管理画面は最初 `127.0.0.1:8787` のみ。外出先からアクセスできるとはまだ判定しない。Cloudflare 等の connector は起動しない。添付の service unit は `-mock-worker` 付きで、実モデルや GitHub を一切起動しない。`-isolated-worker` は `-mock-worker` と排他で、bubblewrap・専用 rootfs・worker binary・`-source-repo`・`-worker-worktree-root`・2つの専用認証 home・管理 token が state 内にあること、および Yu と決めた `-inference-window HH:MM-HH:MM`（JST）を要求する。時間帯外では新規推論を配車せず、UI にも理由を表示する。起動時に Claude/Codex の両 sandbox で state 不可視、worktree/auth 書込、provider binary 存在を確認し、一つでも失敗すれば listener も配車も起動しない。実機未検証のため service unit には指定していない。

## 既に検証できること

- state の atomic write と OS lock による単一 controller。再起動後に task、agent、message を復元。
- App と Platform の profile/knowledge/memo を入力 manifest で区別し、受領した成果物だけを inbox に追加。
- fake worker で依存関係、provider ごと最大1実行、quota待機中の別 task、Mac優先、中断記録を試験。再起動中の実行は自動再送せず inspection 待ちにする。
- UI から変更 task を登録すると Fable→Astra→Sol→Opus の独立 agent を一括作成。`-mock-worker` は各工程の台帳と入力 manifest を模擬実行し、`simulation_only` の成果物だけを残す。外部発行の根拠にはしない。
- 後続工程の入力 manifest は、それより前の工程の結果 artifact ID・版・SHA-256 と作成 agent をすべて明示する。前工程の私的会話は渡さず、成果物の破損・契約/SHAの変化で配車を止める。
- Codex App Server アダプタは ChatGPT account と quota を確認し、API key・モデル変更を拒否する単体試験まで。Astra 設計レビュー/Release Gate は `readOnly`、Sol 実装だけ `workspaceWrite` を thread/turn の両方へ指定。sense での本人認証と実モデル利用は未検証。
- Claude CLI アダプタは専用の private 設定ディレクトリ、subscription のログイン状態、危険な課金設定の不在を検査し、Read-only 工程に限定。fake CLI で stdin prompt とモデル相違拒否を試験。sense での本人認証と実モデル利用は未検証。
- bubblewrap 起動器の引数検査では host `/`、state と重複する mount、symlink alias、共有 auth home、runtime 外の実行ファイル、欠けた rootfs mount point を拒否。Fable/Astra/Opus は worktree を OS mount でも読取専用にし、Sol 実装だけ書込可。起動時 preflight は両モードを検査する。専用 rootfs 以外のホスト経路は mount しない。ネットワークは購読認証のため共有するので、egress 隔離ではない。sense で bubblewrap/namespace を実行した証拠はなく、既定の実配車は引き続き無効。
- 隔離 worker の通信契約は prompt と既存 session 以外の任意パス・token・コマンドを入力に持たず、未知の JSON フィールドと上限超過を拒否。client は session 永続化コールバック後だけ ACK を返し、結果先行・別 session・余分なイベントを拒否する。実機の OS 隔離・認証・モデル turn は未検証。
- 実 worker は task ID から専用 Git 作業ツリーを解決し、base SHA を台帳へ固定してから manifest/配車へ進む。準備に失敗した task は attempt を消費せず待機し、別 task は進められる。reviewer も自分の task 作業ツリーのみを読取専用で見る。clone は `.git` を内部に持ち、remote・alternates を持たず、元 repo を見失っても Git status が動くことを試験済み。ただし bubblewrap 内での Git 実測と、コード差分を immutable 成果物へ取り込む verifier は未完了。
- `mock-` session を含む模擬 state は実 worker 起動時に拒否する。実認証後の運転には別の保護された state directory を準備し、模擬成果物を本番入力へ昇格させない。既存 state の削除・移行は自動実行しない。
- 版・hash・契約・SHA・宛先を検査する配送と、message ID による重複防止。
- 作者とは別 session の Release Gate。`allow` には author の成果物、controller 所有の Git 差分 scan、その両方を明示した Gate 入力 manifest、repo/ref/operation/base/head SHA の一致が必要。scan は送信予定の全 commit を検査し、中間 commit の秘密・公開経路・binary・高リスク path を保留する。scan の `candidate` は機密なし・非公開の証明ではなく、独立 agent・CI・配備影響の追加確認が必要。外部操作前に intent を保存する。
- UI の loopback bind、管理 token login、HttpOnly cookie、CSRF、停止・Mac優先。provider の自動推論はまだ起動しない。
- UI から質問への回答と、Release Gate が `needs_human` とした操作の判断を記録。契約/SHA/操作/期限と再送IDを照合し、同じ送信は冪等、古いカードは拒否する。承認記録だけでは publisher は起動せず、独立 Gate の新しい `allow` が必要。
- 回答草稿は同じブラウザタブの `sessionStorage` で再読込・再認証から復元。送信が確定した質問の草稿は削除する。保存できないブラウザでも通常フォームは利用可能。
- JST日付で日報 preview を1日1件だけ保存し、案件・判断リンクと未配信状態を表示。宛先未設定のため送信機構と日次 timer はまだない。
- 既存 Playwright 基盤で 360/390/430 CSS px の依頼・Mac優先・停止・日報previewと横はみ出しを Chrome で確認。Agent は工程順に並べ、依存待ち・未設定などの理由を表示。実機 Safari/Chrome と外出先経路は未検証。

## 未完了と再開点

1. 専用 rootfs を用意し、sense で bubblewrap/user namespace の fail-closed preflight、state/token不可視、実プロセス kill と再起動照合を通してから実 worker を controller の配車へ接続する。auth待機と中断照合・再開、verifier も未完了。現 service unit は fake runner 専用。Claude の書込系 tool は隔離 worker が完成するまで解放しない。
2. Release Gate の CI/PR本文/添付/公開経路/配備影響/可視性を実状態に照らし、限定 publisher と外部操作の reconcile を実装する。現時点の Git scan は一次スクリーニング、`PublishIntent` は dry-run 台帳だけで、GitHub へは送らない。
3. 管理画面に受入結果の入力・配送、案件詳細を追加し、360/390/430 px と実機幅、切断復旧を検証する。質問/回答・SHA-bound 承認・日報 preview は HTTP テストまでで、実スマホ未検証。日報の送信先と日次 timer は未実装。
4. sense へ read-only 接続して CPU/RAM/ディスク、旧 k3s/cluster membership、待受・既存公開経路を実測する。現在 SSH がタイムアウトするため、private-ready は未判定。
5. 本人の初回認証と subscription 費用経路を確認後、実モデルの4工程を接続する。Cloudflare は公開承認まで inactive。

実装契約は workspace の `projects/kensan-lab/docs/goal.md`。この README は実装の現在地だけを示す。

## sense に接続できた後の G0/G4a 棚卸し

最初は通常の operator アカウントから `deploy/sense-readonly-inventory.sh` を `bash` で実行する。スクリプトは `sudo`、サービス停止、Secret/ConfigMap の内容表示、プロセス引数・環境変数の表示をしない。結果は保護された証拠として扱い、公開 issue/PR やチャットに生データを貼らない。失敗した項目は `UNAVAILABLE` のまま記録し、非公開・停止安全の証明に読み替えない。

旧 k3s の node が複数、共有 PV/PVC や他ホスト依存がある、quorum/復旧経路が不明な場合は停止しない。停止が許される場合でも、対象 unit/コンテナ、データ/volume、現在の自動起動状態、戻すコマンドを先に記録して可逆操作だけを行う。host の待受一覧だけではインターネット非公開を証明できないため、既存 tunnel/proxy/Ingress、IPv4/IPv6、router/firewall、CI preview/GitOps の経路を別途 read-only で照合する。これらが未確認なら `private-ready` は保留する。
