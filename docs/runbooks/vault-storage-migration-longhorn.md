# Vault storage migration: local-path → Longhorn

> **Status: executed (2026-07-06).** Every phase below was carried out exactly as
> written and completed with zero downtime and quorum maintained throughout
> (the leader stepped down and moved to vault-2). Kept from here on as a reference
> procedure for rebuilding storage on any Vault node in general.

## Background

PR #324 (disaster recovery) changed the storageClass in
`kubernetes/secrets/vault/values.yaml` to `longhorn`, but **the live StatefulSet
kept running on `local-path`**. Since `volumeClaimTemplates` is immutable, Argo CD's
server-side diff fails permanently, and the `vault` Application shows
**SYNC STATUS: Unknown** (ComparisonError) indefinitely.

```
StatefulSet.apps "vault" is invalid: spec: Forbidden: updates to statefulset spec for fields
other than 'replicas', 'ordinals', 'template', ... are forbidden
```

On top of that, `local-path` is node-local (a Pod can't move to a different node) and
uses **reclaim=Delete** (deleting the PVC instantly destroys the physical data) —
an unsuitable place to keep Vault's raft data.

## Current state (confirmed live, 2026-07-06)

| Item | Value |
|---|---|
| StatefulSet | `vault` namespace, replicas=3, live VCT: `local-path` 10Gi / git: `longhorn` |
| PVC | `data-vault-{0,1,2}` + `audit-vault-{0,1,2}` (on worker2 / m4neo / worker1) |
| Unseal | AWS KMS auto-unseal (`alias/vault-unseal-kensan`) — **no manual unseal needed** |
| Raft join | `retry_join` ×3 already configured (`docs/runbooks/vault-raft-join.md`) — **no manual join needed** |

Thanks to these two things (auto-unseal + retry_join), **a member whose data was wiped
recovers automatically just by recreating its Pod — it unseals itself, rejoins the raft
cluster, and syncs a snapshot from the leader**. That makes a per-node "strip it and
rejoin" strategy viable.

## Strategy

- **Phase A**: delete the StatefulSet with `--cascade=orphan` → Argo CD recreates the
  StatefulSet using the longhorn VCT (the Pods stay running throughout). → clears the
  ComparisonError, `vault` app returns to Synced
- **Phase B**: migrate members one at a time (always keeping quorum at 2/3): raft
  remove-peer → delete PVC/Pod → the new Pod comes up on a longhorn PVC and auto-rejoins
- **Phase C**: verification and cleanup

## Pre-flight checks (all must pass before starting)

```bash
# 1. raft is healthy with 3 peers (1 leader + 2 followers)
kubectl exec -n vault vault-0 -c vault -- sh -c 'VAULT_TOKEN=$(cat /tmp/token 2>/dev/null || echo $VAULT_TOKEN) vault operator raft list-peers'
# NOTE: requires login first. Run `vault login` with an OIDC admin or the root token before this

# 2. Take a raft snapshot and stash it locally (a last-resort safety net)
kubectl exec -n vault vault-0 -c vault -- vault operator raft snapshot save /tmp/pre-migration.snap
kubectl cp vault/vault-0:/tmp/pre-migration.snap ./temp/vault-pre-migration-$(date +%Y%m%d).snap -c vault

# 3. Confirm KMS auto-unseal is working (e.g. it went Sealed:false after a recent Pod restart)
kubectl exec -n vault vault-0 -c vault -- vault status | grep -E "Sealed|Total Shares"   # Sealed: false / Recovery Seed

# 4. Longhorn is healthy (every volume healthy, >30Gi free)
kubectl get volumes.longhorn.io -n longhorn-system --no-headers | awk '$3!="attached" && $2!="detached"' | head

# 5. ESO / dynamic secrets are healthy (a baseline check so a break during migration is noticeable)
kubectl get externalsecret -A --no-headers | grep -cv SecretSynced   # → should be 0
```

## Phase A: recreate the StatefulSet (no Pod downtime, clears ComparisonError)

```bash
# 1. Delete only the StatefulSet (Pods and PVCs stay)
kubectl delete sts vault -n vault --cascade=orphan

# 2. Wait for Argo CD to recreate the StatefulSet from Git (the longhorn VCT)
kubectl get application vault -n argocd -w   # → should become Synced / Healthy
kubectl get sts vault -n vault -o jsonpath='{.spec.volumeClaimTemplates[0].spec.storageClassName}'  # → longhorn

# 3. Confirm the Pods weren't picked up and restarted (AGE is unchanged)
kubectl get pods -n vault
```

**Check**: the `vault` app's SYNC STATUS flips from Unknown to **Synced**.
Existing Pods keep running on their old local-path PVCs (the VCT only takes effect for new Pods).

## Phase B: migrate members one at a time (vault-2 → vault-1 → vault-0)

Save the leader for last. Check the active node with `vault status | grep "HA Mode"` —
if vault-2 or vault-1 is active, run `vault operator step-down` on it first to move leadership away.

For each member (using vault-2 as the example):

```bash
# 1. Cleanly remove the peer from raft
kubectl exec -n vault vault-0 -c vault -- vault operator raft remove-peer vault-2

# 2. Schedule the PVCs for deletion (they're in use, so this waits in Terminating)
kubectl delete pvc data-vault-2 audit-vault-2 -n vault --wait=false

# 3. Delete the Pod → the PVC deletion completes, and the StatefulSet creates a new Pod + longhorn PVCs
kubectl delete pod vault-2 -n vault

# 4. Confirm the new Pod comes up and recovers automatically (fully automatic via KMS auto-unseal + retry_join)
kubectl get pvc -n vault | grep vault-2          # → should now be longhorn
kubectl get pod vault-2 -n vault -w              # → should reach 2/2 Running
kubectl exec -n vault vault-2 -c vault -- vault status | grep Sealed   # → should be false

# 5. Confirm all 3 raft peers are present and synced before moving to the next member
kubectl exec -n vault vault-0 -c vault -- vault operator raft list-peers
kubectl exec -n vault vault-0 -c vault -- vault operator raft autopilot state | grep -A5 Servers  # healthy: true
```

**Never do this**: work on 2 members at the same time (loses quorum = total outage).
Don't move to the next member until the current one's re-sync is fully complete (autopilot healthy).

When it's vault-0's turn, if it's the active leader, step it down first:

```bash
kubectl exec -n vault vault-0 -c vault -- vault operator step-down
# Confirm the active leader moved to vault-1/2, then apply the same procedure to vault-0
# Run remove-peer against the new leader:
kubectl exec -n vault vault-1 -c vault -- vault operator raft remove-peer vault-0
```

## Phase C: verification and cleanup

```bash
# All PVCs are longhorn
kubectl get pvc -n vault    # all 6 should show STORAGECLASS=longhorn

# raft has 3 peers / autopilot healthy / app Synced
kubectl exec -n vault vault-0 -c vault -- vault operator raft list-peers
kubectl get application vault -n argocd

# Consumers are unaffected (ESO sync, Keycloak dynamic creds, cert-manager)
kubectl get externalsecret -A --no-headers | grep -cv SecretSynced   # → should be 0

# Longhorn side: confirm all 6 vault volumes are healthy and picked up by the RecurringJob (R2 backup)
kubectl get volumes.longhorn.io -n longhorn-system | grep -c healthy
```

- Keep the pre-migration raft snapshot around for about a week, then discard it
- local-path PVs use reclaim=Delete, so they vanish automatically when their PVC is
  deleted (visually confirm no leftovers remain under `/opt/local-path-provisioner/` on the node)

## Rollback

- **If Phase A fails** (the StatefulSet recreation ends up in a bad state):
  `kubectl delete sts vault --cascade=orphan` → the old-VCT StatefulSet can be
  hand-applied again even without a git revert (though the default plan is to always
  move forward — the Pods stay alive throughout, so there's no rush)
- **If a member fails to recover in Phase B**: quorum is still alive on the remaining 2
  peers. Try the manual join procedure in `vault-raft-join.md` → if that still doesn't
  work, recreate the PVC/Pod once more. **As a last resort**, restore from the
  pre-migration snapshot via `vault operator raft snapshot restore`
- **Never do this**: start on the next member while the current one is still in a failed-recovery state

## Remaining work (out of scope for this runbook)

- Migrating the 5 local-path PVCs in the `monitoring` namespace (prometheus / loki /
  tempo / grafana / alertmanager) — since this is retention data, "recreate the
  StatefulSet and accept data loss" is also a viable option
- Once every local-path PVC is gone: remove `local-path-provisioner` and
  `applications/namespaces/local-path-storage` (the W15 completion condition)
