---
name: argocd-sync
description: Check Argo CD sync status for an application — show diff, detect drift, and suggest resolution (does NOT perform sync directly)
argument-hint: [app-name]
---

# Argo CD Sync Status

Check sync status for `$ARGUMENTS[0]` (or all apps if not specified).

## Steps

1. **Get application status**:
   - If app specified:
     ```bash
     kubectl get application $ARGUMENTS[0] -n argocd -o yaml
     ```
   - If no app specified:
     ```bash
     kubectl get applications -n argocd -o custom-columns='NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status,MESSAGE:.status.conditions[0].message'
     ```

2. **Analyze sync state**:
   - `Synced` + `Healthy` → OK
   - `OutOfSync` → show what changed
   - `Degraded` / `Missing` → investigate further

3. **Show diff** (for OutOfSync apps):
   - Check the Application CR in Git vs live state
   - Read the `values.yaml` and `resources/` for recent changes
   - Check recent Git commits: `git log --oneline -5 -- infrastructure/<path>`

4. **Common issues and solutions**:
   - **OutOfSync after values change**: Argo CD may need a manual refresh
   - **CRD not found**: Check if CRD application synced first (dependency order)
   - **Namespace not found**: Ensure environment Application is synced
   - **Helm render error**: Check values.yaml syntax

5. **Resolution guidance**:
   - Present recommended fix (Git-based, not direct sync)
   - Do NOT run `argocd app sync` directly — suggest user do it via UI or CLI
   - If the fix requires code changes, show what to edit
