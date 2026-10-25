# konro（仮称）

レシピ管理・作り置き調理アプリ。閲覧・手入力作成・編集・URL／本文からの取り込みを単独で行い、調理セッション中に複数レシピを「コンロの口を切り替えるように」タブで並行閲覧・進行管理する。kensan と同じファイルベース設計（レシピ 1 品 = Markdown + frontmatter、DB なし）。

**Status: 単独レシピ管理 + 並行調理 + 買い物・作り置き提案** — 一覧から閲覧／編集と調理選択を分ける。Recipe Keeperは任意の移行元で、導入にエクスポートは不要。

## 構成

```
backend/   Go 単一サービス（REST API + ビルド済み SPA 配信 + konro-import CLI）
frontend/  React SPA（Vite。Whetstone共通トークン + キッチン用の操作領域）
```

## 開発・ローカル実行

```bash
# 1) 任意: 過去レシピをZIP移行する（未実施でもアプリは使える）
cd backend && go build -o konro-import ./cmd/konro-import
./konro-import -zip RecipeKeeper_export.zip -out ~/konro-data

# 2) frontend ビルド → Go が配信
cd ../frontend && npm install && npm run build
cd ../backend && KONRO_STATIC_DIR=../frontend/dist go run ./cmd/konro
# → http://localhost:8090

# 開発時は vite dev server（API は :8090 に proxy）
cd frontend && npm run dev   # localhost:5173
```

環境変数: `KONRO_DATA_DIR`（既定 `~/konro-data`）、`KONRO_ADDR`（既定 `:8090`）、`KONRO_STATIC_DIR`（未設定 = API のみ）。

Android 実機評価: 同一 LAN で `http://<MacのIP>:8090`。**Wake Lock は secure context 限定**なので、通し評価は USB + `adb reverse tcp:8090 tcp:8090` → スマホから `http://localhost:8090`。

## レシピ取込

Recipe Keeper のエクスポート zip（`recipes.html` + `images/`）を、`type: recipe` frontmatter 付き md + 画像に変換する。入口は 2 つ:

**UI / API（推奨）** — レシピ一覧ヘッダの「ZIP取込」から zip をアップロード。実体は `POST /api/v1/import`（zip をそのまま body で送る。multipart ではない — コンテナは readOnlyRootFilesystem で multipart のディスクスピル先が無い）。上限 64MB、結果は JSON レポート（件数・warnings・unknown itemprops）:

```bash
curl -X POST --data-binary @export.zip http://localhost:8090/api/v1/import
```

ファイル名はタイトル由来なので、再取込は未編集レシピのみ同名更新する。Konroで編集済みの品は保持し、結果の`skipped`と警告で報告する。Recipe Keeper で削除した品は konro には残る（消したければファイルを直接消す）。

**CLI（konro-import）** — サーバを経由せずローカル変換したいとき:

```bash
cd backend
go build -o konro-import ./cmd/konro-import
./konro-import -zip export.zip -out recipes-out
```

フォーマットは公式仕様が無く、サードパーティインポータ（[Mealie PR #3642](https://github.com/mealie-recipes/mealie/pull/3642)）からの逆算。認識できない `itemprop` はすべて unknown itemprops として報告するので、実エクスポートに対する形式検証を兼ねる（UI では「⚠ 未知フィールド N 種」表示）:

```
recipes:  2 written to recipes-out (recipes with empty sections: 0)
images:   1
unknown itemprops (format drift — inspect these):
  - recipeFutureUnknownField (1)
```

## レシピファイル形式

```markdown
---
type: recipe
title: "鶏むね肉の南蛮漬け"
tags: ["作り置き", "主菜"]
servings: "4人分"
prep_time: PT15M      # ISO8601
cook_time: PT20M
source: "https://..."
rating: 4
images: ["images/nanban.jpg"]
---

## 材料

- 鶏むね肉 2枚

## 手順

1. 鶏むね肉をそぎ切りにする

## メモ
```

`## 手順` の番号付きリスト 1 項目 = 1 ステップが調理ビューのステップ分割の契約。

## Web・本文からの取り込み

一覧の「レシピを追加」からURL／本文を入力し、下書きを確認・編集して保存する。
JSON-LD抽出と手入力は単独で利用でき、agent-runtimeを設定すると購読枠での抽出・整形を
選べる。取り込んだ品は既存の並行調理画面へそのまま追加できる。
接続設定、保存先、対応範囲、E2E手順は[取り込みとruntimeの境界](docs/agent-import.md)を参照。


## レシピの閲覧・手入力・編集

一覧のカードは閲覧画面を開き、「調理に選ぶ」で別途セッションへ選択する。
「手入力で作成」から新規登録でき、閲覧画面の「編集する」でタイトル・人数・材料・手順・メモ・タグ・時間・出典・評価を更新できる。
保存API・競合・ZIP保護と配色の仕様は[単独レシピ管理](docs/library.md)を参照。

## 好きな作り手

「Webで探す」のカードにあるハート、または「好きな作り手を管理」から追加・解除する。管理画面ではYouTubeチャンネル／クラシル料理人の公式URLを確認して登録できる。名前だけでは本人を判定せず、公式ページで確認した固定IDを使う。追加・解除は直後の「取り消す」で戻せる（5分以内・サーバ再起動前・他の変更が入るまで）。

通常検索で見つかったお気に入りは全員を優先する。「追加でも探す」は最大3人で、その人の候補も探す。「この作り手で探す」では登録済みの作り手だけに絞る。設定変更は次の検索に反映し、表示中のカード順は動かさない。指定中の作り手を外した場合は通常検索に戻す。

設定は `KONRO_DATA_DIR/.settings/creators.json` に原子的に保存する。初回のみ笠原将弘・ジョージ（城二郎）・中村和成を登録し、全員外した後も空の一覧を維持する。同じサーバへ接続する端末で共有し、古い画面からの変更は409で検出する。1データディレクトリにつき1サーバで運用する。設定ファイルが壊れている場合は上書きせずエラーを示す。

この機能で生成AIやagent-runtimeは使わない。対応外サイトの作り手はまだ登録できない。

ブラウザ回帰テスト（外部HTMLのみfixture、API・保存・検索・ブラウザは実装を使用）:

```bash
cd backend
go test -c -o ../temp/favorites-http.test ./internal/api
cd ..
KONRO_FAVORITES_TEST_BINARY="$PWD/temp/favorites-http.test" node e2e/favorites.mjs
# Chrome実行ファイルを指定する場合は CHROME_BINARY を設定
```

## ブラウザの戻る・進む

一覧、Web検索、好きな作り手、取り込み、レシピ詳細はブラウザ履歴で移動できる。検索状態はタブ内のsessionStorageへ保存し、取り込みはURLに下書きIDを記録する。進み直しても保存済みの同じ下書きを復元し、新しい抽出を起動しない。

取り込み編集中に戻る場合は変更を下書きへ保存してから移動する。保存中は操作を止め、失敗時は画面・URL・入力を維持してエラーを表示する。通常レシピ編集の破棄確認は維持する。

AI抽出では、レシピと別に`assessment`で読み取れた範囲と不足理由を返す。会員限定等の原因は取得本文に根拠がある場合だけ説明し、画面へ表示する。サイトごとの制限判定は追加しない。説明がない古いジョブも復元できる。

一覧の個人評価は星・点数・ラベルで表示する。調理選択は開始ボタンの上へ全品の名前を並べ、絞り込みで隠れた品も確認・解除できる。

## 保存済みレシピの一覧

初回は自分の評価が高い順。同点は先頭の飾り記号を除いた日本語の名前順、最後にファイルIDで安定化する。「名前順」「調理時間が短い順」も選べ、不明な時間は末尾へ置く。並び順はlocalStorageへ保存する。

検索語・タグ・「すべて／★4以上／未評価」を組み合わせられる。条件と選択したレシピの順番、一覧のスクロール位置はタブ内のsessionStorageへ保存し、詳細から戻る・再読み込みで復元する。並べ替えても調理の選択順は変えない。保存済みレシピや評価の値は書き換えない。

「追加」メニューからURL・本文、手入力、ZIPを選ぶ。個人評価は星と点数の1行で表示する。今回調理する品はPCでは右側、スマホでは料理名を含む要約を下部に置き、展開して確認・解除できる。

最近追加した順は未対応。追加日時を保持する項目が必要なため、編集日時やファイル名で代用しない。


## 買い物と作り置き

選んだ料理から人数・倍率付きの買い物リストを作り、その分量で調理できる。分類タグでの絞り込み、作り置き案の固定・交換・採用、調理実績と好みの保存に対応する。詳細な保存契約・AI境界・制限と復旧は[買い物・作り置き・調理の接続](docs/meal-planning.md)を参照。
