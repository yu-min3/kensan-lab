---
type: project
tags: [observability, prometheus, grafana, slo]
created: 2026-03-01
updated: 2026-07-12
status: active
deadline: 2026-08-31
---

## 概要

「壊れたことに気づけない」をなくす。メトリクス・ログ・トレースを 1 本の OTel パイプに集約し、監視自体もクラスタ外から監視する 3 層モデルを敷く。

## 目標

2026 年 8 月末までに、ノード故障・auth 経路断・ストレージ枯渇の 3 種のサイレント障害を、発生から 5 分以内に検知できる状態にする。

## マイルストーン

- [x] OTel Collector を中央チョークポイント化
- [x] Prometheus + Grafana + Loki + Tempo
- [x] クラスタ外 dead-man's switch（Grafana Cloud remote_write）
- [ ] Blackbox Exporter による auth 経路の能動監視

## タスク

- [ ] etcd WAL fsync latency を microSD 摩耗の先行指標としてアラート化 @today @p(10)
- [ ] Slack 通知のノイズ（Watchdog / InfoInhibitor）を null ルートに @due(2026-07-18) @p(20)
- [ ] SLO ダッシュボード（エラーバジェット）を Sloth で生成 @p(30)
- [ ] worker ノードの microSD 摩耗アラートのしきい値を見直す @p(40)

## ログ

- 2026-07-09: 8 日間サイレントだったノード凍結を機に dead-man's switch を追加
- 2026-06-15: auth 経路の断が 3 週間気づかれなかった。HTTPS 能動監視を計画
