# ADR-017: Remove `apps/kensan-legacy` from the working tree (archived as a git tag)

## Status

**Accepted.**

## Date

2026-07-03

## Context

`apps/kensan-legacy` was the previous kensan application: a full-stack productivity platform
(React frontend, six Go microservices, a Python AI service built on Google ADK, and an
Apache Iceberg data lakehouse with Dagster and Polaris). It was frozen when the unified
file-based kensan (`apps/kensan`) replaced it, and PR #394 (2026-07-02) completed the
cutover: the Argo CD `kensan` Application was removed and the `kensan` namespace was torn
down after final backups. Nothing on the cluster references this code anymore.

The directory still accounted for roughly **630 tracked files / 82,000 LOC — over half the
repository** — imposing a permanent cost on every grep, review, and repository walk-through,
and diluting the repository's purpose as a platform-engineering reference.

PR #394 initially chose to keep the source in-tree as a reference archive. We revisited that
decision: the reference value is real (see below) but does not require the code to live in the
working tree.

## Decision

Delete `apps/kensan-legacy/` from the working tree. Preserve it via the annotated git tag
**`kensan-legacy-final`** (pushed to origin), which points at the last commit containing the
full source.

The legacy app remains valuable as a **working implementation example** of:

- **Apache Iceberg lakehouse** — Medallion architecture (Bronze/Silver/Gold) with Dagster
  orchestration and Polaris REST catalog (`lakehouse/`)
- **Google ADK (Agent Development Kit) agent** — a 39-tool Gemini agent with read/write
  separation, deferred write injection, and prompt self-evaluation (`kensan-ai/`)
- **Go microservices** — six services sharing auth/config/telemetry libraries, with
  Vault dynamic DB credentials and full OpenTelemetry instrumentation (`backend/`)

### How to access the archive

```bash
# Browse on GitHub
https://github.com/yu-min3/kensan-lab/tree/kensan-legacy-final/apps/kensan-legacy

# Restore locally (read-only look)
git checkout kensan-legacy-final -- apps/kensan-legacy

# Or check out the whole tree at the archive point
git worktree add ~/kensan-lab.worktrees/legacy-archive kensan-legacy-final
```

## Alternatives Considered

- **Keep in-tree (PR #394's original stance)** — zero effort, but the 53%-of-repo noise is
  permanent and grows stale; rejected.
- **Move to a separate archive repository** — preserves browsability, but adds a repo to own
  and detaches the code from the history that explains it; the tag achieves the same with
  less machinery.

## Consequences

- The repository now contains a single application lineage (`apps/kensan`).
- Historical docs (bootstrapping records, incident reports) may still mention
  `apps/kensan-legacy` paths; they describe past states and remain valid as history —
  resolve them against the `kensan-legacy-final` tag.
- Restoring any part of the legacy app is a one-line `git checkout` from the tag.
