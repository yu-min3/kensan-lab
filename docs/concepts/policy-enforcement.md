# Policy Enforcement (unified on Kyverno)

Source of truth for the cluster's Policy as Code. For the history of the design decision, see [ADR-012](../adr/012-policy-enforcement-kyverno.md).

**v2 unified design**: PSS enforcement is consolidated into Kyverno. PSA (Pod Security Admission) is deactivated by removing its labels, and a namespace's PSS level is declared with the Kyverno-only label `kensan-lab.platform/pss-level` (reusing PSA's label key would re-activate PSA itself, hence the dedicated key).

## Semantics of the pss-level label

| `kensan-lab.platform/pss-level` | Meaning | Who may declare it | Current namespaces |
|---|---|---|---|
| (unset = default) | The `pss-baseline` floor applies | — | Most namespaces |
| `privileged` | Excluded from the floor (PE-only namespaces that require host access) | `tier=platform` only (enforced by `ns-label-contract` rule 3) | kube-system / istio-system / longhorn-system / local-path-storage |
| `restricted` | PSS restricted, stricter than the floor (opt-in) | — | app-kensan |

Unknown values match nothing and fall back to the floor (fail-safe).

## Layout

| Element | Location | Argo CD app |
|---|---|---|
| kyverno ns | `kubernetes/policy/namespace.yaml` | `kyverno-namespace` (environments pattern, wave -4) |
| Engine (Helm) | `kubernetes/policy/kyverno/values.yaml` | `kyverno` (Pattern A, wave -2) |
| Policies | `kubernetes/policy/kyverno-policies/*.yaml` | `kyverno-policies` (Pattern B, wave -1) |
| Exceptions | `kubernetes/policy/kyverno-policies/exceptions/` | same app |

- Engine and policies are **separate apps** — a policy-change PR never touches the engine's Helm sync
- `PolicyException` is **accepted only in the kyverno ns** (`features.policyExceptions.namespace=kyverno`). Exceptions are always added through Git

## Policy inventory

| Policy | Scope | Mode | Content |
|---|---|---|---|
| `pss-baseline` | **every** ns except those with `pss-level=privileged` (the floor) | **Audit** | PSS baseline (`validate.podSecurity`) |
| `pss-restricted` | ns with `pss-level=restricted` (opt-in) | **Audit** | PSS restricted (carried over from the former PSA enforce=restricted) |
| `disallow-latest-tag` | app tier | **Audit** | forbids `:latest` / omitted tags (lesson of ADR-011) |
| `require-requests` | app tier | **Audit** | cpu / memory requests required |
| `ns-label-contract` (3 rules) | rule 1: all ns (except K8s built-ins) / rule 2: `app-*` (excluding app-prod) / rule 3: ns with `pss-level=privileged` | **Audit** | rule 1: environment + tier required (with value ranges) / rule 2: ADR-006 3-axis + tier=application / rule 3: privileged may only be declared by tier=platform |

"App tier" = namespaces labeled `kensan-lab.platform/tier=application` ∪ namespaces named `app-*` / `kensan`. The label is the SoT; the name match is defense-in-depth against namespaces silently dropping out of scope when the label is missing (the missing label itself is caught by ns-label-contract). The scope definition exists in 3 places — the match blocks of disallow-latest-tag / require-requests and this doc — keep them in sync when changing it.

All policies run with `webhookConfiguration.failurePolicy: Ignore` (the admission controller has 1 replica; ADR-012 §3).

## PolicyException inventory

| Exception | Target | Exempted controls | Reason |
|---|---|---|---|
| `node-exporter-host-access` | monitoring / label selector (`prometheus-node-exporter`) | Host Namespaces / Host Ports (with images) / HostPath Volumes | host access is required to collect node metrics |

**Note on podSecurity in PolicyException**: container-level controls (Host Ports / Capabilities etc.) are **not exempted by `controlName` alone — `images` must be specified too** (pod-level controls like Host Namespaces / HostPath Volumes work with controlName only). After writing an exception, verify it evaluates to skip with the offline check `kyverno apply <policy> --resource <live pod> --exceptions <polex>` before committing (no need to wait for the 1h background scan).

## Operations

### Checking violations

```bash
kubectl get cpol                                  # ClusterPolicy Ready status
kubectl get polr -A                               # PolicyReports for namespaced resources
kubectl get cpolr                                 # cluster-scoped resources (Namespace etc.) are
                                                  # ClusterPolicyReports — ns-label-contract lives here
kubectl get polr -A -o wide | grep -v " 0 *$"     # filter to reports containing FAILs
kubectl describe polr -n <ns> <name>              # violation details
```

PolicyReports come from the background scan (1h interval, explicitly pinned in values.yaml). Per-admission reports are disabled to protect the microSD-backed etcd (`features.admissionReports: false`) — detection can lag up to 1h. Background-scan PolicyReport generation is handled by the reports-controller (the background-controller is disabled since it only serves generate / mutateExisting).

### Cleaning up orphaned PolicyReports (after moving a ns out of scope)

**When a resource leaves a policy's scope** — e.g. after labeling a ns `pss-level=privileged` — **its existing PolicyReport is not removed automatically** (measured in Phase 1, 2026-06-07): the scan only visits in-scope resources, and the report's ownerReference points at the target resource, so GC never fires. Stale FAILs linger as "orphans".

```bash
# Spotting orphans: the ns is out of scope but FAILs remain, and the timestamp is old
kubectl get polr -n <ns> -o json | jq '[.items[].results[].timestamp.seconds] | max'
# Cleanup (safe: in-scope resources are simply regenerated by the next scan)
kubectl get polr -n <ns> -o json | jq -r '.items[] | select(.summary.fail>0) | .metadata.name' \
  | xargs -n10 kubectl delete polr -n <ns>
```

### Violations on Argo CD hook resources persist until the next real sync

PostSync hooks (kensan's minio-init / polaris-init etc.) are **outside Argo CD's diff comparison**. Fixing only a hook's manifest leaves the app Synced, so auto-sync never fires, and both the live hook resource (old spec) and its violation persist. Resolution requires a manual Sync from the Argo CD UI (re-running the hook) or a change to a non-hook resource in the same app (measured in Phase 1).

### Adding a policy

1. Put a ClusterPolicy at `kubernetes/policy/kyverno-policies/<name>.yaml` (start with `failureAction: Audit` and `webhookConfiguration.failurePolicy: Ignore`)
2. commit → push → Argo CD sync
3. Observe violations via PolicyReport before considering Enforce

### Adding an exception

1. Put a PolicyException at `kubernetes/policy/kyverno-policies/exceptions/<workload>.yaml` (namespace is always `kyverno`)
2. Narrow the exempt scope as far as possible with `podSecurity` `controlName`
3. When exempting a Pod controller, also list `autogen-<rule>` in `ruleNames`

### Changing a namespace's PSS level

1. Set the `kensan-lab.platform/pss-level` label in the target ns's `namespace.yaml` (`privileged` / `restricted`; unset = baseline floor)
2. `privileged` violates `ns-label-contract` rule 3 unless the ns is `tier=platform` (denied once Enforce is on)
3. PR → Argo CD sync

### Promoting to Enforce (Phase 3)

1. Confirm the target policy shows zero violations in PolicyReports
2. **Enforce `ns-label-contract` first** (other policies' scopes depend on labels — start from the foundation)
3. Change `validate.failureAction: Audit` → `Enforce` in a PR — pss-baseline / pss-restricted first, then latest / requests
4. **Remove the PSA labels in the same PR as the Enforce promotion** (atomic swap — never leave a gap in enforcement)
5. Remove the name match (`app-*` / `kensan`) from app-tier policies, converging on the label selector
6. Conditions for promoting the webhook to `failurePolicy: Fail` (ADR-012 revision): weeks of stable Enforce + `replicas: 2` + excluding kube-system at the webhook level via `config.webhooks` + Fail only for app-tier policies

## Known remaining issues

- **PSA label removal** (kube-system / istio-system / longhorn-system / local-path-storage / app-kensan / app-prod / kensan / argocd / auth / secrets / kyverno ns): done together with the Phase 3 Enforce promotion (atomic swap). Until then the pss-level label coexists with PSA labels
- ~~**Label remediation for bare namespaces**~~ → **resolved in Phase 2** (4 ns measured in the first scan):
  - `reloader` / `sealed-secrets` / `local-path-storage`: adopted via **ns lifecycle apps** (the unified pattern: `namespace.yaml` co-located in the component directory + extracted by `directory.include` in `applications/namespaces/<ns>/app.yaml`), SSA-adopting the existing ns to declare labels without touching the component itself
  - `cilium-secrets`: a ns rendered by the cilium chart (tracked by the cilium app), but the chart can't inject ns labels (`secretsNamespace` supports create/name only), and dual-app management of one ns causes Argo CD ownership conflicts — handled by **excluding it in rule 1** (remove the exclude once the chart supports labels)
  - ※ `backstage` / `kensan` were **measured as compliant** (both have environment + tier live)
- **blackbox-exporter's pss-baseline FAIL is historical noise** (measured 2026-06-07): the violator is an old-revision ReplicaSet (replicas=0, NET_RAW still in its template). **The current pod is the compliant version with NET_RAW dropped**, so no PolicyException is created (it would permanently grant an unneeded privilege). It disappears naturally with Deployment revision rotation. **Phase 3's "zero violations" gate excludes replicas=0 historical ReplicaSets**
- ~~**app-kensan's unprefixed labels**~~ → **removed in Phase 2**: all live resources holding namespaceSelectors (CCNP / CNP / NetworkPolicy / Gateway / AuthorizationPolicy / webhook configs) were enumerated and zero references to the unprefixed keys were confirmed before deletion
- **app-kensan/syncthing require-requests violation** (measured 2026-06-07): the initContainer has no requests. The manifest lives outside this repo (apps repo) — add requests there
- **2 lingering violations on kensan's hook Jobs**: the fixed manifests (PR #368) haven't reached live because of the hook diff-exclusion issue above. A manual Sync of the kensan app from the Argo CD UI resolves it (see Operations)
- **Missing requests on the platform side** (~50 containers: argocd / cert-manager / longhorn etc.): resolved by adding them to each values.yaml, not by mutation (ADR-012 §2)
- **verifyImages (image signature verification)**: future work, considered together with introducing cosign into Backstage CI
