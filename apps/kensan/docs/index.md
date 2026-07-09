# kensan

**kensan** (研鑽) is a file-based knowledge & goal manager — and the reference
application that runs on the kensan-lab platform.

## What it is

- **Markdown files are the single source of truth.** Notes, daily logs, reviews,
  books, and goals all live as `.md` files with YAML frontmatter. There is no
  database.
- **A single Go service** serves both a REST API and a bundled SPA (built with
  the Whetstone design system), shipped as one container image.
- **Read/write from two sides:** Claude Code edits the files directly, and the
  kensan web app browses and edits them through the API.

## Why it exists

kensan is the platform's dogfooding workload: a real service deployed by Argo CD,
cataloged in Backstage, secured behind the Istio gateway, and observed like any
other component. It proves the platform end-to-end, not just in manifests.

## Where things are

| Concern | Location |
|---|---|
| App source | `apps/kensan/` (backend + frontend) |
| Deploy definition | `kubernetes/apps/app-kensan/` (Argo CD Application via `charts/app-base`) |
| Namespace | `app-kensan` |
| Data | workspace PVC (Longhorn), synced to a laptop over LAN via Syncthing |

See [Architecture](architecture.md) for how the pieces fit together.
