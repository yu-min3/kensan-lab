# sense-dev

sense 上で動く自動開発 controller の実装。現段階はファイル台帳、App/Platform の独立 context、版付き成果物配送、依存付きの fake worker 配車、独立 Release Gate の判定と publish intent、本人用の loopback 管理画面、公式 CLI アダプタまで。service unit の配車は模擬運転のみ。実モデルの実行ループ、GitHub 操作、日報、sense 実機導入は未完了。

## 配置

- `internal/core/`: 単一 writer の台帳、immutable 成果物、team/agent context、message、Release Gate。
- `internal/web/`: loopback 管理画面。Whetstone の `packages/design-tokens/tokens.css` を原本として読む。
- `internal/isolation/`: Linux bubblewrap の worker 起動引数。専用 rootfs を読取専用、worktree と購読認証 home のみ書込可で渡す。controller state/admin token の経路重複を拒否する。実 worker にはまだ未接続。
- `cmd/sense-dev/`: private listener の入口。公開 IP・DNS 名の bind を拒否。`-mock-worker` を明示した時だけ模擬配車する。
- `deploy/`: host systemd unit の候補。実機未導入。

## 開発時の起動

`go test ./...` と `go vet ./...` をこのディレクトリで実行する。起動には保護された管理トークンファイル、絶対パスの state directory、repo の `packages/design-tokens/tokens.css` のパスが必要。トークンは32文字以上、mode 0600。コマンドに値を直接載せない。

起動例（パスとユーザーは sense の実測後に確定）:

```text
sense-dev -listen 127.0.0.1:8787 -data /var/lib/kensan-dev -admin-token-file /var/lib/kensan-dev/admin-token -tokens-css /opt/kensan-dev/source/packages/design-tokens/tokens.css
```

管理画面は最初 `127.0.0.1:8787` のみ。外出先からアクセスできるとはまだ判定しない。Cloudflare 等の connector は起動しない。添付の service unit は `-mock-worker` 付きで、実モデルや GitHub を一切起動しない。

## 既に検証できること

- state の atomic write と OS lock による単一 controller。再起動後に task、agent、message を復元。
- App と Platform の profile/knowledge/memo を入力 manifest で区別し、受領した成果物だけを inbox に追加。
- fake worker で依存関係、provider ごと最大1実行、quota待機中の別 task、Mac優先、中断記録を試験。再起動中の実行は自動再送せず inspection 待ちにする。
- UI から変更 task を登録すると Fable→Astra→Sol→Opus の独立 agent を一括作成。`-mock-worker` は各工程の台帳と入力 manifest を模擬実行し、`simulation_only` の成果物だけを残す。外部発行の根拠にはしない。
- 後続工程の入力 manifest は、それより前の工程の結果 artifact ID・版・SHA-256 と作成 agent をすべて明示する。前工程の私的会話は渡さず、成果物の破損・契約/SHAの変化で配車を止める。
- Codex App Server アダプタは ChatGPT account と quota を確認し、API key・モデル変更を拒否する単体試験まで。sense での本人認証と実モデル利用は未検証。
- Claude CLI アダプタは専用の private 設定ディレクトリ、subscription のログイン状態、危険な課金設定の不在を検査し、Read-only 工程に限定。fake CLI で stdin prompt とモデル相違拒否を試験。sense での本人認証と実モデル利用は未検証。
- bubblewrap 起動器の引数検査では host `/`、state と重複する mount、symlink alias、共有 auth home、runtime 外の実行ファイルを拒否。専用 rootfs 以外のホスト経路は mount しない。ネットワークは購読認証のため共有するので、egress 隔離ではない。sense で bubblewrap/namespace を実行した証拠はなく、実配車は引き続き無効。
- 版・hash・契約・SHA・宛先を検査する配送と、message ID による重複防止。
- 作者とは別 session の Release Gate。`allow` には author の成果物、controller 所有の Git 差分 scan、その両方を明示した Gate 入力 manifest、repo/ref/operation/base/head SHA の一致が必要。scan は送信予定の全 commit を検査し、中間 commit の秘密・公開経路・binary・高リスク path を保留する。scan の `candidate` は機密なし・非公開の証明ではなく、独立 agent・CI・配備影響の追加確認が必要。外部操作前に intent を保存する。
- UI の loopback bind、管理 token login、HttpOnly cookie、CSRF、停止・Mac優先。provider の自動推論はまだ起動しない。
- UI から質問への回答と、Release Gate が `needs_human` とした操作の判断を記録。契約/SHA/操作/期限と再送IDを照合し、同じ送信は冪等、古いカードは拒否する。承認記録だけでは publisher は起動せず、独立 Gate の新しい `allow` が必要。
- 回答草稿は同じブラウザタブの `sessionStorage` で再読込・再認証から復元。送信が確定した質問の草稿は削除する。保存できないブラウザでも通常フォームは利用可能。
- JST日付で日報 preview を1日1件だけ保存し、案件・判断リンクと未配信状態を表示。宛先未設定のため送信機構と日次 timer はまだない。
- 既存 Playwright 基盤で 360/390/430 CSS px の依頼・Mac優先・停止・日報previewと横はみ出しを Chrome で確認。Agent は工程順に並べ、依存待ち・未設定などの理由を表示。実機 Safari/Chrome と外出先経路は未検証。

## 未完了と再開点

1. 専用 rootfs と worker プロセスの IPC/session 固定を実装し、sense で bubblewrap/user namespace の fail-closed preflight を通してから実 worker を接続する。auth待機と中断照合・再開、verifier も未完了。現 service unit は fake runner 専用。Claude の書込系 tool は隔離 worker が完成するまで解放しない。
2. Release Gate の CI/PR本文/添付/公開経路/配備影響/可視性を実状態に照らし、限定 publisher と外部操作の reconcile を実装する。現時点の Git scan は一次スクリーニング、`PublishIntent` は dry-run 台帳だけで、GitHub へは送らない。
3. 管理画面に受入結果の入力・配送、案件詳細を追加し、360/390/430 px と実機幅、切断復旧を検証する。質問/回答・SHA-bound 承認・日報 preview は HTTP テストまでで、実スマホ未検証。日報の送信先と日次 timer は未実装。
4. sense へ read-only 接続して CPU/RAM/ディスク、旧 k3s/cluster membership、待受・既存公開経路を実測する。現在 SSH がタイムアウトするため、private-ready は未判定。
5. 本人の初回認証と subscription 費用経路を確認後、実モデルの4工程を接続する。Cloudflare は公開承認まで inactive。

実装契約は workspace の `projects/kensan-lab/docs/goal.md`。この README は実装の現在地だけを示す。
