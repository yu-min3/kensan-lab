# 常設表示とプロジェクトのAI更新

`/watch` は今日の意図・最新プロジェクトログ・未確認フィードを表示する。通常ホームの地図・羅針盤と `/daily` は維持する。日記全文は常設表示・AI入力へ含めない。表示の再取得（60秒）はAIを起動しない。

## 実行と入力

同じimageの `/usr/local/bin/project-watch` を短命バッチとして実行する。`KENSAN_DATA_DIR` は必須。`--dry-run` は入力hashと資料件数だけを出し、runtime呼出し・状態書込みをしない。

- `projects/<name>/README.md` のactiveプロジェクトから現在地、マイルストーン、日付付きログの最新3件を収集する。未コミット変更も検知する。daily・任意リンク・inner repoは読まない。
- 最新の `feeds/YYYY/MM/DD.md` を文脈として含める。URLやメールの収集は現行Feed取り込みを継続する。
- 内容のhashが前回成功入力と同じなら推論を省く。日次最大8件（JST）。同じ内容の再生成は通常実行ではしない。
- 初回は現存記録の要約を作るため、その内容を「前回から進んだ」と解釈しない。画面では参照記録の日付とAI生成日時を分ける。
- 資料は1README最大1MiB、抽出後1プロジェクト16KB、Feed32KB、全資料のJSON120KB。超過を隠して切り捨てずAI処理のエラーを出す。常設APIでは元ログと前回要約を保持し、入力上限の警告を表示する。

## 保存契約

| パス | 所有者・内容 |
|---|---|
| `feeds/project-watch/latest.json` | project-watchだけが書く、version 1の検証済み派生要約。生成日時、入力hash、items、根拠snapshotを含む |
| `feeds/state/project-watch.json` | 最終確認、成功、日次利用数、基準snapshot、未完了jobの固定入力・key・ID |
| `feeds/state/project-watch.lock` | OSのflockによる排他。プロセス終了で解放される。ファイルがあるだけではロック中を意味しない |

JSON成果物はアプリ表示用で、人間のREADME・目標・日記・todoは変更しない。既存Drive import、日次レビュー、確認済みフィードのファイルはそれぞれの所有者を維持する。状態には最新基準snapshotと未完了1件だけを保持し、成功で未完了snapshotを破棄する。失敗snapshotは明示的な再生成まで保持する。履歴全件を蓄積しない。

原子的renameで保存する。成果物公開後にstate更新が止まっても、次回同jobの結果を再取得して復旧する。途中の新しい人間の記録を再送入力へ混ぜない。prompt・schema・JSON入力も状態へ固定保存し、CLI再起動後も同じpayloadを送る。

## agent-runtime接続

Konroと同じバージョン付きHTTP契約を小さな専用clientで使う。Konroの内部packageやprovider資格には依存しない。

- `KENSAN_AGENT_URL`: TLS runtime URL
- `KENSAN_AGENT_TOKEN_URL`: HTTPS Keycloak token endpoint
- `KENSAN_AGENT_CLIENT_ID`: 専用client `kensan-agent-runtime`
- `KENSAN_AGENT_CLIENT_SECRET_FILE`: 専用サービス資格ファイル（owner-only、または実行groupのみread）
- `KENSAN_AGENT_CA_FILE`: runtimeの信頼CA

ローカル模擬runtimeだけはloopback HTTPと`KENSAN_AGENT_TOKEN_FILE`を利用できる。外部平文HTTP・redirectは許可しない。

送信前にkeyを保存する。同key・同payloadの再送は同じjobとなる。通信断は状態を保持し10分まで同じjobを確認する。waitingは自動retryしない。24時間を超えるjobも同じkey/IDで結果を確認する。未完了なら待機としてCLIを終え、新しい依頼は作らない。failed/cancelledを自動再試行しない。

`--reset-failed` は、runtime上で元jobが終端（failed/cancelled/succeeded）と確認できた場合だけ新しいkeyで再生成を準備する。応答喪失でID不明の依頼を捨てることはできない。元の同keyで受付結果を回収する。日次上限は再生成も含む。

出力はJSON schemaだけでなく、文字数・既知の出典ID・プロジェクトとの対応も検証する。未知の出典・任意HTMLを採用しない。意味の正しさを完全に保証するものではないため、元の根拠へ進むリンクを残す。失敗しても前回成功要約と現在の元ログを表示できる。

## 本番運用

2026-10-30、v0.3.18を配備。専用clientのTLS/認証、初回生成と出典検証、UID65532の別Pod間flockとatomic renameを確認して定期実行を有効化した。以下は再配備・環境移行時の受入手順。

1. multi-arch imageを公開し、CronJobとappのimageを配備する同一releaseへ合わせる。現在のv0.3.18はamd64/arm64とも公開済み。
2. Keycloakに専用confidential clientを作り、audience `agent-runtime` を設定する。Konro資格は流用しない。
3. senseのruntime許可clientへ追加する。配備用unitの共有30件/UTC日・FIFO1件・実行5分と、購読枠を再確認する。
4. app-kensanへ`kensan-agent-runtime` Secret（key `client-secret`）と`kensan-agent-runtime-ca` ConfigMap（key `ca.crt`）をSealedSecret/ConfigMapで用意する。raw secretはGitへ入れない。
5. workspace PVC上でflockが別Pod間でも排他できること、UID65532の書込みとatomic renameを確認する。
6. 許可された本番操作で1回実行し、TLS・認証・schema・根拠・再開・既存Konro動作を確認する。
7. 成功後GitOpsでsuspendを外す。JST08〜22時2時間毎、concurrencyPolicy Forbid、batch retryなし。

停止時はsuspendへ戻す。動作中jobは別に状態を確認し、必要時に取消する。生成物は残せば常設画面から読める。既存READMEや日記の巻き戻しは不要。
