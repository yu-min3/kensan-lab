# Longhorn Restore Test Runbook

Longhorn backup → restore 経路の月次検証手順。Phase 1 終了時 (2026-05-04) に CR ベースで実証した手順をそのまま runbook 化。

## 目的

- backup target (Cloudflare R2) からのリストアが現実に動くことを定期検証
- 新 token rotation や Longhorn upgrade 後の retrograde 互換確認
- バイト一致確認まで通すことで「backup あるけど壊れてた」事故を排除

## 推奨頻度

月次。`/reflection` 月次レビューに組み込み。Phase 3 で RecurringJob backup-monthly に同期させると自然。

## 前提

- ArgoCD で Longhorn Application が `Synced/Healthy`
- BackupTarget CR `default` の `AVAILABLE=true`
- m4neo (`hardware-class=high-performance`) が Schedulable

## 手順

### 1. テスト PVC + 書込 Pod 作成

`temp/longhorn-restore-test.yaml` (gitignored、毎回再生成 OK):

```yaml
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: restore-test-pvc, namespace: default }
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: longhorn
  resources: { requests: { storage: 1Gi } }
---
apiVersion: v1
kind: Pod
metadata: { name: restore-test-writer, namespace: default }
spec:
  nodeSelector: { hardware-class: high-performance }
  restartPolicy: Never
  containers:
    - name: busybox
      image: busybox:1.36
      command: ["sh", "-c"]
      args:
        - |
          set -e
          echo "restore-test $(date -u +%Y-%m-%dT%H:%M:%SZ)" > /data/hello.txt
          cat /data/hello.txt
          sleep 3600
      volumeMounts: [{ name: data, mountPath: /data }]
  volumes:
    - name: data
      persistentVolumeClaim: { claimName: restore-test-pvc }
```

```bash
kubectl apply -f temp/longhorn-restore-test.yaml
kubectl wait pod restore-test-writer -n default --for=condition=Ready --timeout=180s
kubectl logs restore-test-writer -n default
# 期待: hello.txt の内容が出る (タイムスタンプ付き) — この文字列を控える
```

PVC bind 後の Longhorn Volume 名 (PV 名と同じ):

```bash
VOL=$(kubectl get pvc restore-test-pvc -n default -o jsonpath='{.spec.volumeName}')
echo "Volume: $VOL"
```

### 2. Snapshot CR

```yaml
apiVersion: longhorn.io/v1beta2
kind: Snapshot
metadata: { name: restore-test-snap, namespace: longhorn-system }
spec:
  volume: <PV 名>
  createSnapshot: true
```

```bash
kubectl apply -f -
kubectl wait snapshot.longhorn.io/restore-test-snap -n longhorn-system \
  --for=jsonpath='{.status.readyToUse}'=true --timeout=60s
```

### 3. Backup CR (R2 へ push)

```yaml
apiVersion: longhorn.io/v1beta2
kind: Backup
metadata:
  name: restore-test-backup
  namespace: longhorn-system
  labels: { backup-volume: <PV 名> }
spec:
  snapshotName: restore-test-snap
```

```bash
kubectl apply -f -
# 完了まで待つ (1GB 未使用なら数十秒)
until [ "$(kubectl -n longhorn-system get backup restore-test-backup -o jsonpath='{.status.state}')" = "Completed" ]; do sleep 5; done
echo "Backup URL: $(kubectl -n longhorn-system get backup restore-test-backup -o jsonpath='{.status.url}')"
```

### 4. Restore: 新 Volume CR (`fromBackup`) + PV + PVC + Reader Pod

```yaml
---
apiVersion: longhorn.io/v1beta2
kind: Volume
metadata: { name: restore-test-restored, namespace: longhorn-system }
spec:
  fromBackup: "<status.url の値>"
  size: "1073741824"
  numberOfReplicas: 1
  frontend: blockdev
  accessMode: rwo
---
apiVersion: v1
kind: PersistentVolume
metadata: { name: restore-test-restored }
spec:
  capacity: { storage: 1Gi }
  volumeMode: Filesystem
  accessModes: [ReadWriteOnce]
  persistentVolumeReclaimPolicy: Retain
  storageClassName: longhorn
  csi:
    driver: driver.longhorn.io
    fsType: ext4
    volumeAttributes:
      numberOfReplicas: "1"
      staleReplicaTimeout: "30"
    volumeHandle: restore-test-restored
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: restore-test-restored-pvc, namespace: default }
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: longhorn
  resources: { requests: { storage: 1Gi } }
  volumeName: restore-test-restored
---
apiVersion: v1
kind: Pod
metadata: { name: restore-test-reader, namespace: default }
spec:
  nodeSelector: { hardware-class: high-performance }
  restartPolicy: Never
  containers:
    - name: busybox
      image: busybox:1.36
      command: ["sh", "-c", "cat /data/hello.txt && sleep 600"]
      volumeMounts: [{ name: data, mountPath: /data }]
  volumes:
    - name: data
      persistentVolumeClaim: { claimName: restore-test-restored-pvc }
```

```bash
kubectl apply -f -
kubectl wait pod restore-test-reader -n default --for=condition=Ready --timeout=300s
```

### 5. ファイル一致確認

```bash
W=$(kubectl exec restore-test-writer -n default -- cat /data/hello.txt)
R=$(kubectl exec restore-test-reader -n default -- cat /data/hello.txt)
echo "writer:  $W"
echo "reader:  $R"
[ "$W" = "$R" ] && echo "✓ MATCH" || echo "✗ MISMATCH (要調査)"
```

### 6. クリーンアップ

```bash
kubectl delete pod restore-test-writer restore-test-reader -n default --ignore-not-found
kubectl delete pvc restore-test-pvc restore-test-restored-pvc -n default --ignore-not-found
kubectl delete pv pv-of-original-pvc restore-test-restored --ignore-not-found  # PV は Retain なので明示削除
kubectl -n longhorn-system delete volume <元 PV 名> restore-test-restored --ignore-not-found
kubectl -n longhorn-system delete snapshot restore-test-snap --ignore-not-found
kubectl -n longhorn-system delete backup restore-test-backup --ignore-not-found
# Backup CR 削除で R2 上のオブジェクトも Longhorn が削除する
```

## 失敗時のトリアージ

| 症状 | 原因候補 | 確認 |
|---|---|---|
| Backup CR `state=Error` | R2 認証失敗 (token revoke 済み等) | `kubectl -n longhorn-system get backuptarget` の `AVAILABLE`、`status.message` |
| Backup CR が `state=` (空)で進まない | engine image 起動待ち、または node detach | `kubectl -n longhorn-system get pods -l app=longhorn-manager` |
| Restore Volume が `robustness=faulted` | R2 上の backup が破損 / R2 へのアクセス権限消失 | `kubectl -n longhorn-system describe volume restore-test-restored` |
| Reader pod `MountVolume.MountDevice failed` | volumeHandle 不一致、staleReplicaTimeout 短すぎ | PV manifest の volumeAttributes 再確認 |
| ファイル不一致 | Backup/Restore のチェックサム異常 (要 escalate) | Longhorn manager logs、R2 オブジェクト manual download で diff |

## 履歴

- 2026-05-04: Phase 1 終了時に手順実証、Yes 動作。所要時間 backup 約 10s + restore 約 90s (1GiB 未使用、初回 R2 write/read)
