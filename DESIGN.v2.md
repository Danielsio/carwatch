# CarWatch — System Design (v2)

## Overview

A single-binary bot that monitors car listings from Yad2 (Israel's largest classifieds marketplace) and delivers new listings to users via WhatsApp or Telegram in near real-time.

**Initial use case:** Mazda 3, 2.0L engine, Israel (Yad2).
**Design goal:** Configurable, extensible to other cars, users, and marketplaces.

---

## 1. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                        Scheduler                                     │
│              Ticker + Jitter + Adaptive Backoff                      │
│              Active-hours gating (timezone-aware)                    │
└──────────────┬───────────────────────────────────────────────────────┘
               │ for each SearchConfig (sequential, with per-search timeout)
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    Fetcher (Port)                                     │
│         ┌──────────────────┐  ┌────────────────────┐                 │
│         │  Yad2Fetcher     │  │  Future adapters    │                 │
│         │  (Adapter)       │  │                     │                 │
│         └──────────────────┘  └────────────────────┘                 │
│         HTTP GET → extract __NEXT_DATA__ → []RawListing              │
│         Retry with exponential backoff (3 attempts)                  │
└──────────────┬───────────────────────────────────────────────────────┘
               │ []RawListing
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│                     Filter Engine                                     │
│          Pure function, no I/O                                        │
│          Applies: engine cc, mileage, ownership, keywords             │
│          → []RawListing (matching)                                    │
└──────────────┬───────────────────────────────────────────────────────┘
               │ []RawListing
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│                 Dedup Store (Port)                                     │
│          Atomic claim: INSERT OR IGNORE + changes()                   │
│          → []Listing (new only, claimed by this cycle)                │
└──────────────┬───────────────────────────────────────────────────────┘
               │ []Listing (new)
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│              Notification Service (Port)                               │
│         ┌────────────────────┐  ┌──────────────────┐                  │
│         │ WhatsAppNotifier   │  │ TelegramNotifier  │                  │
│         │ (whatsmeow)        │  │ (go-telegram-bot) │                  │
│         └────────────────────┘  └──────────────────┘                  │
│         On failure: release claim → retry next cycle                  │
└──────────────┬────────────────────────────────────────────────────────┘
               │
               ▼
          User's Phone
```

### Key Design Principles

1. **Notify-then-mark.** A listing is only marked as "seen" after at least one recipient successfully receives it. If notification fails, the claim is released and the listing is retried next cycle.
2. **Atomic dedup.** `INSERT OR IGNORE` + `changes()` in a single operation. No read-then-write race.
3. **Adaptive backoff.** Anti-bot challenges dynamically increase poll intervals. Successful fetches decay back to normal.
4. **Fail-open on fetch, fail-safe on notify.** A fetch error skips one cycle (retry next). A notify error preserves the listing for retry. Neither causes data loss.

### Component Breakdown

| Component | Responsibility | Interface |
|---|---|---|
| **Scheduler** | Orchestrates poll cycles with jitter and adaptive backoff | Runs the pipeline on a timer |
| **Fetcher** | Retrieves raw listings from a marketplace | `fetcher.Fetcher` interface |
| **Filter Engine** | Applies user criteria to raw listings | Pure function, no state |
| **Dedup Store** | Tracks which listings have been claimed/seen | `storage.DedupStore` interface |
| **Notifier** | Delivers alerts to users | `notifier.Notifier` interface |
| **Config** | Loads, validates, and applies defaults | YAML → typed struct |

---

## 2. Detailed Component Design

### 2.1 Fetcher

**Port (interface):**
```go
package fetcher

var (
    ErrChallenge   = errors.New("anti-bot challenge detected")
    ErrRateLimited = errors.New("rate limited")
    ErrServerError = errors.New("server error")
)

type Fetcher interface {
    Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error)
}
```

Error types are defined in the `fetcher` package, not in individual adapters. Each adapter wraps these sentinels. The scheduler switches on error type to decide retry/backoff strategy without importing adapter packages.

**Yad2 Adapter:**
- Sends HTTP GET to Yad2 with query parameters built via `net/url.Values`
- Parses HTML response, extracts `<script id="__NEXT_DATA__">` JSON via targeted regex (not full DOM parse)
- Unmarshals JSON into `[]RawListing`
- Built-in retry: 3 attempts with 2s/4s/8s exponential backoff on retriable errors (429, 5xx, timeouts)
- Respects `Retry-After` header on 429
- Limits response body to 10 MB via `io.LimitReader`
- Only advertises `Accept-Encoding: gzip` (not `br` — we don't ship a Brotli decoder)

**Browser emulation headers:**
- Rotate User-Agent from a configurable pool
- Full browser header set (Accept-Language he-IL, Sec-Fetch-*, DNT, etc.)
- Optional SOCKS5 proxy support via config (credentials via env var interpolation)

**URL construction:**
```go
func buildURL(base string, params config.SourceParams) string {
    u, _ := url.Parse(base)
    v := url.Values{}
    if params.Manufacturer > 0 {
        v.Set("manufacturer", strconv.Itoa(params.Manufacturer))
    }
    // ... other params
    v.Set("Order", "1") // newest first
    u.RawQuery = v.Encode()
    return u.String()
}
```

### 2.2 Filter Engine

Pure, stateless filtering. No I/O. Zero values disable a filter.

```go
type FilterCriteria struct {
    EngineMinCC float64  // minimum engine displacement in cubic centimeters
    EngineMaxCC float64  // maximum engine displacement in cubic centimeters
    MaxKm       int
    MaxHand     int      // max ownership count
    Keywords    []string // all must appear in description (case-insensitive)
    ExcludeKeys []string // none may appear in description (case-insensitive)
}
```

**Unit convention:** Engine volume is always in **cubic centimeters (cc)**. Yad2 returns cc natively (e.g., 1998 for a 2.0L engine). The formatter converts to liters for display.

**Config validation:** If `EngineMinCC > 0 && EngineMinCC < 100`, warn at startup — the value likely uses liters instead of cc.

### 2.3 Deduplication Layer

**Port:**
```go
type DedupStore interface {
    // ClaimNew atomically inserts the token if unseen.
    // Returns true if the token was new (inserted), false if already seen.
    ClaimNew(ctx context.Context, token string, searchName string) (bool, error)

    // ReleaseClaim deletes a previously claimed token (for retry on notify failure).
    ReleaseClaim(ctx context.Context, token string) error

    Prune(ctx context.Context, olderThan time.Duration) (int64, error)
    Close() error
}
```

**SQLite Adapter:**
```sql
CREATE TABLE IF NOT EXISTS seen_listings (
    token TEXT PRIMARY KEY,
    search_name TEXT NOT NULL,
    first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

```go
func (s *Store) ClaimNew(ctx context.Context, token, searchName string) (bool, error) {
    result, err := s.db.ExecContext(ctx,
        "INSERT OR IGNORE INTO seen_listings (token, search_name) VALUES (?, ?)",
        token, searchName)
    if err != nil {
        return false, err
    }
    rows, err := result.RowsAffected()
    return rows > 0, err
}
```

Single operation, no race condition. If two goroutines claim simultaneously, exactly one wins.

**Directory creation:** `New()` calls `os.MkdirAll(filepath.Dir(dbPath), 0750)` before opening the database.

**Prune frequency:** Track `lastPruneTime`. Only prune if >24 hours since last prune. Run on startup unconditionally.

### 2.4 Notification Service

**Port:**
```go
type Notifier interface {
    Connect(ctx context.Context) error
    Notify(ctx context.Context, recipient string, listings []model.Listing) error
    Disconnect() error
}
```

**WhatsApp Adapter (whatsmeow):**
- Uses `go.mau.fi/whatsmeow` library
- Authenticates via QR code on first run (rendered with `qrencode` for terminal display)
- Session persisted in SQLite (separate from dedup DB)
- Sends formatted text messages with listing details
- On connection loss: whatsmeow auto-reconnects; log event + alert if reconnection fails for >30 min

**Telegram Adapter (recommended for MVP):**
- Uses `go-telegram-bot-api` library
- Zero ban risk, free, first-class bot API support
- Bot token from config (via env var interpolation)
- Recipients are Telegram chat IDs

**Message format (both channels):**
```
🚗 New Car Listing

*Mazda 3*

📅 Year: 2021
⚙️ Engine: 2.0L, Automatic
📍 Location: Tel Aviv
💰 Price: ₪95,000
🔗 https://www.yad2.co.il/item/abc123
```

### 2.5 Config Manager

YAML-based configuration loaded at startup. Env var interpolation for secrets.

```yaml
polling:
  interval: 15m
  jitter: 5m
  active_hours:
    start: "08:00"
    end: "22:00"
  timezone: "Asia/Jerusalem"

log_level: info  # debug | info | warn | error

searches:
  - name: "mazda3-2.0"
    source: yad2
    params:
      manufacturer: 27
      model: 10332
      year_min: 2018
      year_max: 2026
      price_min: 40000
      price_max: 200000
    filters:
      engine_min_cc: 1800    # cubic centimeters — NOT liters
      engine_max_cc: 2100
      max_km: 150000
      max_hand: 3
    recipients:
      - "+972XXXXXXXXX"

whatsapp:
  db_path: "./data/whatsapp.db"

storage:
  db_path: "./data/dedup.db"
  prune_after: 720h  # 30 days

http:
  user_agents:
    - "Mozilla/5.0 ..."
  proxy: "${PROXY_URL}"  # env var interpolation
```

**Config loading pipeline:**
1. Read YAML bytes
2. `os.ExpandEnv()` on raw bytes (env var interpolation)
3. Unmarshal into struct (with pre-applied defaults)
4. Validate (pure — returns errors only, no mutation)

**Validation rules:**
- At least one search configured
- Each search has name, source, and at least one recipient
- Active hours format `HH:MM` validated at load time (fail fast, not silently ignored)
- `EngineMinCC < 100` triggers a warning log
- `source` matches a known fetcher (currently: `yad2`)

### 2.6 Scheduler

The scheduler is the orchestration core. It owns the poll loop, backoff state, and active-hours gating.

**Adaptive backoff state:**
```go
type Scheduler struct {
    // ...
    backoffMultiplier float64 // starts at 1.0
    consecutiveErrors int
    lastPruneTime     time.Time
}
```

**Poll loop (pseudocode):**
```
1. Run initial cycle immediately (if within active hours)
2. Loop:
   a. Compute delay = (base_interval * backoff_multiplier) + random_jitter
   b. Sleep for delay (or until shutdown signal)
   c. Check active hours — if outside, compute sleep-until-start and continue
   d. For each search (with 60-second child context timeout):
      - Fetch listings (adapter handles retries internally)
      - Apply filters
      - ClaimNew for each filtered listing (atomic)
      - Notify recipients of claimed listings
      - On notify failure: ReleaseClaim → listing retried next cycle
   e. On challenge: backoffMultiplier = min(backoffMultiplier * 2, 4.0)
      On success: backoffMultiplier = max(backoffMultiplier / 2, 1.0)
   f. If lastPruneTime > 24h ago: prune dedup store
   g. Log cycle summary
```

**Active hours improvements:**
- Validated at config load (fail-fast on bad format)
- When outside active hours, compute exact duration until next active window start and sleep that amount (instead of sleeping a jitter interval and rechecking)
- Handle midnight-crossing active hours (e.g., `22:00` to `06:00`)

---

## 3. Data Models

```go
type RawListing struct {
    Token        string
    Manufacturer string
    Model        string
    SubModel     string
    Year         int
    Month        int
    EngineVolume float64 // always in cubic centimeters
    HorsePower   int
    EngineType   string
    GearBox      string
    Km           int
    Hand         int
    Price        int
    City         string
    Area         string
    Description  string
    ImageURL     string
    PageLink     string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type Listing struct {
    RawListing
    SearchName string
}
```

---

## 4. Data Flow (Revised)

```
1. Scheduler fires (base interval * backoff multiplier + jitter)
2. Check active hours:
   - If outside: compute duration until next active window, sleep that, skip cycle.
3. For each SearchConfig (with 60s context timeout):
   a. Fetcher.Fetch(params) → []RawListing
      - HTTP GET with browser headers, retry on 429/5xx (3 attempts, exp backoff)
      - Parse __NEXT_DATA__ JSON from HTML
      - Return structured error types (ErrChallenge, ErrRateLimited, ErrServerError)
   b. FilterEngine.Apply(criteria, raw) → []RawListing
      - Engine cc, mileage, keywords (pure function)
   c. For each filtered listing:
      - DedupStore.ClaimNew(token) → bool (atomic, no race)
      - If new: add to newListings batch
   d. If len(newListings) > 0:
      - For each recipient:
        - Notifier.Notify(recipient, newListings)
        - On failure: log, continue to next recipient
      - If ALL recipients failed:
        - ReleaseClaim for each token → retry next cycle
   e. On ErrChallenge: increase backoff multiplier
      On success: decrease backoff multiplier
4. Prune if >24h since last prune
5. Log cycle summary (fetched, filtered, new, notified, errors)
```

---

## 5. Error Handling Strategy

### Error Categories

| Category | Examples | Strategy |
|---|---|---|
| **Retriable (fetcher-level)** | HTTP timeout, DNS failure, 429, 5xx | Retry 3x with exp backoff (2s/4s/8s). Respect `Retry-After`. |
| **Challenge** | Anti-bot page detected | Skip cycle. Increase scheduler-level backoff multiplier (2x, cap at 4x). |
| **Fatal fetch** | 403 (not rate-limit), parse error, schema change | Skip search. Log error. Do not backoff (unlikely to self-resolve). Alert if persistent. |
| **Notify failure** | WhatsApp disconnected, Telegram API error | Release claimed tokens. Retry next cycle. Log with masked recipient. |
| **Config error** | Invalid YAML, missing required fields | Fail startup. Exit 1 with clear message. |

### Failure Alerting

Track `consecutiveErrors`. If >6 consecutive cycles fail (all searches), send an alert to the *first* recipient of the *first* search (if notifier is connected). Log an error regardless.

### Schema Change Detection

Log a warning when:
- `__NEXT_DATA__` script tag is missing
- Feed items array is empty on a query that previously returned results
- >50% of items fail `itemToListing` parsing

---

## 6. Security Considerations

| Concern | Mitigation |
|---|---|
| WhatsApp session credentials | Stored in local SQLite DB. Directory created with 0750 permissions. Never committed to git. |
| Config with phone numbers | `.gitignore` excludes `config.yaml`. Provide `config.example.yaml` with placeholder values. |
| Proxy credentials | Env var interpolation: `proxy: "${PROXY_URL}"`. Never in plaintext YAML. |
| PII in logs | Phone numbers masked in log output (`+972***XXX`). Token values are not PII. |
| Container security | Dockerfile runs as non-root user (UID 1000). `/data` volume owned by bot user. |
| Response size | `io.LimitReader` caps response body at 10 MB. Prevents OOM from malformed responses. |
| SQLite path | Validate `dbPath` doesn't contain suspicious characters or pragmas before appending connection params. |
| Scraping legality | Personal use, non-commercial, low-frequency polling. Respect `robots.txt`. |

---

## 7. Scaling Strategy

### Phase 1: Single User (MVP)
- Single binary, sequential polling loop
- SQLite for dedup, WhatsApp session in separate SQLite
- One search config → one recipient
- Runs on any Linux box, Raspberry Pi, or Docker container

### Phase 2: Multiple Users
- Multiple search configs, each with own recipients
- Sequential polling per search (shared rate limiter for same domain)
- Still single binary, still SQLite

### Phase 3: Multi-Marketplace
- Fetcher factory maps `source` field to adapter constructors
- Each adapter implements `fetcher.Fetcher` interface + wraps `fetcher.Err*` sentinels
- Shared dedup store (tokens are namespaced by source)

### Phase 4: Scale-up (if needed)
- Replace SQLite with PostgreSQL
- Add Redis for hot dedup cache
- HTTP API for config management
- Telegram/email notifiers alongside WhatsApp
- Prometheus metrics + Grafana dashboard

---

## 8. Testing Strategy

### Unit Tests (required before merge)
- **Filter engine:** All criteria combinations, zero-value disabling, edge cases (table-driven)
- **Yad2 parser:** Parse saved `__NEXT_DATA__` JSON fixtures → assert correct struct mapping. Test schema drift detection (missing fields, renamed fields).
- **Message formatter:** Assert output format for various listing states
- **Dedup store:** ClaimNew atomicity, ReleaseClaim, Prune with in-memory SQLite
- **Config loader:** Valid config, missing fields, invalid active hours format, env var interpolation, unit validation warnings
- **Scheduler:** Mock all dependencies. Test: successful cycle, all-seen cycle, challenge backoff, notify failure + release claim, active hours boundary, per-search timeout

### Integration Tests
- **Full pipeline:** Mock fetcher (saved HTML) → real filter → in-memory SQLite dedup → mock notifier → assert correct listings delivered with no duplicates
- **Crash recovery:** Mark-then-crash simulation → verify listings are retried

### Test Fixtures
- Save real Yad2 HTML responses as `testdata/*.html`
- Save extracted `__NEXT_DATA__` JSON as `testdata/*.json`
- Anonymize any personal data in fixtures

---

## 9. Implementation Plan

### Phase 1: MVP (Corrected)

1. **Project scaffolding** — Go module, directory structure, Makefile, `.gitignore`
2. **Config loading** — YAML parsing, env var interpolation, validation (fail-fast on bad active hours)
3. **Data models** — `RawListing`, `Listing`, error sentinels in `fetcher` package
4. **Yad2 fetcher** — HTTP client (gzip only), targeted `__NEXT_DATA__` extraction, response size limit, built-in retry
5. **Filter engine** — In-memory criteria matching, cc units, table-driven tests
6. **Dedup store** — SQLite with atomic `ClaimNew`, `ReleaseClaim`, directory auto-creation
7. **Telegram notifier** — Simple, zero ban risk, working notification channel for MVP
8. **Scheduler** — Adaptive backoff, per-search timeout, notify-then-mark, active hours with sleep-to-start
9. **Main orchestrator** — Fetcher factory (by `source`), graceful shutdown, non-root Docker
10. **Tests** — Unit tests for parser, filter, dedup, scheduler. Integration test for full pipeline.

### Phase 2: WhatsApp + Hardening
11. WhatsApp notifier (whatsmeow, QR auth, reconnection)
12. Health check endpoint (`:8080/healthz`)
13. Configurable log level
14. Version/build info (`--version`)
15. Config hot reload (SIGHUP)

### Phase 3: Observability
16. Prometheus metrics endpoint
17. Schema change detection alerts
18. Consecutive failure alerting

---

## 10. Deployment

### Dockerfile (improved)
```dockerfile
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /bot ./cmd/bot

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 bot
USER bot
COPY --from=builder /bot /bot
VOLUME /data
ENTRYPOINT ["/bot"]
CMD ["-config", "/config.yaml"]
```

### docker-compose.yaml
```yaml
services:
  bot:
    build: .
    volumes:
      - ./data:/data
      - ./config.yaml:/config.yaml:ro
    environment:
      - TZ=Asia/Jerusalem
    restart: unless-stopped
    # healthcheck:
    #   test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/healthz"]
    #   interval: 60s
    #   timeout: 5s
    #   retries: 3
```

### Makefile (improved)
```makefile
.PHONY: build run test lint clean ci docker-build docker-run

build:
	go build -o bot ./cmd/bot

run: build
	./bot -config config.yaml

test:
	go test ./...

lint:
	golangci-lint run ./...

ci: lint test

clean:
	rm -f bot

docker-build:
	docker build -t carwatch .

docker-run:
	docker compose up -d
```

---

## 11. Project Structure (Revised)

```
carwatch/
├── cmd/
│   └── bot/
│       └── main.go              # Entry point, fetcher factory, wiring
├── internal/
│   ├── config/
│   │   ├── config.go            # YAML config loading, env interpolation, validation
│   │   └── config_test.go       # Config validation tests
│   ├── fetcher/
│   │   ├── fetcher.go           # Fetcher interface + error sentinels
│   │   └── yad2/
│   │       ├── client.go        # HTTP client (gzip only, size-limited)
│   │       ├── parser.go        # __NEXT_DATA__ extraction (regex, not full DOM)
│   │       ├── parser_test.go   # Parse saved fixtures
│   │       ├── retry.go         # Exponential backoff retry wrapper
│   │       └── yad2.go          # Yad2Fetcher adapter
│   ├── filter/
│   │   ├── filter.go            # Filter engine (pure functions)
│   │   └── filter_test.go       # Table-driven filter tests
│   ├── model/
│   │   └── listing.go           # Domain models
│   ├── notifier/
│   │   ├── notifier.go          # Notifier interface
│   │   ├── formatter.go         # Message formatting (cc → liters conversion)
│   │   ├── formatter_test.go    # Formatter tests
│   │   ├── telegram/
│   │   │   └── telegram.go      # Telegram adapter (MVP notifier)
│   │   └── whatsapp/
│   │       └── whatsapp.go      # WhatsApp adapter (Phase 2)
│   ├── storage/
│   │   ├── dedup.go             # DedupStore interface (ClaimNew/ReleaseClaim)
│   │   └── sqlite/
│   │       ├── sqlite.go        # SQLite adapter (atomic claim, dir creation)
│   │       └── sqlite_test.go   # Dedup tests
│   └── scheduler/
│       ├── scheduler.go         # Polling loop, adaptive backoff, notify-then-mark
│       └── scheduler_test.go    # Scheduler tests with mocks
├── testdata/
│   ├── yad2_feed.json           # Saved __NEXT_DATA__ fixture
│   └── yad2_page.html           # Saved HTML fixture
├── config.example.yaml
├── Dockerfile
├── docker-compose.yaml
├── Makefile
├── go.mod
├── go.sum
├── .gitignore
├── DESIGN.md
├── bugs.md
├── enhancements.md
└── code-quality.md
```

---

## 12. Decision Log

| Decision | Rationale | Alternative Considered |
|---|---|---|
| Telegram for MVP notifier | Zero ban risk, free, instant setup. WhatsApp stub is not a shipping product. | WhatsApp (ban risk, stub state) |
| Atomic ClaimNew over HasSeen+MarkSeen | Eliminates race condition and notify-before-mark data loss bug. | Separate read/write with transaction (heavier, same result) |
| Regex for __NEXT_DATA__ over goquery | Lower memory, faster, fewer dependencies for extracting a single known element. | goquery full DOM parse (500KB → multi-MB DOM) |
| Error sentinels in `fetcher` package | Scheduler doesn't import adapter packages. Clean ports & adapters boundary. | Concrete error types in each adapter (leaky abstraction) |
| Gzip only, no Brotli | Avoids shipping a Brotli decoder. Yad2 respects Accept-Encoding and falls back to gzip. | Accept `br` and add `andybalholm/brotli` (extra dependency) |
| Sequential search processing | Simple, predictable. Rate limiter is implicit (one request per search per cycle). | Concurrent goroutines with shared rate limiter (complex for MVP) |
| Non-root Docker | Defense in depth. SQLite + network = attack surface worth reducing. | Root (simpler volume permissions) |
