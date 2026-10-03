---
title: sense v0 日報配送
status: active
---

# 日報配送

JST 20:00 以降に controller が当日の snapshot を outbox に固定する。`-report-slack-channel`、`-report-slack-token-file`、`-report-base-url` をすべて明示した場合だけ、Slack `chat.postMessage` へ1通送る。未設定なら `waiting_destination` のまま preview と画面確認ができる。宛先 ID は固定設定で、本文や UI に token は入れない。リンクの base URL は HTTPS の本人限定入口ができた後に設定する。

## 状態

| 状態 | 意味 | 次の操作 |
|---|---|---|
| `waiting_destination` | 宛先未設定 | Yu が宛先と外部入口を決める |
| `queued` | 固定 snapshot と宛先があり送信待ち | timer が `sending` を先行記録して1回送信 |
| `sending` | 外部呼び出し中 | restart 時は `unknown` へ移す |
| `sent` | Slack の receipt `ts` を取得 | 二度送らない |
| `unknown` | timeout/停止などで結果が曖昧 | Slack 履歴を確認し、UI で `sent`/`failed` と証拠を記録 |
| `failed` | 未送信を確認済み | UI で根拠を付けて新 attempt を予約 |
| `missed` | controller 停止中の過去日 | 当時の snapshot を捏造しない |

失敗や timeout で送られた可能性があるため、外部 API エラーは `unknown` とする。`unknown` は自動再送されない。UI からの照合と再試行は CSRF、日付、attempt ID と状態を検査する。宛先が変わった場合も timer は自動送信を止める。

## 実証前の条件

- Yu が Slack の専用宛先 ID と送信時刻を確定する。現在の 20:00 は仮値。
- bot token は sense 上の mode `0600` file とし、controller 以外の worker へ渡さない。
- `-report-base-url` は実スマホで本人限定到達でき、タスク・判断リンクが開く HTTPS 入口にする。
- 連続2日の実送信と receipt、意図的な timeout/不明照合を T030 / AC-11 の証拠に残す。

現行 mock-only service の unit にはこの3 flag が無く、実 Slack 送信は開始していない。
