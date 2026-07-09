# Incident: system-infra ArgoCD auto-sync blocked

## Summary

| Item | Detail |
|------|------|
| Occurred | 2026-03-02 |
| Detected | 2026-03-23 |
| Impact | Label updates on the kube-system namespace went unapplied for ~3 weeks; the hubble-route HTTPRoute was rejected by the Gateway and the Cilium app showed Degraded |
| Root cause | ArgoCD attempted to prune the kube-system namespace and blocked on a DeletionError |

## Timeline

| When | Event |
|------|---------|
| 2025-11-17 | Last successful sync of system-infra (directory reshuffle) |
| 2026-02-21 | PSS baseline label added (last file change to system-infra) |
| 2026-03-02 11:37 UTC | DeletionError: `namespaces "kube-system" is forbidden: this namespace may not be deleted` |
| 2026-03-19 | The goldship → kensan-lab label rename was committed but never synced |
| 2026-03-23 | Cilium Degraded noticed during Backstage work; discovered while investigating |

## Failure chain

```
DeletionError (kube-system prune failed)
  → all auto-sync of system-infra stops
    → goldship.platform/* labels linger on kube-system
      → no match for the Gateway's allowedRoutes selector (kensan-lab.platform/*)
        → hubble-route HTTPRoute becomes NotAllowedByListeners
          → ArgoCD marks the Cilium app Degraded
```

## Root cause analysis

### Direct cause

Most likely the ApplicationSet generator (environments) transiently failed to read system-infra's config.json, or a repo-server cache inconsistency dropped the kube-system namespace from the managed set. With `prune: true`, ArgoCD attempted deletion; the Kubernetes API refused to delete kube-system, producing the DeletionError.

### Why it went undetected for so long

- system-infra manages only namespace labels, so a label diff has no visible user impact
- The DeletionError appears in the ArgoCD UI's conditions, but the app list shows only OutOfSync
- Cilium's Degraded came from hubble-route (a monitoring UI); the CNI itself kept working

### Why it happened on 3/2 with no commit

There were no commits to the repository on 3/2. The likely trigger is a transient inconsistency while the ApplicationSet generator scanned Git during ArgoCD's periodic reconciliation (default 3-minute interval). A repo-server restart or a momentary network blip are also possible, but no logs survive to pinpoint it.

## Response

1. Manual refresh via `kubectl annotate application system-infra argocd.argoproj.io/refresh=hard`
2. Manual sync with prune=false → back to Synced
3. Added the `argocd.argoproj.io/sync-options: Prune=false` annotation to kube-system's namespace.yaml (recurrence prevention)

## Prevention

- Give protected Kubernetes namespaces like kube-system the `Prune=false` sync option
- Defensively annotate any ArgoCD-managed resource that must never be deleted
