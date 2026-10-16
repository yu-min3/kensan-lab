# レシピ取り込みとagent-runtimeの境界

URL／本文から下書きを作り、材料・手順を確認して保存する。構造化データの抽出と
手入力は単独で動く。AIは明示的に押したときだけ使う。

## 責務

| Konro | agent-runtime |
|---|---|
| URL取得、JSON-LD、本文、原文保持 | 購読認証、追加課金OFF確認、実行制限 |
| レシピ型、prompt、JSON schema、整形パターン | 汎用text job、状態、JSON結果 |
| 数値チェック、確認画面、Markdown保存 | 受付・取得・取消・再試行・冪等性・所有者分離 |

Go module、内部型、ファイルシステム、モデル資格、callbackは共有しない。
`internal/agentclient`にHTTP v1の外部DTOを閉じ込め、`internal/importer`は小さい
interfaceで接続する。runtimeの障害時も手編集と既存レシピの調理を続けられる。
初回providerはruntime側でClaude Codeを選ぶ。Konroの設定にprovider名やmodel名はない。

## 保存と互換性

- `data/.imports/<random-id>.json`: 原文、取得URL、抽出本文、編集内容、revision、job ID、送信key。公開ルートでは配信しない。
- `data/imported/<random-id>.md`: 確認済みレシピ。不変IDで作成し、一覧と既存調理セッションへ返す。
- 既存ZIPインポータはルート直下へ書くため、Web／本文のレシピと衝突しない。
- 曖昧な送信は保存済みkeyで再確認する。同じ下書きの保存は冪等。
- 下書きは明示保存とAI送信前に永続化する。編集中の入力をキー操作ごとには保存しない。
- ブラウザには直近の下書きIDだけを保存する。下書き一覧・削除・同じURLの統合は後続。

Konroは既存構成どおり単一writer／単一ユーザーを想定する。Markdownの直接編集や
複数Konroプロセスから同時編集した場合までrevisionで同期しない。

## 接続設定

未設定ならAIボタンを出さず、URL抽出・手入力・保存を使える。

| 環境変数 | 用途 |
|---|---|
| `KONRO_AGENT_URL` | agentdのorigin。HTTPS、または開発用のliteral loopback HTTP |
| `KONRO_AGENT_TOKEN_FILE` | 開発用Bearer tokenの0600ファイル。runtimeをloopbackモードで起動する |
| `KONRO_AGENT_CA_FILE` | runtime専用CAのPEMファイル。system CAへ追加してTLS検証する |
| `KONRO_AGENT_TOKEN_URL` | 本番のKeycloak token endpoint（HTTPS） |
| `KONRO_AGENT_CLIENT_ID` | service-only confidential client ID |
| `KONRO_AGENT_CLIENT_SECRET_FILE` | client secretの0600ファイル、またはPodの専用fsGroupでのみ読める0440 projected file |

本番は開発token変数を設定せず、Keycloakのclient credentialsを使う。
audience mapperとruntimeのclient allowlistを対応させる。Konroへ渡すのはサービスの
資格だけで、Claude／Codexの資格を渡さない。Secret供給と本番配備は別作業。
runtime側の起動・課金確認の条件は、そのリポジトリの`runtime/README.md`を参照する。

## HTTP API

| 操作 | エンドポイント |
|---|---|
| URLまたは本文から下書き作成 | `POST /api/v1/import/drafts` (`url`または`text`) |
| 取得・runtime状態確認 | `GET /api/v1/import/drafts/{id}` |
| 編集を保持 | `PATCH /api/v1/import/drafts/{id}` (`revision`, `content`) |
| AI抽出・整形 | `POST .../{id}/normalize` (`revision`, `mode: faithful/steps`) |
| 取消・枠再確認 | `POST .../{id}/cancel` / `retry` (`revision`) |
| AI結果の受け取りを終了し手入力へ | `POST .../{id}/manual` (`revision`) |
| 確認済みレシピを保存 | `POST .../{id}/save` (`revision`) |

`manual`はruntimeへ到達できないときもローカルで編集を解除する。接続先の実行が
続く場合があるが、後からその出力が編集内容を上書きすることはない。実行自体を
止めたい場合は`cancel`を使う。runtimeの実行上限は5分。

## 取得・検証の範囲

URLは標準portのHTTP/HTTPSのみ。全DNS応答を検査し、private／loopback／link-local／
metadata向けアドレス等を拒否する。検査したIPへ直接接続してDNS再解決を避ける。
redirectも毎回検査する。proxyを使わず、15秒・展開後5MiBまで。ブラウザのJSは実行せず、
ログイン・画像ダウンロード・取得拒否の回避もしない。取得できない場合は本文入力を使う。

JSON-LDのRecipe、@graph、配列、HowToSection／HowToStepを扱う。複数Recipeは最初を
下書きにして警告する。microdata、サイト固有HTML、動画、PDFは専用対応していない。
AIでは「表記を整える」「操作ごとに手順を分ける」を選べる。

runtimeは汎用JSON schemaを検証し、Konroはさらに必須フィールド、型、件数、改行、
長さを検証する。原文または既存下書きにない数字を含む結果は採用しない。
この検査は数値tokenの追加検出であり、単位変更・数値の取り違え・省略・意味の正確さを
保証しない。画面で分量・時間・温度・手順を確認して保存する。
材料の品名／数量／単位の完全構造化や、既存レシピ全件のv2移行は含めない。

## 検証

backendは`go test -race ./...`と`go vet ./...`、frontendは`npm run build`。
ブラウザ検証は実際のKonroと独立したruntimeのHTTP/state実装を使い、モデル推論だけを
runtimeの汎用テストfixtureへ差し替える。購読資格や従量APIは使わない。

1. runtime repoで`go test -c -o <absolute-path>/runtime.test ./runtime`。
2. Konro backendで`go build -o <absolute-path>/konro ./cmd/konro`。
3. frontendで`npm ci && npm run build`。
4. `AGENT_RUNTIME_TEST_BINARY`と`KONRO_TEST_BINARY`へそれぞれの絶対pathを設定し、frontendで`npm run test:e2e`。

既存Chromeでheadless実行するなら`CHROME_BINARY`へbinaryの絶対pathを指定する。
未指定ならPlaywright管理のChromiumを使う。結果・ログ・画像は`apps/konro/temp/`。
8シナリオで確認保存→調理、再読込、枠待ち、数値改変、schema違反、取消、runtime再起動、
接続断からの手編集を確認する。実subscription推論と実Keycloakへの接続は別途確認が必要。
