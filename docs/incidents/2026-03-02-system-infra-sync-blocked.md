# Incident: system-infra ArgoCD auto-sync blocked

## Summary

| Item | Detail |
|------|------|
| Occurred | 2026-03-02 |
| Detected | 2026-03-23 |
| Impact | Label updates to the `kube-system` namespace went unapplied for ~3 weeks; the `hubble-route` HTTPRoute was rejected by the Gateway and the Cilium app went Degraded |
| Root cause | Argo CD attempted to prune the `kube-system` namespace and got blocked by a DeletionError |

## Timeline

| Time | Event |
|------|---------|
| 2025-11-17 | Last successful sync of `system-infra` (a directory reorganization) |
| 2026-02-21 | Added PSS baseline label (`system-infra`'s last file change) |
| 2026-03-02 11:37 UTC | DeletionError occurs: `namespaces "kube-system" is forbidden: this namespace may not be deleted` |
| 2026-03-19 | The goldship → kensan-lab label rename is committed but never syncs |
| 2026-03-23 | Cilium's Degraded state is noticed while investigating Backstage; root-caused from there |

## Chain of impact

```
DeletionError (kube-system prune fails)
  → system-infra's auto-sync stops entirely
    → the goldship.platform/* labels stay on kube-system
      → they no longer match the Gateway's allowedRoutes selector (kensan-lab.platform/*)
        → the hubble-route HTTPRoute becomes NotAllowedByListeners
          → Argo CD marks the Cilium app Degraded
```

## Root cause analysis

### Direct cause

Argo CD's ApplicationSet generator (environments) likely failed to read `system-infra`'s `config.json` momentarily, or a repo-server cache inconsistency dropped the `kube-system` namespace out of the managed set. With `prune: true` set, Argo CD attempted to delete it, and the Kubernetes API rejected the deletion, producing the DeletionError.

### Why it went undetected for so long

- `system-infra` only manages namespace labels, so a label diff alone doesn't produce visible damage
- The DeletionError shows up in the Argo CD UI's conditions, but the app list itself just shows OutOfSync
- Cilium's Degraded state was a problem with `hubble-route` (a monitoring UI) — the CNI itself kept working normally

### Why it happened on 3/2 with no corresponding commit

There's no repository commit on 3/2. This is presumed to be a transient inconsistency hit during one of Argo CD's periodic reconciliations (default 3-minute interval) while the ApplicationSet generator was scanning Git. A repo-server restart or a brief network blip are also possible, but no logs survived to confirm either.

## Response

1. Manually refreshed via `kubectl annotate application system-infra argocd.argoproj.io/refresh=hard`
2. Triggered a manual sync with `prune=false` → returned to Synced
3. Added the `argocd.argoproj.io/sync-options: Prune=false` annotation to `kube-system`'s `namespace.yaml` (to prevent recurrence)

## Prevention

- Attach the `Prune=false` sync option to Kubernetes-protected namespaces like `kube-system`
- Defensively annotate any Argo CD-managed resource that must never be deleted
