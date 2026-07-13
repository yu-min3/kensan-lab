---
type: book
title: "SREサイトリライアビリティエンジニアリング"
author: "Betsy Beyer 他"
status: reading
started: 2026-06-15
tags: [sre, slo, reliability]
created: 2026-06-15
updated: 2026-07-10
---

## 総評

（読了後に記入）

## 学び

- SLO は「ユーザーが感じる」指標で立てる。CPU 使用率のような内部指標を SLO にしない
- エラーバジェットは「どれだけ壊していいか」の予算。使い切ったら機能追加を止めて信頼性に振る、という合意が肝
- toil（手作業の繰り返し）を定量化して削る文化。homelab の kubectl 手作業もまさに toil

## その他

- 分厚いので興味のある章から拾い読みしている。まず SLO / エラーバジェットの章
