# Brand assets — kensan-lab

ロゴ・ファビコン・OGP の単一の真実。**ここ以外でロゴを書き直さない**。

## ファイル

| ファイル | 用途 | サイズ |
|---|---|---|
| `kensan-logo-mark.svg` | マークのみ（light 背景向け） | 100×100（ViewBox） |
| `kensan-logo-mark-dark.svg` | マークのみ（dark 背景向け） | 100×100 |
| `kensan-logo-wordmark.svg` | マーク + "kensan-lab" + PLATFORM DESIGN SYSTEM | 320×100 |
| `favicon.svg` | ファビコン（簡略版） | 32×32 |
| `og-image.svg` | OGP / Twitter Card | 1200×630 |

## シンボルの意味

```
         ✦  ← スパークル：研鑽の瞬間、AI と人の協働で技を研ぐ
        ╱
       ╱
      ╱     ← 成長曲線（sky blue）：研ぎ澄まされて伸びる
     ╱
  ▔▔▔▔▔▔▔  ← 砥石（warm brown）：プラットフォームの基礎
```

**砥石（whetstone）で道具を研ぐように、プラットフォーム上のあらゆる app を同じ言語で磨き上げる**——というのがブランドの中核メタファー。

## カラー

| 要素 | Light | Dark | Token |
|---|---|---|---|
| 砥石 | `#7A6E5D` | `#A89C8A` | `--muted-foreground` 系 |
| 成長曲線 + ✦ | `#0284C7` | `#38BDF8` | `--brand` |
| 背景（OG/favicon） | `#F5F2EC` | — | `--background` |
| ワードマーク | `#171411` | `#F5F2EC` | `--foreground` |

## 使い方

### React + Tailwind

```tsx
// vite-plugin-svgr 等で SVG を component import（パスは各 app から brand/ への相対 or alias）
import Logo from "@/assets/kensan-logo-mark.svg?react";
<Logo className="w-6 h-6" />
```

> ロゴ SVG の正本はこの `docs/design/brand/` のみ。app へは build 時にコピー or alias で参照し、**app 内で再描画しない**。

### HTML
```html
<link rel="icon" href="/brand/favicon.svg" type="image/svg+xml" />
<meta property="og:image" content="https://platform.yu-min3.com/og-image.png" />
```

> OGP は **PNG での配信を推奨**（一部 SNS は SVG 非対応）。`og-image.svg` から `og-image.png` (1200×630) をエクスポートして配信。

## NG

- ❌ マークの **色を任意に変える**（青→緑など）。brand 色は `--brand` 経由でのみ
- ❌ 砥石の上下を **崩す**（成長曲線の起点は砥石上面に接していること）
- ❌ 角丸を増やす・減らす（砥石の `rx="2.5"` は固定）
- ❌ スパークルだけを単独で使う（マークの一部）
- ❌ PNG ラスタライズ済みのロゴを **ベクター用途で**使う

## 由来

- v1.0 (2026-05) — 初版。Whetstone テーマ確定時に作成。
