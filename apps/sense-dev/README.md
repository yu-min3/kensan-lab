# sense-dev

sense 上で動く自動開発 controller の実装。現段階はファイル台帳、App/Platform の独立 context、版付き成果物配送、依存付きの fake worker 配車、独立 Release Gate の判定と publish intent、本人用の loopback 管理画面、公式 CLI アダプタまで。隔離 worker の明示的 opt-in 配線はあるが、service unit は模擬運転のみ。sense の rootfs/namespace/認証を検証した実モデル運転、GitHub 操作、日報送信、実機導入は未完了。

## 配置

- `internal/core/`: 単一 writer の台帳、immutable 成果物、team/agent context、message、Release Gate。
- `internal/web/`: loopback 管理画面。Whetstone の `packages/design-tokens/tokens.css` を原本として読む。
- `internal/isolation/`: Linux bubblewrap の worker 起動引数。モデル worker は専用 rootfs を読取専用、Sol の worktree と購読認証 home のみ書込可で渡す。検証器は認証 home を渡さず、ネットワークなし・worktree 読取専用で起動する。controller state/admin token の経路重複を拒否する。
- `internal/workerwire/`: controller→隔離 worker の版付き JSON 入力と session/result/failure イベントの契約。provider/model の組合せ、入力上限、未知のフィールドを拒否する。
- `internal/workerclient/` と `cmd/sense-dev-worker/`: bubblewrap 内の worker との stdin/stdout IPC。session ID を controller が永続化して ACK を返すまでモデル turn は開始しない。現 service unit からは起動しない。
- `internal/worktree/`: task ID ごとの自己完結した Git 作業ツリーを作成・再照合する。隔離内で元 repo の `.git` を参照しないよう、linked worktree ではなくローカル clone を使い、remote と共有 object を除去する。別 branch・symlink・base SHA の不一致を拒否する。連携 App 受入は元 repo ではなく、レビュー済み Platform task のコミットから専用 clone を作る。
- `internal/verifier/`: 実装と Opus レビューの間で、operator 固定の検証計画と `git diff --check` を credentialless sandbox で実行する。失敗時の出力を attempt に固定し、レビューへの昇格を止める。
- `cmd/sense-dev/`: private listener の入口。公開 IP・DNS 名の bind を拒否。既定は推論 off、既存 service unit は `-mock-worker` のみ。隔離実 worker は明示 opt-in と事前検査が必要。
- `cmd/private-route-probe/`: sense 導入後に別ホストから LAN IPv4 と global IPv6 の 8787 直結を試験。接続拒否だけを成功とし、timeout/経路なしを未知として残す。tunnel/proxy/CI の非公開証明は別途必要。
- `deploy/`: host systemd unit、初回導入・rollback、sense の read-only 棚卸しと導入後 listener smoke。実機未導入。

## 開発時の起動

`go test ./...` と `go vet ./...` をこのディレクトリで実行する。起動には保護された管理トークンファイル、絶対パスの state directory、repo の `packages/design-tokens/tokens.css` のパスが必要。トークンは32文字以上、mode 0600。コマンドに値を直接載せない。

起動例（パスとユーザーは sense の実測後に確定）:

```text
sense-dev -listen 127.0.0.1:8787 -data /var/lib/kensan-dev -admin-token-file /var/lib/kensan-dev/admin-token -tokens-css /opt/kensan-dev/source/packages/design-tokens/tokens.css
```

管理画面は最初 `127.0.0.1:8787` のみ。外出先からアクセスできるとはまだ判定しない。Cloudflare 等の connector は起動しない。添付の service unit は `-mock-worker` 付きで、実モデルや GitHub を一切起動しない。`-isolated-worker` は `-mock-worker` と排他で、bubblewrap・専用 rootfs・worker binary・`-source-repo`・`-worker-worktree-root`・2つの専用認証 home・管理 token が state 内にあること、`-verification-plan` で指定する operator 管理下の JSON、Yu と決めた `-inference-window HH:MM-HH:MM`（JST）を要求する。時間帯外では新規推論を配車せず、UI にも理由を表示する。起動時に Claude/Codex と credentialless verifier の sandbox を実行検査し、一つでも失敗すれば listener も配車も起動しない。実機未検証のため service unit には指定していない。`deploy/verification-plan.example.json` は sense-dev 自身のテスト用の例であり、Golden Path canary の検証計画を代用しない。対象変更に合う計画を作り、worker が書き込めない場所へ置く。検証側はネットワークを持たないため、依存は vendor などでオフライン利用可能にする。依存不足はテスト失敗として残し、レビュー合格に読み替えない。

## 既に検証できること

- state の atomic write と OS lock による単一 controller。再起動後に task、agent、message を復元。
- 実 controller process の E2E で、loopback 管理画面からの task 登録→HTTP client の待機中も mock 配車→SIGTERM→同じ state で再起動→既存 task/attempt 復元→新 task の自走を確認。これはローカル模擬運転であり、sense の systemd・host reboot・実モデルの復旧証拠ではない。
- App と Platform の profile/knowledge/memo を入力 manifest で区別し、受領した成果物だけを inbox に追加。
- fake worker で依存関係、provider ごと最大1実行、quota待機中の別 task、Mac優先、中断記録を試験。再起動中の実行は自動再送せず inspection 待ちにする。
- UI から変更 task を登録すると Fable→Astra→Sol→credentialless 検証→Opus の独立工程を一括作成。`-mock-worker` は各工程の台帳と入力 manifest を模擬実行し、`simulation_only` の成果物だけを残す。外部発行の根拠にはしない。
- 後続工程の入力 manifest は、それより前の工程の結果 artifact ID・版・SHA-256 と作成 agent をすべて明示する。前工程の私的会話は渡さず、成果物の破損・契約/SHAの変化で配車を止める。
- Codex App Server アダプタは ChatGPT account と quota を確認し、API key・モデル変更を拒否する単体試験まで。Astra 設計レビュー/Release Gate は `readOnly`、Sol 実装だけ `workspaceWrite` を thread/turn の両方へ指定。sense での本人認証と実モデル利用は未検証。
- Claude CLI アダプタは専用の private 設定ディレクトリ、subscription のログイン状態、危険な課金設定の不在を検査し、Read-only 工程に限定。fake CLI で stdin prompt とモデル相違拒否を試験。sense での本人認証と実モデル利用は未検証。
- bubblewrap 起動器の引数検査では host `/`、state と重複する mount、symlink alias、共有 auth home、runtime 外の実行ファイル、欠けた rootfs mount point を拒否。Fable/Astra/Opus は worktree を OS mount でも読取専用にし、Sol 実装だけ書込可。起動時 preflight は両モードを検査する。専用 rootfs 以外のホスト経路は mount しない。ネットワークは購読認証のため共有するので、egress 隔離ではない。sense で bubblewrap/namespace を実行した証拠はなく、既定の実配車は引き続き無効。
- 隔離 worker の通信契約は prompt と既存 session 以外の任意パス・token・コマンドを入力に持たず、未知の JSON フィールドと上限超過を拒否。client は session 永続化コールバック後だけ ACK を返し、結果先行・別 session・余分なイベントを拒否する。実機の OS 隔離・認証・モデル turn は未検証。
- 実 worker は task ID から専用 Git 作業ツリーを解決し、base SHA を台帳へ固定してから manifest/配車へ進む。準備に失敗した task は attempt を消費せず待機し、別 task は進められる。reviewer も自分の task 作業ツリーのみを読取専用で見る。clone は `.git` を内部に持ち、remote・alternates を持たず、元 repo を見失っても Git status が動くことを試験済み。Sol の回答だけでなく、commit 済み差分・base/head SHA・diff SHA-256 を immutable 成果物へ固定する。未コミット変更・空変更・大きすぎる差分ではレビューへ進まない。検証器は固定計画の実行結果を別 artifact に残す。ただし sense の bubblewrap 内での Git・テスト実測は未完了。
- `mock-` session を含む模擬 state は実 worker 起動時に拒否する。実認証後の運転には別の保護された state directory を準備し、模擬成果物を本番入力へ昇格させない。既存 state の削除・移行は自動実行しない。
- 版・hash・契約・SHA・宛先を検査する配送と、message ID による重複防止。UI で Platform task ID に紐づく App 受入を登録でき、実モデル工程の固定差分・credentialless 検証・Opus 合格がそろった場合だけ controller が3成果物を自動配送する。App はシナリオ・契約・SHA・期待/実測を固定した判定を返し、不合格は Platform 実装 agent に配送して実装/検証/レビューを新 session 世代で再実行する。修正 SHA の受領後、同じ App task の作業ツリーを fast-forward し再試験する。2修正後の再不合格は Yu 判断待ち。完了済み受入成果物の破損・不正形式は当該 App task だけ確認待ちへ隔離し、他案件の配車を止めない。mock 成果物や不合格レビューは合格配送に昇格しない。ここまでローカル試験で、sense 実モデル未検証。
- 作者とは別 session の Release Gate。`allow` には Sol の commit 済み差分、credentialless 検証の pass、Opus の SHA 固定 JSON 合格、controller 所有の Git 差分 scan、操作・配備先・影響・戻し方を固定した候補、実際の Astra 出力が必要。未束縛の Gate agent は配車しない。caller が Astra の `deny` や別操作を `allow` に読み替えることを拒否し、publish intent 作成時にも再照合する。scan は送信予定の全 commit を検査し、中間 commit の秘密・公開経路・binary・高リスク path を保留する。scan の `candidate` は機密なし・非公開の証明ではなく、独立 agent・CI・配備影響の追加確認が必要。外部操作前に intent を保存する。
- UI の loopback bind、管理 token login、HttpOnly cookie、CSRF、停止・Mac優先。provider の自動推論はまだ起動しない。
- UI から質問への回答と、Release Gate が `needs_human` とした操作の判断を記録。契約/SHA/操作/期限と再送IDを照合し、同じ送信は冪等、古いカードは拒否する。承認記録だけでは publisher は起動せず、独立 Gate の新しい `allow` が必要。
- 回答草稿は同じブラウザタブの `sessionStorage` で再読込・再認証から復元。送信が確定した質問の草稿は削除する。保存できないブラウザでも通常フォームは利用可能。
- JST日付で日報 preview を1日1件だけ保存。別の独立 timer が毎日 00:05 以降に前日分を immutable な outbox 原本へ固定し、宛先待ち・送信中・送信済み・送信不明・欠測を区別する。再起動時の送信中は `unknown` に移して自動再送しない。宛先未設定のため実送信はまだない。
- 既存 Playwright 基盤で 360/390/430 CSS px の依頼・Mac優先・停止・日報previewと横はみ出しを Chrome で確認。Agent は工程順に並べ、依存待ち・未設定などの理由を表示。実機 Safari/Chrome と外出先経路は未検証。

## 未完了と再開点

1. 専用 rootfs と canary に対応する固定検証計画を用意し、sense で bubblewrap/user namespace の fail-closed preflight、state/token不可視、credentialless verifier のネットワーク/認証遮断、実プロセス kill と再起動照合を通す。auth待機と中断照合・再開、実検証器の運転証拠は未完了。現 service unit は fake runner 専用。Claude の書込系 tool は隔離 worker が完成するまで解放しない。
2. Release Gate の CI/PR本文/添付/公開経路/配備影響/可視性を実状態に照らし、限定 publisher と外部操作の reconcile を実装する。構造化判定との一致はローカル試験のみで、実モデル判定の品質証拠はない。現時点の Git scan は一次スクリーニング、`PublishIntent` は dry-run 台帳だけで、GitHub へは送らない。
3. 管理画面に案件詳細を追加し、360/390/430 px と実機幅、切断復旧を検証する。App 差し戻し→Platform 修正→新 SHA の再試験は controller/作業ツリーのローカル試験までで、実モデル・sense 実機・既存 app の同一シナリオは未検証。質問/回答・SHA-bound 承認・日報 preview は HTTP テストまでで、実スマホ未検証。日報の宛先/送信 adapter と送信不明の実照合は未実装。timer/outbox はローカル試験のみ。
4. sense への SSH は復旧し、CPU/RAM/ディスクと単一ノード k3s 等を read-only 棚卸し済み。ただし別作業で k3s が停止・libvirt が導入されたため、この作業からの host 変更は調整待ち。bootstrap code/manifest と導入後 listener smoke は独立レビュー済みだが、実機導入・IPv4/IPv6/Cloudflare dashboard 等の公開経路・host reboot は未検証で `private-ready` 未達。詳細は `docs/private-ready-audit.md`。
5. 本人の初回認証と subscription 費用経路を確認後、実モデルの4工程と独立検証を接続する。Cloudflare は公開承認まで inactive。

実装契約は workspace の `projects/kensan-lab/docs/goal.md`。この README は実装の現在地だけを示す。

## sense の G0/G4a 棚卸しと再確認

最初の read-only 棚卸しは実施済み。実機導入前には通常の operator アカウントから `deploy/sense-readonly-inventory.sh` を `bash` で再実行する。スクリプトは `sudo`、サービス停止、Secret/ConfigMap の内容表示、プロセス引数・環境変数の表示をしない。結果は保護された証拠として扱い、公開 issue/PR やチャットに生データを貼らない。失敗した項目は `UNAVAILABLE` のまま記録し、非公開・停止安全の証明に読み替えない。

旧 k3s の node が複数、共有 PV/PVC や他ホスト依存がある、quorum/復旧経路が不明な場合は停止しない。停止が許される場合でも、対象 unit/コンテナ、データ/volume、現在の自動起動状態、戻すコマンドを先に記録して可逆操作だけを行う。host の待受一覧だけではインターネット非公開を証明できないため、既存 tunnel/proxy/Ingress、IPv4/IPv6、router/firewall、CI preview/GitOps の経路を別途 read-only で照合する。これらが未確認なら `private-ready` は保留する。
