# CarWatch -- Enhancements

> **18 proposals** | 2 P0 | 4 P1 | 5 P2 | 3 P3 | 4 P4

---

## P0 -- Must Have

### E-01 -- Atomic dedup-then-notify pattern

- **Complexity:** Low
- **Fixes:** B-01 (data loss), B-08 (race condition)

Replace separate `HasSeen`/`MarkSeen` with a single
`ClaimNew(token) bool` using `INSERT OR IGNORE` + `changes()`.
Only notify on successful claim. Move to mark-after-notify
with a `ReleaseUnclaimed` fallback on notification failure.

---

### E-02 -- Fetch retry with categorized backoff

- **Complexity:** Medium

Wrap `Fetch` with a retry loop: 3 attempts with 2s/4s/8s delays
for retriable errors (429, 5xx, timeout, DNS). Immediately fail on
4xx (except 429), parse errors. Respect `Retry-After` header on 429.

---

## P1 -- Should Have

### E-03 -- Adaptive backoff on anti-bot challenges

- **Complexity:** Low

Add a `backoffMultiplier` field to `Scheduler`. On challenge, double it
(cap at 4x). On success, halve it back to 1x. Apply multiplier to
`nextDelay()`. Prevents IP bans. Automatically recovers.

---

### E-04 -- Per-search fetch timeout

- **Complexity:** Low

Create a 60-second child context for each `processSearch` call.
Prevents a single hung connection from blocking all searches.

---

### E-05 -- Notification queue with at-least-once delivery

- **Complexity:** Medium

Store unsent notifications in SQLite (`pending_notifications` table).
On notify success, delete. On startup, retry pending. No listing is
ever lost, even across crashes and restarts.

---

### E-06 -- Health check endpoint

- **Complexity:** Low

Add a minimal HTTP server (`:8080/healthz`) returning last successful
poll time, cycle count, error count. Enables Docker healthcheck,
Kubernetes probes, or any uptime checker.

---

### E-07 -- Structured error types for fetcher

- **Complexity:** Medium

Define `ErrChallenge`, `ErrRateLimited`, `ErrServerError` in the
`fetcher` package. Each adapter maps its errors to these. Scheduler
switches on error type for retry/backoff strategy. New adapters
don't require scheduler changes.

---

## P2 -- Nice to Have

### E-08 -- Configurable log level

- **Complexity:** Low

Add `log_level: debug|info|warn|error` to config.
Enables debugging in production without code changes.

---

### E-09 -- Pagination support

- **Complexity:** Medium

Fetch page 0, then page 1 if all tokens on page 0 are new
(indicating the listing window scrolled). Stop early when
encountering seen tokens. Catches listings during high-volume periods.

---

### E-10 -- Version/build info embedded

- **Complexity:** Low

Use `ldflags` to embed version, git SHA, build time. Add `--version` flag.
Enables support debugging. Required for any release process.

---

### E-11 -- Telegram notifier

- **Complexity:** Medium

Implement `TelegramNotifier` using `go-telegram-bot-api`.
Zero ban risk, free, first-class bot support. Provides an immediately
usable notification channel without WhatsApp's issues.

---

### E-12 -- Config hot reload via SIGHUP

- **Complexity:** Medium

Watch for SIGHUP, re-read config, swap search configs in scheduler.
Add/remove searches and recipients without restarting the bot.

---

## P3 -- Future

### E-13 -- Metrics via Prometheus

- **Complexity:** Medium

Expose counters: `fetch_total`, `fetch_errors_total`, `listings_new_total`,
`notifications_sent_total`, `challenge_detected_total`.
Expose gauges: `last_successful_fetch_timestamp`.
Enables alerting on failure patterns.

---

### E-14 -- Response caching

- **Complexity:** Low

Cache parsed listings in memory with 5-minute TTL. If a retry hits
within TTL, use cached result. Reduces Yad2 request rate during retries.

---

### E-15 -- Price change detection

- **Complexity:** Medium

Track `(token, price)` history. Notify when a previously seen listing
drops in price by more than a configurable threshold. High-value feature
for car buyers -- price drops are exactly what they want to know.

---

## P4 -- Backlog

### E-16 -- Listing history/dashboard

- **Complexity:** High

Add a simple HTTP server with SQLite-backed listing history. Serve a
minimal HTML page showing all seen listings with filter/sort. Gives users
a way to review listings without scrolling WhatsApp.

---

### E-17 -- Multi-marketplace support

- **Complexity:** High

Add fetcher adapters for AutoTrader, Facebook Marketplace, or other
Israeli car sites. Each implements `Fetcher` interface. `source` field
in config selects the adapter. Completes the pluggable design.

---

### E-18 -- Proxy rotation

- **Complexity:** Medium

Support a pool of proxies with round-robin or random selection.
Mark failed proxies as unhealthy and skip temporarily.
Distributes scraping load across IPs.
