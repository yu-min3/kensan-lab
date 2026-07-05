# ADR-018: Placement of Backstage deploy manifests (`backstage/manifests` → `kubernetes/backstage`)

## Status

**Proposed.** Drafted as part of the refactoring master plan (Phase 5 / W12). Decision and
execution are deliberately separated: this ADR records the options and a recommendation;
the move itself (if accepted) is a follow-up PR.

## Date

2026-07-06

## Context

Every GitOps-managed component in this repository keeps its deploy definition under
`kubernetes/<category>/<component>` (Pattern A/B, see `kubernetes/README.md`) — with one
exception: **Backstage's raw manifests live at top-level `backstage/manifests/`**, next to
the application source (`backstage/app/`). The Argo CD `backstage` Application points its
source path there.

This predates the Pattern A/B convention and the `apps/kensan` precedent, which settled on
a different split for in-repo workloads:

| Workload | Source code | Deploy definition |
|---|---|---|
| kensan | `apps/kensan/` | `kubernetes/apps/app-kensan/` |
| Backstage | `backstage/app/` | `backstage/manifests/` ← the outlier |

Costs of the outlier: `kubernetes/` is not the single place to look for "what runs on the
cluster"; newcomers (and validation tooling scoped to `kubernetes/**`) miss it. Notably, the
Manifest CI introduced in #399 (yamllint / kubeconform paths) does not cover
`backstage/manifests/` precisely because of this placement.

## Decision (recommended)

Move the deploy definition to **`kubernetes/backstage/`** as a Pattern B (flat raw YAML)
component, and update the Application source path accordingly:

```
backstage/
└── app/                  # Backstage source (packages, templates, catalog) — stays
kubernetes/
└── backstage/            # ← moved from backstage/manifests/ (Pattern B, flat)
    ├── backstage-deployment.yaml
    ├── postgresql-statefulset.yaml
    ├── httproute.yaml
    └── ...
```

A dedicated top-level category (rather than nesting under `kubernetes/apps/`) reflects that
Backstage is a **platform service** (PE-owned, `platform-project`), not an application-tier
workload like kensan (`app-project`).

### Execution constraints (why this is Phase 5, not a quick rename)

- **Do NOT rename the Argo CD Application.** Application rename = old prune + new create,
  which has destroyed a destination namespace before. Only `spec.source.path` changes —
  an in-place update.
- The PostgreSQL StatefulSet in `backstage` ns holds catalog/auth state. Before the move,
  confirm its PVC carries `Prune=false` (per-resource annotation) and take a backup.
- Do the move as a **single PR that only moves files + updates the one path line**; verify
  with `helm-render`-equivalent local checks that rendered objects are byte-identical, so
  the sync is a no-op apply.
- After the move, extend Manifest CI paths if needed (they already cover `kubernetes/**`).

## Alternatives considered

- **Keep as-is, document the exception** — zero risk, but permanently weakens the
  "`kubernetes/` = everything deployed" invariant and keeps Backstage outside Manifest CI.
- **`kubernetes/apps/backstage/`** — groups it with kensan, but Backstage is PE-owned
  platform tooling under `platform-project`; putting it under `apps/` would blur the
  three-layer ownership model (ADR-006 / environment-separation).
- **Convert to `charts/app-base` consumer** — over-reach: app-base targets app-tier
  workloads (HTTPRoute+oauth2, per-app ns model). Backstage's StatefulSet+initdb layout
  doesn't fit; a forced fit would churn live resources for no operational gain.

## Consequences

- `kubernetes/` becomes the exhaustive inventory of cluster state; `backstage/` is purely
  source code (mirroring `apps/kensan`).
- Backstage manifests gain yamllint/kubeconform coverage from Manifest CI automatically.
- One-time review cost of a ~13-file move PR; no runtime change if executed as specified.
