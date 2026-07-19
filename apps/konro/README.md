# konro（仮称）

作り置き調理サポートアプリ。調理セッション中に複数レシピを「コンロの口を切り替えるように」タブで並行閲覧・進行管理する。kensan と同じファイルベース設計（レシピ 1 品 = Markdown + frontmatter、DB なし）。

**Status: Phase 0 PoC** — Recipe Keeper エクスポート zip → レシピ md 変換のインポータのみ。アプリ本体（調理セッション UI）は Phase 1。

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
