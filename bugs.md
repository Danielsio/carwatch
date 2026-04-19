# CarWatch -- Bugs

> **15 issues** | 2 Critical | 4 High | 5 Medium | 4 Low

---

## Critical

### B-01 -- Mark-before-notify data loss

- **Where:** `scheduler.go:136` vs `scheduler.go:154`
- **Impact:** Users silently miss car listings. The system's core purpose is defeated.

`processSearch` marks tokens as seen *before* attempting notification.
If `Notify` fails for all recipients, listings are permanently lost --
never received, never retried.

**Fix:** Move `MarkSeen` to *after* successful notification. Or batch:
collect new listings, attempt notify, only mark the ones delivered
to at least one recipient.

---

### B-02 -- Engine volume unit mismatch

- **Where:** `config.example.yaml`, `DESIGN.md`, `FilterCriteria`
- **Impact:** Filters either pass everything or reject everything.

`config.example.yaml` uses cc (`1800`, `2100`). DESIGN.md uses liters
(`1.8`, `2.0`). `FilterCriteria` is `float64` with no documented unit.
Yad2 returns cc in JSON. Following DESIGN.md examples breaks filtering.

**Fix:** Standardize on cc. Rename fields to `EngineMinCC float64`.
Add validation: if `EngineMin < 100`, warn it looks like liters.

---

## High

### B-03 -- No `data/` directory creation

- **Where:** `sqlite.New("./data/dedup.db")`
- **Impact:** Crashes on first run in a clean environment.

No `os.MkdirAll` anywhere in the codebase. Fresh Docker containers
or cloned repos will fail immediately.

**Fix:** Add `os.MkdirAll(filepath.Dir(dbPath), 0750)` in `sqlite.New()`
before opening the database.

---

### B-04 -- Brotli response crashes parsing

- **Where:** `client.go:42`, `yad2.go:52-58`
- **Impact:** Silent data corruption or crash on every poll cycle.

Client sends `Accept-Encoding: gzip, deflate, br` but only handles gzip
decompression. Brotli-encoded responses are fed raw to goquery.

**Fix:** Remove `br` from Accept-Encoding, or add Brotli decompression
via `github.com/andybalholm/brotli`. Simplest: only advertise `gzip`.

---

### B-05 -- No retry or backoff implemented

- **Where:** `yad2.go:47-49`
- **Impact:** Transient errors cause missed poll cycles.

DESIGN.md describes exponential backoff on 429/403/5xx. The actual code
has zero retry logic -- a single network blip fails the entire search.

**Fix:** Add retry with exponential backoff (3 attempts, starting at 2s)
in `Fetch()`. Differentiate retriable (429, 5xx, timeout) from fatal (4xx).

---

### B-06 -- Anti-bot challenge has no backoff

- **Where:** `scheduler.go:113-115`
- **Impact:** Bot hammers Yad2 during a challenge, likely escalating to IP ban.

When `ErrChallenge` is detected, scheduler logs a warning and continues
at normal interval. DESIGN.md says to double the interval and cap at 1h.

**Fix:** Track consecutive challenge count. Multiply next delay by
2^n (capped at 1h). Reset counter on successful fetch.

---

## Medium

### B-07 -- `runCycle` swallows all errors

- **Where:** `scheduler.go:85-108`
- **Impact:** No way to detect persistent failures.

Logs search failures but always returns `nil`. The error check in `Run`
at line 76-79 is dead code for cycle errors.

**Fix:** Return an error when ALL searches in a cycle fail, or track
consecutive failure count and log/alert at thresholds.

---

### B-08 -- HasSeen/MarkSeen race condition

- **Where:** `scheduler.go:129-136`
- **Impact:** Duplicate notifications (currently unlikely, planned for Phase 2).

Two separate DB operations with no transaction. Concurrent searches with
overlapping params could both pass `HasSeen` and both send notifications.

**Fix:** Use `INSERT OR IGNORE` + `changes()` in a single operation.
If the row was inserted (changes > 0), it's new. Eliminates the
read-then-write race entirely.

---

### B-09 -- Active hours parse failure silently disables the feature

- **Where:** `scheduler.go:189-194`
- **Impact:** User thinks active hours are configured but bot polls 24/7.

`isActiveHours()` returns `true` if `parseTimeOfDay` fails.
Invalid config like `start: "8am"` silently polls around the clock.

**Fix:** Validate active hours format at config load time in `validate()`.
Fail startup on invalid format.

---

### B-10 -- No response body size limit

- **Where:** `parser.go:21`
- **Impact:** OOM crash on malformed or malicious response.

`goquery.NewDocumentFromReader(body)` reads the entire response into memory.
A malformed response could be gigabytes.

**Fix:** Wrap the reader: `io.LimitReader(body, 10*1024*1024)` (10 MB cap).
Yad2 pages are typically ~500KB.

---

### B-11 -- No per-fetch context timeout

- **Where:** `scheduler.go:111`
- **Impact:** A hung TCP connection blocks the entire scheduler.

The root context (signal cancellation only) is passed to `Fetch`.
No timeout means indefinite blocking.

**Fix:** Create a child context with 60-second timeout:
`ctx, cancel := context.WithTimeout(ctx, 60*time.Second)`.

---

## Low

### B-12 -- Prune runs every cycle

- **Where:** `scheduler.go:98-105`
- **Impact:** Unnecessary I/O and SQLite write lock contention.

`Prune` executes a `DELETE FROM` scan every 10-20 minutes
for a 30-day retention window.

**Fix:** Track last prune time; only prune once every 24 hours
(or on startup + daily).

---

### B-13 -- `make clean` destroys production data

- **Where:** `Makefile:11`
- **Impact:** Accidental `make clean` loses all state.

`rm -rf data/` deletes the dedup database and WhatsApp session.
Duplicate notifications flood, WhatsApp re-auth required.

**Fix:** Remove `rm -rf data/` from clean target, or rename to
`make purge` with a confirmation prompt. `clean` should only
remove build artifacts.

---

### B-14 -- `.PHONY` incomplete

- **Where:** `Makefile`
- **Impact:** Targets silently stop working if a file shares the name.

Only `build run test clean` are declared `.PHONY`.
`lint`, `docker-build`, `docker-run` are missing.

**Fix:** Add all targets to `.PHONY`.

---

### B-15 -- Scheduler imports concrete `yad2` package

- **Where:** `scheduler.go:10`
- **Impact:** Adding a new fetcher adapter requires modifying the scheduler.

`scheduler.go` imports `yad2` to match `ErrChallenge`.
This breaks the ports & adapters boundary.

**Fix:** Define `fetcher.ErrChallenge` sentinel in the `fetcher` package.
Each adapter wraps it. Scheduler checks against the abstract error.
