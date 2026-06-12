# Findings Register — June 2026 Audit

Every row was verified by direct code read during the audit. Candidates that failed verification are listed at the bottom — they are recorded so the same false positives aren't re-reported by future reviews.

Severity: **P1** fix now · **P2** fix this cycle · **P3** scheduled hardening.

Tracking issues (filed 2026-06-12): epics #1283–#1290; findings F1 #1292, F2/F3 #1293, F4 #1297, F5 #1295, F6 #1300, F7 #1294, F8 #1299, F9/F13 #1301, F10 #1296, F11 #1291, F12 #1298. Testing/ops/arch/UX children: #1302–#1313. See [06-roadmap.md](06-roadmap.md) for sequencing.

## Confirmed findings

### F1 — Stealth-client goroutines outlive cancelled contexts, unbounded (P1)
- **Evidence:** `internal/fetcher/yad2/client.go:66-95`. azuretls has no native context support; on `ctx.Done()` the fetch goroutine is abandoned, a counter increments, and a warning logs at ≥5 outstanding. Nothing ever refuses new work.
- **Impact:** every cancelled cycle under Yad2 slowness strands a goroutine holding a connection for up to the 30 s session timeout (`client.go:42`). Under sustained challenge/timeout conditions the scraper accumulates stranded connections at exactly the moment Yad2 is already unhappy with it.
- **Fix:** semaphore-bound in-flight stealth fetches; refuse (fail fast) when the bound is hit instead of warning. Do **not** touch header order, TLS fingerprint, or UA rotation.
- **Issue:** #1292 · quick-win PR 2

### F2 — Market-cache refresh goroutine untracked; races store close at shutdown (P2)
- **Evidence:** `internal/scheduler/scheduler.go:245-258` (anonymous goroutine, no WaitGroup); `cmd/scraper/main.go:95,176` — `sched.Run(ctx)` returns on cancel and the deferred `store.Close()` runs while `refreshMarketView` may still be mid-query.
- **Impact:** shutdown-time races against a closing pgx pool; noisy errors at best, undefined pool behavior at worst. Hits on every deploy (deploys are automatic on merge).
- **Fix:** track with `sync.WaitGroup` (or errgroup); `Run` waits before returning.
- **Issue:** #1293 · quick-win PR 3

### F3 — Cleanup paths discard request context entirely (P3)
- **Evidence:** `internal/scheduler/processing.go:79,106` and `processing.go:142` use `context.WithTimeout(context.Background(), 5*time.Second)`.
- **Impact:** intentional survive-cancellation pattern, but `context.Background()` also drops trace/span and log context. `context.WithoutCancel(ctx)` (Go 1.21+) is the precise tool.
- **Fix:** mechanical swap to `context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)`.
- **Issue:** #1293 · quick-win PR 3

### F4 — Price recorded before persist; failed revert ⇒ spurious price-drop alerts (P2)
- **Evidence:** `internal/scheduler/scheduler.go:829-851` — `RecordPrice` writes during match accumulation, before `persistListings`. On persist failure `revertPriceRecords` (`processing.go:138-153`) compensates, but a failed revert only logs; the stale price row then makes the next cycle's comparison see a phantom drop.
- **Impact:** users receive false "price dropped" notifications — a direct trust hit for the product's core promise. Window requires persist failure + revert failure (correlated: both are DB writes), so probability is low but the failure is user-visible and confusing.
- **Fix:** stage price observations in memory during matching; write them in the same transaction as (or strictly after) successful persist. Compare-and-detect can still use the read at match time.
- **Issue:** #1297 · quick-win PR 7 (last in sequence — highest behavioral risk)

### F5 — Enrichment backfill ignores stream depth ⇒ silent drops under backlog (P2)
- **Evidence:** `internal/scheduler/maintenance.go:62-76` publishes 30 backfill requests per cycle unconditionally. The enrich stream is XAdd-trimmed at `MaxLen: 50000` (`internal/broker/enrich.go:52`), so when the consumer lags, **old entries are evicted silently** — the backlog appears to drain while work is lost. An `XLen` helper already exists (`enrich.go:63`) and is unused here.
- **Impact:** persistent enrichment starvation under consumer lag is invisible: listings stay without km/city/image and nothing reports why.
- **Fix:** check stream depth before backfill publish; skip + log when above a watermark (e.g. 80% of MaxLen). Use the existing `XLen` helper.
- **Issue:** #1295 · quick-win PR 5

### F6 — Enrich consumer retries all errors identically (P3, softened)
- **Evidence:** `internal/broker/enrich_consumer.go:142-152` — any enrich error leaves the message pending for retry. Softening: `reclaimPending` dead-letters after max retries with `MaxLen: 10000` DLQ (`enrich_consumer.go:18,218-220`), so poison messages are bounded, just wasteful.
- **Impact:** non-retriable failures (parse error, listing deleted) burn retry cycles and enricher rate-limit budget before dead-lettering.
- **Fix:** classify errors; dead-letter immediately on known-permanent failures.
- **Issue:** #1300 · short-term

### F7 — Stored SSRF via push subscription endpoints (P2)
- **Evidence:** `internal/api/push.go:36-43` validates only `https://` prefix + ≤2048 chars. The notifier then POSTs to the stored URL with no host validation (`internal/notifier/webpush/webpush.go:121-135`).
- **Impact:** any authenticated user can make the server POST VAPID-signed requests to arbitrary HTTPS hosts — internal services, cloud metadata endpoints behind HTTPS, or third parties (making CarWatch the abuse source). Payloads are encrypted so data exfil is limited; the request itself is the weapon.
- **Fix:** allowlist known push-service hosts (`fcm.googleapis.com`, `*.push.services.mozilla.com`, `*.notify.windows.com`, `*.push.apple.com`, `updates.push.services.mozilla.com`) and reject anything resolving to private/loopback ranges as defense-in-depth.
- **Issue:** #1294 · quick-win PR 4

### F8 — Dev-auth mode permitted on non-localhost bind with only a warning (P2)
- **Evidence:** `internal/api/api.go:135-140` — when Firebase auth is unconfigured and the bind address is non-local, the server logs a warning and starts anyway, accepting dev-auth identities.
- **Impact:** a config mistake (missing `FIREBASE_CREDS` in prod) silently downgrades the API to unauthenticated-equivalent on a public bind. Note: this finding replaced a falsified CSRF claim (below).
- **Fix:** refuse to start on a non-local bind without a verifier unless an explicit `--allow-insecure-dev-auth` flag is set.
- **Issue:** #1299 · short-term

### F9 — `MaxConcurrentFetches` has a floor but no ceiling (P3)
- **Evidence:** `internal/api/api.go:142-145` — `<=0` becomes 10; `1000000` is accepted as-is.
- **Impact:** operator-error DoS amplification. Config is operator-controlled, so hardening, not vulnerability.
- **Fix:** clamp to a sane ceiling (e.g. 64) at config load with a startup log.
- **Issue:** #1301 · short-term (grouped hardening)

### F10 — No metric/alert on persist failure or failed claim release (P2)
- **Evidence:** `internal/scheduler/processing.go:97-120`. Persist failure correctly releases claims for retry; a **failed release is permanent listing loss for that user** (`processing.go:110-115` says so itself: "listing may be permanently lost… manual intervention may be needed") — and both paths emit only log lines. The `health.Observer` interface exists and is already threaded through the scheduler.
- **Impact:** the system's only permanent-data-loss path is observable solely by reading logs after the fact.
- **Fix:** counters (persist failures, claim-release failures) via the existing observer/telemetry path, surfaced on `/healthz` and `/metrics`.
- **Issue:** #1296 · quick-win PR 6

### F11 — No concurrent-claim test for the dedup atomicity guarantee (P2)
- **Evidence:** `internal/storage/postgres/dedup.go:9-22` is the integrity cornerstone; `postgres_test.go:569` covers sequential claim/release only. Nothing exercises concurrent `ClaimNew` for the same (token, chat, search).
- **Impact:** the property every notification depends on ("exactly one claim wins") is assumed, not tested. Any future change to the SQL or driver behavior regresses silently.
- **Fix:** parallel-goroutine claim test against real Postgres asserting exactly one winner; covers `ReleaseClaim` re-claim too.
- **Issue:** #1291 · quick-win PR 1 (first — buys coverage headroom)

### F12 — Reachable dependency vulnerability in x/net; test-path vulns in x/crypto (P2)
- **Evidence:** `govulncheck ./...` (2026-06-12): GO-2026-5026 in `golang.org/x/net@v0.53.0`, fixed v0.55.0, reachable via `internal/scheduler/scheduler.go:164` call chain. GO-2026-5013/5017/5018/5019/5020 in `golang.org/x/crypto@v0.50.0`, fixed v0.52.0, reachable only via `internal/storage/pgtest` (testcontainers SSH — test builds only).
- **Impact:** prod-reachable advisory plus stale crypto in test tooling.
- **Fix:** `go get golang.org/x/net@v0.55.0 golang.org/x/crypto@v0.52.0 && go mod tidy`. Consider adding govulncheck to CI.
- **Issue:** #1298 · quick-win PR 8

### F13 — Grouped P3 code-quality findings (one issue)
| Item | Evidence | Note |
|---|---|---|
| Admin email check uses `EqualFold` | `internal/api/admin.go:73-76` | Defense-in-depth; normalize once at config load and compare exactly |
| `quoteIdent` string-building | `internal/storage/postgres/admin.go:68-70` | Currently safe (whitelisted table map) but a fragile pattern to copy |
| `sync.Map` clear via Range+Delete | `internal/scheduler/scheduler.go` (langCache et al.) | Atomic swap of a fresh map is simpler and race-proof |
| Dual Yad2 parsers drift risk | `internal/fetcher/yad2/parser.go` vs `item_parser.go` | Both parse `__NEXT_DATA__`; extract shared JSON-extraction core |
| `fetchAndMatch` god function | `internal/scheduler/scheduler.go:705-1141` | ~450 lines, 5 responsibilities; decompose **only after** coverage uplift |
| Inconsistent error wrapping | repo-wide | Pick one format; enforce via wrapcheck or review convention |
| No retry/backoff on user-deactivation failure | `internal/scheduler/processing.go:25-31` | Self-heals next cycle; log-noise only |

## Falsified candidates (do not re-report)

| Candidate | Why it's wrong |
|---|---|
| "CSRF missing on admin endpoints" | Auth is `Authorization: Bearer` header (`internal/api/api.go:411,442`); no cookie-based auth exists anywhere in `internal/api`, so CSRF does not apply. The *real* adjacent issue is F8. |
| "persistListings failure = silent data loss" | Claims are released for retry (`processing.go:106-116`); loss only occurs if the release also fails — that observability gap is F10. |
| "dedup.go has zero tests" | `TestPostgres_Dedup` exists (`postgres_test.go:569`); the gap is specifically concurrency (F11). |
| "Enrich stream grows unbounded in Redis" | XAdd trims at MaxLen 50000 (`broker/enrich.go:52`); the real failure mode is silent eviction (F5). |
| "gofmt violations in 3 files" | Local CRLF checkout artifact (`core.autocrlf=true`); index is LF, CI lint is clean. |
