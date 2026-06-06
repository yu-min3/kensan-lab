# Patterns — kensan-lab Design System

> 部品（Components）の上のレイヤー。「タスク一覧」「設定」「監視」のような画面を、毎回ゼロから考えずに済むようにするための共通の組み方の辞書。
> ビジュアル一覧は [`Patterns.html`](../../Patterns.html)。

---

## パターンの索引

| # | パターン | 主な用途 |
|---|---|---|
| 00 | App Shell | アプリのルートレイアウト |
| 01 | Page Header | 画面の入り口（eyebrow / title / subtext / actions） |
| 02 | Form Layout | 入力系の画面 |
| 03 | Empty / Loading / Error | 同一画面の3状態 |
| 04 | Confirmation | 操作の確認 |
| 05 | Toast / Notification | 一時的・持続的な通知 |
| 06 | List · Detail | 沢山あって個々を開く系の画面 |

---

## 00. App Shell

### Variant A — Side Nav + Main（kensan 既定）

```
┌──────────┬───────────────────────────┐
│  Brand   │  Page Header              │
│          │───────────────────────────│
│  グループ │                           │
│  · 項目   │  Main                     │
│  · 項目   │                           │
│  グループ │                           │
│  · 項目   │                           │
│          │                           │
│  Avatar  │                           │
└──────────┴───────────────────────────┘
```

- `grid-template-columns: 200px 1fr`（comfortable）／ `180px 1fr`（compact）
- モバイル時はオーバーレイ表示（hamburger → drawer）
- active は **`bg-accent` + `text-accent-foreground`**。border の左帯は使わない
- グループは **小ぶりな uppercase label**（10px / letter-spacing 0.12em）
- count バッジは右寄せ・mono。0 のときは出さない
- foot は **自分のアバター + 名前**

### Variant B — Topbar + Main（dashboard 系）

```
┌────────────────────────────────────────┐
│ Brand · Nav · Nav · Nav         🔍  YM │
├────────────────────────────────────────┤
│  Page Header                           │
│  [KPI] [KPI] [KPI] [KPI]               │
│  ┌──────────────────────────────────┐  │
│  │  Table (compact)                 │  │
│  └──────────────────────────────────┘  │
└────────────────────────────────────────┘
```

- topbar 48–56px、`position: sticky`
- 中身は **`data-density="compact"`** で情報量を稼ぐ
- KPI 行: 4列 grid、card の padding 控えめ、値は mono + tnum、状態色は各 1 値ずつ

---

## 01. Page Header（必須4層）

```tsx
<header className="ph">
  <div>
    <div className="eyebrow">ノート · ADR · 0042</div>
    <h2 className="t h-serif">Istio Gateway API への移行</h2>
    <p className="s">本ノートでは移行コスト、回避策、段階的移行計画を整理する。</p>
  </div>
  <div className="actions">
    <Button variant="ghost" size="sm">複製</Button>
    <Button variant="outline" size="sm">共有</Button>
    <Button variant="primary" size="sm">編集</Button>
  </div>
</header>
```

### 4層の役割
| 層 | 内容 | スタイル |
|---|---|---|
| **eyebrow** | コンテクスト（パンくず的） | uppercase + 11px + letter-spacing 0.18em + muted |
| **title** | 画面のタイトル | **h-serif** + 24–30px（comfortable）/ 20–24px（compact） |
| **subtext** | 画面の目的を1行で | 12.5–13.5px + muted + max-width 540–680px |
| **actions** | 主アクション群 | primary 1つ + ghost/outline、右端揃え |

### ルール
- header の下に **1px hsl(var(--border))** を引いて section 開始を視覚化
- primary は **1 つだけ**。残りは ghost / outline
- icon-only ボタンは tooltip 必須

---

## 02. Form Layout

```
┌─────────────────────────────────────┐
│  Page Header                        │
├─────────────────────────────────────┤
│  ┌─────────────┐  ┌───────────────┐ │
│  │  Field full       │             │ │
│  └─────────────┘  └───────────────┘ │
│  ┌─────────────┐  ┌───────────────┐ │
│  │  Field      │  │  Field        │ │
│  └─────────────┘  └───────────────┘ │
│                                     │
│  ─────────────────────────────────  │
│  * は必須        [Cancel] [Save]    │
└─────────────────────────────────────┘
```

### ルール
- `grid-template-columns: 1fr 1fr` + `gap: 18px 24px`、`<560px` でシングル
- 長い入力（textarea, 説明）は `.full` で全幅
- ラベルは **全部 上付き**。inline label は禁止（読み順が壊れる）
- エラーは **色 + 文章** 両方で
- フッターは **左 hint / 右 actions**。primary は右端、Cancel は左隣 ghost
- 「下書き保存」が必要なら primary の左に secondary で置く

---

## 03. Empty / Loading / Error の3状態

「データがあるとき」だけ作るのは半分の仕事。**同じ画面の3バリエーション**を必ずデザインする。

### 共通
- 画面の枠（card / 表組み）は **そのまま維持**。中身だけ入れ替わる
- 状態ごとに違うアイコンを置くと一目で区別できる

### Empty
- **状態の説明** + **次の一手** + **主アクション**（最低1つ・最大2つ）
- 「データなし」だけは禁止
- アイコンは muted、ボタンは primary + outline

### Loading
- **実レイアウトを模した skeleton**。spinner は最後の手段
- 200ms 未満で完了する場合は表示しない（チラつき防止）
- 表組みなら skeleton で行を3〜5本

### Error
- **人間語の説明** + **技術的詳細**（エラーコード）+ **復旧アクション**
- 例: `503 backend unavailable` + 「kensan-backend の Pod が再起動中の可能性があります」+「再試行」「詳細」
- ユーザがログを探せるようにエラーコードを必ず出す

---

## 04. Confirmation

### Dialog（破壊的操作のみ）

| 条件 |
|---|
| **削除 / 上書き / 不可逆** な操作 |
| 複数フィールド入力（API key 発行など） |
| 警告を読まずに進ませたくない |

- 危険系の最終ボタンは `destructive`、キャンセルは `ghost`
- **primary は使わない**（誤クリック誘発）

### Inline + Undo（取り消し可能な操作）

| 条件 |
|---|
| 完了/未完了の切替、保存、並び替え、ステータス変更 |
| **Undo** で取り消せるもの |
| 連続して行う操作（タスク一括チェックなど） |

- Toast の中に `[元に戻す]` を 6–8 秒表示
- 「全部 Dialog で確認」は **クリック疲労を生むので禁止**

---

## 05. Toast / Notification

### Toast — 一時的（自動消滅）
- 位置: **bottom-right**（モバイルは bottom-center）
- duration: `success` / `info` = **6s**、`destructive` = **10s または手動閉じのみ**
- stack: 最大 **3件**まで縦に積む、古いものから消える
- 必ず **タイトル**を入れる。本文・リンクは任意

### Alert — 画面に残る
- 画面の状態（証明書期限・通信障害）を伝える
- 閉じても **戻ってくる**のが本質
- 同じ場所に並べていい（並ぶようなら状況を疑え）

### 絶対にやらない
- 同じ事象を **Toast と Alert の両方** で出す
- フォームのバリデーションエラーを Toast で出す（**フィールド側に出す**）
- destructive な確認を Toast で代用する（**Dialog を使う**）

---

## 06. List · Detail

ノート・タスク・Application のような **沢山あって個々を開く** 系の画面。

```
┌──────────────┬───────────────────────┐
│  🔍 search   │  Page Header          │
├──────────────┤───────────────────────│
│  ▸ 選択中    │                       │
│    item      │  Detail content       │
│    item      │                       │
│    item      │                       │
└──────────────┴───────────────────────┘
```

### ルール
- `grid-template-columns: 280px 1fr`（comfortable）／ `240px 1fr`（compact）
- 各行は **title + meta（mono の id・更新日）+ snippet**
- 選択中は **`bg-accent`** で示す（青帯は使わない）
- URL は `/notes/:id` で **deep-link 可能**にする
- モバイル（`< 768px`）は **list と detail を切替表示**、breadcrumb で戻る
- list 上部に検索 / フィルタが入る場合は **sticky header** で

---

## Decision Matrix — 迷ったらこの表

| 状況 | 使うもの | 使わないもの |
|---|---|---|
| 破壊的操作の確認 | **Dialog** + destructive | Toast / インライン |
| 取り消し可能な操作の通知 | **Toast** + Undo（6–8s） | Dialog |
| 画面が抱える持続的な警告 | **Alert**（画面に残る） | Toast |
| フォームのバリデーションエラー | **`.field.error`**（フィールドの下） | Toast / Alert |
| 「データなし」 | **Empty**（説明+次の一手+ボタン） | "No data" だけ |
| 読み込み中 | **Skeleton**（実レイアウトを模す） | spinner だけ |
| 1画面で複数の主要アクション | **primary 1 + 残り ghost/outline** | primary 複数 |
| 状態を表現する小さなマーク | **dot badge + ラベル** | 色ドットだけ |
| 数値・時刻・ID | **font-mono + tabular-nums** | 本文書体のまま |
| 密な一覧（dashboard） | **`data-density="compact"`** | 個別に padding 上書き |
