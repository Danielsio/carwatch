# CarWatch -- Code Quality Audit

> **20 issues** across architecture, testing, security, and maintainability

---

## Architecture & Design

### CQ-01 -- Leaky abstraction: scheduler imports concrete `yad2`

- **Where:** `scheduler.go:10`

Imports `yad2` to match `ErrChallenge`. The scheduler should depend only
on the `fetcher.Fetcher` interface.

**Fix:** Define `var ErrChallenge = errors.New(...)` in `fetcher` package.
Yad2 adapter wraps it. Scheduler uses `errors.Is(err, fetcher.ErrChallenge)`.

---

### CQ-02 -- `validate()` mutates config

- **Where:** `config.go:119`

A function named `validate` sets `cfg.HTTP.UserAgents` as a side effect.
Violates the principle of least surprise.

**Fix:** Split into `validate()` (pure) and `applyDefaults()` (mutates).
Call `applyDefaults` before `validate` in `Load()`.

---

### CQ-03 -- `FilterCriteria` field names don't encode units

- **Where:** `filter.go`, `config.go`

`EngineMin float64` -- is that cc or liters? Causes bug B-02.

**Fix:** Rename to `EngineMinCC` / `EngineMaxCC`.
Validate at config load that values > 100.

---

### CQ-20 -- `source` field in config is never used

- **Where:** `go.mod`, `main.go`

`SearchConfig.Source` is validated but never read to select a fetcher.
`main.go` hardcodes `yad2.NewFetcher`.

**Fix:** Either remove the field (YAGNI) or implement a fetcher factory:
`fetcherFor(source string) (fetcher.Fetcher, error)`.

---

## Parsing & HTTP

### CQ-04 -- URL construction uses string concatenation

- **Where:** `yad2.go:71-102`

`buildURL` manually concatenates query params with `+` and `&`.
No escaping, no `net/url` usage. Works for integers but is fragile.

**Fix:** Use `url.Values{}` and `u.RawQuery = values.Encode()`.

---

### CQ-05 -- Hardcoded `baseURL` constant

- **Where:** `yad2.go`

Cannot be overridden for testing or if Yad2 changes URL structure.

**Fix:** Accept base URL as a parameter to `NewFetcher` or make it
a field on `Yad2Fetcher`. Default to production URL.

---

### CQ-06 -- Silent item skip on parse error

- **Where:** `parser.go:54`

`parseNextData` skips items that fail `itemToListing` with `continue`,
no logging. If JSON schema drifts, listings silently disappear.

**Fix:** Log a warning with the item token on parse failure.
Track skip count and surface it in cycle metrics.

---

### CQ-07 -- Full DOM parse for one script tag

- **Where:** `parser.go`

`goquery.NewDocumentFromReader` builds a complete DOM tree
just to find `<script id="__NEXT_DATA__">`. A 500KB page becomes
multi-MB DOM in memory.

**Fix:** Use `io.ReadAll` (with size limit), then regex to extract
the JSON from the script tag. Faster, lower memory, fewer deps.

---

### CQ-19 -- No connection pooling configuration

- **Where:** `client.go`

Uses cloned default transport but doesn't configure `MaxIdleConns`,
`MaxIdleConnsPerHost`, or `IdleConnTimeout`.

**Fix:** Set explicit transport values or add a comment that
defaults are intentional.

---

## Testing

### CQ-09 -- No test coverage: scheduler

- **Where:** `scheduler.go`

The scheduler is the orchestration core. It has zero tests.
All its bugs (B-01, B-06, B-07, B-11) are untested.

**Fix:** Add unit tests using interface mocks. Test: successful cycle,
all-seen listings, fetch errors, challenge detection, active hours,
notification failure after mark-seen.

---

### CQ-10 -- No test coverage: parser

- **Where:** `parser.go`

The most fragile component (depends on Yad2's internal JSON schema).
Zero tests. Schema changes won't be caught until production.

**Fix:** Save a real `__NEXT_DATA__` JSON blob as `testdata/yad2_feed.json`.
Write tests asserting field mapping. Add a "schema changed" detection test.

---

### CQ-11 -- No test coverage: wiring

- **Where:** `main.go`

The `run()` function wires all components. A type mismatch would
only be caught at runtime.

**Fix:** Add `TestRun_InvalidConfig`. Ideally, add an integration test
with mock fetcher + in-memory SQLite + mock notifier.

---

### CQ-18 -- No table-driven test pattern

- **Where:** `filter_test.go`

Each filter has a separate test function with inline setup.
Adding a new filter requires copy-paste.

**Fix:** Consolidate into `TestApply` with
`[]struct{name, criteria, listings, expected}` table.

---

## Security & Privacy

### CQ-15 -- Dockerfile: no non-root user

- **Where:** `Dockerfile`

Container runs as root. A vulnerability gives full container access.

**Fix:** Add `RUN adduser -D -u 1000 bot` and `USER bot`.
Ensure `/data` is writable by this user.

---

### CQ-16 -- No env var interpolation for secrets

- **Where:** `config.go`

DESIGN.md says to support env var interpolation.
Not implemented. Proxy credentials and phone numbers sit in plaintext YAML.

**Fix:** Add `os.ExpandEnv()` on raw YAML bytes before unmarshalling.
Config can then use `proxy: "${PROXY_URL}"`.

---

### CQ-17 -- Phone numbers logged on notification failure

- **Where:** `scheduler.go:157`

Logs `recipient` (a phone number). If logs are shipped to a
centralized system, this is a PII leak.

**Fix:** Log a masked version: `+972***XXX`.
Or log only a recipient index/alias from config.

---

## Maintainability

### CQ-08 -- WhatsApp notifier is a stub

- **Where:** `whatsapp.go`

The system's primary output channel is a no-op that prints to stdout.
The commented-out implementation has issues (text QR, no reconnection,
no message queuing).

**Fix:** Either implement whatsmeow properly, or implement Telegram first
(simpler, no ban risk, no QR flow). A shipping product needs at least
one real notifier.

---

### CQ-12 -- `Listing` embeds `RawListing` -- tight coupling

- **Where:** `model/listing.go`

Every consumer gets all `RawListing` fields promoted.
Makes refactoring `RawListing` ripple everywhere.

**Fix:** Acceptable for MVP. If `RawListing` grows, consider composition:
`Listing{Raw RawListing; SearchName string}`.

---

### CQ-13 -- `isActiveHours` called after sleep, not before

- **Where:** `scheduler.go`

The loop sleeps 10-20 min, *then* checks active hours. Could compute
a delay at 21:50 and reject the cycle at 22:05.

**Fix:** Check active hours *before* computing delay. If outside hours,
sleep until start of next active window.

---

### CQ-14 -- `test` target doesn't fail on lint

- **Where:** `Makefile`

`test` and `lint` are separate targets with no dependency.
CI could pass tests but ship lint failures.

**Fix:** Add `lint` as a prerequisite to a `ci` target: `ci: lint test`.
