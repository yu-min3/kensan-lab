# ADR-014: Namespace Naming & Label Contract v2 (Partially Supersedes ADR-006)

## Status

Accepted — **Partially supersedes [ADR-006](006-namespace-naming.md)** (the `platform-{component}` naming
convention and the 3-axis label table). ADR-006's `app-{name}` flat naming and the label-based Gateway
selector strategy remain in force.

> **Note on timing**: This ADR is a *retroactive record*. The implementation and
> `docs/concepts/namespace-label-design.md` drifted away from ADR-006 incrementally (bare-name platform
> namespaces, `tier` / `component` labels, Kyverno enforcement) without the ADR being updated. The
> 2026-06-07 design review found ADR-006 and the concept doc defining "3-axis" as two different things.
> This document records what is actually decided and enforced.

## Date

2026-06-07

## Context

ADR-006 defined two things that the implementation has since walked away from:

1. **`platform-{component}` namespace naming** (`platform-keycloak`, `platform-vault`, `platform-argocd`).
   In reality, almost all platform namespaces use **bare names** (`vault`, `cert-manager`, `kyverno`,
   `argocd`, `external-secrets`, `longhorn-system`, ...). The only `platform-*` namespace is
   `platform-auth-prod` — which itself does not follow ADR-006's format.
2. **3-axis labels = `environment` / `team` / `app`**. In reality the labels carried by every namespace and
   enforced by machinery are **`environment` / `tier` / `component`** (`tier` on 29 manifests, `component`
   on 17). `team` / `app` appear on exactly one namespace (`app-kensan`).

Meanwhile two structural facts made the original naming convention unnecessary:

- **Machine processing keys off labels, not name patterns** (ADR-006 itself established this for Gateway
  `allowedRoutes`; it now also holds for NetworkPolicy CCNPs, Kyverno policies, and PSS scoping)
- **The label contract became machine-enforced**: Kyverno `ns-label-contract` (ADR-012) validates required
  labels, the `tier=platform`-only condition for `pss-level: privileged`, and the app-namespace label set

A namespace's *name* therefore carries no contract weight; its *labels* do. Renaming namespaces to
`platform-{component}` would buy nothing and cost a destructive migration (namespace rename = delete +
recreate, with PV/PVC blast radius — a known incident class on this cluster).

## Decision

### 1. Namespace naming

```
app-{name}            <- application namespaces (unchanged from ADR-006, e.g. app-kensan)
{bare-name}           <- platform namespaces keep their natural / upstream-default names
                         (vault, cert-manager, kyverno, argocd, longhorn-system, ...)
```

The `platform-{component}` convention from ADR-006 §1 is **retired**. Existing irregular names
(`platform-auth-prod`, `auth-system`) are grandfathered; renaming them is explicitly *not* worth the
blast radius.

### 2. Label contract (the operative "3-axis")

| Label | Values | Role |
|---|---|---|
| `kensan-lab.platform/environment` | `production` / `development` / `infrastructure` | Gateway allowedRoutes selector |
| `kensan-lab.platform/tier` | `platform` / `application` | PE vs AD responsibility boundary; `pss-level: privileged` requires `tier=platform` |
| `kensan-lab.platform/component` | e.g. `keycloak`, `gitops`, `observability` | Component identification (platform tier) |

`team` / `app` (ADR-006 §2) are demoted to **optional extra axes for application-tier namespaces** —
they are part of the app-namespace label set that `ns-label-contract` checks on `app-*` namespaces, but
are not a platform-wide contract.

### 3. Single sources of truth

- **Label semantics / value catalogs**: [`docs/concepts/namespace-label-design.md`](../concepts/namespace-label-design.md)
- **Enforcement**: Kyverno `ns-label-contract` (`kubernetes/policy/kyverno-policies/`), per ADR-012
- **Why**: this ADR (and ADR-006 for the parts that survive)

A future change to label keys/values is a change to `ns-label-contract` + the concept doc, reviewed as one PR.

## Consequences

### Positive

- The ADR trail again matches what `kubectl get ns --show-labels` shows and what Kyverno enforces
- New namespaces have one unambiguous rule: bare name (or `app-{name}`) + the label contract
- No rename migration; zero blast radius

### Trade-offs

- Namespace listings do not visually group platform components (mitigated by `kubectl get ns -L
  kensan-lab.platform/tier,kensan-lab.platform/component`)
- Two grandfathered irregular names (`platform-auth-prod`, `auth-system`) remain as documented exceptions

## References

- [ADR-006](006-namespace-naming.md) — partially superseded; `app-{name}` flat naming and the
  label-selector insight remain its lasting contributions
- [ADR-012](012-policy-enforcement-kyverno.md) — `ns-label-contract` as the enforcement mechanism
- `docs/concepts/namespace-label-design.md` — label SoT
