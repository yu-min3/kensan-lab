---
title: Today Dashboard
date: 2026-09-10
---

# Focused dashboard

The dashboard combines three levels of useful information: the map shows the relationship between direction and evidence, the original project cards show the work being pursued, and the diary captures daily intentions and observations. The technical radar and annual view retain their original role.

The diary can be read, created, edited and appended to in place, with date navigation and a calendar. Its content stays free-form.

## Removed management UI

The separate Today, task board and full project management interfaces are retired. Their components and drag-and-drop dependencies are deleted rather than hidden behind expandable sections. There are no task triage, routine check, completed-work input, project creation or project metadata editors on the dashboard.

The original project cards are restored from the pre-integration implementation: headline, progress, next milestone, technologies and achievements. Selecting a technology still highlights the radar. Project references on the map/compass point to the corresponding card, not to a separate management interface.

Legacy `/today` and `/tasks` URLs redirect to the dashboard. `/projects?name=...` points to the corresponding summary card; `/daily?date=...` opens that diary on the dashboard. Old management query parameters do not revive removed features. Existing Markdown and backend compatibility APIs are preserved.

## Diary saves

Edits are serialized per query-client/file. Each session owns its path, draft and base mtime; an old response cannot overwrite another date. The editor observes document transactions immediately, before the Milkdown listener's 200ms debounce, so closing immediately after typing can flush the final input. Opening and closing without editing does not rewrite a file.

Date changes and ending an edit wait for pending saves; failures leave the editor and date intact. Unmounting sends pending changes, while a tab-local `sessionStorage` draft supports recovery after failed requests. Conflict recovery retains a separate copy of the input before loading the server version. Browser shutdown delivery is not guaranteed; unsaved changes trigger the normal unload warning. This is a recovery cache, not a second canonical store.

Dates use JST. A diary already open across midnight keeps its selected date; the Today button opens the new date after saving.

## Data contract

| Source | Responsibility |
|---|---|
| `goals.md` | North Star and explicitly linked focus projects |
| `projects/<name>/README.md` | Goals, milestones, tasks and routine definitions |
| `todo.md` | Existing standalone records; not exposed as a task-management screen |
| `daily/YYYY/MM/DD.md` | Free-form diary, routine records (`## 習慣`) and archived tasks |
| Project metrics files | Current measurements and evidence-based forecasts |

`GET /api/v1/today` remains a compatibility API and is not loaded by the dashboard. `PUT /api/v1/routines/state` records a routine; `POST /api/v1/tasks/done` adds a completed task to the selected project or `todo.md`. The legacy triage and one-line diary APIs remain available to existing clients. Their `@seen` and `## 日記`-based written-day statistics retain their historical meaning; the home does not use them to classify free-form prose.

## History in Markdown

| What | Where it is written | Example |
|---|---|---|
| Task completed | `@done(YYYY-MM-DD)` on the task line. Undo removes it | `- [x] Ship it @done(2026-10-10)` |
| Routine done | A line in that day's daily under `## 習慣`. Undo removes the line, and the heading when the section becomes empty | `- [x] 英語 30分 @routine(life)` |
| Triage seen | `@seen(YYYY-MM-DD)` on the task line, for every choice. `today` also adds `@today`; `skip` marks `[-]` | `- [ ] Draft @seen(2026-10-10)` |
| Task identity | `@id(hex)` added on the first state change or triage | `@id(3f2a…)` |

- Tags the app adds (`@done`, `@seen`, `@id`) are hidden from display and kept when a task is edited (text, band, due, milestone), moved between projects, or archived to daily. Only a state change such as undo removes `@done`. They are not compared as text for optimistic locking, so a retry after the app added them still matches.
- The heatmap counts `@done` dates across project READMEs (including `projects/_archive/`), `todo.md` and daily files, plus routine lines. Archiving a finished project does not erase its history. A completed task without `@done` (for example one checked by hand before this change) is a current task, not a history entry. Agents that complete tasks by hand may add `@done(date)` so they count.
- Routine history is keyed by project and routine text. Changing the schedule keeps history; changing the text starts a new habit.
- Today's triage prompt is finished when any project task carries today's `@seen`. Candidates rotate by oldest `@seen` first (untagged first).

## Calendar and evidence

- Dates use JST midnight; weeks begin Monday and months follow the calendar. This differs from reflection's 06:00 daily cutoff.
- Supported routines: `[毎日]`, comma-separated Japanese weekdays, `[週N回]` (1–7), `[月N]` (1–31), and `[N日]` (monthly scheduled day). Missing dates such as February 31 are skipped.
- Repeated checks on a date count once; cancellation removes that date's completion. Weekly and monthly streaks use their own periods, not daily streaks.
- Periods before a routine's first record are unknown. An unfinished current period is pending, not missed. Natural-language exceptions such as `どちらか` allow recording but do not claim streaks or failures.
- Forecasts require a fresh increasing metric, current and target values, at least three recent observations spanning fourteen days, and positive measured progress. Otherwise no date is shown; no historical observations are invented.

## Operational limitations

Each operation changes one file under the in-process write lock, so there is no cross-file transaction to break. State and triage retries are idempotent; changes to the task text or band still conflict. Multiple server processes and simultaneous external editors are not coordinated by the in-process lock; edits from the Mac reach the pod through Syncthing like any other Markdown edit, and a true simultaneous edit of the same file produces a normal Syncthing conflict copy.

Existing incomplete routine copies in `todo.md` are retained for explicit migration; `/reflection` stops copying routines once a `## 習慣` record exists. Completed copies are archived with a confirmed date, not silently discarded or backfilled.

## Verification

Run `npm test` and `npm run build` in `frontend`, and `npx playwright test` in `e2e`. Tests cover diary creation/edit/append, immediate editor close, delayed date changes, conflict recovery, read-only viewing, original project-card/radar/annual interactions, retired routes, no management-data requests, and desktop/mobile light/dark layout. All browser writes use isolated fixtures. Backend history contracts remain unchanged.
