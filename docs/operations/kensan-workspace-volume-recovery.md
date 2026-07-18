# kensan-workspace volume recovery runbook

Procedure for when kensan's life-data source-of-truth volume
(`app-kensan/kensan-workspace`, Longhorn RWX) is broken or needs to be
recreated. Written in response to PR #365 review finding H-1.

## Assumed setup

```
PV: pvc-cd6553e2-2bd6-4b7c-b43d-18b0a600b707   ← 3 places depend on this exact name
 ├─ the PVC's spec.volumeName (kubernetes/apps/app-kensan/resources/pvc-workspace.yaml)
 ├─ the NFS Service's selector (kubernetes/storage/longhorn/resources/service-workspace-nfs.yaml)
 └─ the Mac's autofs map (the export path in /etc/auto_kensan)
```

- StorageClass `longhorn-workspace`: 2 replicas / **Retain** — deleting the PVC leaves the PV and its data intact
- Backups: snapshot-daily (14d) / backup-weekly (R2, 8w) / backup-monthly (R2, 12m)
  (auto-enrolled via the StorageClass's `recurringJobSelector: default` group)
- Mac-side local mirror: `~/kensan-workspace-mirror` (launchd, daily at 03:30)

## Case A: the PVC was deleted by mistake (the PV survives as Released)

1. Clear the PV's `claimRef` so it can be re-bound:
   `kubectl patch pv pvc-cd6553e2-… --type=json -p '[{"op":"remove","path":"/spec/claimRef"}]'`
2. Re-apply the PVC manifest (`pvc-workspace.yaml`, which already pins `volumeName`) → it binds to the same PV
3. Restart the app pod — the NFS Service's selector is unchanged, so it recovers on its own

## Case B: recreating the volume from scratch (the PV name changes)

1. Restore a new volume from an R2 backup (Longhorn UI: Backup → Restore)
2. Note the new PV name, and **update all 3 places at once**:
    - `pvc-workspace.yaml`'s `spec.volumeName`
    - `service-workspace-nfs.yaml`'s selector `longhorn.io/share-manager`
    - The Mac: the export path in `/etc/auto_kensan` → `sudo automount -vc`
3. PR → merge → confirm the app recovers

## Case C: total cluster loss

- First choice: rebuild from the Mac mirror `~/kensan-workspace-mirror` (includes git history; up to 24h stale)
- Second choice: restore an R2 backup into the new cluster

## Mount option design rationale (recorded for finding L-1)

The Mac side uses `soft`. `hard` is more resilient for data consistency, but during a
cluster outage it makes Finder / the shell hang permanently, holding the whole Mac
hostage (a significant impact on daily life). The risk that `soft` carries (a write
during a network drop returns an I/O error, which could corrupt the git index, etc.) is
absorbed three ways: ① the daily mirror ② Longhorn snapshots ③ the workspace's own git
history. If experience shows a stable "hanging is fine" operating pattern, revisiting
`hard` + `timeo` is worth reconsidering.
