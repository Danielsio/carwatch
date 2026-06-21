# CarWatch — Mobile-First Design Language

> The anchor for the CSS reset. If a change isn't justified by this doc, don't make it.
> Goal: an app that **looks good and shows data professionally** — clean + style — that
> hits **mobile (consumer) first** and **web (admin) second**, with **no heavy GPU work**.

## 0. Direction in one line

**Linear-grade restraint as the spine** — near-monochrome + one quiet accent, flat depth —
expressed as **iOS-style grouped cards on mobile** and **Stripe-style data tables on web/admin**.
The data is the hero; the chrome is quiet.

## 1. What we're deliberately leaving behind

This is the break from the current look (verifiable — these must be gone):

- `AuroraBackground` and all decorative blur blobs
- `backdrop-filter` / "glass" surfaces (`glass-card`)
- `spotlight` (cursor-follow glow), `glow-border` (animated conic gradient), `shine` sweep, `grain` noise
- Animated/large `box-shadow`, `box-shadow` transitions, multi-layer glows
- Ambient infinite animations (`aurora-drift`, `pulse-glow`, `shimmer` as decoration)
- Hover **lifts** (`translateY` on cards) — replaced by a cheap tint (see §6)

## 2. Performance budget (hard rules)

1. **No** `backdrop-filter`. **No** `filter: blur()` on any persistent or animated element.
2. **No** animated gradients, conic glows, noise overlays, or `box-shadow` transitions.
3. Animation may only touch **`transform` and `opacity`**, ≤ **180ms**, **ease-out**.
4. **No** continuous/infinite decorative animation by default. (A status dot pulse is the only exception.)
5. Shadows are **static and small** (two tokens only — §4). Depth comes from **1px borders + spacing**, not blur.
6. Honour `prefers-reduced-motion` and `prefers-color-scheme`. Respect `Save-Data` where trivial.
7. Long lists use `content-visibility: auto`.

If a future "wow" effect is wanted, it is **opt-in, desktop-only (`pointer: fine`), and never on a scroll/hover hot path.**

## 3. Color system (tokens — keep these semantic names; rewrite the values)

Neutral-forward. **One** accent, used sparingly (primary action, active nav, links, one key metric).
Semantic colors are **muted**, never neon. Starting values (tune in review):

### Light (default — consumer daylight use)
```
--background      #F6F7F9   page
--card            #FFFFFF   surfaces
--muted           #F1F2F4   secondary fills
--border          #E6E8EB   hairlines (1px)
--foreground      #15171A   primary text
--muted-foreground#6A6F76   secondary text
--accent          #4F46E5   the ONE accent (swap freely; used sparingly)
--success         #157F4B   muted green
--warning         #9A6700   muted amber
--danger          #C0362C   muted red
```
### Dark
```
--background      #0B0C0E
--card            #15171A
--muted           #1C1F23
--border          rgba(255,255,255,.08)
--foreground      #F4F5F6
--muted-foreground#9AA0A6
--accent          #6366F1
--success         #34D399  --warning #F0B429  --danger #F26257
```

Rule of thumb: a screen should be ~90% neutral. If you see the accent more than a few times per screen, it's overused.

## 4. Type, space, radius, shadow

- **Font:** keep **Heebo** (Hebrew-first, already self-hosted) as the sans. Tabular numerals for all data/metrics.
- **Type scale (px):** 12 · 13 · 14 · 16 · 18 · 20 · 24 · 30. Weights 400/500/600/700. Headings: tight tracking (`-0.01em`).
- **Spacing:** 4px base (4·8·12·16·20·24·32·40). Mobile sections breathe; admin tables compact.
- **Radius:** `sm 8` (inputs, table chrome) · `md 12` · `lg 16` (mobile cards) · `pill 999`.
- **Shadow (only two, static):** `sm = 0 1px 2px rgba(0,0,0,.06)` · `md = 0 2px 8px -2px rgba(0,0,0,.10)`. Default is **no shadow** — most surfaces use a 1px border instead.

## 5. Layout & responsive

- **Mobile-first**, single column. Sticky **compact** header (≤48px). Primary actions thumb-reachable.
- **Tables → stacked cards on mobile**: each row becomes a self-contained card (label→value pairs), scrolled vertically. Never shrink a grid to fit.
- **Web/admin**: real multi-column data tables, sticky header row, hairline dividers, optional density. Sidebar nav.
- **RTL-correct**: logical properties only (`ms-/me-`, `start/end`, `ps-/pe-`) — Hebrew is the primary locale.

## 6. Interaction model (this is where "feel" lives)

- **Hover (desktop / `pointer: fine` only):** background **tint** + accent **1px border** — *not* a lift. One cheap paint, feels precise, no compositor promotion.
- **Press/tap:** `transform: scale(0.98)` (compositor-cheap) for tactile feedback. Great on mobile.
- **Focus:** visible **2px accent ring** (a11y, non-negotiable).
- **Transitions:** `transform`/`opacity` only, 120–160ms ease-out. Color/tint changes ≤120ms.
- **Active nav / selected row:** accent-tinted fill + accent text; no glow.

## 7. Core component patterns

- **Metric/stat card:** big tabular number, thin uppercase-tracked label, optional tiny delta. Border, no shadow. (Drop the count-up animation, or make it ≤400ms opacity-only.)
- **Search card (consumer, mobile-primary):** logo chip · title · status dot · filter chips · footer stat. Stacked, generous, tap-to-open; press-scale feedback.
- **Data table (admin, web):** hairline rows, sticky header, row hover-tint, right-aligned tabular numbers, inline status badges. Collapses to stacked cards < `md`.
- **Buttons/inputs/badges:** flat, 1px border or solid accent for primary; muted fills for secondary; no gradients.

## 8. Migration approach (how we execute, surface by surface)

1. **Foundation reset** — rewrite `web/src/index.css` to: tokens (§3) + Tailwind base + a *small* set of cheap utilities. Delete every effect utility/keyframe (§1). Remove `<AuroraBackground/>` and decor blobs. Because components use semantic tokens, the app re-skins globally here.
2. **Strip stragglers** — grep for the deleted effect classes (`glass-card`, `spotlight`, `glow-border`, `shine`, `grain`, `aurora-*`) and remove their usages.
3. **Dashboard, mobile-first** — restyle to §5–7 at phone width first, then enhance for web.
4. **Roll out** — searches/listings (consumer) → admin tables → settings, same language.

Each step is reviewable live in the running local app. Work lands on a new branch off `main` (its own PR), separate from the telegram-redesign branch.

## 9. Definition of done (per surface)

- 60fps hover/scroll on a mid phone **with GPU accel off** (the failure mode that started this).
- No banned effect present (§1/§2). Looks good on a 360px phone first, clean on desktop admin second.
- Light + dark both correct; RTL correct; reduced-motion respected; focus rings visible.
