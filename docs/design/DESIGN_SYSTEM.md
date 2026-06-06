# kensan-lab Design System

**Whetstone — kensan-lab platform design system, v1.0**

> 砥石 (whetstone) で道具を研ぐように、プラットフォーム上のあらゆる app を同じ言語で磨き上げるための単一の真実。

このドキュメントは **AI コーディングエージェントが最初に読む**こと、そして **人間の設計判断の根拠を残す**ことの両方を目的としている。

---

## TL;DR — エージェントへの命令書

UI を書く / 直す前に **必ず守る** 5 つだけ：

1. **トークン以外の色・サイズを使わない。** `bg-brand`, `text-muted-foreground` のような Tailwind の semantic クラスだけ。`#0EA5E9` や `bg-[#fff]` は禁止。
2. **新しいコンポーネント variant を発明しない。** Button / Badge / Card 等の variant は本ドキュメントに列挙したものだけ。足りないと思ったら**まず議論**。
3. **見出しは `.h-serif`、本文は sans 既定、数値・コード・ID は `font-mono` + `.tnum`。** 混ぜない。
4. **密度は `data-density` で切替。** 各コンポーネントの padding / 高さ / フォントサイズを個別に上書きしない。
5. **新しいコンポーネントを作る前に `docs/design/components.md` を読む。** 既存で表現できないか確認する。

---

## 1. ブランドの軸

| 軸 | この値 | 理由 |
|---|---|---|
| 語 | **研鑽** (kensan) | プラットフォーム名そのもの。「磨く・研ぎ澄ます」のメタファー |
| シンボル | 砥石 + 成長曲線 + スパークル | `docs/design/brand/kensan-logo-*.svg`。AI/手作業の協働で技を研ぐ |
| 主色 | **Sky 600** `#0284C7`（ライト）/ **Sky 400** `#38BDF8`（ダーク） | 既存ロゴの sky blue を1段絞った彩度 |
| 紙肌 | 暖色オフホワイト `#F5F2EC` | 青寄りの白（蛍光灯的）を避け、目に優しい温度 |
| 見出し書体 | **Noto Serif JP** | 研鑽の語の重みを取り戻す。見出しと数値の一部にだけ |
| 地書体 | **Inter + Noto Sans JP** | 高い可読性。本文・UIラベル |
| 数値書体 | **JetBrains Mono** | tabular-nums で縦に揃う |

**伝えたい印象**: 知性的 / 落ち着いている / でも研究室っぽい遊び心。
**伝えたくない印象**: 派手 / ガジェット臭 / 何でもAI / SaaSのテンプレ。

---

## 2. レイヤー構造

```
┌─────────────────────────────────────────────────────────┐
│  apps/<app>/                                             │
│   └─ React + Tailwind v4 + shadcn/ui                     │
│       └─ @import "@kensan-lab/design-tokens/tokens.css"  │
└─────────────────────────────────────────────────────────┘
                          ▲
                          │ 参照
                          │
┌─────────────────────────────────────────────────────────┐
│  packages/design-tokens/                                  │
│   ├─ tokens.css     ← 色・タイポ・余白・影・動き・密度    │
│   ├─ tokens.json    ← 同じ内容の構造化版                  │
│   └─ README.md                                            │
└─────────────────────────────────────────────────────────┘
                          ▲
                          │ 根拠
                          │
┌─────────────────────────────────────────────────────────┐
│  docs/design/                                             │
│   ├─ DESIGN_SYSTEM.md  ← 本書                             │
│   ├─ components.md     ← コンポーネント仕様              │
│   ├─ patterns.md       ← 画面パターン                     │
│   └─ brand/            ← ロゴ・OGP                        │
└─────────────────────────────────────────────────────────┘

.claude/rules/design-system.md  ← Claude Code が全 app で自動読込
```

**新しい app を立ち上げるとき**: `packages/design-tokens/tokens.css` を `@import` して、shadcn/ui CLI で components を生成すれば、本デザインシステムが効いた状態で開発が始まる。

---

## 3. トークン使用ルール

詳細は [`packages/design-tokens/README.md`](../../packages/design-tokens/README.md)（実装の真実）。要点：

### 3.1 色 — Semantic クラス一覧

| Semantic | Tailwind | 用途 |
|---|---|---|
| `--background` | `bg-background` | ページ全体の背景 |
| `--foreground` | `text-foreground` | 本文テキスト |
| `--card` | `bg-card` | カード・パネル |
| `--muted` | `bg-muted` | 控えめ背景・無効状態 |
| `--muted-foreground` | `text-muted-foreground` | サブテキスト |
| `--border` | `border` (auto) | ヘアライン罫線 |
| `--border-strong` | `border-border-strong` | セクション区切り |
| `--brand` | `bg-brand` / `text-brand` | **強調にだけ使う** |
| `--brand-muted` | `bg-brand-muted` | 選択時の淡い帯 |
| `--accent` / `--accent-foreground` | `bg-accent` / `text-accent-foreground` | ホバー・nav active |
| `--success` / `--warning` / `--destructive` / `--info` | 同名 | 状態色（dot+text 推奨） |

**Brand 色は1画面に主役を1つだけ。** 全タスクのチェックボックスが青、はNG。「今 NOW」「研鑽AI」「主要アクション」だけに絞る。

### 3.2 タイポグラフィ — 3書体ルール

- **明朝 (Serif)** = `class="h-serif"`
  - 用途: h1 / h2 / ページタイトル / セクションタイトル / カード head の h3
  - **混ぜない**: 本文や UI ラベルに明朝は使わない
- **サンセリフ (Sans)** = 既定
  - 用途: 本文・UIラベル・ボタン・密度高い表
- **モノスペース (Mono)** = `class="font-mono"`
  - 用途: 数値 / 時刻 / ID / コード / コマンド / IPアドレス
  - 数値は **必ず** `.tnum` (`tabular-nums`) を併用

### 3.3 角丸 — `--radius: 0.5rem` 基準

| | 値 | 用途 |
|---|---|---|
| sm | 4px | badge / tag |
| md | 6px | input / small button |
| **lg** | **8px** | **既定。card / button** |
| xl | 12px | dialog / popover |
| full | 999px | pill / avatar |

**1画面で `--radius` ファミリーを混ぜない。** カードが 8px なら、その中のボタンも 8px (またはbase-2px の 6px)。

### 3.4 影 — 最小限

- **既定: 影なし。** 罫線で区切る。
- `shadow-xs`: 罫線の代わり（ごく稀に）
- `shadow-sm`: ホバー時のカード
- `shadow-md`: dropdown / popover / tooltip
- `shadow-lg`: dialog / drawer
- **どこかで `shadow-md` 以上を常時表示してたら設計を疑う。**

### 3.5 動き — `ease-out` で素早く着地

- `duration-fast` (120ms): hover / click feedback
- `duration-base` (200ms): dialog open / drawer / tab switch
- `duration-slow` (320ms): page transition / large state change
- バウンス系 `ease-spring` は **意図的にカジュアルに見せたい時だけ**。AI承認ダイアログなど真面目な場面では `ease-out`

### 3.6 密度 — `data-density="comfortable" | "compact"`

- **app の root に1個だけ指定。** コンポーネント側で個別に上書きしない。
- `comfortable`: kensan のような生産性アプリ / 読み物
- `compact`: ダッシュボード / 監視 / 密な一覧

部分的に密度を変えたいときは、その親要素に `data-density="compact"` を付ければそのサブツリーだけ追従する。

---

## 4. コンポーネント — 列挙されたものだけ使う

詳細仕様: [`docs/design/components.md`](./components.md)

| Component | Variants | Sizes |
|---|---|---|
| **Button** | primary / secondary / outline / ghost / destructive / link | sm / md / lg + icon-only |
| **Badge** | brand / success / warning / destructive / muted / outline (+ dot variant) | 単一 |
| **Input / Textarea / Select** | default / error / disabled | 密度連動 |
| **Checkbox / Switch** | Checkbox=複数選択。Switch=即時反映 | — |
| **Card** | head / body / foot（任意組み合わせ） | — |
| **Tabs** | 下線型のみ | — |
| **Table** | 既定 / `compact` | 密度連動 |
| **Dialog** | 破壊的操作・確認 | — |
| **Alert** | info / success / warning / destructive | — |
| **Empty State** | 既定 | — |
| **Tooltip / Avatar / Skeleton** | 既定 | — |

**ここにないものを作るときは：**
1. 既存の組み合わせで表現できないか検証
2. それでも必要なら、`components.md` を更新する PR を出す
3. variant は最大限既存と一貫させる

---

## 5. 画面パターン — `patterns.md` 参照

主要パターンの索引：

- **App Shell**: サイドナビ + メイン（kensan 既定）／トップバー + メイン（ダッシュボード向け）
- **Header**: eyebrow + h-serif タイトル + 主アクション + サブテキストの4層
- **Empty / Loading / Error**: 必ず3状態をデザインする。「データなし」だけで終わらせない
- **Form**: label → field → hint の3層、エラーは色＋文章で
- **Confirmation**: 破壊的操作は Dialog。それ以外はインライン承認
- **Notification**: 永続→Alert、一時→Toast（未実装、必要に応じて追加）

---

## 6. やってはいけないこと（NG リスト）

| NG | 代わりに |
|---|---|
| `bg-[#0EA5E9]` のような hex 直書き | `bg-brand` |
| `style={{ color: "#1A1814" }}` | `className="text-foreground"` |
| `bg-blue-500` のような Tailwind raw palette | semantic（`bg-brand` 等） |
| 1画面で primary ボタンを 2 つ以上 | 1つに絞る（残りは ghost / outline） |
| 状態を色だけで表現（赤丸/黄丸/緑丸） | `<Badge dot>Synced</Badge>` で必ずラベルも |
| 数値を proportional font で並べる | `font-mono` + `.tnum` |
| カードに常時 shadow-lg | 罫線のみ、ホバーで `shadow-sm` |
| 同画面で角丸が混在（4px と 12px） | 同じ `--radius` ファミリーに揃える |
| 「タスクがありません」だけの空状態 | 説明 + 次の一手 + ボタンの3点セット |
| アイコンライブラリの混在 | **lucide-react** 一択 |
| 絵文字をUIに使う | 使わない。lucide のアイコンで表現 |

---

## 7. アクセシビリティの最低限

- フォーカスリングは必ず表示（`focus-visible` で `--ring`）
- color contrast は WCAG AA を満たす（本トークンは既に満たしている）
- 状態を色だけで伝えない（色 + アイコン or テキスト）
- インタラクティブ要素のヒット領域は **最低 32×32 px**（compact密度時）、推奨 **44×44 px**（comfortable）
- フォームには必ず `<label>`（または `aria-label`）

---

## 8. バージョニング

- **major**: semantic 名の削除・改名 / brand hue を 30deg 以上動かす
- **minor**: トークン追加 / ステータス色の微調整 / 新コンポーネント追加
- **patch**: 値のごく軽微な調整

各 app は `package.json` で `@kensan-lab/design-tokens` のバージョンを固定し、minor 以上は意図的にバンプする。

---

## 9. 参考リンク

- [`packages/design-tokens/`](../../packages/design-tokens/) — 実装の真実
- [`docs/design/components.md`](./components.md) — コンポーネント詳細仕様
- [`docs/design/patterns.md`](./patterns.md) — 画面パターン
- [`docs/design/brand/`](./brand/) — ロゴ・ファビコン・OGP
- [`.claude/rules/design-system.md`](../../.claude/rules/design-system.md) — Claude Code 向けルール
- [`docs/design/density-demo.html`](./density-demo.html) — 密度切替の視覚リファレンス

---

## 由来 (Changelog)

- **v1.0 "Whetstone"** (2026-05) — 初版。
  - AB-1 ハイブリッド（A骨格 + B肌）を採用
  - Sky 600/400 (light/dark) + 暖色オフホワイト #F5F2EC
  - Density 2モード (comfortable / compact)
  - 主要コンポーネント仕様確定
