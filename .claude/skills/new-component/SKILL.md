---
name: new-component
description: Scaffold a new infrastructure component, choosing the right layout (Helm multi-source / raw YAML / ApplicationSet) for its category
argument-hint: <category> <component-name>
---

# New Infrastructure Component

Scaffold `$ARGUMENTS[1]` under category `$ARGUMENTS[0]`.

## Steps

1. **Validate category**:
   - The source tree under `kubernetes/` uses these categories: `apps`, `argocd`, `auth`, `environments`, `kube-system`, `network`, `observability`, `policy`, `secrets`, `storage`
   - Note: secrets/sealed-secrets live under `secrets` (not `security`); Argo CD itself lives under `argocd` (its Application CR is `kubernetes/argocd/applications/gitops/`)
   - If invalid, suggest the closest match

2. **Pick the component layout**. Read `kubernetes/README.md` (Pattern A/B) and `.claude/rules/helm-multisource.md` for the authoritative rules, then choose:

   - **Pattern A — Helm chart (`values.yaml` + `resources/`)**: the Argo CD app renders an upstream Helm chart and combines it with extra raw YAML. Most of `secrets`, `storage`, `auth`, `network` Helm components use this.
     ```
     kubernetes/$ARGUMENTS[0]/$ARGUMENTS[1]/
     ├── values.yaml           # Helm values
     └── resources/            # chart 外の生 YAML (namespace, HTTPRoute, SealedSecret 等)
     ```
   - **Pattern B — raw YAML only (flat)**: no Helm chart; the Argo CD app reads plain manifests. Put `.yaml` files directly in the component dir (do NOT add `resources/`). Examples: `kubernetes/kube-system/`, `kubernetes/namespaces/*`.
     ```
     kubernetes/$ARGUMENTS[0]/$ARGUMENTS[1]/
     └── *.yaml                # raw manifests, flat
     ```
   - **observability — ApplicationSet + `config.json`**: components under `kubernetes/observability/` are NOT scaffolded with their own `app.yaml`. They are discovered by `kubernetes/argocd/applications/observability/applicationset.yaml`, which reads a per-component `config.json`. To add one, create `kubernetes/observability/$ARGUMENTS[1]/` with `config.json` (+ `values.yaml` / `resources/` per the chart) and let the ApplicationSet pick it up — do NOT create an `app.yaml`.
     ```
     kubernetes/observability/$ARGUMENTS[1]/
     ├── config.json           # name/chart/chartRepo/chartVersion/namespace/hasResources/createNamespace
     ├── values.yaml           # if Helm-based
     └── resources/            # if hasResources: "true"
     ```

3. **Create the Application CR** (skip for observability — the ApplicationSet handles it):
   ```
   kubernetes/argocd/applications/$ARGUMENTS[0]/$ARGUMENTS[1]/app.yaml
   ```
   - Ask user for: Helm chart repo URL, chart name, target version (Pattern A only)
   - Use existing app.yaml files as template (read one from the same category)
   - Set namespace and project appropriately

4. **Populate sources**:
   - Pattern A: minimal `values.yaml` with placeholder comment; `helm show values <repo>/<chart>` for reference
   - Pattern B: the raw manifests directly
   - observability: fill in `config.json` (mirror an existing one like `grafana/config.json`)

5. **Add `resources/` only when it applies** (Pattern A or observability with `hasResources: "true"`):
   - Initial HTTPRoute / SealedSecret / namespace as needed

6. **Verify structure**:
   - List created files
   - Confirm the Application CR (or ApplicationSet config) references correct paths
   - Remind user to check if the Root App / ApplicationSet auto-discovers this component

7. **Do NOT auto-commit** — let user review the scaffolded files
