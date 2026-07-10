# ADR-012: Policy Enforcement with Kyverno (validate-only / Audit → Enforce)

## Status

**Accepted** (2026-06-06) / **v2 revision** (2026-06-07): PSA coexistence → Kyverno-only (see the revision section at the end)

## Date

2026-06-06

## Context

### The limits of PSA (Pod Security Admission)

The cluster's Pod Security had been managed with PSA namespace labels, but PSA is **all-or-nothing per namespace**. In `kubernetes/observability/namespace.yaml`, node-exporter (which requires hostNetwork / hostPID / hostPort / hostPath) is incompatible with baseline, forcing us to give up enforcement for the whole monitoring ns and leave this TODO:

```
TODO(kyverno): after Kyverno integration, restore enforcement via ClusterPolicy
  baseline + per-workload exemptions for known workloads (node-exporter / vault etc.)
```

### Other motivations

1. **The ADR-011 silent upgrade accident**: Vault silently crossed a major version on the `:latest` tag (PR #271). Image-tag discipline should be enforced by an engine
2. **ADR-006 label enforcement**: ADR-006 itself demands that "new-namespace creation procedure must enforce it as a required field". A live audit (2026-06-06) found app-kensan using unprefixed `team` / `app` labels — the absence of an enforcement mechanism was already producing drift
3. **Guardrails for AD tenants**: quality of Backstage-driven deployments (required requests etc.) should be guaranteed by Git-managed policy

### Options considered

| Aspect | Kyverno (adopted) | OPA Gatekeeper | PSA only (status quo) |
|---|---|---|---|
| Policy authoring | YAML-native | Rego language | labels only |
| PSS support | built-in `validate.podSecurity` | hand-written ConstraintTemplates | ◯ (but per-namespace) |
| Per-workload exceptions | `PolicyException` CRD | label-based only | ✗ |
| GitOps affinity | CRD-based, extensive Argo CD track record | comparable (two-tier setup) | — |

## Decision

**Adopt Kyverno (chart 3.8.1 / v1.16 line) validate-only, with a staged Audit → Enforce rollout.**

### 1. Layout

- New category `kubernetes/policy/`. The engine (`kyverno` app, Pattern A) and the policies (`kyverno-policies` app, Pattern B) are **separate Applications** — a policy-change PR never touches the engine's Helm sync (same philosophy as network-policy)
- `PolicyException` is accepted **only in the kyverno ns** (`features.policyExceptions.namespace=kyverno`). Exceptions also always go through Git (`kubernetes/policy/kyverno-policies/exceptions/`)
- sync-waves: ns bootstrap `-4` → engine `-2` → policies `-1` (guaranteeing CRD registration order)

### 2. validate only; no mutate

Mutation conflicts with Argo CD's reconciliation loop (CNCF: "GitOps and mutating policies: the tale of two loops"): mutate → live diverges from Git → selfHeal re-applies → re-mutate loops / permanent OutOfSync.

- Values we want injected are **written directly into the Git manifests** — the honest path for this repo (everything is Git-managed, so mutation has no structural role). Example: the ~50 platform containers missing requests get them added in each values.yaml
- If mutation ever becomes necessary, pair it with Argo CD 2.10+ Server-Side Diff scoped to the affected Application: `argocd.argoproj.io/compare-options: ServerSideDiff=true,IncludeMutationWebhook=true` (diffing goes through an SSA dry-run and compares against the mutation-inclusive predicted live state). Enabling it fleet-wide has a known admission-load issue, so keep it per-app

### 3. Homelab constraints

| Item | Setting | Reason |
|---|---|---|
| `features.admissionReports` | **disabled** | the master's etcd lives on a microSD. Stop per-admission EphemeralReport writes; violation visibility consolidates on background-scan (1h) PolicyReports |
| `backgroundScanInterval` | explicitly pinned to `1h` | don't leave a design value to the chart default (preventing the same class of silent change as ADR-011) |
| `features.reporting` | validate only | mutate / generate / imageVerify unused |
| controller replicas | 1 each | no HA needed in a homelab |
| webhook `failurePolicy` | `Ignore` (set per policy) | an admission-controller (1 replica) outage must not block pod creation cluster-wide. Promotion to `Fail` is judged after Enforce stabilizes |
| `cleanupController` | disabled | CleanupPolicy unused; saves Pi resources |
| `backgroundController` | disabled | it only serves generate / mutateExisting, permanently idle in a validate-only setup (background-scan PolicyReports are the reports-controller's job) |
| nodeAffinity | preferred `hardware-class=high-performance` (weight 80) | keeps webhook latency down; follows the Medium-category scheduling rule |

### 4. Initial policies and scope

| Policy | Scope | Initial mode |
|---|---|---|
| `pss-baseline` | **every** ns except those labeled PSA `enforce=privileged` (label-selector exclusion) | Audit |
| `disallow-latest-tag` | app tier (`tier=application` label ∪ ns names `app-*` / `kensan`) | Audit |
| `require-requests` | same | Audit |
| `require-ns-labels` | `app-*` ns (ADR-006); app-prod excluded as the env-shared landing zone | Audit |

The privileged-by-design namespaces (then kube-system / istio-system / longhorn-system) are PE-only territory with PSA `enforce: privileged` already declared, so they are scoped out via a **label selector that treats the ns-side PSA declaration as SoT** (avoiding a hardcoded name list, so adding a 4th privileged ns requires no policy edit). This leaves exactly **one PolicyException: node-exporter** (live audit 2026-06-06; kensan's 2 hook Jobs were missing requests, fixed by adding requests in the same PR).

### 5. Staged rollout

1. **Phase 1**: land all policies in Audit; observe violations and etcd load via PolicyReports for 1–2 weeks
2. **Phase 2**: triage violations; reach zero via PolicyExceptions or Git-side remediation
3. **Phase 3**: promote to Enforce starting from the app tier → platform policies as their exceptions are ready. Decide on webhook failurePolicy `Fail` promotion here too

## Consequences

### Positive

- The monitoring-ns TODO(kyverno) is repaid: baseline auditing + node-exporter exempted on the minimal controls only (Host Namespaces / Host Ports / HostPath Volumes)
- Exceptions become declarative, Git-managed resources (in the PSA era the only option was "give up on the whole namespace")
- ADR-006 / ADR-011 discipline gains an enforcement mechanism
- All policies start in Audit with failurePolicy Ignore, so the introduction has zero blast radius

### Trade-offs

- No enforcement during the Audit period (violations are only reported until Phase 3)
- While failurePolicy is Ignore, violations slip through during an admission-controller outage (the background scan still detects them after the fact). Rebalanced against Fail promotion at Enforce time
- With admissionReports disabled, violation detection lags up to the background-scan interval (1h)
- Kyverno's resource footprint (~200m / 256Mi requests, 3 controllers) — measured and confirmed to fit capacity

## v2 revision (2026-06-07): PSA coexistence → Kyverno-only

### Why the revision

The first version kept PSA as an "in-process backstop while Kyverno (1 replica + failurePolicy Ignore) is down", but an operational review made the case for **consolidating on Kyverno**:

1. **The backstop's real value ≈ ε**: the only write path into the cluster is Argo CD (Git / PR-reviewed). Pods recreated during a Kyverno outage carry already-vetted, identical specs; there is no realistic path for a new violation to enter. Anything that slips through is recorded in a PolicyReport by the background scan within 1h
2. **The zoning has real cognitive cost**: the three-way split "PSA-enforce zone / Kyverno-only zone / privileged zone" forces the operator to constantly reason about "which layer rejects this ns?". The exception incompatibility (PSA enforce rejects upstream first, neutering PolicyException) was the root cause requiring the zoning
3. **Alignment with the industry mainstream**: organizations that adopt Kyverno seriously don't run PSA alongside — Giant Swarm (explicitly rejected PSA in an RFC), DoD Big Bang (Kyverno-only enforcing). Gatekeeper shipping with failurePolicy Ignore by default plus Audit compensation is further evidence that Ignore-based operation is not heresy

### The v2 design

- **PSA is deactivated by removing its labels** (it's in-tree, so nothing to uninstall — without labels it does nothing)
- **A namespace's PSS level is declared with the Kyverno-only label `kensan-lab.platform/pss-level`**: unset = the `pss-baseline` floor / `privileged` = excluded from the floor (tier=platform only, enforced by `ns-label-contract` rule 3) / `restricted` = the `pss-restricted` policy applies (opt-in). PSA's label key is not reused (reusing it would re-activate PSA itself)
- **`ns-label-contract`** (absorbing require-ns-labels): enforces the label contract (required labels / 3-axis / privileged declaration conditions) as policy — the validate flavor of OpenShift's label-syncer idea, "a machine guards the truthfulness of labels"
- **Migration order (no enforcement gap)**: PR #2 adds the pss-level labels (coexisting with PSA labels, all policies Audit) → **the PSA labels are removed in the same PR as the Phase 3 Enforce promotion** (atomic swap)

### Trade-offs accepted in v2

- A window exists where PSS enforcement drops to zero while Kyverno is down (v1's PSA floor would have held). Accepted based on the ε assessment in point 1
- After unification Kyverno is the sole enforcement layer, which raises the value of promoting failurePolicy to `Fail` once Enforce is stable. Promotion conditions: ① weeks of zero violations under Enforce ② `admissionController.replicas: 2` (spread across two nodes) ③ excluding kube-system at the webhook level via `config.webhooks` (preventing the CNI deadlock on power-loss recovery) ④ Fail only for app-tier policies (per-policy setting). Note that neither HA nor Fail helps against the master/etcd SPOF, so "don't promote" remains a permanently valid choice

## Errata (2026-06-07)

- §4's enumeration of privileged-by-design namespaces ("then kube-system / istio-system / longhorn-system") **omits local-path-storage** (4 namespaces carried `pss-level: privileged` at time of writing; the enumeration in `docs/concepts/policy-enforcement.md` is authoritative). Per the "no hardcoded name list" design, the policy itself is unaffected
- The label contract's tiering was formalized in [ADR-014](014-namespace-naming-label-contract-v2.md) (required for all ns = `environment` + `tier`; required for `app-*` ns = `team` + `app`; `component` is a platform convention), resolving the clash with ADR-006's 3-axis definition

## References

- [ADR-006](006-namespace-naming.md): Namespace Naming (3-axis labels)
- [ADR-011](011-vault-version-pinning.md): the Vault `:latest` silent upgrade accident
- [`docs/concepts/policy-enforcement.md`](../concepts/policy-enforcement.md): policy inventory / operations (SoT)
- [CNCF: GitOps and mutating policies — the tale of two loops](https://www.cncf.io/blog/2024/01/18/gitops-and-mutating-policies-the-tale-of-two-loops/)
- [Argo CD Diff Strategies (Server-Side Diff)](https://argo-cd.readthedocs.io/en/stable/user-guide/diff-strategies/)
