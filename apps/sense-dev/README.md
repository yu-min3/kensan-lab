# sense-dev

sense 上で動く自動開発 controller の実装。現段階はファイル台帳、App/Platform の独立 context、版付き成果物配送、依存付きの fake worker 配車、独立 Release Gate の判定と publish intent、本人用の loopback 管理画面、公式 CLI アダプタまで。service unit の配車は模擬運転のみ。実モデルの実行ループ、GitHub 操作、日報、sense 実機導入は未完了。

## 配置

- `internal/core/`: 単一 writer の台帳、immutable 成果物、team/agent context、message、Release Gate。
- `internal/web/`: loopback 管理画面。Whetstone の `packages/design-tokens/tokens.css` を原本として読む。
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
- Codex App Server アダプタは ChatGPT account と quota を確認し、API key・モデル変更を拒否する単体試験まで。sense での本人認証と実モデル利用は未検証。
- Claude CLI アダプタは専用の private 設定ディレクトリ、subscription のログイン状態、危険な課金設定の不在を検査し、Read-only 工程に限定。fake CLI で stdin prompt とモデル相違拒否を試験。sense での本人認証と実モデル利用は未検証。
- 版・hash・契約・SHA・宛先を検査する配送と、message ID による重複防止。
- 作者とは別 session の Release Gate。`allow` は委任操作、非公開、機密なし、可逆性、必要な CI を満たす場合だけ台帳に保存。外部操作前に intent を保存する。
- UI の loopback bind、管理 token login、HttpOnly cookie、CSRF、停止・Mac優先。provider の自動推論はまだ起動しない。
- UI から質問への回答と、Release Gate が `needs_human` とした操作の判断を記録。契約/SHA/操作/期限と再送IDを照合し、同じ送信は冪等、古いカードは拒否する。承認記録だけでは publisher は起動せず、独立 Gate の新しい `allow` が必要。
- 回答草稿は同じブラウザタブの `sessionStorage` で再読込・再認証から復元。送信が確定した質問の草稿は削除する。保存できないブラウザでも通常フォームは利用可能。
- JST日付で日報 preview を1日1件だけ保存し、案件・判断リンクと未配信状態を表示。宛先未設定のため送信機構と日次 timer はまだない。

## 未完了と再開点

1. 実 worker の権限隔離、auth待機と中断照合・再開、verifier を実装する。現 service unit は fake runner 専用。Claude の書込系 tool は隔離 worker が完成するまで解放しない。
2. Release Gate の証拠を実際の Git 差分/CI/公開経路に照らし、限定 publisher と外部操作の reconcile を実装する。現時点の `PublishIntent` は dry-run 台帳だけで、GitHub へは送らない。
3. 管理画面に受入結果の入力・配送、案件詳細を追加し、360/390/430 px と実機幅、切断復旧を検証する。質問/回答・SHA-bound 承認・日報 preview は HTTP テストまでで、実スマホ未検証。日報の送信先と日次 timer は未実装。
4. sense へ read-only 接続して CPU/RAM/ディスク、旧 k3s/cluster membership、待受・既存公開経路を実測する。現在 SSH がタイムアウトするため、private-ready は未判定。
5. 本人の初回認証と subscription 費用経路を確認後、実モデルの4工程を接続する。Cloudflare は公開承認まで inactive。

実装契約は workspace の `projects/kensan-lab/docs/goal.md`。この README は実装の現在地だけを示す。
