# Policy Enforcement (Kyverno-unified design)

The source of truth for Policy as Code across the cluster. See [ADR-012](../adr/012-policy-enforcement-kyverno.md) for the design rationale.

**v2 unified design**: PSS enforcement is consolidated onto Kyverno. PSA (Pod Security
Admission) is deactivated by removing its label, and a namespace's PSS level is instead
declared via the Kyverno-only label `kensan-lab.platform/pss-level` (using PSA's own
label key would re-activate PSA itself, hence the dedicated key).

## Semantics of the pss-level label

| `kensan-lab.platform/pss-level` | Meaning | Conditions to declare it | Current targets |
|---|---|---|---|
| (unset = default) | The `pss-baseline` floor applies | — | Most namespaces |
| `privileged` | Excluded from the floor (PE-exclusive namespaces that require host access) | Only `tier=platform` (enforced by `ns-label-contract` rule 3) | kube-system / istio-system / longhorn-system / local-path-storage |
| `restricted` | Applies PSS restricted, stricter than the floor (opt-in) | — | app-kensan |

Unknown values match nothing and fall back to the floor (a safe default).

## Structure

| Element | Location | Argo CD app |
|---|---|---|
| kyverno namespace | `kubernetes/policy/namespace.yaml` | `kyverno-namespace` (environments pattern, wave -4) |
| Engine (Helm) | `kubernetes/policy/kyverno/values.yaml` | `kyverno` (Pattern A, wave -2) |
| Policy bundle | `kubernetes/policy/kyverno-policies/*.yaml` | `kyverno-policies` (Pattern B, wave -1) |
| Exceptions | `kubernetes/policy/kyverno-policies/exceptions/` | Same as above |

- The engine and the policies are **separate apps** — a policy-change PR never touches the engine's Helm sync
- `PolicyException` is **only accepted in the `kyverno` namespace** (`features.policyExceptions.namespace=kyverno`). Every exception addition goes through Git

## Policy inventory

| Policy | Scope | Mode | What it does |
|---|---|---|---|
| `pss-baseline` | **All namespaces except** those with `pss-level=privileged` (the floor) | **Audit** | PSS baseline (`validate.podSecurity`) |
| `pss-restricted` | Namespaces with `pss-level=restricted` (opt-in) | **Audit** | PSS restricted (carries forward the old PSA `enforce=restricted`) |
| `disallow-latest-tag` | Application tier | **Audit** | Forbids `:latest` / an omitted tag (the ADR-011 lesson) |
| `require-requests` | Application tier | **Audit** | Requires cpu / memory requests |
| `ns-label-contract` (3 rules) | rule1: all namespaces (excluding K8s built-ins) / rule2: `app-*` (excluding app-prod) / rule3: namespaces with `pss-level=privileged` | **Audit** | rule1: requires `environment` + `tier` (with a fixed value domain) / rule2: ADR-006's 3-axis labels + `tier=application` / rule3: only `tier=platform` namespaces may declare `privileged` |

"Application tier" = namespaces with the `kensan-lab.platform/tier=application` label ∪ namespace names matching `app-*` / `kensan`.
The label is the source of truth; the name match exists as defense-in-depth so a namespace
missing its label doesn't quietly fall out of scope (a missing label is itself caught by
`ns-label-contract`). The scope definition is duplicated across the `disallow-latest-tag` /
`require-requests` match blocks and this doc in 3 places — keep them in sync when changing it.

Every policy sets `webhookConfiguration.failurePolicy: Ignore` (because the admission controller runs 1 replica; see ADR-012 §3).

## PolicyException inventory

| Exception | Target | Exempted control | Reason |
|---|---|---|---|
| `node-exporter-host-access` | `monitoring` / label selector (`prometheus-node-exporter`) | Host Namespaces / Host Ports (scoped to specific images) / HostPath Volumes | Host access is required to collect node metrics |

**Note on PolicyException's podSecurity targeting**: container-level controls (Host Ports /
Capabilities, etc.) are **not** exempted by `controlName` alone — **`images` must also be
specified** (pod-level controls like Host Namespaces / HostPath Volumes work with
`controlName` alone). After writing an exception, verify it produces a skip via an offline
evaluation — `kyverno apply <policy> --resource <live pod> --exceptions <polex>` — before
committing (this avoids waiting out the 1h background-scan interval to confirm).

## Operations

### Checking violations

```bash
kubectl get cpol                                  # ClusterPolicy Ready status
kubectl get polr -A                               # PolicyReport for namespaced resources
kubectl get cpolr                                 # cluster-scoped resources (e.g. Namespace) show up as
                                                  # ClusterPolicyReport — this is where ns-label-contract lands
kubectl get polr -A -o wide | grep -v " 0 *$"     # narrow down to reports containing a FAIL
kubectl describe polr -n <ns> <name>              # violation details
```

PolicyReports come from the background scan (1h interval, explicitly pinned in values.yaml).
Per-admission reports are disabled to protect the microSD-backed etcd
(`features.admissionReports: false`) — detection can lag by up to 1h. Background-scan
PolicyReport generation is handled by the reports-controller (the background-controller
is disabled since it only handles generate / mutateExisting).

### Cleaning up orphaned PolicyReports (after moving a namespace out of scope)

When a resource falls out of a policy's scope — e.g. adding `pss-level=privileged` to a
namespace — **its existing PolicyReport doesn't disappear on its own** (observed in
practice during Phase 1, 2026-06-07): the scan only walks resources currently in scope, and
a report's ownerReference points at the target resource itself, so garbage collection
never fires either. Old FAILs linger as "orphans."

```bash
# How to spot an orphan: FAILs remain even though the target namespace is now out of scope, and the timestamp is stale
kubectl get polr -n <ns> -o json | jq '[.items[].results[].timestamp.seconds] | max'
# Cleanup (safe — if the resource is still in scope, the next scan just regenerates the report)
kubectl get polr -n <ns> -o json | jq -r '.items[] | select(.summary.fail>0) | .metadata.name' \
  | xargs -n10 kubectl delete polr -n <ns>
```

### Violations on Argo CD hook resources persist until the "next real sync"

PostSync hooks (e.g. kensan's minio-init / polaris-init) are **excluded from Argo CD's diff
comparison**. Fixing only the hook's manifest leaves the app Synced with no auto-sync
trigger, so both the live hook resource (the old spec) and its violation persist. Resolving
this requires either a manual Sync from the Argo CD UI (which re-runs the hook) or a change
to a non-hook resource within the same app (observed in practice during Phase 1).

### Adding a policy

1. Place a ClusterPolicy at `kubernetes/policy/kyverno-policies/<name>.yaml` (start with `failureAction: Audit`, `webhookConfiguration.failurePolicy: Ignore`)
2. commit → push → Argo CD sync
3. Observe violations via PolicyReport before considering Enforce

### Adding an exception

1. Place a PolicyException at `kubernetes/policy/kyverno-policies/exceptions/<workload>.yaml` (namespace is always `kyverno`)
2. Use `podSecurity`'s `controlName` to scope the exemption as narrowly as possible
3. When exempting a Pod controller, also list `autogen-<rule>` in `ruleNames`

### Changing a namespace's PSS level

1. Set the `kensan-lab.platform/pss-level` label in the target namespace's `namespace.yaml` (`privileged` / `restricted`; unset = the baseline floor)
2. `privileged` requires the namespace to have `tier=platform`, or it violates `ns-label-contract` rule 3 (rejected once Enforce is active)
3. PR → Argo CD sync

### Promoting to Enforce (Phase 3)

1. Confirm zero violations for the target policy in PolicyReport
2. **Enforce `ns-label-contract` first** (other policies' scope depends on its labels, so start with the foundation)
3. PR changing `validate.failureAction: Audit` → `Enforce`, in order: pss-baseline / pss-restricted → latest-tag / requests
4. **Remove the PSA label in the same PR as the Enforce promotion** (an atomic swap — never leave a gap in enforcement)
5. Remove the app-tier policies' name match (`app-*` / `kensan`) and consolidate onto the label selector alone
6. Conditions for promoting the webhook to `failurePolicy: Fail` (per the ADR-012 revision): Enforce stable for several weeks + `replicas: 2` + `config.webhooks` excludes `kube-system` at the webhook level + `Fail` applies only to application-tier policies

## Known remaining work

- **Removing the PSA label** (kube-system / istio-system / longhorn-system / local-path-storage / app-kensan / app-prod / kensan / argocd / the auth namespaces / the secrets namespaces / kyverno): to be done in the same PR as the Phase 3 Enforce promotion (an atomic swap). Coexists with the pss-level label until then
- ~~**Remediating labels on bare namespaces**~~ → **resolved in Phase 2** (4 namespaces found during the first live scan):
  - `reloader` / `sealed-secrets` / `local-path-storage`: adopted the existing namespace via SSA and declared its labels using the **ns-lifecycle-app pattern** (co-locating `namespace.yaml` inside the component's own directory, picked up by `applications/namespaces/<ns>/app.yaml`'s `directory.include`) — the component itself is untouched
  - `cilium-secrets`: a namespace rendered by the Cilium chart (already tracked by the cilium app), but the chart doesn't support injecting labels onto it (`secretsNamespace` only supports create/name), and having two Applications manage the same namespace would create an Argo CD ownership conflict — so this is handled via an **exclude in rule 1** (to be removed once the chart supports labels)
  - Note: `backstage` / `kensan` were confirmed **already compliant** on inspection (both have `environment` + `tier` live)
- **`blackbox-exporter`'s `pss-baseline` FAIL is stale-history noise** (observed 2026-06-07): the violator is an old-revision ReplicaSet (replicas=0, whose template still has NET_RAW). **The current Pod is a compliant revision with NET_RAW dropped**, so no PolicyException is created (that would be a permanent grant of a permission the live workload doesn't need). It disappears naturally as Deployment revisions rotate out. **Phase 3's "zero violations" check should exclude replicas=0 historical ReplicaSets**
- ~~**Unprefixed labels on `app-kensan`**~~ → **removed in Phase 2**: confirmed zero live references to the unprefixed keys across every resource carrying a namespaceSelector (CCNP / CNP / NetworkPolicy / Gateway / AuthorizationPolicy / webhook configs) before deleting them
- **`require-requests` violation on `app-kensan`/syncthing** (observed 2026-06-07): its initContainer has no requests. The manifest lives outside this repo (in the apps repo) — requests need to be added there
- **2 remaining violations on kensan's hook Jobs**: the fix (PR #368) is already in the manifest but hasn't landed live, due to hooks being excluded from diff comparison. A manual Sync of the kensan app from the Argo CD UI resolves it (see the Operations section above)
- **Missing requests on the platform side** (~50 containers: argocd / cert-manager / longhorn, etc.): resolved by adding them to each `values.yaml`, not via mutate (ADR-012 §2)
- **verifyImages (image signature verification)**: future work, to be considered alongside introducing cosign into Backstage CI
