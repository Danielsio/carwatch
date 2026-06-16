# UI Refactor: Premium Stack (shadcn/ui + Origin UI + Aceternity)

> **GitHub Issue Title:** `feat!: refactor web UI to premium stack (shadcn/ui + Origin UI + Aceternity)`
>
> **Labels:** `enhancement`, `web`, `breaking-change`
>
> **Milestone:** UI Overhaul

---

## Summary

Replace CarWatch's hand-rolled Tailwind component library with the **Premium Stack**:

| Layer | Library | Role |
|-------|---------|------|
| Foundation | **shadcn/ui** | All structural components — Sidebar, Command, Dialog, Sheet, Tabs, Form, Toast, DataTable, Skeleton, Tooltip, DropdownMenu, Select |
| Visual polish | **Origin UI** | Elevated variants — listing cards, inbox tabs, settings inputs, stat displays, filter chips |
| Wow moments | **Aceternity / Magic UI** | Selective effects (3-5 max) — spotlight on listing cards, animated counters on admin stats, landing page hero |
| Data tables | **TanStack Table** | Listing comparisons with faceted make/model/year/price filtering (via shadcn DataTable) |
| Charts | **Recharts** (keep) | Price history, sparklines — themed via shadcn Chart component |
| Animation | **Motion** (keep) | Powers Aceternity effects + page transitions |
| Styling | **Tailwind CSS 4** (keep) | Unified across all layers |

### Why

The current UI was built incrementally with custom primitives. It works but looks like a side project, not a premium product. The goal is **Linear / Vercel / Stripe quality** — a dashboard users want to screenshot.

### Why a single PR

- The UI refactor touches styling, components, and layout holistically. Splitting it creates broken intermediate states where half the app uses the old design system and half uses the new one.
- All pages share the Shell layout, theme tokens, and UI primitives. Changing the foundation affects everything downstream.
- Tests exist for components and pages — a single PR lets us update them together and ensure CI passes in one shot.
- The backend (Go) is completely untouched. This is a pure `web/` change.

**Estimated diff:** ~80-100 files in `web/src/`, 0 files outside `web/`.

---

## Background: Library Evaluation

10 independent AI agents evaluated libraries on visual quality, dashboard aesthetics, animation, dark mode, design coherence, data-dense UI, component richness, theming, showcase quality, and mobile responsiveness.

### Final Rankings (Design Quality Score /100)

| Rank | Library | Score | Design Tier | Verdict |
|------|---------|-------|-------------|---------|
| **1** | **shadcn/ui + Origin UI + Aceternity** | **88** | Premium | Highest visual ceiling. Three-layer composition is how the best React apps are built. |
| 2 | Origin UI (on shadcn/ui) | 82 | Premium | Conservative premium — polished without animation complexity. |
| 3 | Ant Design 5 | 74 | Professional | Most complete, but "enterprise Chinese tech" aesthetic is a ceiling. |
| 4 | Mantine v7 | 70 | Professional | Feature-complete, visually mediocre. CSS modules conflict with Tailwind. |
| 5 | NextUI / HeroUI | 68 | Professional (parts) | Beautiful individual components, fatally incomplete for dashboards. |

**Eliminated:** Tremor (dying), Chakra v3 (incomplete), MUI v6 (Material = generic), Park UI (tiny community), Catalyst (pre-1.0, commercial), Radix alone (too DIY).

### Key Decision Rationale

- **shadcn/ui** wins foundation because: copy-paste ownership model, 117k GitHub stars, Vercel-backed, uses same CVA/tailwind-merge/Lucide stack we already have, React 19 + Tailwind CSS 4 native, cmdk Command palette, DataTable guide with TanStack Table.
- **Origin UI** wins polish because: every component looks designer-reviewed, notification/inbox and card components are specifically CarWatch-relevant, 100% shadcn/ui compatible.
- **Aceternity/Magic UI** wins "wow" because: spotlight cards, animated counters, aurora backgrounds. Used sparingly (3-5 effects) to create visual identity without over-animating.
- **No new npm dependencies** — all three use copy-paste model into the codebase.

---

## Current State (What Exists)

### File Counts
- **32** UI primitives in `components/ui/`
- **20** top-level components (SearchCard, ListingCardBody, CommandPalette, etc.)
- **12** admin components
- **10** landing page components
- **26** page files
- **25** custom hooks (unchanged — data layer stays)
- **1** CSS file (`index.css` with @theme block, animations, utilities)
- **1** layout (`Shell.tsx` — sidebar + bottom nav)

### Current UI Primitives (to be replaced/upgraded)

| Current Component | shadcn/ui Replacement | Notes |
|-------------------|----------------------|-------|
| `Button` (CVA) | `shadcn Button` | Direct swap, same CVA pattern |
| `Badge` | `shadcn Badge` | Direct swap |
| `Input` | `shadcn Input` + Origin UI enhanced inputs | Origin has integrated labels, character counts |
| `FormField` | `shadcn Form` (react-hook-form) | Full form system with validation |
| `Select` | `shadcn Select` (Radix) | Upgrade from native `<select>` to Radix Select |
| `Skeleton` | `shadcn Skeleton` | Direct swap |
| `Toast` (custom) | `shadcn Sonner` | Replace custom toast with Sonner |
| `ConfirmDialog` (custom) | `shadcn AlertDialog` | Radix-based, better a11y |
| `Pagination` | `shadcn Pagination` | Direct swap |
| `EmptyState` | Keep custom (no shadcn equiv) | Polish styling |
| `ErrorState` | Keep custom (no shadcn equiv) | Polish styling |
| `PageHeader` | `shadcn Breadcrumb` + custom | Combine with breadcrumb navigation |
| `PageShell` | Remove (use layout patterns) | Unnecessary wrapper |
| `SectionHeader` | Keep custom | Polish styling |
| `MatchScoreBox` | Keep custom + Origin UI badge style | Unique to CarWatch |
| `Sparkline` | Keep custom | Already good, theme to match |
| `RangeSlider` | `shadcn Slider` | Radix-based dual range |
| `ChipButton` | `shadcn Toggle` or Origin UI chips | Better interaction states |
| `ConnectionBanner` | Keep custom | Unique to CarWatch |
| `AuroraBackground` | Aceternity Aurora Background | Upgrade to Aceternity version |
| `CommandPalette` (custom) | `shadcn Command` (cmdk) | Major upgrade — fuzzy search, grouping |
| `AppCommandPalette` | Refactor to use shadcn Command | Wire commands to new component |

### Current Layout → New Layout

| Current | New |
|---------|-----|
| Custom `Shell.tsx` (sidebar + bottom nav) | `shadcn Sidebar` component (collapsible, responsive, with SidebarProvider) |
| Custom mobile bottom nav | shadcn Sidebar mobile mode (Sheet-based drawer) |
| No breadcrumbs | `shadcn Breadcrumb` in content header |
| Custom theme toggle | `shadcn` theme with `next-themes` pattern |

---

## Implementation Plan

### Prerequisites (before any component work)

```bash
# 1. Initialize shadcn/ui in the Vite project
npx shadcn@latest init

# 2. Configure for:
#    - Style: New York (tighter spacing, more premium)
#    - Base color: Zinc or Slate (neutral dark)
#    - CSS variables: Yes
#    - Tailwind CSS 4: Yes (auto-detected)
#    - React 19: Yes

# 3. Add all needed components
npx shadcn@latest add sidebar command dialog alert-dialog sheet tabs \
  form input label select textarea checkbox switch slider badge button \
  card skeleton tooltip dropdown-menu popover breadcrumb separator \
  avatar collapsible scroll-area table pagination toggle toggle-group \
  sonner chart hover-card navigation-menu
```

### Phase 1: Foundation — Design Tokens & Layout Shell

**Files changed:** `index.css`, `Shell.tsx`, `App.tsx`, new `components/ui/` files

1. **Replace `index.css` @theme block** with shadcn/ui CSS variable system
   - Map current CarWatch colors to shadcn semantic tokens (--background, --foreground, --card, --primary, --muted, etc.)
   - Choose a premium dark-first palette (dark zinc/slate base, blue-ish primary accent)
   - Keep CarWatch-specific tokens: --score-great, --score-good, --score-low, --deal-good, --deal-bad, --freshness-hot, --freshness-today
   - Keep custom animations (aurora, shimmer, fade-in, slide-up) but add new shadcn animation tokens

2. **Replace Shell.tsx** with shadcn Sidebar
   - Use `SidebarProvider` + `Sidebar` + `SidebarContent` + `SidebarGroup` + `SidebarMenuItem`
   - Port navigation sections: Main, Library, System
   - Add `SidebarTrigger` for mobile (replaces custom bottom nav with Sheet-based drawer)
   - Keep notification badge count on nav items
   - Keep RTL `dir="rtl"` support
   - Add collapsible mode (icon-only collapsed state)

3. **Replace CommandPalette** with shadcn Command (cmdk)
   - Port existing command list from `AppCommandPalette.tsx`
   - Add `CommandDialog` wrapper for `⌘K` trigger
   - Group commands: Navigation, Searches, Actions, Theme
   - Add recent searches with car thumbnails in command results

4. **Replace Toast** with Sonner
   - Remove custom `Toast.tsx` and `showGlobalToast()`
   - Replace with `sonner` toast calls throughout the app
   - Configure Sonner theme to match dark-first palette

### Phase 2: UI Primitives — Replace All `components/ui/`

**Files changed:** all `components/ui/*.tsx`, all files that import them

5. **Swap primitives one by one:**
   - `Button.tsx` → shadcn Button (keep existing variant names as aliases if needed)
   - `Badge.tsx` → shadcn Badge
   - `Input.tsx` → shadcn Input (+ Origin UI enhanced variant with floating label for forms)
   - `FormField.tsx` → shadcn Form (react-hook-form integration)
   - `Select.tsx` → shadcn Select (Radix-based, searchable)
   - `Skeleton.tsx` → shadcn Skeleton
   - `Pagination.tsx` → shadcn Pagination
   - `RangeSlider.tsx` → shadcn Slider (dual range)
   - `ChipButton.tsx` → shadcn Toggle or ToggleGroup
   - `ConfirmDialog.tsx` → shadcn AlertDialog

6. **Keep & polish custom components:**
   - `EmptyState.tsx` — restyle with shadcn tokens
   - `ErrorState.tsx` — restyle with shadcn tokens
   - `MatchScoreBox.tsx` — restyle with Origin UI badge patterns
   - `Sparkline.tsx` — theme with shadcn Chart colors
   - `ConnectionBanner.tsx` — restyle with shadcn tokens
   - `AuroraBackground.tsx` — replace with Aceternity Aurora or keep custom (evaluate visual quality)

### Phase 3: Page Components — Restyle Everything

**Files changed:** all `pages/*.tsx`, all `components/*.tsx`

7. **SearchCard.tsx** — Rebuild with shadcn Card + Origin UI card patterns
   - Car manufacturer logo, model name, filter summary
   - Price sparkline inline
   - Actions (pause/resume/delete) as shadcn DropdownMenu
   - Subtle hover effect (card-hover or Origin UI hover)

8. **ListingCardBody.tsx** — Rebuild as premium listing card
   - Use shadcn Card as base
   - Origin UI card layout for image + content
   - Aceternity Spotlight effect on hover (the cursor-following glow)
   - Price badge with deal indicator (good deal = green glow)
   - Freshness badge (hot/today) with animation
   - Match score as refined badge

9. **ListingsFilterBar.tsx** — Rebuild with shadcn Popover + ToggleGroup
   - Filter chips as shadcn Toggle buttons
   - Sort dropdown as shadcn Select
   - Price/mileage range as shadcn Slider
   - Density toggle as shadcn ToggleGroup

10. **SearchFormFields.tsx** — Rebuild with shadcn Form
    - Step-by-step wizard using shadcn Tabs or custom stepper
    - Origin UI enhanced inputs with floating labels
    - Manufacturer/model as shadcn Combobox (searchable select)
    - Year/price/km ranges as shadcn Slider pairs

11. **InboxTabs.tsx** — Rebuild with shadcn Tabs
    - Origin UI tab variants with unread count badges
    - Animated tab indicator

12. **PriceHistoryChart.tsx** — Theme with shadcn Chart
    - Wrap Recharts in shadcn ChartContainer
    - Use shadcn chart color tokens
    - Add ChartTooltip for hover details

13. **Admin components** — Full rebuild
    - `StatCard.tsx` → Origin UI stat card with animated counters (Aceternity NumberTicker)
    - `OverviewTab.tsx` → Dashboard grid with shadcn Cards + Charts
    - `ListingsTab/SearchesTab/UsersTab` → shadcn DataTable (TanStack Table) with sorting, filtering, pagination
    - `ActivityChart.tsx` → shadcn Chart wrapper
    - `DetailModal.tsx` → shadcn Dialog
    - `ConfirmModal.tsx` → shadcn AlertDialog

14. **Landing page components** — Visual upgrade
    - `HeroSection.tsx` — Aceternity spotlight/aurora background, animated headline
    - `FeaturesSection.tsx` — Origin UI feature cards or Aceternity BentoGrid
    - `StatsSection.tsx` — Aceternity animated number counters
    - `LandingNav.tsx` — shadcn NavigationMenu
    - Keep others, restyle with new tokens

15. **All pages** — Update imports and styling
    - Replace all custom primitive imports with shadcn equivalents
    - Update Tailwind classes to use new design tokens
    - Add page transition animations (Motion AnimatePresence)

### Phase 4: Polish & Testing

16. **Dark mode audit** — Verify every component in dark mode (primary mode)
17. **RTL audit** — Verify Hebrew layout with new components (shadcn's logical CSS properties)
18. **Mobile audit** — Test sidebar drawer, bottom nav replacement, card layouts on small screens
19. **Update all tests** — Fix imports, update component queries (new DOM structure)
20. **Accessibility audit** — Verify keyboard navigation, screen reader, focus management
21. **Performance check** — Bundle size comparison before/after, Lighthouse scores

---

## Files Changed (Exhaustive List)

### New files (shadcn/ui components — generated by CLI)
```
web/src/components/ui/button.tsx          (replaces existing Button.tsx)
web/src/components/ui/badge.tsx           (replaces existing Badge.tsx)
web/src/components/ui/input.tsx           (replaces existing Input.tsx)
web/src/components/ui/label.tsx           (new)
web/src/components/ui/form.tsx            (replaces FormField.tsx)
web/src/components/ui/select.tsx          (replaces existing Select.tsx)
web/src/components/ui/skeleton.tsx        (replaces existing Skeleton.tsx)
web/src/components/ui/dialog.tsx          (new)
web/src/components/ui/alert-dialog.tsx    (replaces ConfirmDialog.tsx)
web/src/components/ui/sheet.tsx           (new — mobile sidebar)
web/src/components/ui/sidebar.tsx         (new — main layout)
web/src/components/ui/command.tsx         (replaces CommandPalette.tsx)
web/src/components/ui/tabs.tsx            (new)
web/src/components/ui/card.tsx            (new)
web/src/components/ui/tooltip.tsx         (new)
web/src/components/ui/dropdown-menu.tsx   (new)
web/src/components/ui/popover.tsx         (new)
web/src/components/ui/breadcrumb.tsx      (new)
web/src/components/ui/separator.tsx       (new)
web/src/components/ui/avatar.tsx          (new)
web/src/components/ui/collapsible.tsx     (new)
web/src/components/ui/scroll-area.tsx     (new)
web/src/components/ui/table.tsx           (new)
web/src/components/ui/pagination.tsx      (replaces existing)
web/src/components/ui/toggle.tsx          (replaces ChipButton.tsx)
web/src/components/ui/toggle-group.tsx    (new)
web/src/components/ui/slider.tsx          (replaces RangeSlider.tsx)
web/src/components/ui/switch.tsx          (new)
web/src/components/ui/checkbox.tsx        (new)
web/src/components/ui/textarea.tsx        (new)
web/src/components/ui/chart.tsx           (new)
web/src/components/ui/sonner.tsx          (new — replaces Toast.tsx)
web/src/components/ui/hover-card.tsx      (new)
web/src/components/ui/navigation-menu.tsx (new)
web/src/lib/utils.ts                     (update cn() if needed)
web/components.json                       (shadcn config)
```

### Modified files (existing components and pages)
```
web/src/index.css                         (complete rewrite — new design tokens)
web/src/App.tsx                           (SidebarProvider wrapper, layout changes)
web/src/main.tsx                          (Sonner Toaster setup)
web/src/components/layout/Shell.tsx       (complete rewrite — shadcn Sidebar)
web/src/components/AppCommandPalette.tsx  (rewrite to use shadcn Command)
web/src/components/CommandPalette.tsx     (remove — replaced by shadcn Command)
web/src/components/SearchCard.tsx         (restyle with shadcn Card)
web/src/components/ListingCardBody.tsx    (restyle with shadcn Card + spotlight)
web/src/components/ListingCardSkeleton.tsx(update to shadcn Skeleton)
web/src/components/CompactListingCard.tsx (restyle with shadcn Card)
web/src/components/ListingsFilterBar.tsx  (restyle with shadcn Select/Toggle/Slider)
web/src/components/SearchFormFields.tsx   (restyle with shadcn Form/Input/Select)
web/src/components/InboxTabs.tsx          (restyle with shadcn Tabs)
web/src/components/PriceHistoryChart.tsx  (wrap in shadcn Chart)
web/src/components/NextScanCountdown.tsx  (restyle with new tokens)
web/src/components/ErrorBoundary.tsx      (restyle with new tokens)
web/src/pages/SearchesPage.tsx            (update imports + styling)
web/src/pages/ListingsPage.tsx            (update imports + styling)
web/src/pages/ListingDetailPage.tsx       (update imports + styling)
web/src/pages/NewSearchPage.tsx           (update imports + styling)
web/src/pages/EditSearchPage.tsx          (update imports + styling)
web/src/pages/SettingsPage.tsx            (update imports + styling)
web/src/pages/AdminPage.tsx              (update imports + styling)
web/src/pages/HistoryPage.tsx            (update imports + styling)
web/src/pages/NotificationsPage.tsx      (not found, likely SavedPage or similar)
web/src/pages/LandingPage.tsx            (update imports + styling)
web/src/pages/AuthPage.tsx               (update imports + styling)
web/src/pages/LoginPage.tsx              (update imports + styling)
web/src/pages/SignupPage.tsx             (update imports + styling)
web/src/pages/TrySearchPage.tsx          (update imports + styling)
web/src/pages/NotFoundPage.tsx           (update imports + styling)
web/src/components/admin/OverviewTab.tsx  (restyle with shadcn Card/Chart)
web/src/components/admin/ListingsTab.tsx  (add shadcn DataTable)
web/src/components/admin/SearchesTab.tsx  (add shadcn DataTable)
web/src/components/admin/UsersTab.tsx     (add shadcn DataTable)
web/src/components/admin/CyclesTab.tsx    (restyle)
web/src/components/admin/LogsTab.tsx      (restyle)
web/src/components/admin/ActivityChart.tsx(wrap in shadcn Chart)
web/src/components/admin/StatCard.tsx     (Origin UI stat card + Aceternity counter)
web/src/components/admin/DetailModal.tsx  (replace with shadcn Dialog)
web/src/components/admin/ConfirmModal.tsx (replace with shadcn AlertDialog)
web/src/components/landing/LandingNav.tsx (shadcn NavigationMenu)
web/src/components/landing/HeroSection.tsx(Aceternity aurora/spotlight)
web/src/components/landing/FeaturesSection.tsx (restyle)
web/src/components/landing/StatsSection.tsx    (Aceternity number ticker)
web/src/components/landing/SmartScoreSection.tsx (restyle)
web/src/components/landing/HowItWorks.tsx      (restyle)
web/src/components/landing/ProblemSolution.tsx  (restyle)
web/src/components/landing/FinalCTA.tsx        (restyle)
web/src/components/landing/LandingFooter.tsx   (restyle)
```

### Deleted files (replaced by shadcn equivalents)
```
web/src/components/ui/Toast.tsx           (replaced by Sonner)
web/src/components/ui/ConfirmDialog.tsx   (replaced by AlertDialog)
web/src/components/ui/FormField.tsx       (replaced by shadcn Form)
web/src/components/ui/RangeSlider.tsx     (replaced by shadcn Slider)
web/src/components/ui/ChipButton.tsx      (replaced by shadcn Toggle)
web/src/components/CommandPalette.tsx      (replaced by shadcn Command)
```

### Test files to update
```
web/src/components/CommandPalette.test.tsx (rewrite for shadcn Command)
web/src/components/CompactListingCard.test.tsx
web/src/components/InboxTabs.test.tsx
web/src/components/ListingCardBody.test.tsx
web/src/components/ListingsFilterBar.test.tsx
web/src/components/ErrorBoundary.test.tsx
web/src/components/ProtectedRoute.test.tsx
web/src/components/ui/EmptyState.test.tsx
web/src/components/ui/ErrorState.test.tsx
web/src/components/ui/PageHeader.test.tsx
web/src/components/ui/PageShell.test.tsx
web/src/pages/ListingsPage.test.tsx
web/src/pages/SearchesPage.test.tsx
web/src/pages/SettingsPage.test.tsx
web/src/pages/TrySearchPage.test.tsx
web/src/pages/HistoryPage.test.tsx
web/src/pages/ListingDetailPage.test.tsx
web/src/pages/NewSearchPage.test.tsx
web/src/pages/NotFoundPage.test.tsx
```

### Unchanged (no modifications needed)
```
web/src/contexts/AuthContext.tsx          (data layer — untouched)
web/src/contexts/ThemeContext.tsx          (may need minor update for shadcn theme)
web/src/hooks/*                           (all 25 hooks — data layer untouched)
web/src/lib/api.ts                        (API client — untouched)
web/src/lib/firebase.ts                   (untouched)
web/src/lib/auth-token.ts                 (untouched)
web/src/lib/scoringAlgorithm.ts           (untouched)
web/src/lib/error-messages.ts             (untouched)
web/package.json                          (only add: sonner, cmdk, react-hook-form, @radix-ui/*)
```

---

## New npm Dependencies

```json
{
  "dependencies": {
    "sonner": "^1.x",
    "cmdk": "^1.x",
    "@radix-ui/react-dialog": "^1.x",
    "@radix-ui/react-select": "^2.x",
    "@radix-ui/react-tabs": "^1.x",
    "@radix-ui/react-tooltip": "^1.x",
    "@radix-ui/react-dropdown-menu": "^2.x",
    "@radix-ui/react-popover": "^1.x",
    "@radix-ui/react-alert-dialog": "^1.x",
    "@radix-ui/react-collapsible": "^1.x",
    "@radix-ui/react-scroll-area": "^1.x",
    "@radix-ui/react-toggle": "^1.x",
    "@radix-ui/react-toggle-group": "^1.x",
    "@radix-ui/react-slider": "^1.x",
    "@radix-ui/react-switch": "^1.x",
    "@radix-ui/react-checkbox": "^1.x",
    "@radix-ui/react-avatar": "^1.x",
    "@radix-ui/react-separator": "^1.x",
    "@radix-ui/react-navigation-menu": "^1.x",
    "@radix-ui/react-hover-card": "^1.x",
    "@radix-ui/react-slot": "^1.x",
    "react-hook-form": "^7.x",
    "@hookform/resolvers": "^3.x",
    "zod": "^3.x"
  }
}
```

**Note:** Radix packages are tree-shakeable and individually small (~2-8KB each). Total added bundle is estimated at ~40-60KB gzipped.

---

## Handoff Instructions for Continuation

If this work is picked up by another agent (Opus 4.8 or later):

### Context
- This issue documents a complete UI refactor of the CarWatch web frontend
- The backend (Go) is completely untouched — all changes are in `web/src/`
- The data layer (hooks, contexts, API client) is untouched — only presentation changes
- The project uses React 19, Vite, Tailwind CSS 4, TypeScript

### How to Start
1. Read this document fully
2. Read `web/src/index.css` for current design tokens
3. Read `web/src/components/layout/Shell.tsx` for current layout
4. Run `npx shadcn@latest init` in `web/` directory
5. Follow the Implementation Plan phases in order (1→2→3→4)
6. Run `make test` after each phase to verify nothing broke
7. Run `golangci-lint run ./...` is not needed (Go only) — use `cd web && npm run lint` for frontend

### Critical Constraints
- **RTL support is mandatory** — the app serves Israeli users, Hebrew is primary language
- **Dark mode is the primary mode** — design dark-first, light as secondary
- **All existing hooks return the same data shapes** — component props interfaces may change but hook return types do not
- **Firebase auth flow is unchanged** — `useAuth()` context stays as-is
- **React Router routes are unchanged** — same paths, same lazy loading
- **Tests must pass** — update test queries for new DOM structure (shadcn uses different HTML/aria patterns)
- **No new pages or routes** — this is a visual refactor, not a feature addition

### PR Checklist
- [ ] `npx shadcn@latest init` completed with New York style
- [ ] All shadcn components added via CLI
- [ ] Origin UI components copied for: cards, inputs, tabs, stat displays
- [ ] Aceternity effects added for: listing card spotlight, admin stat counters, landing hero
- [ ] `index.css` rewritten with shadcn design tokens + CarWatch custom tokens
- [ ] `Shell.tsx` rewritten with shadcn Sidebar
- [ ] `CommandPalette` replaced with shadcn Command (cmdk)
- [ ] `Toast` replaced with Sonner
- [ ] All UI primitives swapped to shadcn equivalents
- [ ] All pages updated with new imports and styling
- [ ] All admin components rebuilt with DataTable + Charts
- [ ] Landing page components upgraded
- [ ] Dark mode verified on all pages
- [ ] RTL/Hebrew layout verified on all pages
- [ ] Mobile layout verified (sidebar drawer, cards, filters)
- [ ] All tests updated and passing (`npm run test`)
- [ ] Lint passing (`npm run lint`)
- [ ] No console errors in browser
- [ ] Bundle size compared before/after
- [ ] Visual review in browser (dark + light, desktop + mobile)

---

## Design Direction

### Color Palette (Dark Mode Primary)
- **Background:** Near-black with blue undertone (like Linear/Vercel)
- **Cards:** Slightly elevated from background, subtle border
- **Primary accent:** Blue (keep existing but refine)
- **Success/Deal:** Emerald green with glow
- **Warning:** Amber
- **Destructive:** Red
- **Muted text:** Medium gray, good contrast ratio

### Typography
- **Font:** System font stack (already in use) or consider Inter for premium feel
- **Hierarchy:** Large bold numbers for prices/stats, medium for headings, small for labels

### Effects (Aceternity — use sparingly)
1. **Spotlight** on listing cards — subtle cursor-following glow on hover
2. **NumberTicker** on admin stat cards — numbers roll up on load
3. **Aurora background** on landing hero — animated mesh gradient
4. (Optional) **TextGenerateEffect** on landing headline
5. (Optional) **BackgroundBeams** on auth pages

### Component Style
- **Borders:** Subtle, 1px, slightly lighter than background
- **Radius:** Consistent 8px (shadcn `md` radius)
- **Shadows:** Minimal — use borders and background difference instead
- **Hover states:** Subtle background change + slight scale or glow
- **Focus states:** Visible ring for keyboard users (a11y requirement)
