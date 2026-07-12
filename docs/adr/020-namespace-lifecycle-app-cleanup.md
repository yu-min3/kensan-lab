# ADR-020: Namespace lifecycle app cleanup + backstage/app directory flatten

## Status

**Accepted** (2026-07-12). Decided in conversation while auditing why `kube-system` and
`kubernetes/namespaces/` felt structurally out of place compared to other components; the
review then extended to a related directory-nesting oddity in `backstage/`.

## Date

2026-07-12

## Context

### Part 1: namespace lifecycle apps

`kubernetes/namespaces/README.md` claims namespace management "splits into 2 types" but only
ever documented one (the component-colocated `namespace.yaml` pattern). The directory itself
holds nothing but that README — no manifest has ever lived there. Auditing the real
`kubernetes/argocd/applications/namespaces/` app group (5 entries: `kube-system`, `kyverno`,
`monitoring`, `reloader`, `sealed-secrets`) surfaced that they fall into three genuinely
different categories, only one of which the README described:

| Category | Why it needs a dedicated `namespaces/<name>/` app | Examples |
|---|---|---|
| **No owning component** | The namespace has zero component manifests in this repo — nothing to attach `resources/namespace.yaml` + `CreateNamespace=true` to. `kube-system` is adopted from outside GitOps (created by `kubeadm`), not created by us. | `kube-system` |
| **Shared across sibling components** | Multiple independent component dirs share one namespace, so no single owning Application makes sense (`policy/kyverno` + `policy/kyverno-policies` share `policy/namespace.yaml`; all 6 `observability/*` components share `observability/namespace.yaml`). | `kyverno`, `monitoring` |
| **Deliberate lifecycle decoupling** | Single owner exists, but the ns is intentionally split out for protection — e.g. sealed-secrets: no finalizer + `Prune=false` so deleting/recreating the controller app can never delete the ns holding the sealing key. | `sealed-secrets` |

`reloader` doesn't fit any of these three. `secrets/reloader/` has exactly one owning
component (the reloader chart itself), no sibling shares its namespace, and its
`reloader-namespace` Application has ordinary `finalizers` + `prune: true` — no protective
config distinguishing it from a normal owned resource. It was split out as a copy of the
`sealed-secrets`/`kyverno` pattern without the underlying reason applying.

Meanwhile, components with a single owner and no adoption/sharing need (`cert-manager`,
`external-secrets`) already use the clean, existing Pattern A convention: `resources/namespace.yaml`
as an extra Application source alongside the Helm chart, with `CreateNamespace=true` — no
separate app at all. `reloader` should have followed this from the start.

### Part 2: `backstage/app/` nesting

Per [ADR-018](./018-backstage-manifests-placement.md), Backstage's deploy manifests already
moved from `backstage/manifests/` to `kubernetes/backstage/`, leaving `backstage/` with a
single child: `backstage/app/`. The `app/` nesting level existed to disambiguate the source
tree from the sibling `manifests/` dir — that reason no longer applies, since `manifests/` is
gone. `apps/kensan/` (the other in-repo workload) has no such extra nesting; its source sits
directly at the top level. `backstage/app/*` is now a vestigial extra level for the same
reason ADR-018 already fixed once on the manifests side.

Auditing what depends on the current depth (`backstage/app/` = 2 levels below repo root)
found:

- **Doc-only, no functional risk**: `CLAUDE.md`, `.claude/rules/environment-separation.md`,
  `kubernetes/README.md`, `docs/adr/018-*.md`, `docs/refactoring_master_plan.md`,
  `docs/secret-management/index.md`, `docs/bootstrapping/add-worker-node-m4neo.md`,
  `docs/getting-started/installation.md` — all just mention the path in prose.
- **Mechanical, one `../` fewer**: `backstage/app/Makefile` — `-include ../.env` →
  `-include .env`; `MANIFEST := ../../kubernetes/backstage/backstage-deployment.yaml` →
  `../kubernetes/backstage/backstage-deployment.yaml`.
- **Not safely computable by inspection**: `backstage/app/app-config.yaml` has three
  `type: file` catalog `target:` entries, two of which carry this comment:

  ```yaml
  # So we need to go up 5 levels: packages/backend/dist/packages/backend -> backstage-app/
  target: ../../../../../catalog/domains/*.yaml
  ```

  These resolve relative to the **compiled backend's dist output layout**, not to the
  config file's on-disk location — Backstage's own path-resolution quirk. Removing one
  nesting level does not necessarily mean removing exactly one `../`; this can only be
  confirmed by actually starting the backend and checking that the `domains`,
  `organizations`, and `fastapi-template` catalog entries still load.

## Decision

### Part 1 — namespace lifecycle apps

1. **Delete `kubernetes/namespaces/`** (the file-manifest directory — just the README, no
   real content). Replace it with the three-category table above, added to
   `kubernetes/README.md` next to the existing Pattern A/B section.
   `kubernetes/argocd/applications/namespaces/` (the ArgoCD Application definitions) is
   **not** touched — those apps stay; only the half-documented top-level doc-only directory
   goes away.

2. **Keep `kubernetes/kube-system/` at top level, unchanged.** It's not floating — it's the
   "no owning component" category, same structural justification `backstage/` has for being
   at top level per ADR-018 (a legitimate platform-level concern, not required to nest under
   a component-shaped directory).

3. **Merge `reloader` namespace ownership into `secrets/reloader`'s own Application**,
   matching the `cert-manager`/`external-secrets` pattern — executed as **two phases, two
   PRs**, because a plain `git mv` is unsafe here:

   **Why not a plain move**: `reloader-namespace`'s source is `path: kubernetes/secrets/reloader`,
   `directory: {recurse: false, include: 'namespace.yaml'}`. Moving the file out from under
   that path (even in the same commit that adds it elsewhere) makes that app's desired
   resource set empty *immediately on merge*. With `automated: {prune: true}` and a
   `resources-finalizer.argocd.argoproj.io` finalizer, ArgoCD would prune the live `reloader`
   Namespace as soon as it reconciles — taking the Reloader controller down — regardless of
   whether `secrets/reloader`'s new source has already taken over tracking. There's no
   git-commit-ordering trick that avoids this within a single merge, since both Applications
   reconcile independently and the prune isn't gated on the other Application's state.

   **Phase 1 (this PR) — duplicate, don't move**:
   - Add `kubernetes/secrets/reloader/resources/namespace.yaml` as a **copy** (same content)
     of the existing `kubernetes/secrets/reloader/namespace.yaml`. The original stays in place,
     untouched, marked with a comment explaining it's a deliberate temporary duplicate.
   - Add a third source to `kubernetes/argocd/applications/secrets/reloader/app.yaml` pointing
     at `kubernetes/secrets/reloader/resources`, and flip `CreateNamespace=false` →
     `CreateNamespace=true`.
   - `reloader-namespace`'s source is completely untouched by this PR — it still finds its
     file at the original path, so it keeps syncing normally with zero risk of pruning.
     Both apps briefly co-own the same Namespace object (each sync flips the ArgoCD tracking
     annotation to itself) — expected, harmless flapping for the transfer window, not a
     steady state.
   - After merging, confirm in the live cluster that `secrets/reloader`'s Application is
     `Synced`/`Healthy` and applying the Namespace without errors.

   **Phase 2 (follow-up PR, after Phase 1 is confirmed live)**:
   - Delete `kubernetes/argocd/applications/namespaces/reloader/app.yaml` (and the now-empty
     dir) **and** the duplicate `kubernetes/secrets/reloader/namespace.yaml` **in the same
     commit**. With `reloader-namespace` gone, nothing reads the old path anymore, so removing
     both together is safe and leaves no dangling duplicate.

### Part 2 — backstage/app flatten

Flatten `backstage/app/*` up to `backstage/*` (source code becomes the direct content of
`backstage/`, matching the `apps/kensan/` shape):

```
backstage/
├── packages/
├── templates/
├── catalog/           # if present under app/
├── Makefile
└── ...                 # everything currently under backstage/app/
```

- `git mv backstage/app/* backstage/app/.[^.]* backstage/` (handle dotfiles separately:
  `.yarn`, `.yarnrc.yml`, `.dockerignore`, `.eslintrc.js`, `.gitignore`, `.claude/`, etc.),
  then remove the now-empty `backstage/app/`.
- Fix the mechanical one-level-shorter paths in the Makefile (`.env`, `MANIFEST`).
- Update `app-config.yaml`'s three `target:` entries **empirically**: start the backend
  locally after the move and confirm `domains`, `organizations`, and `fastapi-template`
  catalog entries still resolve, adjusting `../` counts as needed rather than computing them
  by inspection.
- Update the doc-only mentions listed above for accuracy.

**Execution split**: Part 1 and Part 2 ship as **separate PRs** sharing this one ADR, because
they need different verification methods — Part 1 is verified via ArgoCD/`kubectl` diff
against the live cluster, Part 2 is verified by actually running the Backstage backend
locally and checking catalog entities load. Bundling them would make one PR's review depend
on a verification step irrelevant to its own changes.

## Alternatives considered

- **Move `kube-system` into `namespaces/kube-system/`** (the original idea) — rejected once
  ADR-018's precedent made clear top-level placement doesn't require a component; moving it
  would just be churn with no structural gain.
- **Leave `reloader` split as-is** — zero risk, but keeps an unjustified inconsistency that
  looks like it protects something (mirroring `sealed-secrets`) when it doesn't.
- **Leave `backstage/app/` as-is** — zero risk, but keeps a nesting level whose only reason
  (disambiguating from `manifests/`) was already removed by ADR-018.

## Consequences

- `kubernetes/` top level no longer has a doc-only directory whose own description
  contradicts its contents.
- The three real reasons a namespace needs a dedicated lifecycle app are documented where
  someone will actually find them (`kubernetes/README.md`), instead of a half-written note.
- One fewer ArgoCD Application (`reloader-namespace` removed); `reloader`'s ns now follows
  the same pattern as `cert-manager`/`external-secrets`.
- `backstage/` mirrors `apps/kensan/`'s shape (source directly at the top level); one-time
  review + local-verification cost for the path changes in `Makefile` and `app-config.yaml`.
- Execution is a multi-step sequence (ownership transfer before cleanup in Part 1; move-then-
  verify in Part 2) — not single commits, for the reasons described above.
