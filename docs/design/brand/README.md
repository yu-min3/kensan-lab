# Brand assets — kensan-lab

The single source of truth for the logo, favicon, and OGP. **Never redraw the logo anywhere else.**

## Files

| File | Purpose | Size |
|---|---|---|
| `kensan-logo-mark.svg` | Mark only (for light backgrounds) | 100×100 (ViewBox) |
| `kensan-logo-mark-dark.svg` | Mark only (for dark backgrounds) | 100×100 |
| `kensan-logo-wordmark.svg` | Mark + "kensan-lab" + PLATFORM DESIGN SYSTEM | 320×100 |
| `favicon.svg` | Favicon (simplified) | 32×32 |
| `og-image.svg` | OGP / Twitter Card | 1200×630 |

## What the symbol means

```
         ✦  ← Sparkle: the moment of refinement — human and AI sharpening a skill together
        ╱
       ╱
      ╱     ← Growth curve (sky blue): honed and rising
     ╱
  ▔▔▔▔▔▔▔  ← Whetstone (warm brown): the platform's foundation
```

The brand's core metaphor: **just as a whetstone sharpens a tool, this hones every app on the platform to the same visual language.**

## Colors

| Element | Light | Dark | Token |
|---|---|---|---|
| Whetstone | `#7A6E5D` | `#A89C8A` | `--muted-foreground` family |
| Growth curve + ✦ | `#0284C7` | `#38BDF8` | `--brand` |
| Background (OG/favicon) | `#F5F2EC` | — | `--background` |
| Wordmark | `#171411` | `#F5F2EC` | `--foreground` |

## Usage

### React + Tailwind

```tsx
// Import the SVG as a component via vite-plugin-svgr etc. (path is relative or aliased from each app to brand/)
import Logo from "@/assets/kensan-logo-mark.svg?react";
<Logo className="w-6 h-6" />
```

> The only source of truth for the logo SVGs is this `docs/design/brand/` directory. Apps reference it via a build-time copy or alias — **never redraw it inside the app**.

### HTML
```html
<link rel="icon" href="/brand/favicon.svg" type="image/svg+xml" />
<meta property="og:image" content="https://platform.yu-min3.com/og-image.png" />
```

> **Serve OGP as PNG** (some social platforms don't support SVG). Export `og-image.svg` to `og-image.png` (1200×630) for delivery.

## Don't

- ❌ **Change the mark's color** arbitrarily (e.g. blue → green). Brand color only ever flows through `--brand`
- ❌ **Break the whetstone's alignment** (the growth curve must start touching the top of the whetstone)
- ❌ Add or remove corner rounding (the whetstone's `rx="2.5"` is fixed)
- ❌ Use the sparkle on its own (it's part of the mark, not a standalone element)
- ❌ Use a rasterized PNG of the logo **where a vector is needed**

## History

- v1.0 (2026-05) — Initial version, created when the Whetstone theme was finalized.
