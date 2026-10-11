---
title: Portfolio Dashboard
date: 2026-10-10
---

# Portfolio Dashboard

The home page (`/`) shows where evidence toward the North Star exists (map), where recent time went (compass), what each project aims for and which technologies it uses (projects, tech radar), and what was achieved this year (annual). The former Today screen moved to `/today` unchanged. Everything is rebuilt from Markdown on each request; no AI runs in the app.

## Data contract

| Source | Responsibility |
|---|---|
| `portfolio.md` | Evidence per cell. Headings `## <rung> × <domain>` (rungs: 技術を選ぶ / 投資を決める / 事業に効かせる; domains: インフラ / AI / 事業) `## 技術` (tech radar: `### <quadrant>` then `- <採用|試す|調べる|やめる> <name> — <what it is for>`), and `## 年報の章` (`- 2026-Q3 name`). One line per item: `- <date or label> <title> — <situation and decision> → <impact> [@public] [@project(name)]` |
| `projects/<name>/README.md` | `aims: [<rung>×<domain>]` in frontmatter marks the cell a project is filling; its first open milestone is the next step. `headline:` (what the project aims for, in plain words) and `stack: [names]` (technologies; names matching `## 技術` link to the radar) put the project in the projects list. Completed milestones (`✅ YYYY-MM-DD` or `@done`) feed the annual view; archived projects are included |
| `goals.md` | North Star and `## 配分` (`- name percent`), the compass targets |
| `reflect/inbox/YYYY-MM-DD.md` | Evidence proposals: `- [ ] <rung> × <domain> \| <evidence line> [@source(path)]`. `[x]` accepted, `[-]` rejected |
| `reflect/state.json` | Status of the nightly proposal job (written only by that job) |

## Rules

- A cell is `public` if any evidence has `@public`, `internal` if it has evidence without it, and `empty` otherwise. Status is never written by hand.
- The compass counts, per project, `@done` dates and dated `## ログ` entries in the last 28 days (four 7-day buckets). Git history is not available in the cluster. It measures volume, not value, and excludes work outside the workspace.
- Gap share is the part of that count belonging to projects whose `aims` point at non-public cells.
- The projects list shows active projects that have `headline` or `aims`, ordered by `## 配分`. Progress is the first metric in `metrics.yaml` that has a target, otherwise completed / total milestones (skipped ones excluded). The roadmap shows the last four completed milestones in file order; evidence stays on the map.
- Tech radar rings are fixed (採用 / 試す / 調べる / やめる); quadrants follow the `###` headings. A `stack` name without a radar line is shown as unregistered.
- The annual view lists this year's completed milestones and dated evidence by quarter. Evidence on the same day and project as a milestone is shown once, as the milestone.

## Writes

`POST /api/v1/portfolio/proposals/accept|reject {file, line, text}`. Accept first appends the evidence (without `@public` or `@source`) under the cell heading in `portfolio.md`, then marks the inbox line `[x]`. A retry does not append twice, because an identical line is skipped. Reject marks `[-]`. A changed or already decided line returns 409. The nightly job and other agents never write `portfolio.md` or inbox decisions; they only create new inbox files. Editing `portfolio.md` by hand is fine.
