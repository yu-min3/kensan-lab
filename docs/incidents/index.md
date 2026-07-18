# Incidents

Post-incident write-ups for notable failures on the kensan-lab cluster — what broke, the timeline, the root cause, and the follow-up actions. They exist so the same failure mode is recognized faster next time.

!!! note "Why so short"
    Formal write-ups here start from 2026-03. Earlier failure modes (namespace prune, PV loss under `Retain`-less reclaim policies, ArgoCD ownership takeover, etc.) are captured as rationale inside the relevant [ADRs](../adr/index.md) rather than as standalone incidents — that backlog is being migrated over time.

## Index

| Date | Incident |
|---|---|
| 2026-03-02 | [system-infra Sync Blocked](2026-03-02-system-infra-sync-blocked.md) |
