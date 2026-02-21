---
name: new-component
description: Scaffold a new infrastructure component with Helm multi-source Application pattern (app.yaml + values.yaml + resources/)
argument-hint: <category> <component-name>
---

# New Infrastructure Component

Scaffold `$ARGUMENTS[1]` under category `$ARGUMENTS[0]`.

## Steps

1. **Validate category**:
   - Must be one of: `observability`, `network`, `security`, `environments`, `storage`, `gitops`
   - If invalid, suggest closest match

2. **Create component directory**:
   ```
   infrastructure/$ARGUMENTS[0]/$ARGUMENTS[1]/
   ├── values.yaml
   └── resources/
   ```

3. **Create Application CR**:
   ```
   infrastructure/gitops/argocd/applications/$ARGUMENTS[0]/$ARGUMENTS[1]/app.yaml
   ```
   - Ask user for: Helm chart repo URL, chart name, target version
   - Use existing app.yaml files as template (read one from same category)
   - Set namespace and project appropriately

4. **Populate values.yaml**:
   - Create minimal values.yaml with placeholder comment
   - If user provides chart repo, fetch default values for reference: `helm show values <repo>/<chart>`

5. **Create resources/ placeholder**:
   - Add `.gitkeep` or initial HTTPRoute if the component needs external access

6. **Verify structure**:
   - List created files
   - Confirm Application CR references correct paths
   - Remind user to check if Root App auto-discovers this category

7. **Do NOT auto-commit** — let user review the scaffolded files
