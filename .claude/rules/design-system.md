---
description: Whetstone design system — tokens, component variants, typography, and density rules for all platform UIs
globs: "apps/**, packages/design-tokens/**, backstage/**, docs/design/**"
---

# Design System (Whetstone)

プラットフォーム上の **全 UI（apps/*, Backstage plugin, 内部ダッシュボード）** が従う視覚言語。
UI を書く / 直す前に必ずこのルールを読む。

- 完全仕様: [`docs/design/DESIGN_SYSTEM.md`](../../docs/design/DESIGN_SYSTEM.md)（憲法）
- 実装の真実: [`packages/design-tokens/`](../../packages/design-tokens/)（tokens.css / tokens.json）
- コンポーネント仕様: [`docs/design/components.md`](../../docs/design/components.md)
- 画面パターン: [`docs/design/patterns.md`](../../docs/design/patterns.md)

## 必ず守る 5 つ

1. **トークン以外の色・サイズを使わない。** `bg-brand` `text-muted-foreground` のような semantic クラスだけ。`#0EA5E9` / `bg-[#fff]` / `bg-blue-500`（raw palette）は禁止。
2. **コンポーネント variant を発明しない。** Button / Badge / Card 等の variant は `components.md` に列挙したものだけ。足りなければ**まず議論 → components.md を更新する PR**。
3. **書体は 3 系統を混ぜない。** 見出し = `.h-serif`（明朝）、本文・UIラベル = sans 既定、数値・コード・ID = `font-mono` + `.tnum`。
4. **密度は `data-density` で切替。** app root に 1 個だけ。各コンポーネントの padding / 高さ / フォントサイズを個別に上書きしない。
5. **新コンポーネントを作る前に `components.md` を読む。** 既存の組み合わせで表現できないか確認する。

## 新規アプリの導入（React + Tailwind v4 + shadcn/ui）

```css
/* src/index.css */
@import "tailwindcss";
@import "../../../../packages/design-tokens/tokens.css";   /* 色・タイポ・余白・影・密度の単一の真実 */
```

shadcn の `components.json` は `"baseColor": "slate"` / `"cssVariables": true` のままで動く。

> **toolchain メモ**: 現状 pnpm は未導入で、monorepo は npm 運用。tokens は上記の **相対 import** で共有する。
> 将来 pnpm workspace を materialize したら `package.json` に `"@kensan-lab/design-tokens": "workspace:*"` を足し、
> `@import "@kensan-lab/design-tokens/tokens.css"` の package 指定に切り替えられる（follow-up）。

## アプリ採用状況

まだ Whetstone を採用したアプリは無い（このコミットは design system 本体のみ）。
`apps/kensan-legacy/`（旧 kensan、凍結）と `apps/kensan/`（新 / 現行）が対象候補。
採用する app は **`src/index.css` 内に `--background` 等の色トークンを再定義せず**、
`packages/design-tokens/tokens.css` を import する（package が唯一の真実）。
コンポーネント実装の正は各 app の `components/ui/`（shadcn）。新しい共通 UI も `components.md` の仕様に従う。
現状と follow-up: [`docs/design/adoption-status.md`](../../docs/design/adoption-status.md)。

## やってはいけないこと（抜粋）

| NG | 代わりに |
|---|---|
| hex 直書き `bg-[#0EA5E9]` / `style={{color:"#..."}}` | semantic（`bg-brand` / `text-foreground`） |
| 1 画面で primary ボタン 2 つ以上 | 1 つに絞り残りは ghost / outline |
| 状態を色だけで表現 | dot badge + ラベル文字 |
| 数値を proportional font で並べる | `font-mono` + `.tnum` |
| アイコンライブラリ混在 / 絵文字 | **lucide-react 一択** |
| `index.css` で色トークンを再定義 | `@kensan-lab/design-tokens` を import |
