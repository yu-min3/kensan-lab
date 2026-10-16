# レシピ取り込みとagent-runtimeの境界

KonroはURLの取得、レシピ抽出、整形用prompt/schema、原文との照合、下書き、
確認画面、レシピ保存を持つ。agent-runtimeは購読認証、text job、JSON結果だけを持つ。
Go module、内部型、ファイルシステム、モデル資格、callbackは共有しない。

接続契約はruntimeのHTTP v1。Konroの`internal/agentclient`に外部DTOを閉じ込め、
`internal/importer`は小さいinterfaceで接続する。runtimeの障害があっても、
JSON-LDの抽出・手編集・既存レシピの調理は使える。

初回は新しい取り込みレシピを専用の`imported/`へ不変IDで保存し、既存readerが
一覧・詳細として返す。従来のzipインポータはルート直下に書くため、同名のWeb
レシピを上書きしない。既存244品全体のschema移行を入口機能の前提にしない。

agentの出力は文字列の材料・手順と原文参照を持ち、Konroが検証する。
汎用JSON schema検証と、料理の内容が正しいかの検証は別である。
原文対応・数量の変更は警告として確認画面に出し、ユーザーの確認なしに保存しない。
