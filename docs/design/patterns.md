# Patterns — kensan-lab Design System

> A layer above Components. A dictionary of common screen assemblies — "task list," "settings," "monitoring" — so no one has to design them from scratch each time.
> A visual reference for the density tokens lives at [`density-demo.html`](./density-demo.html).

---

## Pattern index

| # | Pattern | Primary use |
|---|---|---|
| 00 | App Shell | The app's root layout |
| 01 | Page Header | A screen's entry point (eyebrow / title / subtext / actions) |
| 02 | Form Layout | Input-heavy screens |
| 03 | Empty / Loading / Error | The three states of the same screen |
| 04 | Confirmation | Confirming an action |
| 05 | Toast / Notification | Ephemeral vs. persistent notifications |
| 06 | List · Detail | Screens with many items you open individually |

---

## 00. App Shell

### Variant A — Side Nav + Main (kensan's default)

```
┌──────────┬───────────────────────────┐
│  Brand   │  Page Header              │
│          │───────────────────────────│
│  Group   │                           │
│  · Item  │  Main                     │
│  · Item  │                           │
│  Group   │                           │
│  · Item  │                           │
│          │                           │
│  Avatar  │                           │
└──────────┴───────────────────────────┘
```

- `grid-template-columns: 200px 1fr` (comfortable) / `180px 1fr` (compact)
- Overlay on mobile (hamburger → drawer)
- Active state: **`bg-accent` + `text-accent-foreground`**. No left-border stripe
- Groups get a **small uppercase label** (10px / letter-spacing 0.12em)
- Count badges are right-aligned and mono. Hidden when 0
- The foot shows **your own avatar + name**

### Variant B — Topbar + Main (dashboard-style)

```
┌────────────────────────────────────────┐
│ Brand · Nav · Nav · Nav         🔍  YM │
├────────────────────────────────────────┤
│  Page Header                           │
│  [KPI] [KPI] [KPI] [KPI]               │
│  ┌──────────────────────────────────┐  │
│  │  Table (compact)                 │  │
│  └──────────────────────────────────┘  │
└────────────────────────────────────────┘
```

- Topbar is 48–56px, `position: sticky`
- Content runs **`data-density="compact"`** to pack in more information
- KPI row: a 4-column grid, restrained card padding, values in mono + tnum, one status color per value

---

## 01. Page Header (a mandatory four-layer stack)

```tsx
<header className="ph">
  <div>
    <div className="eyebrow">Note · ADR · 0042</div>
    <h2 className="t h-serif">Migrating to the Istio Gateway API</h2>
    <p className="s">This note lays out the migration cost, workarounds, and a phased migration plan.</p>
  </div>
  <div className="actions">
    <Button variant="ghost" size="sm">Duplicate</Button>
    <Button variant="outline" size="sm">Share</Button>
    <Button variant="primary" size="sm">Edit</Button>
  </div>
</header>
```

### What each layer does
| Layer | Content | Style |
|---|---|---|
| **eyebrow** | Context (breadcrumb-like) | uppercase + 11px + letter-spacing 0.18em + muted |
| **title** | The screen's title | **h-serif** + 24–30px (comfortable) / 20–24px (compact) |
| **subtext** | The screen's purpose, in one line | 12.5–13.5px + muted + max-width 540–680px |
| **actions** | The primary action group | 1 primary + ghost/outline, right-aligned |

### Rules
- Draw a **1px `hsl(var(--border))`** line below the header to visually mark the start of the section
- **Exactly one** primary action. Everything else is ghost / outline
- icon-only buttons require a tooltip

---

## 02. Form Layout

```
┌─────────────────────────────────────┐
│  Page Header                        │
├─────────────────────────────────────┤
│  ┌─────────────┐  ┌───────────────┐ │
│  │  Field full       │             │ │
│  └─────────────┘  └───────────────┘ │
│  ┌─────────────┐  ┌───────────────┐ │
│  │  Field      │  │  Field        │ │
│  └─────────────┘  └───────────────┘ │
│                                     │
│  ─────────────────────────────────  │
│  * required        [Cancel] [Save]  │
└─────────────────────────────────────┘
```

### Rules
- `grid-template-columns: 1fr 1fr` + `gap: 18px 24px`; single-column under `560px`
- Long inputs (textarea, description) use `.full` to span the full width
- **All labels are top-aligned**. Inline labels are forbidden (they break reading order)
- Errors get **both** color and text
- Footer layout: **hint on the left, actions on the right**. Primary sits at the far right, Cancel sits ghost next to it
- If a "save as draft" action is needed, place it as secondary, to the left of primary

---

## 03. The three states: Empty / Loading / Error

Only designing "when there's data" is half the job. Always design **all three variations of the same screen**.

### Shared
- Keep the screen's frame (card / table structure) **as-is** — only the contents swap out
- A different icon per state makes them instantly distinguishable

### Empty
- **A description of the state** + **the next move** + **a primary action** (at least 1, at most 2)
- "No data" **on its own** is forbidden
- Icons are muted; buttons are primary + outline

### Loading
- **A skeleton that mimics the real layout**. A spinner is the last resort
- Don't show it if it completes in under 200ms (prevents flicker)
- For a table, show 3–5 skeleton rows

### Error
- **A plain-language explanation** + **technical detail** (error code) + **a recovery action**
- Example: `503 backend unavailable` + "the kensan-backend pod may be restarting" + "Retry" / "Details"
- Always show the error code so the user can go find the logs

---

## 04. Confirmation

### Dialog (destructive actions only)

| Condition |
|---|
| A **delete / overwrite / irreversible** action |
| Multi-field input (e.g. issuing an API key) |
| Situations where the user must not proceed without reading the warning |

- The final button for a dangerous action is `destructive`; cancel is `ghost`
- **Never use primary** here (invites misclicks)

### Inline + Undo (reversible actions)

| Condition |
|---|
| Toggling done/undone, saving, reordering, changing status |
| Anything **Undo** can reverse |
| Actions done repeatedly, in sequence (e.g. bulk-checking tasks) |

- Show `[Undo]` inside the Toast for 6–8 seconds
- **"Confirm everything with a Dialog" is forbidden** — it just causes click fatigue

---

## 05. Toast / Notification

### Toast — ephemeral (auto-dismisses)
- Position: **bottom-right** (bottom-center on mobile)
- Duration: `success` / `info` = **6s**, `destructive` = **10s, or manual dismiss only**
- Stack: up to **3** stacked vertically; oldest disappears first
- Always include a **title**. Body text and links are optional

### Alert — stays on screen
- Communicates a screen-level state (certificate expiry, a connectivity failure)
- The essential trait: dismissing it, it **comes back**
- Fine to stack several in the same place (if they're piling up, question the underlying situation)

### Never do this
- Surface the same event through **both Toast and Alert**
- Show a form validation error via Toast (**put it on the field instead**)
- Substitute a Toast for a destructive confirmation (**use a Dialog**)

---

## 06. List · Detail

For screens like notes, tasks, or Applications — **many items, opened individually**.

```
┌──────────────┬───────────────────────┐
│  🔍 search   │  Page Header          │
├──────────────┤───────────────────────│
│  ▸ selected  │                       │
│    item      │  Detail content       │
│    item      │                       │
│    item      │                       │
└──────────────┴───────────────────────┘
```

### Rules
- `grid-template-columns: 280px 1fr` (comfortable) / `240px 1fr` (compact)
- Each row shows **title + meta (mono id · updated date) + a snippet**
- Selection is shown via **`bg-accent`** (no blue stripe)
- The URL should be **deep-linkable**, e.g. `/notes/:id`
- On mobile (`< 768px`), **toggle between list and detail views**, with a breadcrumb to go back
- If search/filter sits above the list, make it a **sticky header**

---

## Decision Matrix — when in doubt

| Situation | Use | Don't use |
|---|---|---|
| Confirming a destructive action | **Dialog** + destructive | Toast / inline |
| Notifying a reversible action | **Toast** + Undo (6–8s) | Dialog |
| An ongoing warning the screen is carrying | **Alert** (stays on screen) | Toast |
| A form validation error | **`.field.error`** (below the field) | Toast / Alert |
| "No data" | **Empty** (description + next move + button) | Just "No data" |
| Loading | **Skeleton** (mimics the real layout) | Just a spinner |
| Multiple primary-feeling actions on one screen | **1 primary + everything else ghost/outline** | Multiple primaries |
| A small mark conveying state | **Dot badge + label** | A color dot alone |
| Numbers, timestamps, IDs | **font-mono + tabular-nums** | The default body typeface |
| A dense list (dashboard) | **`data-density="compact"`** | Overriding padding per-component |
