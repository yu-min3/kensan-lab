# kensan-lab Design System

**Whetstone — kensan-lab platform design system, v1.0**

> Just as a whetstone sharpens a tool, this is the single source of truth for honing every app on the platform to the same visual language.

This document exists for two purposes: it's **the first thing an AI coding agent reads**, and it's **the record of the human design rationale behind it**.

---

## TL;DR — orders for agents

Just **5 rules** to follow, always, before writing or touching UI:

1. **Never use a color or size outside the tokens.** Only Tailwind semantic classes like `bg-brand`, `text-muted-foreground`. `#0EA5E9` or `bg-[#fff]` are forbidden.
2. **Never invent a new component variant.** Button / Badge / Card etc. variants are limited to what's listed in this document. If you think you need more, **discuss it first**.
3. **Headings use `.h-serif`, body text defaults to sans, numbers/code/IDs use `font-mono` + `.tnum`.** Never mix these.
4. **Density switches via `data-density`.** Never override an individual component's padding / height / font size on its own.
5. **Read `docs/design/components.md` before building a new component.** Check whether an existing combination already covers it.

---

## 1. Brand axes

| Axis | Value | Rationale |
|---|---|---|
| Word | **研鑽 / kensan** ("honing/refinement") | The platform's own name. A metaphor for "polishing, sharpening" |
| Symbol | Whetstone + growth curve + sparkle | `docs/design/brand/kensan-logo-*.svg`. Human and AI collaborating to hone a skill |
| Primary color | **Sky 600** `#0284C7` (light) / **Sky 400** `#38BDF8` (dark) | The existing logo's sky blue, dialed one step down in saturation |
| Paper tone | Warm off-white `#F5F2EC` | Avoids a blue-leaning white (fluorescent-lit feel); a warmer, easier-on-the-eyes temperature |
| Heading typeface | **Noto Serif JP** | Restores the weight of the word 研鑽. Used only for headings and some numerals |
| Body typeface | **Inter + Noto Sans JP** | High legibility. Body text and UI labels |
| Numeral typeface | **JetBrains Mono** | tabular-nums keeps digits aligned vertically |

**Impression we want**: intelligent / calm / with a bit of lab-coat playfulness.
**Impression we don't want**: flashy / gadget-y / "AI does everything" / a generic SaaS template.

---

## 2. Layer structure

```
┌─────────────────────────────────────────────────────────┐
│  apps/<app>/                                             │
│   └─ React + Tailwind v4 + shadcn/ui                     │
│       └─ @import "@kensan-lab/design-tokens/tokens.css"  │
└─────────────────────────────────────────────────────────┘
                          ▲
                          │ references
                          │
┌─────────────────────────────────────────────────────────┐
│  packages/design-tokens/                                  │
│   ├─ tokens.css     ← color, typography, spacing,         │
│   │                    shadows, motion, density            │
│   ├─ tokens.json    ← the same content, structured         │
│   └─ README.md                                            │
└─────────────────────────────────────────────────────────┘
                          ▲
                          │ rationale
                          │
┌─────────────────────────────────────────────────────────┐
│  docs/design/                                             │
│   ├─ index.md          ← this document                    │
│   │                       (formerly DESIGN_SYSTEM.md)      │
│   ├─ components.md     ← component spec                   │
│   ├─ patterns.md       ← screen patterns                  │
│   └─ brand/            ← logo, OGP                         │
└─────────────────────────────────────────────────────────┘

.claude/rules/design-system.md  ← auto-loaded by Claude Code for every app
```

**Bringing up a new app**: `@import` `packages/design-tokens/tokens.css`, generate components with the shadcn/ui CLI, and this design system is already wired in from day one.

---

## 3. Token usage rules

Full detail: [`packages/design-tokens/README.md`](https://github.com/yu-min3/kensan-lab/blob/main/packages/design-tokens/README.md) (the implementation's source of truth). Summary below.

### 3.1 Color — the semantic class list

| Semantic | Tailwind | Use |
|---|---|---|
| `--background` | `bg-background` | The overall page background |
| `--foreground` | `text-foreground` | Body text |
| `--card` | `bg-card` | Cards / panels |
| `--muted` | `bg-muted` | Understated backgrounds / disabled state |
| `--muted-foreground` | `text-muted-foreground` | Secondary text |
| `--border` | `border` (auto) | Hairline rules |
| `--border-strong` | `border-border-strong` | Section dividers |
| `--brand` | `bg-brand` / `text-brand` | **Emphasis only** |
| `--brand-muted` | `bg-brand-muted` | The faint band behind a selected item |
| `--accent` / `--accent-foreground` | `bg-accent` / `text-accent-foreground` | Hover, active nav state |
| `--success` / `--warning` / `--destructive` / `--info` | Same names | Status colors (dot + text recommended) |

**Only one brand-color protagonist per screen.** Making every task's checkbox blue is a violation. Reserve it for things like "today / NOW", "the AI assistant", or "the primary action" — nothing more.

### 3.2 Typography — the three-typeface rule

- **Serif** = `class="h-serif"`
  - Use: h1 / h2 / page titles / section titles / card-head h3
  - **Never mix**: don't use serif in body text or UI labels
- **Sans-serif** = the default
  - Use: body text, UI labels, buttons, dense tables
- **Monospace** = `class="font-mono"`
  - Use: numbers / timestamps / IDs / code / commands / IP addresses
  - Numbers **must always** pair with `.tnum` (`tabular-nums`)

### 3.3 Corner radius — anchored on `--radius: 0.5rem`

| | Value | Use |
|---|---|---|
| sm | 4px | badge / tag |
| md | 6px | input / small button |
| **lg** | **8px** | **default. card / button** |
| xl | 12px | dialog / popover |
| full | 999px | pill / avatar |

**Never mix `--radius` families on one screen.** If the card is 8px, its buttons should be 8px too (or the base-minus-2px value, 6px).

### 3.4 Shadow — minimal

- **Default: no shadow.** Use hairline rules for separation instead.
- `shadow-xs`: a substitute for a hairline rule (rarely)
- `shadow-sm`: a hovered card
- `shadow-md`: dropdown / popover / tooltip
- `shadow-lg`: dialog / drawer
- **If `shadow-md` or heavier is shown persistently anywhere, question the design.**

### 3.5 Motion — land quickly with `ease-out`

- `duration-fast` (120ms): hover / click feedback
- `duration-base` (200ms): dialog open / drawer / tab switch
- `duration-slow` (320ms): page transition / large state change
- Bouncy `ease-spring` easing is **only for when a deliberately casual feel is wanted**. For serious moments like an AI-approval dialog, use `ease-out`

### 3.6 Density — `data-density="comfortable" | "compact"`

- **Set exactly once, on the app root.** Never override it per-component.
- `comfortable`: productivity apps / reading-oriented UI like kensan
- `compact`: dashboards / monitoring / dense lists

To change density for just part of a screen, set `data-density="compact"` on that subtree's parent element.

---

## 4. Components — use only what's listed

Full spec: [`docs/design/components.md`](./components.md)

| Component | Variants | Sizes |
|---|---|---|
| **Button** | primary / secondary / outline / ghost / destructive / link | sm / md / lg + icon-only |
| **Badge** | brand / success / warning / destructive / muted / outline (+ dot variant) | Single |
| **Input / Textarea / Select** | default / error / disabled | Density-linked |
| **Checkbox / Switch** | Checkbox = multi-select. Switch = takes effect immediately | — |
| **Card** | head / body / foot (any combination) | — |
| **Tabs** | Underline style only | — |
| **Table** | default / `compact` | Density-linked |
| **Dialog** | Destructive actions, confirmations | — |
| **Alert** | info / success / warning / destructive | — |
| **Empty State** | Default | — |
| **Tooltip / Avatar / Skeleton** | Default | — |

**Before building something not on this list:**
1. Check whether an existing combination can express it
2. If it's genuinely needed, open a PR updating `components.md`
3. Keep the variant as consistent as possible with existing ones

---

## 5. Screen patterns — see `patterns.md`

An index of the main patterns:

- **App Shell**: side-nav + main (kensan's default) / top-bar + main (for dashboards)
- **Header**: a four-layer stack of eyebrow + h-serif title + primary action + subtext
- **Empty / Loading / Error**: always design all three states. Never stop at "no data"
- **Form**: a three-layer label → field → hint stack; errors get both color and text
- **Confirmation**: a Dialog for destructive actions; inline approval for everything else
- **Notification**: persistent → Alert, ephemeral → Toast (not yet implemented; add when needed)

---

## 6. Don't (the NG list)

| Don't | Instead |
|---|---|
| A raw hex like `bg-[#0EA5E9]` | `bg-brand` |
| `style={{ color: "#1A1814" }}` | `className="text-foreground"` |
| Tailwind's raw palette, like `bg-blue-500` | A semantic class (`bg-brand`, etc.) |
| Two or more primary buttons on one screen | Pick one; make the rest ghost / outline |
| Conveying state with color alone (red/yellow/green dots) | `<Badge dot>Synced</Badge>` — always pair color with a label |
| Lining up numbers in a proportional font | `font-mono` + `.tnum` |
| A card with a persistent `shadow-lg` | Hairline border only; `shadow-sm` on hover |
| Mixed corner radii on one screen (4px and 12px together) | Stay within the same `--radius` family |
| An empty state that just says "No tasks" | Pair it with a description + next action + button |
| Mixing icon libraries | **lucide-react**, and only lucide-react |
| Emoji in the UI | Don't. Use a lucide icon instead |

---

## 7. Accessibility baseline

- Always show a focus ring (`focus-visible` using `--ring`)
- Color contrast must meet WCAG AA (these tokens already do)
- Never convey state through color alone (color + icon or text)
- Interactive hit targets: **minimum 32×32px** (compact density), **recommended 44×44px** (comfortable)
- Every form field needs a `<label>` (or `aria-label`)

---

## 8. Versioning

- **major**: removing/renaming a semantic name, or shifting the brand hue by 30° or more
- **minor**: adding a token, tweaking a status color, adding a new component
- **patch**: a minor value adjustment

Each app pins its `@kensan-lab/design-tokens` version in `package.json`; minor-or-above bumps are always deliberate.

---

## 9. References

- [`packages/design-tokens/`](https://github.com/yu-min3/kensan-lab/tree/main/packages/design-tokens) — the implementation's source of truth
- [`docs/design/components.md`](./components.md) — the full component spec
- [`docs/design/patterns.md`](./patterns.md) — screen patterns
- [`docs/design/brand/`](./brand/) — logo, favicon, OGP
- [`.claude/rules/design-system.md`](https://github.com/yu-min3/kensan-lab/blob/main/.claude/rules/design-system.md) — the rules Claude Code follows
- [`docs/design/density-demo.html`](./density-demo.html) — a visual reference for the density toggle

---

## History (Changelog)

- **v1.0 "Whetstone"** (2026-05) — Initial version.
  - Adopted the AB-1 hybrid (an A-shaped skeleton with B-shaped skin)
  - Sky 600/400 (light/dark) + warm off-white #F5F2EC
  - Two density modes (comfortable / compact)
  - The core component spec finalized
