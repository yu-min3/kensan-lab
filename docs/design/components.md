# Components — kensan-lab Design System

> プラットフォーム上のすべての app が共通で使うコンポーネントの **完全な仕様**。
> 実装の正本は各 app の `src/components/ui/`（shadcn/ui ベース）。
> 新しい app は shadcn CLI で生成し、ここの仕様に合わせる。共通 package 化（`@kensan-lab/ui`）は follow-up。

---

## 設計原則

1. **一つの形に2つの密度**（comfortable / compact）。app 側で `data-density` を1箇所だけ指定すれば自動追従
2. **variant は最小限**。優先順位は variant ではなく **配置と1画面の主役性** で表現する
3. **色だけに頼らない**。状態は dot + ラベル、エラーは色 + 文章
4. **アイコンライブラリは lucide-react 一択**

---

## 00. Density

```html
<html data-density="comfortable">   <!-- app 全体 -->
<div  data-density="compact">…</div> <!-- 一部だけ密に -->
```

| | comfortable（既定） | compact |
|---|---|---|
| `--row-h` | 2.75rem | 2rem |
| `--control-h` | 2.25rem | 1.875rem |
| `--card-pad-y/x` | 1.25 / 1.5 rem | 0.75 / 1 rem |
| `--text-size` | 0.875rem | 0.8125rem |
| 用途 | 生産性アプリ・読み物・設定 | ダッシュボード・監視・密一覧 |

**切替コスト ゼロ**。CSS 変数が、それを **消費するコンポーネント** を引き連れて追従する。

### 密度を「実際に効かせる」消費クラス

トークンを定義しただけでは何も起きない。下の `.ds-*` クラス（`tokens.css` で定義）を実コンポーネントに付けて初めて `data-density` が見た目に反映される。

| クラス | 効果 | 主な用途 |
|---|---|---|
| `.ds-page` | `padding: var(--page-pad)` | ページ最外コンテナ |
| `.ds-card` | `padding: var(--card-pad-y/x)` | Card / Panel |
| `.ds-row` | `height: var(--row-h)` | テーブル行・リスト行・メニュー項目 |
| `.ds-control` | `height: var(--control-h)` | Button / Input の高さ |
| `.ds-section` | 縦 flex + `gap: var(--section-gap)` | カード間 |
| `.ds-stack` | 縦 flex + `gap: var(--gap-stack)` | カード内の縦並び |
| `.ds-inline` | 横 flex + `gap: var(--gap-inline)` | 隣接コントロール |
| `.ds-text` / `.ds-label` | `font-size`/`line-height` を密度連動 | 本文 / uppercase ラベル |

```html
<div data-density="compact">
  <div class="ds-card ds-stack"> … </div>   <!-- compact の padding/gap で描画 -->
</div>
```

ビジュアル証明: [`density-demo.html`](./density-demo.html)（同じカードが comfortable / compact で変わる）。

---

## 01. Button

```tsx
<Button variant="primary" size="md">タスクを追加</Button>
```

### Props
| prop | values | default |
|---|---|---|
| `variant` | `primary` `secondary` `outline` `ghost` `destructive` `link` | `secondary` |
| `size` | `sm` `md` `lg` | `md` |
| `iconOnly` | `boolean` | `false` |
| `loading` | `boolean` | `false` |
| `disabled` | `boolean` | `false` |

### ルール
- **1画面 = primary 1つ。** 残りは ghost / outline で
- destructive の最終ボタンには **必ずキャンセル**（ghost）を併置
- アイコンは lucide-react、サイズは 14 / 16 / 18px（sm/md/lg）
- icon-only は **必ず** `aria-label` を付与
- link variant はテキストフローの中だけ。スタンドアロンのアクションには使わない

---

## 02. Badge

```tsx
<Badge variant="success" dot>Synced</Badge>
```

### Variants
`brand` / `success` / `warning` / `destructive` / `muted` / `outline`

### ルール
- 状態列（status column）は **dot variant + ラベル文字** を必ずセットで
- text-only badge と dot badge を **1コンテキストで混在させない**
- ラベルは全角3〜6文字 or 半角6〜12文字を目安（はみ出すならカテゴリ設計が雑）

---

## 03. Input / Textarea / Select

```tsx
<div className="field">
  <label htmlFor="title">タスク名</label>
  <input id="title" className="input" />
  <div className="hint">空白を含めずに入力</div>
</div>
```

### States
default / `:focus` / `[disabled]` / `.field.error`

### ルール
- **label → field → hint の3層** を必ず守る
- ラベル省略は不可（`aria-label` で代替可）。プレースホルダはラベル代わりにならない
- エラーは色だけでなく **hint の文章**で何が悪いか伝える
- 数値入力は **mono + tabular-nums** 推奨（揃いが綺麗になる）

---

## 04. Checkbox / Switch

```tsx
<label className="check"><input type="checkbox" /><span className="box" />通知を有効化</label>
<label className="switch"><input type="checkbox" /><span className="track" />自動保存</label>
```

### 使い分け（**混用禁止**）
| | Checkbox | Switch |
|---|---|---|
| 意味 | **複数選択 / 任意フラグ** | **即時反映される設定** |
| 例 | 利用規約に同意 / 配信先選択 | 自動保存 / 通知 ON/OFF / ダークモード |
| 保存 | フォーム送信時 | 切替の瞬間 |

---

## 05. Card

```tsx
<Card>
  <CardHead title="今週のフォーカス" sub="2026-05-12 週" badge={<Badge>進行中</Badge>} />
  <CardBody>…</CardBody>
  <CardFoot>…</CardFoot>
</Card>
```

### ルール
- 既定は **border のみ、影なし**
- `:hover` の subtle な `shadow-sm` は許容（クリック可能なカードのみ）
- head / body / foot の3パートが基本構成。foot は任意
- h3 は **h-serif**

---

## 06. Tabs

```tsx
<Tabs defaultValue="overview">
  <Tab value="overview">概要</Tab>
  <Tab value="tasks">タスク</Tab>
</Tabs>
```

### ルール
- **下線型のみ**。pill 型は brand 色と競合するため不採用
- active は `font-weight: 600` + `border-bottom: 2px solid hsl(var(--brand))`
- 6個以上になるなら設計を疑え（Sidebar に出すかカテゴリ分割を検討）

---

## 07. Table

```tsx
<table className="table">
  <thead>…</thead>
  <tbody>
    <tr>
      <td><strong>cilium</strong></td>
      <td><Badge dot variant="success">Synced</Badge></td>
      <td className="num-cell">4/4</td>
    </tr>
  </tbody>
</table>
```

### ルール
- ダッシュボードでは **`data-density="compact"`** を親に（行高32px、font 13px）
- 数値・時刻・ID は **`.num-cell`** クラス（右寄せ + mono + tnum）
- 状態列は **dot badge** で
- **行全体クリック不可**。最終列にメニューボタン（`btn-ghost btn-icon`）を置く
- 横スクロールが発生するなら列構成を再考。固定列は最後の手段

---

## 08. Dialog

```tsx
<Dialog open={open} onOpenChange={setOpen}>
  <DialogHead title="削除しますか？" desc="この操作は取り消せません。" />
  <DialogBody>…</DialogBody>
  <DialogFoot>
    <Button variant="ghost">キャンセル</Button>
    <Button variant="destructive">削除</Button>
  </DialogFoot>
</Dialog>
```

### いつ使う
- **破壊的・不可逆**な操作（削除・上書き）
- 警告を読まずに進ませたくない場面
- 複数フィールド入力（API key 発行など）

### いつ使わない
- 軽い設定変更 → **インライン**で
- 完了/未完了の切替・保存・並び替え → **Toast + Undo** で
- 連続して行う操作 → **Toast** で

### ルール
- 危険系の最終ボタンは **destructive**、キャンセルは **ghost**
- primary は使わない（誤クリック誘発）
- 見出しは「○○しますか？」と疑問形で、本文は影響範囲を具体的に

---

## 09. Alert

```tsx
<Alert variant="warning" title="証明書の期限が近づいています" desc="…" />
```

### Variants
`info` / `success` / `warning` / `destructive`

### Alert vs Toast（背反）
| | Alert | Toast |
|---|---|---|
| 寿命 | 画面に残る | 自動消滅（6–8s） |
| 用途 | 持続的な状態通知 | 操作の完了/失敗通知 |
| 閉じて | また戻ってくる | 戻ってこない |
| 例 | 証明書期限・障害情報 | 保存完了・同期失敗 |

**両方で同じ事象を出すのは禁止**。

---

## 10. Empty State

```tsx
<Empty
  icon={<Inbox />}
  title="まだタスクがありません"
  desc="今日の予定を1つ追加して、研鑽の習慣を始めましょう。"
  actions={[<Button variant="primary">＋ 追加</Button>, <Button variant="outline">AIに任せる</Button>]}
/>
```

### ルール（**3点セット必須**）
1. **状態の説明**（何が無いのか）
2. **次の一手**（どうしたら状況が変わるか）
3. **主アクション**（最低1つ、最大2つ）

「データがありません」**だけ**は禁止。

---

## 11. Tooltip · Avatar · Skeleton

### Tooltip
- 短いラベル（マウスオーバーで補足が必要なアイコンボタン等）に
- 12文字以上の説明文を入れるなら **Popover** を使う（未実装、必要時に追加）

### Avatar
- 既定 32px、`-sm` 24px、`-lg` 44px
- 画像がないときは **イニシャル2文字**（姓名 or ハンドル先頭）
- AI / システム発のものは brand 色背景 + ✦ アイコン

### Skeleton
- **本物のレイアウトを模す**こと（行数・幅・高さを実体に近づける）
- ロード時間が 200ms 未満なら表示しない（チラつき防止）
- spinner は最後の手段（位置情報が失われるため）

---

## アクセシビリティ最低限

- フォーカスリングは必ず表示（`focus-visible` で `--ring`）
- 状態を色だけで伝えない（色 + アイコン or テキスト）
- インタラクティブ要素のヒット領域: **最低 32×32 px**（compact）、推奨 **44×44 px**（comfortable）
- フォームには必ず `<label>`（または `aria-label`）

---

## やってはいけないこと（NG リスト）

| NG | 代わりに |
|---|---|
| `bg-[#0EA5E9]` 等の hex 直書き | `bg-brand` |
| 1画面で primary 2つ以上 | 1つに絞り残りは ghost / outline |
| 色だけで状態を表現 | dot badge + テキスト |
| 数値を proportional font で | `font-mono` + `.tnum` |
| カードに常時 shadow-lg | 罫線のみ、hover で `shadow-sm` |
| 同画面で角丸混在（4px と 12px） | 同じ `--radius` ファミリーに揃える |
| 「データなし」だけ | 説明 + 次の一手 + ボタン |
| spinner だけのローディング | 実レイアウトを模した Skeleton |
| 絵文字を UI に使う | lucide-react のアイコン |
| アイコンライブラリ混在 | lucide-react 一択 |
