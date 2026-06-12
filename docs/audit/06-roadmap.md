# Modernization Roadmap — June 2026 Audit

Verdict first: **no rewrite is warranted.** The architecture is the right shape for the product's scale; the risks are concentrated and surgical. The roadmap is eight quick-win PRs, a short-term hardening list, two medium-term structural items, and a deliberately thin long-term list.

Every merge to main auto-deploys to production (`release.yml`), so Phase-9 execution is **serialized**: one PR merged at a time, deploy health gate + one full scraper cycle verified before the next merge.

## Quick wins — implemented in this engagement (8 PRs, in order)

| # | PR (conventional title) | Finding | Why this position |
|---|---|---|---|
| 1 | `test: add concurrent dedup claim coverage` | F11 | Zero risk; protects everything after it; buys coverage-gate headroom |
| 2 | `fix: bound orphaned azuretls fetch goroutines` | F1 | The only P1. Concurrency bound only — **no change to request shape** (headers/TLS/UA), which is the anti-bot survival envelope |
| 3 | `fix: track scheduler goroutines and preserve cleanup context` | F2+F3 | Small mechanical lifecycle PR |
| 4 | `fix: validate push subscription endpoints against known push services` | F7 | Isolated to api/push.go; closes stored SSRF |
| 5 | `fix: add backpressure check to enrichment backfill` | F5 | Uses existing `XLen` helper; isolated to maintenance.go |
| 6 | `feat: alert on listing persistence failures` | F10 | Observability lands **before** the behavioral change in PR 7 |
| 7 | `fix: record listing prices only after successful persist` | F4 | Highest behavioral risk; protected by PRs 1 & 6; merge in a low-activity window |
| 8 | `fix: bump x/net and x/crypto to patch vulnerabilities` | F12 | `go get golang.org/x/net@v0.55.0 golang.org/x/crypto@v0.52.0 && go mod tidy` (pin to the fixed versions, not `@latest`) |

## Short-term (1–2 weeks, filed as issues)

- Refuse non-localhost bind without a configured token verifier unless explicitly flagged (F8).
- Enrich-error classification: dead-letter permanent failures immediately (F6).
- Config hardening group: `MaxConcurrentFetches` ceiling, admin-email normalization at load (F9, F13-partial).
- `internal/storage/postgres` coverage 33% → 60% (T-3) and API authz matrix (T-4).
- OpenAPI spec for `/api/v1` (D-9).
- CI: add `govulncheck` and `-race` on scheduler/fetcher/broker (policy items from [04-testing-strategy.md](04-testing-strategy.md)).

## Medium-term (1–2 months)

- **`fetchAndMatch` decomposition** (D-1) — after coverage uplift, never before.
- Frontend test stack: MSW integration tests (T-5), Playwright smoke E2E (T-6), axe-core a11y (T-7).
- Alerting baseline (D-8): dead-man's switch on scraper cycles + counters from PR 6; admin-page surfacing.
- Yad2-blocked runbook (D-10).
- **Enrich-before-notify experiment** (UX-1): hold notifications up to a grace window for unenriched matches; measure latency cost; keep a hot-listing bypass.
- Parser unification (D-4); Web Vitals decision (D-11).

## Long-term (strategic, revisit quarterly)

- Composed-Store removal (D-2) alongside D-1.
- Load/soak test establishing the search-count ceiling (T-8).
- Backup posture: keep quarterly restore drills; alert on missed backups (D-3). HA explicitly deferred until ~10× growth.
- Digest-by-default notification policy with deal-score instant override (UX-4); contextual push opt-in prompt (UX-6).

## What we explicitly decided NOT to do

- **No microservice split, no queue replacement, no ORM adoption, no frontend framework change.** Each was considered; none pays at this scale.
- **No HA Postgres/Redis** — recovery hardening instead (D-3).
- **No rewrite of the stealth fetcher** — it is the most operationally sensitive code in the repo; it gets a concurrency bound (PR 2) and nothing else.
