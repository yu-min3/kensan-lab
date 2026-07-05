# kensan リファクタリング マスタープラン

作成日: 2026-07-03 / 対象: `apps/kensan`（backend + frontend + ビルド・デプロイ一式）
ステータス: **調査・計画のみ。実装は未着手。**

---

## 1. 調査結果

### 1.1 プロジェクト構造の現状

```
apps/kensan/
├── backend/            Go 単一サービス（REST API + SPA 配信 + kensan CLI）
│   ├── cmd/kensan/     main.go（serve）+ task.go（CLI サブコマンド）
│   └── internal/
│       ├── api/        HTTP ハンドラ（server/handlers/write_handlers/reviews/history_handler/static）
│       ├── workspace/  ファイル走査・frontmatter・読み書き・楽観ロック（中核）
│       ├── tasks/      チェックボックス行の抽出・行内タグ・行編集（中核）
│       ├── projects/   projects/<name>/README.md の構造化パース
│       ├── goals/      goals.md のパース
│       ├── history/    git log/show の読み取り
│       └── telemetry/  OTel セットアップ
├── frontend/           React SPA（Vite + Tailwind v4 + Whetstone tokens）
│   └── src/
│       ├── pages/      9 ページ（Dashboard/Tasks/Projects/Daily/Memo/Notes/Life/Reviews/EditorLab）
│       ├── components/ TaskBoard・各種カード・editors/（Milkdown/TipTap）・ui/（shadcn 風）
│       └── lib/        api.ts（API クライアント + 型）・memosFile.ts
├── Dockerfile          単一 image（frontend build → dist を Go バイナリと同梱）
├── Makefile / deploy.sh  test → tag bump → multi-arch build/push → PR の 1 発デプロイ
├── docs/               実質空（.DS_Store のみ）← 本ドキュメントが最初の実体
│
│ ---- 以下は git 未追跡の残骸（ディスク上のみ存在）----
├── kensan-ai/          .venv + .pytest_cache のみ（756MB）
├── lakehouse/          .venv + .pytest_cache + .env のみ（577MB）
├── temp/               kensan-ai-credentials-{dev,prod}-raw.yaml（生クレデンシャル）
├── .env                （未追跡・中身未確認）
└── .DS_Store / .claude/settings.local.json ×3
```

- git 追跡ファイルは **73 個**。コード規模は backend 約 4,200 行（テスト込み）、frontend 約 4,900 行。
- `kensan-ai/`・`lakehouse/` の実体は PR #351 で `apps/kensan-legacy/` へ rename 済み。残っているのは rename 前の venv/キャッシュの**抜け殻**（合計 約1.3GB）。
- デプロイ先は `kubernetes/apps/app-kensan/`（app-base chart の最初の消費者）。`kensan-preview.app.yu-min3.com` で稼働中（Keycloak SSO 越し）。

### 1.2 主要モジュールの責務と依存の流れ

```
cmd/kensan ──> api ──> tasks ──┐
                  ├──> projects ──> tasks, workspace
                  ├──> goals（独立）
                  ├──> history（独立・git CLI 呼び出し）
                  └──> workspace（最下層・ファイル I/O と楽観ロックの唯一の場所）
frontend: pages ──> components ──> lib/api.ts ──> backend REST
```

依存は一方向で健全。循環依存なし。`workspace.Mutate`（プロセス内 mutex + read-modify-write）に全書き込みが集約されており、この設計自体は守るべき資産。

設計原則（README 記載、実装とも一致）:
- インデックスなし。毎リクエスト WalkDir + stat、frontmatter パースのみ mtime/size キーでメモ化
- 寛容なパース（壊れた frontmatter は「未分類」）
- `conventions.md` がファイル契約。**Claude Code の /morning・/reflection スキルと app が同じファイルを同じ規約で触る**

### 1.3 肥大化している箇所

| ファイル | 行数 | 内訳 |
|---|---|---|
| `frontend/src/pages/ProjectsPage.tsx` | 760 | ページ + 12 サブコンポーネント + **md セクションパーサ実装**が同居 |
| `backend/internal/tasks/move.go` | 551 | 行編集 API 8 種 + セクション挿入 + daily 骨組み生成 |
| `frontend/src/components/TaskBoard.tsx` | 503 | かんばん 2 レーン + DnD + 優先度再採番アルゴリズム |
| `backend/internal/api/write_handlers.go` | 349 | ほぼ同型の decode→validate→呼び出し ×12 |
| `frontend/src/lib/api.ts` | 313 | API クライアント + 全型定義 + date/skeleton ユーティリティ |

### 1.4 重複している処理（本丸）

**フロントエンド内の重複:**

1. **autosave パターン ×4** — `DailyPage`・`ProjectsPage/FreeSpace`・`LifeGoalsPage`・`Whiteboard` が「800ms デバウンス + baseMtime 楽観ロック + savedBody dirty 判定 + アンマウント時 flush + 409 conflict UI」をそれぞれ手書きで再実装している。微妙に挙動が違う（Whiteboard は flush なし・タイマー破棄のみ、LifeGoals は非制御エディタ用に latest ref 持ち）。**最大の重複源。**
2. **`splitFm()`（frontmatter 分離）×3** — DailyPage / LifeGoalsPage / Whiteboard に同一実装のコピー。
3. **md セクション操作 ×2 実装** — `ProjectsPage`（sectionBounds/extractSection/spliceSection）と `memosFile.ts`（sectionBody、正規表現ベースで別実装）。
4. **日付ユーティリティ散在** — `todayISO`（api.ts）、`shiftDate`/`weekdayLabel`（DailyPage）、`pad2`/`isoOffset`（ProjectsPage）、`WEEKDAYS` 定数 ×2（Dashboard / DailyPage）。
5. **409 conflict の ErrorState UI** — ほぼ同文のメッセージとリカバリ処理が 5 箇所。

**バックエンド内の重複:**

6. **「行の位置照合」ブロック ×9** — `move.go` の Move / SetToday / EditTask（2 回）/ SetText / SetDue / SetPriority / DeleteLine / SetState が全て
   `lines 分割 → line range 検査 → checkboxRe 照合 → expectText 一致確認` の同一ブロックをコピペしている。`mutateTaskLine(ws, file, line, expectText, fn)` 1 個に畳める。
7. **`headingRe` の 3 重定義** — tasks / projects / goals の各パッケージに同一の正規表現。セクション抽出ロジック（`sectionBy` 相当）も tasks（insertIntoSection）と projects（sectionBy/replaceSection）で類似実装。
8. **frontmatter パーサ 2 実装** — `workspace/frontmatter.go`（yaml.v3、寛容設計）と `projects.go` の `frontmatter()`（手書き行分割）。挙動差異（引用符・複数行値の扱い）が潜在バグ源。
9. **write_handlers.go の boilerplate ×12** — 構造体定義 + Decode + 必須チェック + writeOpError がハンドラごとにコピペ。

**フロント⇔バック間の重複（挙動乖離リスク）:**

10. **daily 骨組み生成 ×2** — frontend `api.ts dailySkeleton()` と backend `move.go newDailySkeleton()`。現状は一致しているが、片方だけ変えると「app で作った daily」と「タスク退避で作られた daily」の形が割れる。
11. **セクション差し替え** — FreeSpace は frontend でセクション splice して PUT、タスク系は backend で insertIntoSection。同じ「## 見出しの本文を差し替える」操作が両側に実装されている。

### 1.5 責務が曖昧になっている箇所

- **`ProjectsPage.tsx` が md パーサを持つ** — 「セクション構造の知識」は backend（projects/tasks package）の責務のはずが、FreeSpace 編集のためだけにフロントにも実装されている。`## フリースペース` を Detail API / 専用 PATCH で扱えば消せる。
- **`api.ts` が型 + クライアント + ユーティリティの 3 役** — date/skeleton ヘルパーは別モジュールへ。
- **API エンドポイントの粒度不統一** — `/tasks/save`（作成 + 編集 + ファイル間移動の統一口）と、`/tasks/today`・`/tasks/due`・`/tasks/priority`・`/tasks/text` の個別タグ操作口が併存。UI 都合の歴史的経緯で、意味の重なりがある（save で due を変えられるのに setDue もある）。
- **`EditorLabPage` が本番 nav に常設** — エディタ選定の実験ページ（比較表・pro/con 付き）が「振り返り」グループに露出したまま。選定が済んだら役目を終える一時ページ。
- **Go の nil slice がフロント型に漏れている** — `Board.today: Task[] | null` 等。フロント全域に `?? []` が散る。backend で空 slice を保証すれば型から null が消える。

### 1.6 技術的負債

| # | 負債 | 深刻度 |
|---|---|---|
| D1 | **生クレデンシャルの残置**: `temp/kensan-ai-credentials-{dev,prod}-raw.yaml` と `.env` ×2 が作業ディレクトリに残っている（未追跡だが、リポジトリ規約では raw secret はレポジトリルートの `temp/` のみ・作業後破棄が前提） | 高 |
| D2 | **エディタ 3 系統併存**: CodeMirror（DailyPage・FreeSpace）/ Milkdown（LifeGoalsPage）/ TipTap（EditorLab のみ）。TipTap 系 6 パッケージは実験ページ専用の deps | 中 |
| D3 | **TZ 問題の疑い**: runtime は distroless（TZ 未設定 = UTC）で、`time.Now()` 依存の判定が多数（`isToday` の `@due` 比較、`ReflectionDate` の 0-6 時判定、daily パス解決、`TouchUpdated`）。JST 前提の「0:00〜6:00 は前日扱い」規約が **UTC で評価されている可能性が高い**（values.yaml に TZ env なし）。日本時間 9:00 までタスクの「今日」が前日にズレる系の症状が出る設計 | 高（要実機確認） |
| D4 | **CORS 全開**: `Access-Control-Allow-Origin: *` を本番でも常時付与。Vite dev 用と明記されているのに環境で分岐していない。SSO + 同一オリジン構成なので実害は限定的だが、意図しない cross-origin 書き込みを許す | 中 |
| D5 | **frontend にテスト 0・lint 設定なし**: `eslint-disable` コメントが 6 箇所あるのに ESLint config が存在しない（死んだ注釈）。`make test` の frontend 側は `tsc + vite build` のみ | 中 |
| D6 | **CI なし**: テスト実行は deploy.sh / 手動のみ（DEPLOY.md 明記）。リファクタリング期間中の安全網として弱い | 中 |
| D7 | `apps/kensan-legacy/`（649 ファイル・6.4MB）が凍結のまま残存。Phase 7 cutover 後に削除予定のまま | 低（別トラック） |
| D8 | `handlers.go snippet()` のバイト長判定 + rune 切り詰めの混在など、細かい読みにくさ | 低 |

### 1.7 テストが不足している箇所

**backend**（コアは意外と厚い。境界が薄い）:
- テストあり: workspace（scan/frontmatter）、tasks（抽出 4 + move 系 11）、projects（5）、goals（2）、api（3 + reviews 2）
- **テストなし**: `history`（git 呼び出し）、`telemetry`、`cmd/kensan`（CLI 引数処理）、`static.go`（SPA fallback）
- **薄い**: `api` パッケージの Test 関数は 5 個のみ。`/tasks/save`・`/tasks/due`・`/tasks/reorder`・`/tasks/text`・`/tasks/delete`・`/search`・`/stats`・`/tags`・`/history` あたりの HTTP 層の網羅は要確認・要増強
- **TZ 依存関数**（`ReflectionDate`、`collect` の today 注入はテスト可能な構造になっているが、TZ 明示のテストがない）

**frontend**: **ゼロ**。特に手続きが複雑な `memosFile.ts`（パース/シリアライズの round-trip）、`ProjectsPage` のセクション splice、TaskBoard の優先度再採番（中間値・隙間切れフォールバック）は純関数なのにテストがない。

### 1.8 影響範囲が大きく慎重に扱うべき箇所

1. **`workspace/write.go`（Mutate / 楽観ロック）** — 全書き込みの通り道。ここのバグは **workspace（人生の記録）のデータ破壊**に直結。しかも Syncthing が Mac ⇄ クラスタで双方向同期しているため、破壊は即座に全端末へ伝播する。
2. **`tasks/move.go` の 2 段 Mutate**（移動元から削除 → 移動先へ挿入、失敗時は末尾復元）— アトミックでない設計を「復元でリカバリ」して成立させている。挙動を変える変更は行消失リスク。
3. **`conventions.md` のファイル契約** — `## タスク`・`## Now`・`@today` 等のセクション名・タグ記法は Claude Code スキル（/morning・/reflection）と共有。**app 側の勝手な変更はスキル側を壊す。**契約変更はリファクタリング対象外とする。
4. **`server.go` のルート定義と `api.ts` の対応** — API 形の変更はフロントと同一 PR で。
5. **Dockerfile の packages/design-tokens 相対 import** — frontend の import パスを動かすと image build が壊れる（build context は repo root）。

### 1.9 削除候補（コード・設定・リソース）

| 対象 | 種別 | 根拠 | リスク |
|---|---|---|---|
| `kensan-ai/`・`lakehouse/` ディレクトリ丸ごと | 未追跡残骸 1.3GB | 実体は #351 で kensan-legacy へ移動済み。残るは venv/キャッシュのみ | なし（git 影響ゼロ） |
| `temp/kensan-ai-credentials-*-raw.yaml`・`.env` ×2 | 生 secret | 規約上も作業後破棄が前提。**中身が現行有効なら破棄前に rotate 判断** | 破棄自体はなし |
| `.DS_Store` ×2・`.claude/settings.local.json` ×3（apps/kensan 配下） | ノイズ | ローカル設定・OS ノイズ | なし |
| `frontend/tsconfig.tsbuildinfo` | ビルド生成物 | gitignore 済みなので放置可（掃除ついでに削除） | なし |
| `EditorLabPage.tsx` + nav 項目 | 実験ページ | エディタ選定完了と同時に役目終了 | 選定の意思決定が前提 |
| `editors/TipTapEditor.tsx` + `@tiptap/*` 5 packages + `tiptap-markdown` | 依存 | 使用箇所は EditorLab のみ | 上と同時 |
| （選定次第）`editors/MilkdownEditor.tsx` + `@milkdown/crepe` **または** CodeMirror 系 4 packages | 依存 | 2 系統併存の解消。Milkdown は LifeGoalsPage が使用中なので**要 Yu 判断** | 使用ページの編集体験が変わる |
| `/tasks/*` API の重複エンドポイント（統合後） | API | save に集約できる可能性 | 後述の通り後回し |
| `apps/kensan-legacy/` | 旧アプリ | Phase 7 cutover 完了後 | **本計画のスコープ外**（cutover 計画に従う） |

---

## 2. リファクタリング計画

### 2.1 目的

1. **安全性の回復**: 生クレデンシャルの残置解消、TZ バグ疑いの決着、CORS の本番遮断
2. **重複の一本化**: フロント autosave ×4・バック行照合 ×9 に代表されるコピペを共通部品化し、修正が 1 箇所で済む状態にする
3. **削除による軽量化**: 実験用エディタ・残骸ディレクトリ・使われない依存を落とし、「読むべきコード」だけにする
4. **変更容易性**: 760 行ページの分割・handler boilerplate の解消で、次の機能追加（ダッシュボード UX 改善等）のコストを下げる
5. **上記すべてを、workspace のデータと Claude スキル契約を一切壊さずに行う**

### 2.2 現状の問題点（要約）

- 未追跡の残骸 1.3GB + 生 secret がプロジェクト直下に同居している
- 同じ処理の手書きコピーが front 5 系統・back 4 系統・front⇔back 間 2 系統
- エディタ 3 実装が併存し、依存ツリーの相当部分が実験ページのためだけに存在
- frontend はテスト 0・lint なし、backend は HTTP 層とユーティリティ層に穴。CI なし
- TZ・CORS という「動いているが正しくない可能性がある」挙動が放置

### 2.3 フェーズ分割

依存関係: P0 → P1 は独立で並行可。P2 以降は P0（テスト安全網）完了を前提。

```
P0 安全網          P1 掃除(即日可)      P2 エディタ決着
   │                                     │
   └───────┬─────────────────────────────┘
           ▼
P3 フロント共通化 ──> P5 大物分割
P4 バック共通化   ──> P6 仕上げ(挙動修正)
```

---

#### Phase 0: 安全網の整備（挙動変更なし）

**目的**: 以降のフェーズで「壊したら気づける」状態を作る。プロダクションコードは触らない。

| 実施内容 | 対象範囲 |
|---|---|
| ESLint（typescript-eslint + react-hooks）導入。既存 `eslint-disable` コメントを実効化 | `frontend/` 設定のみ |
| Vitest 導入 + 純関数テスト: `memosFile.ts` round-trip、`ProjectsPage` の section splice（テスト可能にするため `lib/markdown.ts` へ**移動はせず** export だけ追加）、TaskBoard の `reorder` 採番ロジック | `frontend/src/lib`、`package.json` |
| backend HTTP 層テスト増強: `/tasks/save`・`/tasks/due`・`/tasks/reorder`・`/tasks/text`・`/tasks/delete`・`/search`・`/history` の httptest。**move.go リファクタ前の挙動固定（characterization test）**として、各行編集操作の before/after ファイル内容ゴールデンテスト | `backend/internal/api`、`internal/tasks` |
| TZ 挙動の実機確認: 稼働 pod の `date` 出力・`ReflectionDate` の期待動作を検証し、D3 の疑いを確定/棄却する（**調査のみ。修正は P6**） | 調査タスク |
| `make test` に lint を組み込み | `Makefile` |

**完了条件**: `make test` が lint + backend test + frontend test + build を一括実行し green。

#### Phase 1: 無リスクの掃除（git 履歴に影響しない削除）

**目的**: 残骸と secret を消す。コードは触らない。P0 と並行可・最優先着手。

| 実施内容 | 備考 |
|---|---|
| `temp/kensan-ai-credentials-*-raw.yaml` の扱い決定 → 破棄。**破棄前に、当該 credential が現行クラスタで有効かを確認し、有効なら rotate を Yu に確認** | セキュリティ最優先 |
| `apps/kensan/.env`・`lakehouse/.env` の中身確認 → 不要なら破棄（有効 token を含むなら同上） | |
| `kensan-ai/`・`lakehouse/` ディレクトリ削除(約1.3GB 回収) | git 未追跡・影響ゼロ |
| `.DS_Store`・`tsconfig.tsbuildinfo`・`temp/.claude/` 等のローカルノイズ削除 | |
| `docs/` を実体化（本ドキュメント配置。必要なら README から参照） | |

**完了条件**: `git status` がクリーンで、apps/kensan 配下に未追跡の残骸・secret が存在しない。

#### Phase 2: エディタ戦略の決着（**決定済み: 2026-07-04 Yu 判断 = Milkdown 一本化**）

**決定**: 案 C（Milkdown 一本化）を採用。Yu が EditorLab で打ち比べて「使い心地」を優先。
Claude Code との併用で md 正規化の違和感が出たら、その時点で再検討する（CodeMirror 実装は git 履歴から復元可能）。

**目的**: 3 実装 → 1 実装（Milkdown/Crepe）へ。依存削減とページ削除。

| 実施内容 | 対象範囲 |
|---|---|
| `EditorLabPage.tsx` 削除、`main.tsx` の route と `AppShell` の nav 項目削除 | frontend 3 ファイル |
| `TipTapEditor.tsx` 削除、`@tiptap/*` ×5 + `tiptap-markdown` を package.json から除去 | 依存 6 個削減 |
| `DailyPage`・`ProjectsPage/FreeSpace` のエディタを CodeMirror → Milkdown に置換 | 2 画面 |
| `MarkdownEditor.tsx`（CodeMirror ラッパー）削除、`@uiw/react-codemirror` + `@codemirror/*` ×3 + `@lezer/highlight` を除去 | 依存 5 個削減 |
| **P3 の `useAutosaveFile` hook をここで前倒し実装** — Milkdown は非制御（defaultValue + remount）のため、置換する 3 画面すべてが LifeGoalsPage と同型の「latest ref + editorKey remount + debounce + 楽観ロック」になる。同じパターンを 3 回コピーして P3 で捨てるより、hook を先に 1 つ作って 3 画面で使う | hooks 新設 + 3 画面 |

**完了条件**: `npm run build` の bundle が縮小し、editors/ に Milkdown のみ残る。日記・フリースペース・人生ページの自動保存・409 競合・離脱時保存が動作。

**注意（Milkdown 固有）**: 保存時に md が正規化されるため、「開いただけで保存→リフォーマット」を防ぐ初回 markdownUpdated スキップ（MilkdownEditor 実装済み）を必ず維持する。

#### Phase 3: フロントエンド共通化

**目的**: 重複 1.4 節 #1〜5 の解消。**挙動は変えない**（リフト & 統合のみ）。

| 実施内容 | 対象範囲 |
|---|---|
| `lib/dates.ts` 新設: `todayISO`・`shiftDate`・`isoOffset`・`weekdayLabel`・`WEEKDAYS` を集約 | api.ts / DailyPage / ProjectsPage / Dashboard |
| `lib/markdown.ts` 新設: `splitFm`・`sectionBounds`・`extractSection`・`spliceSection`・`dailySkeleton` を集約（memosFile.ts はメモ固有なので残すが、セクション抽出は共通実装を使う） | DailyPage / LifeGoalsPage / Whiteboard / ProjectsPage / memosFile |
| `hooks/useAutosaveFile.ts` 新設: 「file query + splitFm + 800ms debounce + baseMtime 楽観ロック + dirty/flush + SaveState 算出」を 1 hook に。制御（CM/textarea）・非制御（Milkdown）両対応の形にする | DailyPage / FreeSpace / LifeGoalsPage / Whiteboard の 4 箇所を置換 |
| `components/ui/ConflictState.tsx`: 409 リカバリ UI の共通化 | 5 箇所 |
| P0 で書いた純関数テストを新モジュールに追従・拡充 | |

**完了条件**: 4 画面の自動保存が 1 hook 経由になり、`splitFm` の実装がリポジトリに 1 つ。各画面の保存・競合・離脱時 flush の手動確認。

#### Phase 4: バックエンド共通化

**目的**: 重複 1.4 節 #6〜9 の解消。**P0 のゴールデンテストを不変のまま通す**ことが正しさの定義。

| 実施内容 | 対象範囲 |
|---|---|
| `tasks` に `mutateTaskLine(ws, file, line, expectText, fn)` を導入し、SetToday / SetText / SetDue / SetPriority / SetState / DeleteLine / EditTask / Move の行照合ブロックを畳む（move.go 551 行 → 体感半減） | `internal/tasks` |
| `internal/mdsec`（or workspace 配下）新設: `headingRe`・セクション抽出・セクション挿入/差し替えを 1 パッケージに。tasks / projects / goals から参照 | tasks / projects / goals |
| projects.go の手書き `frontmatter()` を `workspace.parseFrontmatter` ベースに統合（Extra フィールド経由で deadline/repo を取得） | `internal/projects` |
| write_handlers.go に generic decode ヘルパー（`decodeJSON[T]`）+ locator 構造体の共通化 | `internal/api` |
| Board の slice を常に非 nil で初期化（`[]Task{}`）→ frontend 型から `\| null` を除去し `?? []` を一掃 | api + frontend 型（同一 PR） |

**完了条件**: `go test ./...` 全 green（ゴールデン不変）、`headingRe` 定義が 1 箇所、move.go のコピペブロック 0。

#### Phase 5: 大物ファイルの分割（構造のみ、挙動不変）

| 実施内容 | 対象範囲 |
|---|---|
| `ProjectsPage.tsx`(760行) を分割: `ProjectList` / `ProjectDetailView` / `MilestoneList` / `FreeSpace` / `MetaEditor` / 小物（DuePicker, Timeline, badges）へ。md パーサは P3 で lib へ移動済み | `pages/projects/` ディレクトリ化 |
| `TaskBoard.tsx`(503行) を分割: レーン・DnD 行・優先度採番（`lib/priority.ts`、P0 でテスト済み）へ | `components/taskboard/` |
| `api.ts` を `lib/api/`（client + types + 各リソース）に分割（任意。効果が薄ければスキップ可） | `lib/` |

#### Phase 6: 仕上げ（意図的な挙動変更・要個別レビュー）

ここまでのフェーズと違い**挙動が変わる**。1 変更 = 1 PR で分離する。

| 実施内容 | 内容 |
|---|---|
| **TZ 修正**（P0 の調査結果に基づく）: values.yaml に `TZ: Asia/Tokyo` を足す（distroless は tzdata を持つ static:nonroot なら `TZ` が効くことを確認の上）か、backend 側で `time.LoadLocation` を明示注入。`ReflectionDate`・`isToday`・daily パス解決に TZ 明示のテストを追加 | 挙動修正 |
| **CORS を dev 限定に**: `KENSAN_DEV_CORS` 等の opt-in env で `*` を付け、本番は付けない | 挙動修正 |
| **API 整理（任意・議論後）**: `/tasks/{due,text,priority}` を `/tasks/save` へ寄せる案の是非。フロント同時改修が必要で利得が薄ければ**見送り**してよい | 要判断 |
| README / DEPLOY.md / conventions 参照のドキュメント追従 | docs |

### 2.4 先に着手すべき箇所 / 後回しにすべき箇所

**先に（価値が高く・依存の根元）:**
1. Phase 1 の secret 破棄（1.3GB 回収は副産物。本命は credential）
2. Phase 0 のゴールデンテスト（これがないと P4 に入れない）
3. TZ 調査（バグ確定なら、ユーザー体験に直結するため P6 を前倒しする価値あり）
4. `useAutosaveFile`（フロント最大の重複。以降の画面追加が全部楽になる）

**後回しに（動いている・利得が薄い・判断が必要):**
- API エンドポイント統合（P6・任意）: 動作中の API の形を変えるリスク > 重複コスト
- `api.ts` の分割（P5・任意）
- `apps/kensan-legacy` の削除: Phase 7 cutover という別トラックの完了待ち。本計画では触らない
- kensan-app-v2 / 旧 kensan など `repositories/` 配下の別リポジトリ: スコープ外

### 2.5 リスクが高い箇所（変更時の注意）

| 箇所 | リスク | 緩和策 |
|---|---|---|
| `workspace/write.go` | データ破壊が Syncthing 経由で全端末に伝播 | **原則触らない**。P4 でも Mutate の中身は不変。触る場合は workspace のバックアップ（git commit）を確認してから |
| `tasks/move.go` の 2 段 Mutate | 途中失敗で行消失 | ゴールデンテストで失敗系（挿入失敗 → 復元）も固定してからリファクタ |
| セクション名・`@タグ` 記法 | /morning・/reflection スキルとの契約破壊 | 記法・セクション名は**一切変更しない**を全フェーズの不変条件にする |
| Dockerfile の build context | tokens 相対 import 前提 | frontend のディレクトリ移動時は `make build` でローカル image build を検証 |
| LifeGoalsPage のエディタ置換（案 B） | 「人生でやりたいこと」ファイルの意図しないリフォーマット | 置換直後に git diff でファイル本文が変わっていないことを確認 |
| TZ 修正 | daily の日付が過去分とズレて二重ファイル化 | 切替日は daily 作成前の朝に実施。既存ファイルの遡及修正はしない |

### 2.6 事前に追加すべきテスト（P0 の具体リスト）

1. `internal/tasks`: 各行編集操作（SetToday/SetDue/SetPriority/SetText/SetState/DeleteLine/EditTask/Move）の**ファイル全文ゴールデン**（インデント付き行・タグ複合・移動失敗復元を含む）
2. `internal/api`: `/tasks/save`（create/edit/project 移動）、`/tasks/due`、`/tasks/reorder`、`/tasks/delete`、`/search`（1MB skip・truncate）、`/history`（rev 検証）の httptest
3. `internal/tasks`: `collect()` の today 判定を日付注入で境界テスト（due==today / due<today / @today+done）
4. frontend: `memosFile.ts` parse→serialize round-trip（Pinned あり/なし、Scratch 見出しなし、after セクションあり）
5. frontend: section splice（見出しなし→末尾追加、ネスト見出し、本文空）
6. frontend: TaskBoard `reorder` 採番（中間値、隙間切れ→全体再採番、priority 未設定混在）

### 2.7 レビュー方針

- リポジトリ規約（`.claude/rules/collaboration.md`）に従う: **PR 作成まで自走可、merge はレビューエージェント + Yu の指示待ち**
- 1 フェーズ = 1〜3 PR。**「挙動不変のリファクタ PR」と「挙動変更 PR」を絶対に混ぜない**
- 挙動不変 PR のレビュー観点: ゴールデンテスト・既存テストが**変更なしで** green か（テストを書き換えて通した PR は差し戻し）
- 挙動変更 PR（P6）は変更点を PR 本文に before/after で明記
- P1（secret 破棄）は PR 不要（未追跡ファイルの削除）だが、破棄内容を記録として残す
- レビュー優先度は CLAUDE.md の P0/P1/P2 基準を適用（特に P0: 生 secret の commit 混入がないこと）

### 2.8 ロールバック方針

- **アプリ**: GitOps なので `values.yaml` の `image.tag` を前バージョンへ戻す PR（または deploy commit の `git revert`）→ ArgoCD auto-sync。過去 image は GHCR に残存
- **フェーズ単位**: 各フェーズを独立 PR にしているため `git revert` 1 発で戻せる。P3/P4 の共通化はモジュール追加 + 呼び出し置換のみで、旧実装の復元が容易
- **workspace データ**: 万一 app 経由でファイルを壊した場合は workspace 側の git で復元（`git show <commit>:<file>`）。Syncthing versioning には依存しない
- **リリース戦略**: P2〜P5 は機能追加がないため、まとめて 1 回のデプロイ（patch bump）でよい。P6 の TZ/CORS はそれぞれ単独デプロイし、24h 様子見してから次へ

### 2.9 実装を GPT-5.5 / Opus 4.7 に依頼する場合の作業単位

原則: **1 タスク = 1 PR = コンテキスト自己完結**（対象ファイル列挙 + 不変条件 + 完了条件をプロンプトに含める）。以下の順で直列依頼、`†` は並行可。

| # | タスク | 規模 | 前提 |
|---|---|---|---|
| T1† | P1: 残骸削除 + secret 確認・破棄（確認結果の報告付き） | S | なし |
| T2† | P0-a: frontend に ESLint + Vitest 基盤導入、`make test` 統合 | S | なし |
| T3 | P0-b: `internal/tasks` 行編集のゴールデンテスト一式 | M | なし |
| T4 | P0-c: `internal/api` httptest 増強（2.6 の #2） | M | なし |
| T5† | P0-d: frontend 純関数テスト（memosFile / splice / reorder） | S | T2 |
| T6 | P0-e: TZ 実機調査レポート（pod 内 date、UTC/JST での isToday・ReflectionDate 挙動表） | S | クラスタアクセス |
| T7 | P2: Milkdown 一本化（EditorLab/TipTap/CodeMirror 削除 + `useAutosaveFile` 前倒し + 3 画面置換） | L | **決定済み・着手** |
| T8 | P3-a: `lib/dates.ts` + `lib/markdown.ts` 抽出・置換 | M | T5 |
| T9 | P3-b: 残りの autosave 統合（Whiteboard）+ ConflictState 共通化 | M | T7, T8 |
| T10 | P4-a: `mutateTaskLine` 導入で move.go 畳み込み | M | T3 |
| T11 | P4-b: mdsec パッケージ統合 + frontmatter 一本化 | M | T3, T10 |
| T12 | P4-c: handler boilerplate + 非 nil slice（front 型同時修正） | M | T4 |
| T13 | P5: ProjectsPage / TaskBoard 分割 | L | T8, T9 |
| T14 | P6-a: TZ 修正 + TZ 明示テスト | M | T6 の結論 |
| T15 | P6-b: CORS dev 限定化 | S | T4 |
| T16 | P6-c: ドキュメント追従（README/DEPLOY） | S | 全体後 |

- L サイズ（T9, T13)は 2 PR に割ってよい（例: T9 = hook 導入 + 2 画面 → 残り 2 画面）
- 各タスクのプロンプトに必ず含める不変条件: 「conventions.md のファイル契約（セクション名・@タグ記法・frontmatter 形式）を変更しない」「既存テストを書き換えて通さない」「kensan-lab の worktree 運用・commit 規約（Conventional Commits 日本語）に従う」

---

## 3. スコープ外（明示）

- `apps/kensan-legacy/` の削除（Phase 7 cutover トラック）
- `kubernetes/apps/app-kensan/` の構成変更（TZ env 追加を除く）
- `packages/design-tokens` / Whetstone design system 本体
- conventions.md のファイル契約の変更
- 新機能（ダッシュボード UX 改善等は本計画完了後の別トラック）
