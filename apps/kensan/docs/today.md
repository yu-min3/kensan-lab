---
title: Today Dashboard
date: 2026-09-10
---

# Today Dashboard

The home page connects project goals to today's tasks and routines. Markdown is the only source of truth, history included: there is no database and no separate event log. Every record the screen shows can be read and edited in the same files as everything else, and reaches the Mac through the same Syncthing volume.

## Data contract

| Source | Responsibility |
|---|---|
| `goals.md` | North Star and explicitly linked focus projects |
| `projects/<name>/README.md` | Goals, milestones, tasks and routine definitions |
| `todo.md` | Existing standalone tasks; still visible on Today |
| `daily/YYYY/MM/DD.md` | Diary (`## 日記`), routine records (`## 習慣`) and archived tasks |

The diary counts as written only when `## 日記` has prose of its own. Headings and the `### 完了タスク` block that reflection archives into it do not count, and a one-line entry is inserted above that block.
| Project metrics files | Current measurements and evidence-based forecasts |

`GET /api/v1/today` joins these sources. Task edits retain the existing task API; `PUT /api/v1/routines/state` records a routine check or cancellation, `POST /api/v1/tasks/triage` accepts `today`, `later`, or `skip`, and `POST /api/v1/daily/line` appends one diary line.

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

Run `go test -race ./...` in `backend`, `npm test` in `frontend`, and `npx playwright test` in `e2e`. Browser tests use isolated fixture data and cover completion/cancellation persistence, triage restoration, project links, cross-screen cache updates, empty projects, the one-line diary at the top of the mobile view, the current-state card, accessible daily activity values, and mobile light/dark overflow. Go tests cover completion/triage/routine retries, `@done` surviving archival to daily, habit-section edits that leave the diary untouched, and history read back from Markdown.
