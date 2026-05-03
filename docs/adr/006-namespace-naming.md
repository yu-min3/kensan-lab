# ADR-006: Application Namespace Naming Convention (`app-{name}` flat + 3-axis labels)

## Status

Accepted

## Date

2026-05-03

## Context

Within the three-layer model (Infrastructure / Environment / Application) defined in `environment-separation.md`, the Application Layer namespaces have used the env-first pattern `app-prod-<name>` / `app-dev-<name>`.

In parallel with the Phase 1 auth design (ADR-005), we need a structure where AuthorizationPolicy / NetworkPolicy / Argo CD project scope can be sliced along a team axis. With env-first naming as-is:

1. The namespace name itself only carries env and app; the team axis cannot be expressed
2. Using namespace name patterns for machine processing (Gateway API allowedRoutes selector, etc.) is fragile because renaming has wide blast radius

### Verifying the Existing Implementation

Existing Gateways such as `infrastructure/network/istio/resources/gateway-platform.yaml` already use **label-based selectors** to control namespaces:

```yaml
allowedRoutes:
  namespaces:
    from: Selector
    selector:
      matchLabels:
        kensan-lab.platform/environment: infrastructure
```

=> Machine processing depends on labels, not namespace name patterns. **Changing the naming convention does not require touching any Gateway YAML.**

### Requirements

1. **Express the team axis**: Enable team-scoped control via AuthorizationPolicy / NetworkPolicy
2. **No changes to existing Gateway resources**: Since they use label-based selectors, naming should be operable independently
3. **Fit a single-operator homelab**: Naming should not over-emphasize teams (yet survive when PE / AD separation becomes real later)
4. **Backstage compatibility**: When Backstage Software Template owner control is taken seriously later, the team label is referenceable

### Patterns Considered

#### Pattern A: Status Quo (`app-{env}-{name}`)

Examples: `app-prod-streamlit`, `app-dev-streamlit`

**Pros:**
- No changes to existing resources
- env is obvious from the namespace name

**Cons:**
- Cannot express the team axis
- Requires separate namespaces per env; naming becomes verbose

#### Pattern B: team-first (`app-{team}-{name}`)

Examples: `app-team-a-streamlit`, `app-team-b-iceberg-ui`

**Pros:**
- Team axis is obvious from the namespace name
- Team-scoped operations via AuthorizationPolicy appear straightforward

**Cons:**
- In a single-operator homelab, "team" has no real substance and the naming overhead is high
- Removing env from the namespace name raises the risk of prod/dev confusion

#### Pattern C: flat (`app-{name}`) + 3-axis labels (Adopted)

Examples: `app-streamlit`, `app-iceberg-ui`

The namespace name carries only the app name. env / team are expressed via labels.

**Pros:**
- Namespace names are simple and human-readable
- The team / env / app axes can be sliced independently via labels
- Structural changes such as "add team later" or "merge envs" do not require renaming
- ADR-005's AuthorizationPolicy can be expressed via group claim + label combinations

**Cons:**
- env is invisible in the namespace name (must reference labels)
- A coexistence period with the existing `app-prod-<name>` naming will occur

## Decision

**Adopt Pattern C (`app-{name}` flat + 3-axis labels).**

### 1. Naming Convention

```
app-{name}                  <- For applications (flat)
  e.g. app-streamlit
       app-iceberg-ui
       app-jupyterhub

platform-{component}        <- For platform components (existing convention preserved)
  e.g. platform-keycloak
       platform-vault
       platform-argocd
```

### 2. 3-axis Labels

| Label | Example values | Purpose | Existing? |
|---|---|---|---|
| `kensan-lab.platform/environment` | `production` / `development` / `infrastructure` | Gateway API allowedRoutes selector | **Existing** |
| `kensan-lab.platform/team` | `team-a` / `team-b` / `platform` | AuthorizationPolicy / NetworkPolicy / Argo CD project scope | **New** |
| `kensan-lab.platform/app` | `streamlit` / `iceberg-ui` / ... | Identification (optional) | New |

### 3. Argo CD Projects: Existing Layout in Phase 1

Existing:
- `platform-project` (Infrastructure)
- `app-project-prod` / `app-project-dev` (env axis)

Splitting Projects along the team axis (`app-project-team-a`, etc.) is reconsidered in Phase 2 or later. In Phase 1, simply append the new-naming namespaces (`app-{name}`) to the existing Projects' `destinations`.

### 4. Migration Schedule

| Target | Phase 1 migration |
|---|---|
| New apps | Created under this convention as `app-{name}` |
| Existing `app-prod-<name>` / `app-dev-<name>` | **No rush.** Rename at the next major rework. Coexistence is allowed |
| `app-prod` / `app-dev` namespaces (env-shared ns) | Keep as-is |

`app-prod` / `app-dev` are kept as env-shared namespaces (a landing zone for existing apps). Consider deletion once they become empty.

## Consequences

### Positive

- Namespace names are simple and human-readable (just remember `app-streamlit`)
- The team axis can be retrofitted (since it slices via labels alone, adding `kensan-lab.platform/team` later suffices)
- Migration is possible without touching any existing Gateway resources (label-based selectors)
- ADR-005's Gateway-level AuthorizationPolicy can be written declaratively as a combination of label + group claim

### Trade-offs

- env is invisible in the namespace name. Operations rely on label display such as `kubectl get ns -L kensan-lab.platform/environment`
- A coexistence period with the existing `app-prod-<name>` arises. Naming unification takes time
- Forgetting to set the `kensan-lab.platform/team` label on a resource breaks AuthorizationPolicy. The Backstage Software Template / new-namespace creation procedure must enforce it as a required field

## References

- ADR-005: Phase 1 Authentication via Istio Native oauth2 + Keycloak (the label is referenced from AuthorizationPolicy)
- `.claude/rules/environment-separation.md` (definition of the three-layer model)
- Design source: `kensan-workspace/projects/kensan-lab/secrets-phase1-design.md` § Namespace separation strategy
