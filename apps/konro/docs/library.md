# 単独レシピ管理

Konro自身をレシピの正本とする。Recipe KeeperのZIPは任意の移行入口。

| 操作 | 契約 |
|---|---|
| 閲覧 | `GET /api/v1/recipes/{name}`。`version`はファイル内容のSHA-256 |
| 手入力作成 | `POST /api/v1/recipes`。JSONの編集対象項目を指定。`Idempotency-Key`に32桁hexを指定すれば応答喪失後も同じ作成を再送できる |
| 編集保存 | `PUT /api/v1/recipes/{name}`。編集対象全項目と取得時の`version`を指定。競合は409で原本維持 |
| ZIP再取込 | `konro_edited: true`の既存ファイルは保持。結果`skipped`と警告で報告 |

編集対象はtitle/servings/tags/prepTime/cookTime/source/rating/ingredients/steps/notes。画像・rk_id・created・未知frontmatterは保持する。手入力作成は`imported/<id>.md`へ保存し、既存ZIPの名前空間と衝突しない。未入力の材料・手順も保存できる。

書き込みはJSON Content-Typeと同originを検査し、サイズ・文字列・パスを検証する。単一server内の編集とZIP書き込みを直列化し、同一ディレクトリの一時ファイルをfsync→rename→directory fsyncする。ディスクを直接変更する別processとのロック共有は行わない。

レシピの既知metadataはアプリ生成形式の1行値／inline listを扱う。手編集で既知フィールドを複数行YAML・block list等へ変えた場合は、安全のため編集保存を拒否し原本を維持する。未知metadata内のネストはそのまま保持する。ZIP画像自体は従来どおり取り込む（画像編集UIは今回含めない）。

配色・書体・影は`packages/design-tokens`を直接参照する。OSのlight/dark変更で共通`.dark`を切り替える。PWA launch colorもtokens.jsonからbuild時に生成する。調理のタッチ領域・配置は維持し、全面的なUI部品／密度置換は別の範囲。

検証: backend全体race/vet、`npm run build`、`KONRO_TEST_BINARY`を指定して`npm run test:library`（閲覧・編集永続化・複数tab競合・作成→調理・light/dark色比）、既存の`test:e2e`（runtime別processの取込8シナリオ）。
