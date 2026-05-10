# Cilium WiFi cluster stability tuning

> **2026-05-07 更新**: kensan-lab は有線 LAN メイン運用に移行済み。WiFi は fallback として残しているため、ここに記載した緩和値（`leaseDuration: 60s` 等）はそのまま維持している（fallback 切替時のゆとり）。本ドキュメントは過去 WiFi only 運用時の経緯を残す参考資料。

WiFi 経由で接続する homelab cluster (RPi 5 群が `wlan0`、Bosgame M4 Neo が `wlp3s0`) では、レイテンシスパイクで lease renewal や API 呼び出しが頻繁に timeout する。`infrastructure/network/cilium/values.yaml` でデフォルト値より緩めに調整している。

## API client rate limit

Cilium → API server の client rate limit を上げる。デフォルト (qps=10 / burst=20) では WiFi スパイク中にスロットリングが発生し、Pod 復帰後の reconcile burst が apiserver 到達不能を起こす。

```yaml
k8sClientRateLimit:
  qps: 20
  burst: 40
```

## L2 announcement lease timing

LoadBalancer IP の lease renewal が WiFi 経由で頻繁に `context deadline exceeded` を出していたため、段階的に緩めて以下に着地した。

```yaml
l2announcements:
  enabled: true
  leaseDuration: 60s
  leaseRenewDeadline: 40s
  leaseRetryPeriod: 8s
```

### 試行履歴

| 設定 | 結果 |
|---|---|
| Cilium chart default (15s / 10s / 2s) | 数分おきに lease renewal failure → LoadBalancer IP 不通 |
| 30s / 20s / 4s | 改善するが数十分に 1 回 timeout 残る |
| 60s / 40s / 8s (現在) | 安定稼働 |

### トレードオフ

renewDeadline を伸ばす = lease holder Pod がクラッシュした際に IP take-over までの遅延が長くなる。homelab スケール (4 ノード) では十分許容範囲。production グレードのクラスタでは default に近い値を推奨。

## 関連

- [Cilium update strategy](./cilium-update-strategy.md)
- 既知問題: WiFi 経由での Cilium L2 lease 不安定 (CLAUDE.md の Known Issue にも記載)
