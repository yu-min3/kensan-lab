---
name: longhorn-health
description: Longhorn の健全性チェック — volume robustness（degraded 検出）、node 容量、R2 バックアップの鮮度、recurring job の動作状況を一括診断
argument-hint:
---

# Longhorn Health Check

replicated storage と R2 バックアップ（`s3://kensan-lab-longhorn-backup@auto/`、weekly backup + daily snapshot + monthly backup の RecurringJob 構成）の状態を横断確認する。**read-only**。

## Steps

1. **コントロールプレーン（manager / engine pods）**:
   ```bash
   kubectl get pods -n longhorn-system --field-selector=status.phase!=Running,status.phase!=Succeeded
   ```

2. **Volume robustness（最重要）**:
   ```bash
   kubectl get volumes.longhorn.io -n longhorn-system -o custom-columns='NAME:.metadata.name,STATE:.status.state,ROBUSTNESS:.status.robustness,NODE:.status.currentNodeID,SIZE:.spec.size,PVC:.status.kubernetesStatus.pvcName,NS:.status.kubernetesStatus.namespace'
   ```
   - `degraded` = replica 欠け（ノード障害や容量不足で再構築が止まっている可能性）→ ❌ で報告
   - `faulted` = データアクセス不能 → 最優先で報告
   - detached + PVC なしの volume は削除候補（orphan）として列挙

3. **Node 容量とスケジューラビリティ**:
   ```bash
   kubectl get nodes.longhorn.io -n longhorn-system -o custom-columns='NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status,SCHEDULABLE:.status.conditions[?(@.type=="Schedulable")].status'
   # disk 名つきで列挙（複数 disk ノードでも対応付けが崩れない）。map の key-value 走査が必要なので go-template を使う（jsonpath は不可）
   kubectl get nodes.longhorn.io -n longhorn-system -o go-template='{{range .items}}{{.metadata.name}}:{{range $d,$s := .status.diskStatus}}{{"\n  "}}{{$d}} available={{$s.storageAvailable}} scheduled={{$s.storageScheduled}}{{end}}{{"\n"}}{{end}}'
   ```
   - Schedulable=False のノードは replica 再配置が止まる。microSD ノード（worker1/2）の容量逼迫に注意
   - storageAvailable はバイト値（÷1024³ で GiB 換算して報告）。worker1/2 は microSD で ~40GiB しかない

4. **バックアップターゲット（R2）の疎通**:
   ```bash
   kubectl get backuptargets.longhorn.io -n longhorn-system -o custom-columns='NAME:.metadata.name,AVAILABLE:.status.available,LAST-SYNC:.status.lastSyncedAt'
   ```
   - `available: false` なら credential（`longhorn-r2-backup` SealedSecret）か R2 側を疑う → `/secret-health` への導線

5. **バックアップ鮮度**:
   ```bash
   kubectl get backupvolumes.longhorn.io -n longhorn-system -o custom-columns='NAME:.metadata.name,LAST-BACKUP:.status.lastBackupName,AT:.status.lastBackupAt'
   ```
   - RecurringJob は **snapshot-daily + backup-weekly + backup-monthly**（`kubernetes/storage/longhorn/resources/recurring-jobs.yaml`）
   - **lastBackupAt が 8 日以上前の volume は ⚠️**（weekly が 1 回飛んでいる）。15 日以上は ❌
   - longhorn-SC の volume は全て default group で自動加入のはず。attached なのに backupvolume が無い volume は、**まず creationTimestamp を直近 weekly 実行時刻と比較する** — weekly 後に作られた新規 volume なら未バックアップは正常（次回 weekly 待ち）。weekly より古いのに無ければ真の selector 漏れとして ❌:
     ```bash
     kubectl get volumes.longhorn.io <name> -n longhorn-system -o jsonpath='{.metadata.creationTimestamp}{"  group="}{.metadata.labels.recurring-job-group\.longhorn\.io/default}{"\n"}'
     ```
     `group=enabled` が出れば default group 加入済み（label の値は group 名ではなく `enabled`）

6. **RecurringJob の存在確認**:
   ```bash
   kubectl get recurringjobs.longhorn.io -n longhorn-system
   ```

7. **Summary**:
   - Volume / Node / BackupTarget / 鮮度 の 4 観点で ✅ / ⚠️ / ❌ を 1 行ずつ
   - degraded volume があれば対象 PVC と載っているワークロードを特定し、`/troubleshoot longhorn` への導線を示す
   - 修復操作（replica 再構築のトリガー、salvage 等）は提案のみ。実行はユーザー確認を取る

## Notes

- Grafana MCP が使える場合は `longhorn_volume_robustness` / `longhorn_backup_state` メトリクスで時系列の劣化傾向も補足できる（ServiceMonitor 設定済み、storage-plan §5.9）
- `local-path` PVC は本 skill の対象外（legacy・node-local。`kubernetes-cluster.md` 参照）
