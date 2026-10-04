---
type: note
title: "Claude追加使用OFFの共有監視"
status: active
created: 2026-10-04
updated: 2026-10-04
---

## 動作

`-isolated-worker` の通常controller起動はhost側の共有監視を必ず組み込む。無効化するflagは設けていない。mock運転では監視APIを呼ばない。

- 起動時に専用Claude認証homeの `.credentials.json` を読み、固定HTTPS endpointで追加使用状態を1回取得する。directoryはprivate、ファイルはprivateかつ通常ファイルであることを確認する。
- `extra_usage.is_enabled == false` と `used_credits == 0` が明示されている応答だけで、新規モデル推論を許可する。欠落・null・不正JSON・ON・非ゼロは拒否する。
- 取得開始時から5分の確認期限を設定する。認証期限が先なら短縮する。定期取得は5分ごとで、取得中も期限を延長しない。
- controller内の全workerは同じ確認結果を使う。配車前、worker起動前、session ACK前に確認する。ACK前の失効では許可を返さず `retry_wait` にする。新規Codex工程も課金確認待ちの間は配車しない。
- HTTP429では秒数/HTTP-date形式のRetry-Afterに従い、その時刻まで取得しない。未失効のOFF確認は期限まで使える。失効後は拒否する。不正なRetry-Afterは5分待つ。
- token変更・期限切れ・ファイル権限変更はローカル検査で直ちに拒否する。以前のtokenで取った確認結果を新しいtokenへ流用しない。次の定期取得で新しいtokenを検証する。
- controller再起動では確認結果を引き継がない。OFFのfresh応答が必要。
- OAuth tokenの自動更新は本監視に含めない。既存の公式CLI認証フローで更新後、新しいtokenの確認を待つ。
- 管理画面に「追加使用OFFの確認待ち」を出し、停止理由の変化は固定reason codeでログへ記録する。token・応答本文・HTTP例外本文は出力しない。
- Claude子processには `DISABLE_EXTRA_USAGE_COMMAND=1` を固定。購読認証・APIキー禁止・provider/model固定・quota待機の既存制約を維持する。

## 限界と運用

サービス側の追加使用OFFが課金防止の本体。監視の確認間隔中に設定がONへ変わることや、既にACKを返した要求の追加費用までゼロ保証にはできない。購入コマンド非表示も課金禁止スイッチではない。

使用しているOAuth usage endpointは安定した公開課金APIの契約を確認できていない。仕様変更・取得不能は新規推論の停止になる。HTTP redirectと環境由来proxyは使用しない。5分間隔はこの実装の初期値であり、providerが保証した頻度ではない。

稼働中のmock-only service、実機rootfs内workerのbinaryはこのコード変更では切り替わらない。独立Gateと限定された配置手順でcontroller/workerを同じ候補版に更新する必要がある。quota到達時は待機し、追加購入や別課金経路へ切り替えない。

## 検証

`go test -race ./...` と `go vet ./...` をsense-dev directoryで実行する。監視の共有、期限切れ、429の期限非延長、Retry-After、異常応答、認証変更、redirect、秘密の非出力、ACK拒否、認証済み画面の待機表示を検査する。実APIへの追加リクエスト・実モデル呼出し・service切替はこのローカル試験では行わない。
