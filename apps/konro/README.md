# konro（仮称）

レシピ管理・作り置き調理アプリ。閲覧・手入力作成・編集・URL／本文からの取り込みを単独で行い、調理セッション中に複数レシピを「コンロの口を切り替えるように」タブで並行閲覧・進行管理する。kensan と同じファイルベース設計（レシピ 1 品 = Markdown + frontmatter、DB なし）。

**Status: 単独レシピ管理 + 並行調理** — 一覧から閲覧／編集と調理選択を分ける。Recipe Keeperは任意の移行元で、導入にエクスポートは不要。

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
