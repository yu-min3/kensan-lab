# @kensan-lab/design-tokens

kensan-lab プラットフォーム全体で共有する**デザイントークン**（色・タイポ・余白・影・動き）の単一の真実（SSoT）。

> テーマ名: **Whetstone（砥石）** — 暖色オフホワイトの紙肌 × Sky 600 のブランド色 × 明朝の見出し。
> 「研鑽 = 砥石で磨く」というプラットフォーム名のメタファーを視覚言語に落とし込んだもの。

---

## このパッケージが提供するもの

| ファイル | 用途 |
|---|---|
| `tokens.css` | Tailwind v4 + shadcn/ui 互換の CSS 変数定義。React 系の app はこれを `@import` するだけ |
| `tokens.json` | 同じトークンの構造化版。Material-UI theme 生成、ドキュメンテーション、他ツール連携用 |

---

## 使う側（アプリ）の導入

### React + Tailwind v4 + shadcn/ui

```css
/* apps/<app>/src/index.css */
@import "tailwindcss";
@import "../../../../packages/design-tokens/tokens.css";   /* 現状は相対 import で共有 */
```

> pnpm/npm workspace を materialize したら `@import "@kensan-lab/design-tokens/tokens.css"` の
> package 指定に切替可（follow-up）。詳細は `docs/design/adoption-status.md`。

shadcn の `components.json` は `"baseColor": "slate"` / `"cssVariables": true` のままでOK。
追加で生成されるコンポーネントは自動で本トークンを使います。

### Backstage プラグイン

`tokens.json` を読み込んで MUI theme を生成（生成スクリプトは別パッケージで提供予定）。

### Plain HTML / その他

`tokens.css` を `<link>` か `<style>` で読み込み、`hsl(var(--background))` 形式で利用。

---

## トークンの三層

1. **Semantic** — アプリは原則ここだけ触る
   `--background` / `--foreground` / `--card` / `--primary` / `--secondary` / `--muted` / `--accent` / `--border` / `--ring` / `--brand` / `--brand-muted`

2. **Status** — 状態色
   `--success` / `--warning` / `--destructive` / `--info`（それぞれ `-foreground` 付き）

3. **Component-scoped** — 特定UI専用のオプトイン値
   `--timeblock-plan-bg` 等。新規追加は最小限に、まずは Semantic で表現できないか検討する。

---

## やってはいけないこと

- **生 hex 値を書かない。** `bg-[#0EA5E9]` のようなクラスは禁止。`bg-brand` を使う。
- **新しいセマンティック名を勝手に増やさない。** 例えば `--text-success` のような派生を作る前に、既存の `text-success` で済まないか確認する。
- **同じ画面内で `--radius` を混ぜない。** カードもボタンも同じ `--radius` ファミリーを使う。
- **影を多用しない。** 平面に区切るのは `border-*` が基本、影は `popover` / `dialog` / `hover` のみ。

---

## バージョニング

semver。**ブレイキングチェンジ**は以下に該当する変更：

- Semantic トークン名の削除・改名
- ライトモードでブランド色（`--brand`）の hue を 30deg 以上動かす
- background 系の lightness を 10pt 以上動かす

**マイナー** はトークン追加、ステータス色の微調整、ダーク値の微調整など。

---

## 関連ドキュメント

- `docs/design/index.md` — エージェント向け憲法（旧 DESIGN_SYSTEM.md、トークン使用ルールは §3）
- `docs/design/components.md` — コンポーネント仕様
- `docs/design/patterns.md` — 画面パターン
- `docs/design/density-demo.html` — 密度切替の視覚リファレンス
- `.claude/rules/design-system.md` — Claude Code が自動で読むルール

---

## 由来

- v1.0 "Whetstone" — 2026-05 初版。AB-1 ハイブリッド（A骨格 + B肌）から確定。
- v1.1 — 2026-06-06。motion トークンを Tailwind v4 の `--transition-duration-*` namespace に配線（`duration-fast/base/slow` ユーティリティが利用可能に）。
