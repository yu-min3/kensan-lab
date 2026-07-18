# Longhorn Restore Test Runbook

Monthly verification procedure for the Longhorn backup → restore path. Turned directly into a runbook from the CR-based procedure proven out at the end of Phase 1 (2026-05-04).

## Purpose

- Regularly verify that restoring from the backup target (Cloudflare R2) actually works
- Confirm backward compatibility after a token rotation or a Longhorn upgrade
- Rule out the "we have a backup, but it was broken" failure mode by checking byte-for-byte equality

## Recommended frequency

Monthly. Fold it into the `/reflection` monthly review. Once Phase 3's `backup-monthly` RecurringJob exists, it's natural to sync this with it.

## Prerequisites

- The Longhorn Application is `Synced/Healthy` in Argo CD
- The `default` BackupTarget CR shows `AVAILABLE=true`
- m4neo (`hardware-class=high-performance`) is Schedulable

## Procedure

### 1. Create a test PVC + a writer pod

`temp/longhorn-restore-test.yaml` (gitignored, safe to regenerate every time):

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
# Expected: prints hello.txt's content (with a timestamp) — note this string down
```

The Longhorn Volume name after the PVC binds (same as the PV name):

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
  volume: <PV name>
  createSnapshot: true
```

```bash
kubectl apply -f -
kubectl wait snapshot.longhorn.io/restore-test-snap -n longhorn-system \
  --for=jsonpath='{.status.readyToUse}'=true --timeout=60s
```

### 3. Backup CR (push to R2)

```yaml
apiVersion: longhorn.io/v1beta2
kind: Backup
metadata:
  name: restore-test-backup
  namespace: longhorn-system
  labels: { backup-volume: <PV name> }
spec:
  snapshotName: restore-test-snap
```

```bash
kubectl apply -f -
# Wait for completion (a few dozen seconds for an under-1GB volume)
until [ "$(kubectl -n longhorn-system get backup restore-test-backup -o jsonpath='{.status.state}')" = "Completed" ]; do sleep 5; done
echo "Backup URL: $(kubectl -n longhorn-system get backup restore-test-backup -o jsonpath='{.status.url}')"
```

### 4. Restore: a new Volume CR (`fromBackup`) + PV + PVC + reader pod

```yaml
---
apiVersion: longhorn.io/v1beta2
kind: Volume
metadata: { name: restore-test-restored, namespace: longhorn-system }
spec:
  fromBackup: "<the value of status.url>"
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

### 5. Verify the file matches

```bash
W=$(kubectl exec restore-test-writer -n default -- cat /data/hello.txt)
R=$(kubectl exec restore-test-reader -n default -- cat /data/hello.txt)
echo "writer:  $W"
echo "reader:  $R"
[ "$W" = "$R" ] && echo "✓ MATCH" || echo "✗ MISMATCH (needs investigation)"
```

### 6. Cleanup

```bash
kubectl delete pod restore-test-writer restore-test-reader -n default --ignore-not-found
kubectl delete pvc restore-test-pvc restore-test-restored-pvc -n default --ignore-not-found
kubectl delete pv pv-of-original-pvc restore-test-restored --ignore-not-found  # PVs use Retain, so delete explicitly
kubectl -n longhorn-system delete volume <original PV name> restore-test-restored --ignore-not-found
kubectl -n longhorn-system delete snapshot restore-test-snap --ignore-not-found
kubectl -n longhorn-system delete backup restore-test-backup --ignore-not-found
# Deleting the Backup CR also has Longhorn delete the corresponding object on R2
```

## Failure triage

| Symptom | Likely cause | How to check |
|---|---|---|
| Backup CR `state=Error` | R2 authentication failure (e.g. a revoked token) | `AVAILABLE` and `status.message` from `kubectl -n longhorn-system get backuptarget` |
| Backup CR stuck with `state=` (empty), no progress | Waiting on the engine image to start, or a node detach | `kubectl -n longhorn-system get pods -l app=longhorn-manager` |
| Restore Volume shows `robustness=faulted` | The backup on R2 is corrupted / access to R2 was lost | `kubectl -n longhorn-system describe volume restore-test-restored` |
| Reader pod: `MountVolume.MountDevice failed` | volumeHandle mismatch, or staleReplicaTimeout too short | Re-check the PV manifest's volumeAttributes |
| File mismatch | A checksum anomaly during backup/restore (escalate this) | Longhorn manager logs; manually download the R2 object and diff |

## History

- 2026-05-04: procedure proven out at the end of Phase 1, worked correctly. Timing: backup ~10s + restore ~90s (under 1GiB, first R2 write/read)
