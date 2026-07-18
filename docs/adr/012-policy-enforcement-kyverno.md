# ADR-012: Policy Enforcement with Kyverno (validate-only / Audit → Enforce)

## Status

**Accepted** (2026-06-06) / **v2 revision** (2026-06-07): PSA-plus-Kyverno → Kyverno-only (see the revision section at the end)

## Date

2026-06-06

## Context

### The limits of PSA (Pod Security Admission)

Pod Security across the cluster has been managed via PSA namespace labels, but PSA can only do **all-or-nothing at the namespace level**. In `kubernetes/observability/namespace.yaml`, node-exporter (which requires hostNetwork / hostPID / hostPort / hostPath) is incompatible with the baseline level, so enforcing baseline across the whole `monitoring` namespace was abandoned, leaving this TODO:

```
TODO(kyverno): After integrating Kyverno, bring back baseline enforcement via a
  ClusterPolicy, exempting known workloads (node-exporter / vault etc.) per-workload
```

### Other motivations

1. **The ADR-011 silent-upgrade incident**: Vault underwent an unplanned major-version silent upgrade via the `:latest` tag (PR #271). We want an engine to enforce image-tag discipline.
2. **ADR-006's label enforcement**: ADR-006 itself demands that "the new-namespace creation procedure must enforce it as a required field." A live audit (2026-06-06) found `app-kensan` using unprefixed `team` / `app` labels — the absence of an enforcement mechanism has already produced drift.
3. **Guardrails for AD tenants**: Guarantee the quality of Backstage-deployed workloads (e.g., mandatory resource requests) via a Git-managed policy.

### Options considered

| Axis | Kyverno (adopted) | OPA Gatekeeper | PSA only (status quo) |
|---|---|---|---|
| Policy authoring | Native YAML | Rego language | Labels only |
| PSS support | Built-in `validate.podSecurity` | Custom ConstraintTemplate | ✅ (but namespace-level only) |
| Per-workload exceptions | `PolicyException` CRD | Label-based only | ✗ |
| GitOps affinity | CRD-based, extensive Argo CD track record | Comparable (two-tier setup) | — |

## Decision

**Adopt Kyverno (chart 3.8.1 / v1.16 line) in validate-only mode, with a staged Audit → Enforce rollout.**

### 1. Structure

- New category `kubernetes/policy/`. The engine (`kyverno` app, Pattern A) and the policy bundle (`kyverno-policies` app, Pattern B) are **split into separate Applications** — a policy-change PR never touches the engine's Helm sync (same philosophy as `network-policy`)
- `PolicyException` is restricted to `features.policyExceptions.namespace=kyverno`, so **only the `kyverno` namespace is accepted**. Exceptions must always go through Git (`kubernetes/policy/kyverno-policies/exceptions/`)
- Sync-wave ordering: namespace bootstrap `-4` → engine `-2` → policies `-1` (guarantees CRD registration order)

### 2. Validate only — no mutate

Mutate conflicts with Argo CD's reconciliation loop (CNCF's "GitOps and mutating policies: the tale of two loops"). Mutate → live drifts from Git → selfHeal re-applies → re-mutate loop / chronic OutOfSync.

- The correct approach in this repo is to **write the desired values directly into the Git manifest** (since every resource is Git-managed, there's structurally no role for mutate to play). Example: the ~50 platform-side containers missing resource requests were fixed by adding them to `values.yaml`.
- If mutate is ever genuinely needed, scope it to the specific Application and pair it with Argo CD 2.10+'s Server-Side Diff: `argocd.argoproj.io/compare-options: ServerSideDiff=true,IncludeMutationWebhook=true` (the diff computation goes through an SSA dry-run and compares against the predicted live state including mutations). Enabling this globally for all apps is a known issue that increases load on the admission controller, so it's scoped to a per-app annotation.

### 3. Accommodating homelab constraints

| Item | Setting | Rationale |
|---|---|---|
| `features.admissionReports` | **Disabled** | master's etcd runs on microSD. Stop per-admission EphemeralReport writes; violation visibility is consolidated into the PolicyReport produced by the background scan (1h) |
| `backgroundScanInterval` | Explicitly pinned to `1h` | Don't leave a design value to the chart default (same silent-change prevention as ADR-011) |
| `features.reporting` | Validate only | mutate / generate / imageVerify are unused |
| Each controller's replicas | 1 | HA isn't needed at homelab scale |
| Webhook `failurePolicy` | `Ignore` (set per-policy) | Don't block cluster-wide Pod creation if the admission controller (1 replica) is down. Revisit promoting to `Fail` once Enforce is stable |
| `cleanupController` | Disabled | CleanupPolicy is unused. Saves resources on the Pis |
| `backgroundController` | Disabled | Only handles generate / mutateExisting, permanently idle in a validate-only setup (background-scan PolicyReport generation is the reports-controller's job) |
| nodeAffinity | Preferred `hardware-class=high-performance` (weight 80) | Keeps webhook latency down; follows the Medium-category scheduling rule |

### 4. Initial policies and scope

| Policy | Scope | Initial mode |
|---|---|---|
| `pss-baseline` | **All namespaces except** those with the PSA `enforce=privileged` label (label-selector exclusion) | Audit |
| `disallow-latest-tag` | Application tier (`tier=application` label ∪ `app-*` / `kensan` namespace names) | Audit |
| `require-requests` | Same as above | Audit |
| `require-ns-labels` | `app-*` namespaces (ADR-006). `app-prod` is excluded as an env-shared landing zone | Audit |

The privileged-by-design namespaces (currently `kube-system` / `istio-system` / `longhorn-system`) already declare PSA `enforce: privileged` and are PE-owned territory, so they're scoped out **via a label selector treating the namespace's own PSA declaration as the source of truth** (avoiding a hardcoded name list, so adding a fourth privileged namespace never requires editing this policy). As a result, the only `PolicyException` actually needed is **the single node-exporter case** (per the 2026-06-06 live audit — 2 kensan hook Jobs were also missing requests, but those were fixed by adding requests in this same PR).

### 5. Staged rollout

1. **Phase 1**: Roll out every policy in Audit mode; observe violations and etcd load via PolicyReport for 1–2 weeks
2. **Phase 2**: Triage violations; drive them to zero either by adding `PolicyException`s or remediating in Git
3. **Phase 3**: Promote the application tier to Enforce first, then promote platform-side policies as their exceptions are cleared. Decide whether to promote webhook `failurePolicy` to `Fail` at this point too

## Consequences

### Positive

- Closes the `TODO(kyverno)` left in the `monitoring` namespace: baseline auditing plus minimal, targeted control (Host Namespaces / Host Ports / HostPath Volumes) exempted only for node-exporter
- Exceptions become declarative, Git-managed resources (in the PSA era, the only option was "give up on the whole namespace")
- Gives ADR-006 / ADR-011's discipline an actual enforcement mechanism
- Since every policy starts in Audit with `failurePolicy: Ignore`, the blast radius of the rollout itself is zero

### Trade-offs

- No enforcement during the Audit period (violations are only reported until Phase 3)
- While `failurePolicy: Ignore` is in effect, violations slip through whenever the admission controller is down (the background scan catches them after the fact). Revisit the balance against `Fail` when promoting to Enforce
- Disabling `admissionReports` means violation detection can lag by up to the background-scan interval (1h)
- Kyverno's own resource footprint (~200m / 256Mi requests across 3 controllers). Confirmed no capacity issues in practice

## v2 revision (2026-06-07): PSA-plus-Kyverno → Kyverno-only

### Why we revised

The original design kept PSA as an "in-process backstop for while Kyverno (1 replica + `failurePolicy: Ignore`) is down." An operational-design review surfaced the following, and we consolidated to **Kyverno-only**:

1. **The backstop's real value ≈ ε**: the only write path into the cluster is Argo CD (Git / PR-reviewed). Any Pod re-created while Kyverno is down is the same already-validated spec — there's no realistic path for a new violation to sneak in. Even if one slipped through, the background scan records it in a PolicyReport within an hour.
2. **The zoning had a real cognitive cost**: splitting namespaces into "PSA-enforce zone / Kyverno-only zone / privileged zone" forces operators to constantly track which bucket a given namespace falls into. An exception-handling incompatibility (PSA's `enforce` rejects upstream before `PolicyException` ever gets a chance to apply) turned out to be the root cause demanding this zoning in the first place.
3. **Alignment with industry practice**: organizations that seriously adopt Kyverno — Giant Swarm (which explicitly rejected PSA in an RFC), DoD Big Bang (Kyverno-only enforcing) — don't run PSA alongside it. The fact that Gatekeeper is widely run in production with a default `failurePolicy: Ignore` plus Audit as a supplement is further evidence that an Ignore-based operating model isn't an outlier.

### v2 design

- **PSA is deactivated by removing its labels** (it's in-tree, so no removal work is needed — with no label, it does nothing)
- **A namespace's PSS level is now declared via a Kyverno-only label, `kensan-lab.platform/pss-level`**: unset = the `pss-baseline` floor / `privileged` = excluded from the floor (`tier=platform` only, enforced by the `ns-label-contract` rule 3) / `restricted` = the `pss-restricted` policy applies (opt-in). It deliberately does not reuse PSA's label key (reusing it would re-activate PSA itself)
- **`ns-label-contract`** (absorbs `require-ns-labels`): enforces the label contract (required labels / the 3-axis model / the conditions for a `privileged` declaration) as policy — the validate-side equivalent of OpenShift's label syncer philosophy of "let the machine guarantee labels are truthful"
- **Migration order (no gap in enforcement)**: PR #2 adds the `pss-level` label (coexisting with the PSA label, all policies still Audit) → **in the same PR** as the Phase 3 Enforce promotion, remove the PSA label (an atomic swap)

### Trade-offs accepted in v2

- There's now a window, while Kyverno is down, where PSS enforcement is fully zero (the original design kept PSA holding the floor during that window). Accepted based on the ε assessment in point 1 above.
- After consolidation, Kyverno is the sole enforcement layer, which raises the value of eventually promoting `failurePolicy` to `Fail` once Enforce is stable. Promotion conditions: ① zero violations sustained for several weeks under Enforce ② `admissionController.replicas: 2` (spread across m4neo + a worker) ③ `config.webhooks` excludes `kube-system` at the webhook level (to avoid a CNI deadlock during power-outage recovery) ④ only promote application-tier policies to `Fail` (a per-policy setting). Note that neither HA nor `Fail` helps against a master/etcd SPOF, so "don't promote" also always remains a valid choice.

## Errata (2026-06-07)

- §4's enumeration of privileged-by-design namespaces ("currently `kube-system` / `istio-system` / `longhorn-system`") **omits `local-path-storage`** (as of this writing, 4 namespaces carry `pss-level: privileged` — the enumeration in `docs/concepts/policy-enforcement.md` is authoritative). As designed ("avoid a hardcoded name list"), this has no effect on the policy itself.
- The label contract's grading was formalized in [ADR-014](014-namespace-naming-label-contract-v2.md) (every namespace requires `environment` + `tier`; every `app-*` namespace additionally requires `team` + `app`; `component` is a platform convention — this resolves the conflict with ADR-006's original 3-axis definition).

## References

- [ADR-006](006-namespace-naming.md): Namespace Naming (3-axis labels)
- [ADR-011](011-vault-version-pinning.md): the Vault `:latest` silent-upgrade incident
- [`docs/concepts/policy-enforcement.md`](../concepts/policy-enforcement.md): the policy inventory / operational procedures (source of truth)
- [CNCF: GitOps and mutating policies — the tale of two loops](https://www.cncf.io/blog/2024/01/18/gitops-and-mutating-policies-the-tale-of-two-loops/)
- [Argo CD Diff Strategies (Server-Side Diff)](https://argo-cd.readthedocs.io/en/stable/user-guide/diff-strategies/)
