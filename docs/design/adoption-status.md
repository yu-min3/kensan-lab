# Adoption Status — Whetstone Design System

## Scope of this commit (the design system itself, only)

| Content | Status |
|---|---|
| `packages/design-tokens/` (tokens.css / tokens.json / README) | ✅ Added. The single source of truth for color, typography, spacing, shadows, motion, and density |
| `docs/design/` (the Whetstone overview / components / patterns / brand pages) | ✅ Added. The constitution + spec + brand assets |
| The `.ds-*` utilities that consume the density tokens | ✅ Implemented in tokens.css. `data-density` actually changes rendered dimensions (proven in `density-demo.html`) |
| `.claude/rules/design-system.md` (the agent entry point) | ✅ Referenced from CLAUDE.md; auto-read before any UI work |

Verifying density behavior: [`density-demo.html`](./density-demo.html) (the same markup renders differently under comfortable / compact).

## Not done yet (follow-up — separate PR)

| Gap | Details | Priority |
|---|---|---|
| **Adoption by apps** | No app has adopted it yet. `apps/kensan/`'s frontend hasn't started. Once the next frontend exists, **building it on Whetstone from day one** is the plan (the old kensan is already removed — tag `kensan-legacy-final`) | Medium |
| **Monorepo / toolchain** | For now, tokens are shared via a **relative import** (`@import "…/packages/design-tokens/tokens.css"`). Whether to move to a pnpm or npm workspace setup and reference it as the `@kensan-lab/design-tokens` package is a separate decision (pnpm isn't adopted yet) | Medium |
| **Packaging shared components** | `@kensan-lab/ui` doesn't exist yet. The implementation source of truth is each app's own `components/ui/` (shadcn). Consider packaging once more apps exist | Low |

## Verification notes

- Rendered `density-demo.html` with the Whetstone tokens and confirmed via screenshot that toggling `data-density` between comfortable / compact actually changes rendered dimensions.
- Token values (palette, typography, density) were finalized during the design phase. This commit implements and wires them up.
