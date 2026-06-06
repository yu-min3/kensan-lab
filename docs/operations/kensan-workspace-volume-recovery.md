# kensan-workspace volume 復旧 runbook

kensan の生活データ SSoT volume（`app-kensan/kensan-workspace`、Longhorn RWX）が
壊れた / 作り直しが必要になったときの手順。PR #365 レビュー指摘 H-1 への回答。

## 構成の前提

```
PV: pvc-cd6553e2-2bd6-4b7c-b43d-18b0a600b707   ← 3 箇所がこの名前に依存
 ├─ PVC spec.volumeName（kubernetes/apps/app-kensan/resources/pvc-workspace.yaml）
 ├─ NFS Service selector（kubernetes/storage/longhorn/resources/service-workspace-nfs.yaml）
 └─ Mac の autofs map（/etc/auto_kensan の export path）
```

- SC `longhorn-workspace`: replica 2 / **Retain** — PVC を消しても PV とデータは残る
- バックアップ: snapshot-daily(14d) / backup-weekly(R2, 8w) / backup-monthly(R2, 12m)
  （SC の recurringJobSelector: default group で自動加入）
- Mac ローカルミラー: `~/kensan-workspace-mirror`（launchd 日次 03:30）

## ケース A: PVC を誤削除した（PV は Released で残存）

1. PV の claimRef をクリアして再 bind 可能にする:
   `kubectl patch pv pvc-cd6553e2-… --type=json -p '[{"op":"remove","path":"/spec/claimRef"}]'`
2. PVC manifest（pvc-workspace.yaml、volumeName pin 済み）を再 apply → 同じ PV に bind
3. app pod 再起動・NFS Service は selector 不変なのでそのまま復帰

## ケース B: volume を新規作成し直す（PV 名が変わる）

1. R2 backup から新 volume を restore（Longhorn UI: Backup → Restore）
2. 新 PV 名を確認し、**3 箇所を同時に更新**:
   - pvc-workspace.yaml の `spec.volumeName`
   - service-workspace-nfs.yaml の selector `longhorn.io/share-manager`
   - Mac: `/etc/auto_kensan` の export path → `sudo automount -vc`
3. PR → merge → app 復帰を確認

## ケース C: クラスタ全損

- 第一候補: Mac ミラー `~/kensan-workspace-mirror`（git 履歴込み・最大 24h 古い）から再出発
- 第二候補: R2 backup を新クラスタに restore

## mount オプションの設計判断（L-1 への記録）

Mac 側は `soft` を採用している。`hard` はデータ整合に強いが、クラスタ停止時に
Finder / シェルが永久ハングし Mac ごと人質になる（生活への影響が大きい）。
`soft` のリスク（ネットワーク断中の書き込みが I/O error → git index 破損等）は
①日次ミラー ②Longhorn snapshot ③workspace 自体の git 履歴、の三重で受ける。
体験が安定して「ハングしても困らない」運用が見えたら hard + timeo 再検討の余地あり。
