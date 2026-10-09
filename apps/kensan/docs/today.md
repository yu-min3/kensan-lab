---
title: Today Dashboard
date: 2026-09-10
---

# Today Dashboard

The home page connects project goals to today's tasks and routines. Markdown remains the source of truth; observed actions are stored in a local append-only `activity.ndjson`, without a database.

## Data contract

| Source | Responsibility |
|---|---|
| `goals.md` | North Star and explicitly linked focus projects |
| `projects/<name>/README.md` | Goals, milestones, tasks and routine definitions |
| `todo.md` | Existing standalone tasks; still visible on Today |
| Project metrics files | Current measurements and evidence-based forecasts |
| `activity.ndjson` | Observed task states, daily routine states and triage actions |

`GET /api/v1/today` joins these sources. Task edits retain the existing task API; `PUT /api/v1/routines/state` records a routine check or cancellation, and `POST /api/v1/tasks/triage` accepts `today`, `later`, or `skip`. Every triage choice finishes that day's prompt. Skipped project tasks can be restored from Today; they are not deleted.

Events contain `ts`, `kind`, `id`, `project`, `text`, `date`, and `state`, with `action` on `task.triaged`. Readers also accept legacy `task.deferred`. A task receives a stable `@id(...)` on its first observed state change or triage. Editors and daily archival must preserve that ID, but must not copy it to unrelated tasks. Routine IDs derive from project, schedule and text: changing a definition starts a new identity.

Project arrays are always `[]` when empty. The lightweight project snapshot reads each README and its metrics once for summary/Today projections; related-document discovery runs only for project details. This is a read model, not a multi-file transaction. Task and project mutations share frontend query invalidation, so returning to Today does not reuse pre-edit state.

## Calendar and evidence

- Dates use JST midnight; weeks begin Monday and months follow the calendar. This differs from reflection's 06:00 daily cutoff.
- Supported routines: `[毎日]`, comma-separated Japanese weekdays, `[週N回]` (1–7), `[月N]` (1–31), and `[N日]` (monthly scheduled day). Missing dates such as February 31 are skipped.
- Repeated checks on a date count once; cancellation removes that date's completion. Weekly and monthly streaks use their own periods, not daily streaks.
- Periods before a routine's first observation are unknown. An unfinished current period is pending, not missed. Natural-language exceptions such as `どちらか` allow recording but do not claim streaks or failures.
- Forecasts require a fresh increasing metric, current and target values, at least three recent observations spanning fourteen days, and positive measured progress. Otherwise no date is shown; no historical observations are invented.

## Operational limitations

The app serializes its own writes and rejects stale locators. State retries are idempotent. It opens the activity log before changing a task; ordinary append failures attempt rollback of both files. **The two files are not crash-atomic:** a process or machine crash between writes can leave a mismatch. Preserve backups and inspect the Markdown and log after such a failure. Multiple server processes and simultaneous external editors are not coordinated by the in-process lock.

Direct Markdown edits are visible as current tasks but are not automatically converted into historical events. The heatmap measures recorded actions, not all work performed. A malformed activity log raises an error rather than silently dropping evidence.

An otherwise valid final JSON record without a trailing newline is safely delimited on append. Replaying the same triage request while its resulting state remains intact does not create a duplicate event; changes to the task text or band still conflict.

**Single writer:** only the server process (`kensan serve`, the deployed pod) appends to `activity.ndjson`. The workspace is synced both ways with Syncthing, and two devices appending to one log produce conflict copies that cannot be merged, so the `kensan task` CLI sets `Workspace.NoActivity` and changes Markdown only. Offline recording from the Mac is intentionally not supported (decided 2026-10-09). Any new tool that writes the log must go through the server API, not the file. Never run a second server against the same synced workspace.

Reflection switches away from routine copies in `todo.md` only after the workspace contains a `routine.state` event. Existing incomplete copies are retained for explicit migration; completed copies without logged evidence should be archived with a confirmed date, not silently discarded or backfilled.

This increment changes Today, not the separate Notes or project-detail redesign. Memo and whiteboard access remains available in a collapsed section. Production rollout and migration of existing routine copies are separate operations.

## Verification

Run `go test -race ./...` in `backend`, `npm test` in `frontend`, and `npx playwright test` in `e2e`. Browser tests use isolated fixture data and cover completion/cancellation persistence, triage restoration, project links, cross-screen cache updates, empty projects, above-the-fold mobile diary access, accessible daily activity values, and mobile light/dark overflow. Go regressions cover missing log newlines and all triage retries with stale-edit rejection.
