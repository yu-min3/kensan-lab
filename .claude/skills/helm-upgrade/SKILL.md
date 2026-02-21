---
name: helm-upgrade
description: Upgrade a Helm chart version for an Argo CD managed component by updating targetRevision in the Application CR
argument-hint: <component> <new-version>
---

# Helm Chart Upgrade

Upgrade `$ARGUMENTS[0]` to chart version `$ARGUMENTS[1]`.

## Steps

1. **Locate Application CR**:
   - Search: `infrastructure/gitops/argocd/applications/**/$ARGUMENTS[0]/app.yaml`
   - If not found, check `config.json` pattern

2. **Read current version**:
   - Show current `targetRevision` value
   - Confirm the upgrade: `current → $ARGUMENTS[1]`

3. **Update targetRevision**:
   - Edit `spec.sources[0].targetRevision` (or equivalent in config.json) to `$ARGUMENTS[1]`

4. **Check for breaking changes**:
   - If major version bump, warn the user to review changelog
   - Suggest checking `values.yaml` compatibility

5. **Show diff**:
   - Display the file diff for review
   - Do NOT auto-commit — let user review and decide

6. **Post-upgrade checklist**:
   - Remind to check Argo CD sync status after push
   - If CRD changes expected, warn about potential sync order issues
