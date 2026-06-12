# Product & UX Review — June 2026 Audit

Reviewed as a user would experience it: Telegram bot flows, web SPA flows, notification quality, and the admin surface. Hebrew-first RTL is treated as the primary experience.

## What works

- **Two front doors with linked identity.** Telegram wizard for low-friction capture, web app for power use, one-time-token account linking. The wizard's skip-buttons-with-defaults pattern keeps time-to-first-search short.
- **`TrySearchPage` (instant preview without an account) is the single best conversion asset** — it demonstrates the core value (scored, filtered listings) before asking for signup.
- **Scoring as a trust signal.** Fitness score, deal score vs. market cohort, base-price comparison, and suspicious-listing flags give users a reason to trust a notification over raw Yad2 browsing. This is the product's moat; everything that risks false signals (see finding F4 — phantom price drops) attacks it directly.
- **RTL/Hebrew is native, not bolted on**: `lang="he" dir="rtl"` at the document root, Heebo font preloaded, localized auth error mapping, backend `internal/locale/` for bot messages in both languages.
- **Dark theme default, theme persistence, Lucide icons, web vitals collection** — visual fundamentals are in place.

## Issues, ranked by user impact

### UX-1 — Notifications arrive before enrichment (high)
Alerts publish at match time; km/city/image arrive later via the enricher (`internal/scheduler/scheduler.go:865-889`). The user's first impression of a match — the push/Telegram message — is routinely missing the mileage, the city, and the photo, which are exactly the three fields an Israeli used-car buyer triages on.
**Recommendation:** hold notification for a short enrichment grace window (e.g. 30–60 s or first-enrich-completion, whichever is sooner) for listings missing km/city/image. Measure delivery-latency cost before committing (medium-term experiment in [06-roadmap.md](06-roadmap.md)). A "hot listing" override (very fresh post date) can skip the wait.

### UX-2 — False price-drop alerts are possible (high, rare)
Direct consequence of finding F4. A user who acts on a phantom "price dropped" message and finds the original price loses trust in every future alert. Fixed by quick-win PR 7.

### UX-3 — Accessibility is untested and visibly incomplete (medium)
No ARIA labeling discipline, no focus management on route change, no `<label for>` coupling in form fields, toasts without `role="alert"`, no axe/a11y CI. For an RTL Hebrew product this matters more, not less (screen-reader users in RTL hit worse defaults).
**Recommendation:** axe-core smoke in CI + a one-pass labeling sweep of form components (`web/src/components/ui/`). Tracked as testing-strategy item T-7.

### UX-4 — Digest/quiet hours vs. instant alerts (medium)
Polling stops at 22:00; anything found 21:5x can still fire late-evening pushes, and morning startup (08:00) can deliver a burst. Digest mode exists (`/settings`) but the default experience is bursty.
**Recommendation:** default new users to digest-outside-peak with instant alerts only for high-deal-score matches; make the threshold a setting.

### UX-5 — Admin observability stops at "look at it" (low, operator-facing)
The admin page (logs SSE, cycle stats, user/search tables) is solid for inspection but has no alerting hooks; the operator learns about scraper degradation by visiting the page. Tied to finding F10 and the alerting roadmap item.

### UX-6 — Web push permission timing (low)
Push subscription is offered from settings; there's no contextual prompt at the moment of highest intent (right after creating a first search). Conversion to push-enabled is likely lower than it could be.
**Recommendation:** post-search-creation inline prompt ("Get notified the minute a match appears?") with the browser permission request deferred until the user opts in.

## Non-issues (checked, fine as-is)

- Onboarding path length: Landing → signup → first search is 3 screens with sensible defaults; no action needed.
- Mobile: viewport-fit/cover, responsive Tailwind layouts; no blocking issues found in code review (E2E on mobile viewport is in the test plan).
- Trust/permissions: Firebase handles credentials; no password handling in-app; CSP headers on the SPA.
