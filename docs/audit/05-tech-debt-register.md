# Technical Debt Register — June 2026 Audit

Ranked by (impact × likelihood) ÷ effort. "Carry" means the debt is acknowledged and deliberately kept.

## Architecture debt

### D-1 — `fetchAndMatch` god function — **pay down (medium-term)**
`internal/scheduler/scheduler.go:705-1141` (~450 lines): percolation, hidden-filtering, dedup, price tracking, enrich publishing, accumulation, and stats counting in one loop with cross-cutting caches (`priceCache`, `hiddenCache`, `searchCounters`). Every product change lands here; it is simultaneously the highest-churn and worst-tested-per-line code in the repo.
**Plan:** decompose into match→filter→claim→price→accumulate stages with explicit inputs/outputs, **only after** the coverage uplift (T-3) and the quick-win fixes land — refactoring it first would invalidate the fixes' review baseline. Effort: 3–5 days including characterization tests.

### D-2 — 16-sub-interface composed `Store` — **pay down opportunistically (long-term)**
`internal/storage/store.go` composes 16 interfaces; consumers receive the same `store` object through a dozen differently-named option fields (`cmd/scraper/main.go:145-166` passes `store` 12 times). The granular interfaces are fine; the *composed* god-interface forces full stubs in tests.
**Plan:** keep the small interfaces, delete the mega-interface, and let constructors take only what they use. Mechanical, wide-touch; do it alongside the D-1 refactor. Effort: 1–2 days.

### D-3 — Single Postgres, single Redis, single VM — **carry, harden recovery**
At personal scale, HA is the wrong spend. The right spend is recovery posture: backups exist (`scripts/backup-pg.sh`, systemd timer, offsite sync, restore drill) — keep drilling them quarterly and add an alert if a backup is missed. Re-evaluate only if user count grows 10×.

### D-4 — Dual Yad2 parsers — **pay down (medium-term)**
`internal/fetcher/yad2/parser.go` and `item_parser.go` both extract `__NEXT_DATA__` JSON; a Yad2 markup change must be fixed twice. Extract the shared extraction/decode core; keep page-specific mapping separate. Effort: ½ day.

## Code debt

- **D-5** Inconsistent error wrapping repo-wide — adopt one convention, enforce in review. Carry until D-1 (touching everything anyway).
- **D-6** `sync.Map` Range+Delete clears (`scheduler.go`) — replace with atomic map swap during D-1.
- **D-7** `quoteIdent` (`storage/postgres/admin.go:68-70`) — safe today via whitelist; add a comment forbidding non-whitelisted use, or inline the whitelist check into the function. Trivial.

## Security debt
Tracked as findings F7 (push SSRF — quick win), F8 (dev-auth bind — short-term), F9/F13 (hardening group). After those, the posture is solid: secrets via env interpolation, gitleaks in CI, non-root read-only containers, bearer-token auth.

## Testing debt
See [04-testing-strategy.md](04-testing-strategy.md). Headline: the storage layer guards the product's integrity invariants at ~33% coverage (T-1, T-3); the frontend has no integration/E2E/a11y layer (T-5–T-7).

## Performance debt — **mostly carry**
- Market-median cache (30 min TTL) and pricelist cache (7-day TTL) are appropriately sized for the data's volatility; no action.
- Frontend bundle: vendor split already in place (vite chunks: vendor 385 kB, recharts 299 kB, firebase 162 kB gzipped to ~124/82/33 kB). Recharts loads only on admin/detail routes — verify route-level code splitting covers it; otherwise lazy-import. Effort: hours, low priority.
- No load test has ever established the search-count ceiling (T-8, long-term).

## Operational debt
- **D-8** No alerting (finding F10 + roadmap): logs and health endpoints exist, nothing pages or pings the operator. Minimum viable: healthchecks.io-style dead-man's switch on the scraper cycle + counters from PR 6.
- **D-9** No OpenAPI spec for `/api/v1` — short-term; generate from handlers or hand-write; unblocks contract tests.
- **D-10** No "Yad2 is blocking us" runbook — the product's existential failure mode has no documented response (what the circuit breaker/backoff do automatically, what to check, when to rotate proxies). Half a day; do it medium-term alongside `docs/yad2-anti-bot-strategy.md`.
- **D-11** Web Vitals collected (`/vitals`, CLS/INP/LCP) but never looked at — either chart them on the admin page or stop collecting. Decide medium-term.
