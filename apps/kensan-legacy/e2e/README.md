# E2E テスト

Playwright による E2E テストスイート。

## セットアップ

```bash
# Playwrightブラウザインストール
make e2e-install

# バックエンド起動（テスト前に必要）
make up
make health
```

## 実行方法

```bash
make e2e           # ヘッドレス実行
make e2e-headed    # ブラウザ表示あり
make e2e-ui        # UIモード（デバッグ用）
```

## テストケース一覧（35テスト）

| Spec | テスト名 | 内容 |
|------|----------|------|
| **auth.spec.ts** | ログインページが表示される | ログインフォーム要素の表示確認 |
| | 正しい認証情報でログイン成功 | `test@kensan.dev` でログイン→ホーム遷移 |
| | 不正な認証情報でエラー表示 | 不正パスワードでエラーメッセージ表示 |
| | ログアウト | ユーザーメニュー→ログアウト→ログインページ遷移 |
| **daily.spec.ts** | デイリーページが表示される | 挨拶、メモ、タイムブロック、タスクリスト表示 |
| | タイムブロックの新規作成 | 追加ボタン→フォーム表示 |
| | タイムブロックの削除 | 既存ブロックの削除操作 |
| | 日付ナビゲーション | 前日/翌日ボタンで日付切替 |
| | 記録セクションのリンク確認 | 学習記録/日記の作成リンク遷移 |
| **notes.spec.ts** | ノート一覧が表示される | 見出し、作成ボタン、タブ、検索欄の表示 |
| | タイプフィルターで絞り込み | 日記タブクリック→URL更新 |
| | 新規ノート作成→保存→一覧に反映 | generalノート作成→保存→一覧遷移 |
| | 既存ノートの編集画面表示 | ノートカードクリック→編集ページ遷移 |
| | ノート削除 | ノート作成→削除→一覧遷移 |
| | 検索フィルタリング | 検索入力→結果フィルタリング |
| | draw.io付きノート作成 | drawioトグルON→保存→一覧反映 |
| | draw.ioトグルON/OFF切り替え | トグル切替→draw.ioエディタ表示/非表示 |
| | 既存ノートのnote_contents読み込み | API作成ノート→drawioコンテンツ読み込み確認 |
| | draw.ioトグルOFF→drawio content削除 | drawio付きノート→トグルOFF→保存→API検証 |
| **tasks.spec.ts** | タスク管理ページが表示される | ガントチャート、目標、マイルストーン、タスク表示 |
| | RecurringTaskWidget表示 | 定期タスクウィジェット表示 |
| | 目標選択→マイルストーン表示 | 目標選択でマイルストーンリスト更新 |
| | 検索フィルタリング | 検索入力→タスクフィルタリング |
| **analytics.spec.ts** | 分析ページが表示される（サマリーカード4つ） | 合計学習時間、完了タスク等の表示 |
| | 期間タブ切り替え | 今日/今週/今月タブ切替 |
| | チャート表示確認 | 円グラフ・棒グラフのSVG描画確認 |
| **settings.spec.ts** | 設定ページが表示される | タイムゾーン、テーマ、保存/キャンセルボタン表示 |
| | テーマ変更 | ダークテーマ選択→保存→`html.dark`クラス確認 |
| **chat.spec.ts** | AIボタンでチャットパネル開閉 | AIボタンクリックでパネルトグル |
| | パネルに空状態メッセージ表示 | 初期表示の空メッセージ確認 |
| | 閉じるボタン | パネル閉じるボタン動作 |
| **navigation.spec.ts** | サイドバー全リンク遷移確認 | 全ナビリンク→正しいURL遷移 |
| | Kensanロゴでホームに戻る | ロゴクリック→`/`遷移 |
| | テーマ切り替えボタン | テーマトグルでクラス変更 |
| | ユーザーメニュー表示 | ユーザーメニュー→設定/ログアウト項目表示 |

## ディレクトリ構成

```
e2e/
├── playwright.config.ts      # Playwright設定（baseURL, storageState等）
├── tsconfig.json              # E2E用TypeScript設定
├── global-setup.ts            # ログインフロー→.auth/user.json保存
├── fixtures/
│   └── index.ts               # カスタムfixtures（認証済みpage + POM）
├── helpers/
│   └── api.ts                 # JWT抽出、認証済みAPIリクエスト
├── pages/                     # Page Object Models
│   ├── login.page.ts
│   ├── layout.page.ts
│   ├── daily.page.ts
│   ├── note-list.page.ts
│   ├── note-edit.page.ts
│   ├── task-management.page.ts
│   ├── analytics.page.ts
│   ├── settings.page.ts
│   └── chat-panel.page.ts
└── tests/                     # テストスペック
    ├── auth.spec.ts
    ├── daily.spec.ts
    ├── notes.spec.ts
    ├── tasks.spec.ts
    ├── analytics.spec.ts
    ├── settings.spec.ts
    ├── chat.spec.ts
    └── navigation.spec.ts
```

## 設計方針

- **認証**: `global-setup.ts` でUI経由ログイン→`storageState`保存。全テストで共有。
- **POM**: Locatorベース。日本語テキストセレクタ優先（`getByRole`, `getByText`, `getByPlaceholder`）。
- **ブラウザ**: Chromium のみ（個人用アプリのため）。
- **テストデータ**: バックエンドのseedデータ前提。CRUD系は自前で作成→削除。

## 注意点

- **TipTapエディタ**: `fill()` 不可。`click()` → `keyboard.type()` で入力。
- **カスタムSelect**: shadcn/uiのSelectは `role="option"` を持たない。`[data-selected]` でアイテム選択。
- **日記の一意制約**: diary タイプは `(user_id, type, date)` で一意制約あり。テストでは `general` タイプを使用。
- **見出しセレクタ**: ノートカードのh3タイトルと衝突しうるため `{ exact: true, level: 1 }` を使用。
