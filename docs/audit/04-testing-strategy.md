# Testing Strategy — June 2026 Audit

## Current state

99 Go test files; CI enforces a 60% aggregate coverage gate with a real Postgres 17 service container plus e2e tests (`.github/workflows/ci.yml`). Frontend: 36 Vitest files, unit-level only. Local development on Windows without Docker cannot run `pgtest`-backed tests or `-race` (needs cgo) — **CI is the authority for both**.

## Coverage map (test files vs. source files, per package)

| Package | Ratio | Risk if wrong | Verdict |
|---|---|---|---|
| `internal/storage/postgres` | ~33% | **Data loss, double delivery** | **Worst risk/coverage mismatch in the repo** |
| `internal/scheduler` | ~63% | Missed/duplicate notifications | OK but `fetchAndMatch` paths under-exercised |
| `internal/fetcher` | ~55% | Scraper breaks silently | OK; parser fixtures exist |
| `internal/bot` | ~65% | Bad UX, not data loss | Healthy |
| `internal/api` | ~38% | Authz mistakes | Needs uplift with authz-focused cases |
| `internal/broker` | ~43% | Lost alerts | Reclaim/dead-letter paths covered; OK |
| `internal/notifier/*` | ~50% | Failed delivery | telegram adapter has tests; webpush send-path mocked |
| `internal/locale` | ~17% | Wrong-language text | Low risk; cheap to fix |
| `web/` | unit only | Regressions ship invisibly | No integration/E2E/a11y at all |

## The specific gaps that matter (ranked)

1. **T-1 — Dedup concurrency (F11).** `ClaimNew` atomicity (`internal/storage/postgres/dedup.go:9-22`) has sequential tests only (`postgres_test.go:569`). Required: N-goroutine concurrent claim on the same (token, chat, search) asserting exactly one winner; release-then-reclaim; distinct-search independence. *Quick-win PR 1.*
2. **T-2 — Every quick-win fix ships with its regression test** (see [06-roadmap.md](06-roadmap.md) PR table): bounded-stealth-client test with a fake slow doer; shutdown-waits test; push-endpoint validation table; backfill watermark test; persist-failure metric assertion; price record/revert/drop pipeline table.
3. **T-3 — storage/postgres uplift to ≥60%**: per-store CRUD against pgtest with emphasis on `ON CONFLICT` upserts, prune cutoffs (TIMESTAMPTZ boundaries), and pagination cursors.
4. **T-4 — API authorization matrix**: table-driven tests asserting cross-user access is denied for every resource type (listings, searches, bookmarks, notifications, push subs). Several `WrongOwner`/`ForeignSearch` cases exist; make the matrix exhaustive per route.
5. **T-5 — Frontend integration (MSW)**: mock the API at the network layer and test page-level flows — create search → see listings → bookmark; auth-expired refresh path; error/empty states.
6. **T-6 — Playwright E2E smoke** (3 journeys): signup→create search→preview; listings browse→detail→price chart; settings→push subscribe (mock service worker). Desktop + one mobile viewport.
7. **T-7 — axe-core a11y smoke** on Landing, Searches, NewSearch, Listings (RTL mode).
8. **T-8 — Load/soak (long-term)**: replay a recorded cycle at 10× search volume against staging Postgres; assert cycle time, pool saturation, memory.

## Policy changes

- Add `govulncheck ./...` to CI lint job (would have caught F12 automatically).
- Run `go test -race` in CI for `scheduler`, `fetcher`, `broker` packages (cheap; these are the concurrency hotspots).
- Keep the 60% gate but treat `storage/postgres` < 60% as the next ratchet target rather than raising the global number.
