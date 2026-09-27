# sense-dev

sense 上で動く自動開発 controller の実装。現段階はファイル台帳、App/Platform の独立 context、版付き成果物配送、独立 Release Gate の判定と publish intent、本人用の loopback 管理画面まで。mission の自動配車、公式 provider 接続、GitHub 操作、日報、sense 実機導入は未完了。

## 配置

- `internal/core/`: 単一 writer の台帳、immutable 成果物、team/agent context、message、Release Gate。
- `internal/web/`: loopback 管理画面。Whetstone の `packages/design-tokens/tokens.css` を原本として読む。
- `cmd/sense-dev/`: private listener の入口。公開 IP・DNS 名の bind を拒否。
- `deploy/`: host systemd unit の候補。実機未導入。

## 開発時の起動

`go test ./...` と `go vet ./...` をこのディレクトリで実行する。起動には保護された管理トークンファイル、絶対パスの state directory、repo の `packages/design-tokens/tokens.css` のパスが必要。トークンは32文字以上、mode 0600。コマンドに値を直接載せない。

起動例（パスとユーザーは sense の実測後に確定）:

```text
sense-dev -listen 127.0.0.1:8787 -data /var/lib/kensan-dev -admin-token-file /var/lib/kensan-dev/admin-token -tokens-css /opt/kensan-dev/source/packages/design-tokens/tokens.css
```

管理画面は最初 `127.0.0.1:8787` のみ。外出先からアクセスできるとはまだ判定しない。Cloudflare 等の connector は起動しない。

## 既に検証できること

- state の atomic write と OS lock による単一 controller。再起動後に task、agent、message を復元。
- App と Platform の profile/knowledge/memo を入力 manifest で区別し、受領した成果物だけを inbox に追加。
- 版・hash・契約・SHA・宛先を検査する配送と、message ID による重複防止。
- 作者とは別 session の Release Gate。`allow` は委任操作、非公開、機密なし、可逆性、必要な CI を満たす場合だけ台帳に保存。外部操作前に intent を保存する。
- UI の loopback bind、管理 token login、HttpOnly cookie、CSRF、停止・Mac優先。provider の自動推論はまだ起動しない。

## 未完了と再開点

1. provider adapter と durable scheduler、task 依存、worker 実行領域、quota/auth 待機、復旧を実装する。
2. Release Gate の証拠を実際の Git 差分/CI/公開経路に照らし、限定 publisher と外部操作の reconcile を実装する。現時点の `PublishIntent` は dry-run 台帳だけで、GitHub へは送らない。
3. 管理画面に agent 割当、質問/回答、受入結果、SHA-bound 承認、日報を追加し、実機幅と切断復旧を検証する。
4. sense へ read-only 接続して CPU/RAM/ディスク、旧 k3s/cluster membership、待受・既存公開経路を実測する。現在 SSH がタイムアウトするため、private-ready は未判定。
5. 本人の初回認証と subscription 費用経路を確認後、実モデルの4工程を接続する。Cloudflare は公開承認まで inactive。

実装契約は workspace の `projects/kensan-lab/docs/goal.md`。この README は実装の現在地だけを示す。
