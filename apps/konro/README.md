# konro（仮称）

作り置き調理サポートアプリ。調理セッション中に複数レシピを「コンロの口を切り替えるように」タブで並行閲覧・進行管理する。kensan と同じファイルベース設計（レシピ 1 品 = Markdown + frontmatter、DB なし）。

**Status: Phase 1 MVP（ローカル評価中）** — インポータ + レシピ一覧 + 調理セッション UI（コンロタブ・ステップ進行・タブ横断タイマー・Wake Lock・PWA manifest）。本番デプロイ（app-konro ns）は Phase 2。

## 構成

```
backend/   Go 単一サービス（REST API + ビルド済み SPA 配信 + konro-import CLI）
frontend/  React SPA（Vite。design system 非依存の軽量キッチン UI）
```

## 開発・ローカル実行

```bash
# 1) レシピデータ投入（初回のみ）
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

## konro-import（PoC CLI）

```bash
cd backend
go build -o konro-import ./cmd/konro-import
./konro-import -zip export.zip -out recipes-out
```

Recipe Keeper のエクスポート zip（`recipes.html` + `images/`）を、`type: recipe` frontmatter 付き md + 画像に変換する。

フォーマットは公式仕様が無く、サードパーティインポータ（[Mealie PR #3642](https://github.com/mealie-recipes/mealie/pull/3642)）からの逆算。認識できない `itemprop` はすべて `unknown itemprops` として報告するので、実エクスポートに対する形式検証を兼ねる:

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
