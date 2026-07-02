# Adoption Status — Whetstone Design System

## このコミットの範囲（design system 本体のみ）

| 内容 | 状態 |
|---|---|
| `packages/design-tokens/`（tokens.css / tokens.json / README） | ✅ 追加。色・タイポ・余白・影・動き・密度の単一の真実 |
| `docs/design/`（DESIGN_SYSTEM / components / patterns / brand） | ✅ 追加。憲法 + 仕様 + ブランド資産 |
| density トークンを消費する `.ds-*` utility | ✅ tokens.css に実装。`data-density` が実際に寸法へ反映（`density-demo.html` で証明） |
| `.claude/rules/design-system.md`（エージェント入口） | ✅ CLAUDE.md から参照。UI 作業前に自動で読まれる |

density の動作確認: [`density-demo.html`](./density-demo.html)（同一マークアップが comfortable / compact で変わる）。

## まだやっていない（follow-up — 別 PR）

| 課題 | 内容 | 優先 |
|---|---|---|
| **アプリへの採用** | まだどの app も未採用。`apps/kensan-legacy/`（旧 kensan）は引退方向、`apps/kensan/` は frontend 未着手。next の frontend ができ次第 **最初から Whetstone で構築**するのが筋。legacy を移行するかは要判断 | 中 |
| **monorepo / toolchain** | tokens は当面 **相対 import**（`@import "…/packages/design-tokens/tokens.css"`）で共有する想定。pnpm workspace 化 or npm workspaces 化して `@kensan-lab/design-tokens` の package 指定にするかは別途決定（pnpm は未導入） | 中 |
| **共通コンポーネント package 化** | `@kensan-lab/ui` は未作成。実装の正本は各 app の `components/ui/`（shadcn）。app が増えたら package 化を検討 | 低 |

## 検証メモ

- `density-demo.html` を Whetstone トークンで描画し、`data-density` の comfortable / compact 切替が寸法へ反映されることを screenshot で確認済み。
- tokens の値（パレット・タイポ・density）は設計フェーズで確定済み。本コミットはそれを実装・結線したもの。
