---
name: sealed-secret
description: Create a new Sealed Secret — generates raw secret, seals it with kubeseal, and places it in the correct resources/ directory
argument-hint: <secret-name> <namespace>
---

# Create Sealed Secret

Create sealed secret `$ARGUMENTS[0]` in namespace `$ARGUMENTS[1]`.

## Steps

1. **Determine target location**:
   - Find the component that manages namespace `$ARGUMENTS[1]`
   - Target: `infrastructure/<category>/<component>/resources/` or `infrastructure/environments/<env>/`

2. **Generate raw secret**:
   ```bash
   kubectl create secret generic $ARGUMENTS[0] \
     --namespace=$ARGUMENTS[1] \
     --from-literal=KEY1=value1 \
     --dry-run=client -o yaml > temp/$ARGUMENTS[0]-raw.yaml
   ```
   - Ask the user what key-value pairs to include
   - Write the command to `temp/create-secret.sh`

3. **Seal the secret**:
   ```bash
   kubeseal --format=yaml < temp/$ARGUMENTS[0]-raw.yaml \
     > <target-resources-dir>/$ARGUMENTS[0]-sealed.yaml
   ```

4. **Verify**:
   - Confirm sealed YAML was created
   - Confirm `temp/$ARGUMENTS[0]-raw.yaml` is git-ignored (matches `*-raw.yaml` pattern)

5. **Commit guidance**:
   - Show `git diff --stat` of changes
   - Remind: raw file stays in `temp/` (never committed), only sealed YAML is committed
   - Do NOT auto-commit — let user decide
