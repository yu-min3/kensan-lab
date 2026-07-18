# Components — kensan-lab Design System

> The **complete specification** for the components every app on the platform shares.
> The implementation source of truth is each app's own `src/components/ui/` (shadcn/ui based).
> New apps generate these via the shadcn CLI and conform to the spec here. Packaging a shared library (`@kensan-lab/ui`) is a follow-up.

---

## Design principles

1. **One shape, two densities** (comfortable / compact). Set `data-density` once on the app side and everything follows automatically
2. **Minimal variants**. Priority is expressed through **placement and single-protagonist-per-screen**, not through variants
3. **Never rely on color alone**. State = dot + label; errors = color + text
4. **One icon library, always**: lucide-react

---

## 00. Density

```html
<html data-density="comfortable">   <!-- the whole app -->
<div  data-density="compact">…</div> <!-- just a subtree, made denser -->
```

| | comfortable (default) | compact |
|---|---|---|
| `--row-h` | 2.75rem | 2rem |
| `--control-h` | 2.25rem | 1.875rem |
| `--card-pad-y/x` | 1.25 / 1.5 rem | 0.75 / 1 rem |
| `--text-size` | 0.875rem | 0.8125rem |
| Use case | Productivity apps, reading-oriented UI, settings | Dashboards, monitoring, dense lists |

**Zero cost to switch**. The CSS variables carry the **components that consume them** along for the ride.

### The utility classes that make density actually take effect

Defining the tokens alone does nothing. `data-density` only changes anything visually once the `.ds-*` classes below (defined in `tokens.css`) are applied to real components.

| Class | Effect | Primary use |
|---|---|---|
| `.ds-page` | `padding: var(--page-pad)` | The outermost page container |
| `.ds-card` | `padding: var(--card-pad-y/x)` | Card / Panel |
| `.ds-row` | `height: var(--row-h)` | Table rows, list rows, menu items |
| `.ds-control` | `height: var(--control-h)` | Button / Input height |
| `.ds-section` | Vertical flex + `gap: var(--section-gap)` | Between cards |
| `.ds-stack` | Vertical flex + `gap: var(--gap-stack)` | Vertical stacking inside a card |
| `.ds-inline` | Horizontal flex + `gap: var(--gap-inline)` | Adjacent controls |
| `.ds-text` / `.ds-label` | Density-linked `font-size`/`line-height` | Body text / uppercase labels |

```html
<div data-density="compact">
  <div class="ds-card ds-stack"> … </div>   <!-- renders with compact padding/gap -->
</div>
```

Visual proof: [`density-demo.html`](./density-demo.html) (the same card, comfortable vs. compact).

---

## 01. Button

```tsx
<Button variant="primary" size="md">Add task</Button>
```

### Props
| prop | values | default |
|---|---|---|
| `variant` | `primary` `secondary` `outline` `ghost` `destructive` `link` | `secondary` |
| `size` | `sm` `md` `lg` | `md` |
| `iconOnly` | `boolean` | `false` |
| `loading` | `boolean` | `false` |
| `disabled` | `boolean` | `false` |

### Rules
- **One primary per screen.** Everything else is ghost / outline
- Always pair the final button of a destructive action with a **Cancel** (ghost)
- Icons come from lucide-react, sized 14 / 16 / 18px (sm/md/lg)
- icon-only buttons **must** carry an `aria-label`
- The link variant is for inline text flow only — never a standalone action

---

## 02. Badge

```tsx
<Badge variant="success" dot>Synced</Badge>
```

### Variants
`brand` / `success` / `warning` / `destructive` / `muted` / `outline`

### Rules
- A status column must always pair the **dot variant with a text label**
- Never mix text-only badges and dot badges **within one context**
- Keep labels to roughly 6–12 characters (if it overflows, the category design is probably too loose)

---

## 03. Input / Textarea / Select

```tsx
<div className="field">
  <label htmlFor="title">Task name</label>
  <input id="title" className="input" />
  <div className="hint">Don't include leading/trailing whitespace</div>
</div>
```

### States
default / `:focus` / `[disabled]` / `.field.error`

### Rules
- Always keep the **three-layer label → field → hint** structure
- Never omit the label (an `aria-label` can substitute). A placeholder is never a substitute for a label
- Errors communicate what's wrong **through the hint text**, not color alone
- Numeric inputs should use **mono + tabular-nums** (keeps digits aligned cleanly)

---

## 04. Checkbox / Switch

```tsx
<label className="check"><input type="checkbox" /><span className="box" />Enable notifications</label>
<label className="switch"><input type="checkbox" /><span className="track" />Auto-save</label>
```

### When to use which (**never mix them**)
| | Checkbox | Switch |
|---|---|---|
| Meaning | **Multi-select / an optional flag** | **A setting that applies immediately** |
| Example | Agree to terms / choose recipients | Auto-save / notifications on-off / dark mode |
| Persisted | On form submit | The instant it's toggled |

---

## 05. Card

```tsx
<Card>
  <CardHead title="This week's focus" sub="Week of 2026-05-12" badge={<Badge>In progress</Badge>} />
  <CardBody>…</CardBody>
  <CardFoot>…</CardFoot>
</Card>
```

### Rules
- Default is **border only, no shadow**
- A subtle `shadow-sm` on `:hover` is acceptable (clickable cards only)
- head / body / foot is the base structure; foot is optional
- h3 uses **h-serif**

---

## 06. Tabs

```tsx
<Tabs defaultValue="overview">
  <Tab value="overview">Overview</Tab>
  <Tab value="tasks">Tasks</Tab>
</Tabs>
```

### Rules
- **Underline style only**. Pill-style tabs conflict visually with the brand color and aren't used
- Active state: `font-weight: 600` + `border-bottom: 2px solid hsl(var(--brand))`
- Six or more tabs is a sign to reconsider the design (consider moving it to the sidebar, or splitting into categories)

---

## 07. Table

```tsx
<table className="table">
  <thead>…</thead>
  <tbody>
    <tr>
      <td><strong>cilium</strong></td>
      <td><Badge dot variant="success">Synced</Badge></td>
      <td className="num-cell">4/4</td>
    </tr>
  </tbody>
</table>
```

### Rules
- On dashboards, put **`data-density="compact"`** on the parent (32px row height, 13px font)
- Numbers, timestamps, and IDs get the **`.num-cell`** class (right-aligned + mono + tnum)
- Status columns use a **dot badge**
- **The whole row is never clickable**. Put a menu button (`btn-ghost btn-icon`) in the final column instead
- If horizontal scrolling shows up, reconsider the column layout. A frozen column is a last resort

---

## 08. Dialog

```tsx
<Dialog open={open} onOpenChange={setOpen}>
  <DialogHead title="Delete this?" desc="This action cannot be undone." />
  <DialogBody>…</DialogBody>
  <DialogFoot>
    <Button variant="ghost">Cancel</Button>
    <Button variant="destructive">Delete</Button>
  </DialogFoot>
</Dialog>
```

### When to use it
- **Destructive, irreversible** actions (delete, overwrite)
- Situations where the user must not proceed without reading a warning
- Multi-field input (e.g. issuing an API key)

### When not to use it
- A minor settings change → do it **inline**
- Toggling done/undone, saving, reordering → use **Toast + Undo** instead
- Actions performed repeatedly, in sequence → use a **Toast** instead

### Rules
- The final button for a dangerous action is **destructive**; cancel is **ghost**
- Never use primary here (invites misclicks)
- Phrase the heading as a question ("Delete this?"); state the concrete impact in the body

---

## 09. Alert

```tsx
<Alert variant="warning" title="A certificate is nearing expiry" desc="…" />
```

### Variants
`info` / `success` / `warning` / `destructive`

### Alert vs. Toast (mutually exclusive)
| | Alert | Toast |
|---|---|---|
| Lifetime | Stays on screen | Auto-dismisses (6–8s) |
| Use | Ongoing state notification | Notifying completion/failure of an action |
| After dismissal | Comes back | Never comes back |
| Example | Certificate expiry, an incident | Save complete, sync failed |

**Never surface the same event through both.**

---

## 10. Empty State

```tsx
<Empty
  icon={<Inbox />}
  title="No tasks yet"
  desc="Add one thing for today and start the habit of honing your craft."
  actions={[<Button variant="primary">＋ Add</Button>, <Button variant="outline">Hand it to the AI</Button>]}
/>
```

### Rules (**the three-part combo is mandatory**)
1. **A description of the state** (what's missing)
2. **The next move** (how the situation changes)
3. **A primary action** (at least 1, at most 2)

"No data" **on its own** is forbidden.

---

## 11. Tooltip · Avatar · Skeleton

### Tooltip
- For short labels (e.g. an icon button whose meaning needs a hover hint)
- If the explanation runs past ~12 words, use a **Popover** instead (not yet implemented; add when needed)

### Avatar
- Default 32px, `-sm` 24px, `-lg` 44px
- When there's no image, show **two-letter initials** (from a name or handle)
- AI/system-originated avatars get a brand-color background + a ✦ icon

### Skeleton
- **Mimic the real layout** (match row count, width, and height to the actual content)
- Don't show it if load time is under 200ms (prevents flicker)
- A spinner is the last resort (it loses positional information)

---

## Accessibility baseline

- Always show a focus ring (`focus-visible` using `--ring`)
- Never convey state through color alone (color + icon or text)
- Interactive hit targets: **minimum 32×32px** (compact), **recommended 44×44px** (comfortable)
- Every form field needs a `<label>` (or `aria-label`)

---

## Don't (the NG list)

| Don't | Instead |
|---|---|
| A raw hex like `bg-[#0EA5E9]` | `bg-brand` |
| Two or more primary buttons on one screen | Pick one; make the rest ghost / outline |
| Conveying state with color alone | Dot badge + text |
| Numbers in a proportional font | `font-mono` + `.tnum` |
| A card with a persistent `shadow-lg` | Hairline border only; `shadow-sm` on hover |
| Mixed corner radii on one screen (4px and 12px together) | Stay within the same `--radius` family |
| An empty state that's just "No data" | Description + next action + button |
| A loading state that's just a spinner | A Skeleton that mimics the real layout |
| Emoji in the UI | A lucide-react icon |
| Mixing icon libraries | lucide-react, and only lucide-react |
