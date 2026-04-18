# CarWatch - System Design Document

## Overview

A production-grade bot that monitors car listings from Yad2 (Israel's largest classifieds marketplace) and sends relevant listings to users via WhatsApp in near real-time.

**Initial use case:** Mazda 3, 2.0L engine, Israel (Yad2).
**Design goal:** Configurable, extensible to other cars, users, and marketplaces.

---

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Scheduler                                │
│                   (Ticker + Jitter)                              │
└──────────────────────┬──────────────────────────────────────────┘
                       │ triggers
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Fetcher (Port)                                │
│              ┌──────────────────┐                                │
│              │  Yad2Fetcher     │  HTTP GET → parse __NEXT_DATA__│
│              │  (Adapter)       │  → []RawListing                │
│              └──────────────────┘                                │
└──────────────────────┬──────────────────────────────────────────┘
                       │ []RawListing
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Filter Engine                                  │
│         Applies user-defined criteria:                           │
│         model, engine, price, year, location, keywords           │
│         → []Listing (filtered)                                   │
└──────────────────────┬──────────────────────────────────────────┘
                       │ []Listing
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│               Deduplication Layer                                │
│          Check token against seen set (SQLite)                   │
│          → []Listing (new only)                                  │
└──────────────────────┬──────────────────────────────────────────┘
                       │ []Listing (new)
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│              Notification Service (Port)                         │
│         ┌────────────────────┐  ┌──────────────────┐            │
│         │ WhatsAppNotifier   │  │ TelegramNotifier  │            │
│         │ (whatsmeow)        │  │ (future)          │            │
│         └────────────────────┘  └──────────────────┘            │
└─────────────────────────────────────────────────────────────────┘
                       │
                       ▼
                 User's Phone
```

### Component Breakdown

| Component | Responsibility | Interface |
|---|---|---|
| **Scheduler** | Triggers polling cycles with jitter | Runs the pipeline on a timer |
| **Fetcher** | Retrieves raw listings from a marketplace | `Fetcher` interface |
| **Filter Engine** | Applies user criteria to raw listings | Pure function, no I/O |
| **Dedup Store** | Tracks which listings have been seen | `DedupStore` interface |
| **Notifier** | Delivers alerts to users | `Notifier` interface |
| **Config Manager** | Loads and validates user preferences | Reads YAML config |

---

## 2. Detailed Component Design

### 2.1 Fetcher

**Port (interface):**
```go
type Fetcher interface {
    Fetch(ctx context.Context, params SearchParams) ([]RawListing, error)
}
```

**Yad2 Adapter:**
- Sends HTTP GET to `https://www.yad2.co.il/vehicles/cars` with query parameters
- Parses HTML response, extracts `<script id="__NEXT_DATA__">` JSON blob
- Unmarshals JSON into `[]RawListing`
- Uses realistic browser headers (User-Agent, Accept, Sec-Fetch-*, etc.)
- Handles pagination (40 items per page, 0-indexed `page` parameter)

**Known Yad2 query parameters:**

| Parameter | Format | Example |
|---|---|---|
| `manufacturer` | Integer ID | `27` (Mazda) |
| `model` | Integer ID | `10332` (Mazda 3) |
| `year` | `MIN-MAX` | `2018-2026` |
| `price` | `MIN-MAX` | `40000-200000` |
| `Order` | Integer | `1` (newest first) |
| `page` | Integer (0-based) | `0` |

**Anti-bot strategy:**
- Rotate User-Agent from a pool of 5-10 realistic browser strings
- Set full browser header set (Accept-Language, Sec-Fetch-Dest, DNT, etc.)
- Random jitter on polling interval (10-20 min base + ±2-5 min)
- Exponential backoff on 429/403/5xx responses
- Detect "Are you for real" challenge page and back off
- Optional: residential proxy support via config

### 2.2 Filter Engine

Pure, stateless filtering. No I/O.

```go
type FilterCriteria struct {
    Models      []string  // e.g., ["mazda 3"]
    EngineMin   float64   // e.g., 1.8
    EngineMax   float64   // e.g., 2.0
    PriceMin    int       // in ILS
    PriceMax    int
    YearMin     int
    YearMax     int
    MaxKm       int
    MaxHand     int       // ownership count
    Keywords    []string  // required keywords in description
    ExcludeKeys []string  // exclude if description contains these
}
```

Filters are applied in-memory after fetching. The Yad2 URL parameters handle coarse filtering (manufacturer, model, price range, year range); the filter engine handles fine-grained criteria (engine size, mileage, keywords, ownership count).

### 2.3 Deduplication Layer

**Port:**
```go
type DedupStore interface {
    HasSeen(ctx context.Context, token string) (bool, error)
    MarkSeen(ctx context.Context, token string) error
    Prune(ctx context.Context, olderThan time.Duration) error
}
```

**SQLite Adapter:**
- Table: `seen_listings(token TEXT PRIMARY KEY, first_seen_at TIMESTAMP)`
- `HasSeen`: SELECT by token
- `MarkSeen`: INSERT OR IGNORE
- `Prune`: DELETE WHERE first_seen_at < threshold (e.g., 30 days)

**Why SQLite:** Zero-config, embedded, survives restarts, sufficient for single-instance use. The interface allows swapping to Redis or PostgreSQL later.

### 2.4 Notification Service

**Port:**
```go
type Notifier interface {
    Notify(ctx context.Context, recipient string, listings []Listing) error
}
```

**WhatsApp Adapter (whatsmeow):**
- Uses `go.mau.fi/whatsmeow` library
- Authenticates via QR code scan on first run
- Session persisted in SQLite (separate from dedup DB)
- Sends formatted text messages with listing details
- Message format:

```
🚗 New Mazda 3 Listing

📅 Year: 2021
⚙️ Engine: 2.0L, Automatic
📍 Location: Tel Aviv
💰 Price: ₪95,000
🔗 https://www.yad2.co.il/item/abc123
```

**Important:** Use a secondary phone number to protect primary account from ban risk.

### 2.5 Config Manager

YAML-based configuration loaded at startup:

```yaml
polling:
  interval: 15m
  jitter: 5m
  active_hours:
    start: "08:00"
    end: "22:00"
  timezone: "Asia/Jerusalem"

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
      engine_min: 1.8
      engine_max: 2.0
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
    - "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
    - "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"
  proxy: ""  # optional: "socks5://user:pass@host:port"
```

### 2.6 Storage Layer

Two SQLite databases (separation of concerns):

1. **Dedup DB** (`data/dedup.db`):
   - `seen_listings(token TEXT PK, search_name TEXT, first_seen_at TIMESTAMP)`

2. **WhatsApp session DB** (`data/whatsapp.db`):
   - Managed by whatsmeow internally

Both are embedded SQLite — zero infrastructure, survives restarts, easy to back up.

---

## 3. Data Models

### Listing

```go
type RawListing struct {
    Token        string
    Manufacturer string
    Model        string
    SubModel     string
    Year         int
    Month        int
    EngineVolume float64
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
    SearchName string // which search config matched
}
```

### Search Configuration

```go
type SearchConfig struct {
    Name       string
    Source     string // "yad2"
    Params     SourceParams
    Filters    FilterCriteria
    Recipients []string
}

type SourceParams struct {
    Manufacturer int
    Model        int
    YearMin      int
    YearMax      int
    PriceMin     int
    PriceMax     int
}
```

---

## 4. Data Flow

```
1. Scheduler fires (every 10-20 min with jitter)
2. For each SearchConfig:
   a. Fetcher.Fetch(params) → []RawListing
      - HTTP GET to Yad2 with query params
      - Parse __NEXT_DATA__ JSON from HTML
      - Handle pagination if needed
   b. FilterEngine.Apply(criteria, rawListings) → []Listing
      - Engine size, mileage, keywords, etc.
   c. DedupStore.FilterNew(listings) → []Listing (unseen only)
      - Check each token against SQLite
      - Mark new tokens as seen
   d. If len(newListings) > 0:
      Notifier.Notify(recipients, newListings)
      - Format message per listing
      - Send via whatsmeow
3. Log cycle summary (fetched, filtered, new, notified)
4. Sleep until next cycle
```

---

## 5. Edge Cases & Failure Handling

### Website Structure Changes
- **Problem:** Yad2 changes HTML structure or `__NEXT_DATA__` schema
- **Mitigation:** Log raw response on parse failure. Alert user via WhatsApp that parsing broke. Keep fetcher logic isolated so only one adapter needs updating.

### Anti-Bot Detection
- **Problem:** Yad2 returns "Are you for real" challenge page
- **Mitigation:** Detect challenge text in response body. Trigger exponential backoff (double interval, cap at 1 hour). Log the event. Resume normal polling after successful fetch.

### Duplicate Detection Edge Cases
- **Problem:** Yad2 re-lists the same car with a new token (seller deletes and re-posts)
- **Mitigation:** Accept this as a new listing — token is the only reliable ID. Alternative: fingerprint by (model + year + km + price + city) but this risks false dedup when two sellers list similar cars.

### API/Network Failures
- **Problem:** HTTP timeouts, DNS failures, connection refused
- **Mitigation:** Retry with exponential backoff (3 attempts). Log errors. Continue to next cycle. Send a summary alert if failures persist for >1 hour.

### WhatsApp Connection Loss
- **Problem:** whatsmeow disconnects (session expires, phone offline)
- **Mitigation:** whatsmeow has auto-reconnect. Log disconnections. Queue unsent messages and retry on reconnect. Alert via stdout/logs if reconnection fails for >30 min.

### Rate Limiting (429)
- **Problem:** Yad2 returns HTTP 429
- **Mitigation:** Exponential backoff starting at 5 minutes. Respect Retry-After header if present.

---

## 6. Security Considerations

| Concern | Mitigation |
|---|---|
| WhatsApp session credentials | Stored in local SQLite DB. Restrict file permissions (0600). Never commit to git. |
| Config with phone numbers | Keep `config.yaml` out of git (`.gitignore`). Provide `config.example.yaml`. |
| Proxy credentials | Support environment variable interpolation in config for sensitive values. |
| Scraping legality | Personal use, non-commercial, low-frequency polling. Respect `robots.txt`. |
| Secondary WhatsApp number | Use a dedicated SIM/number to isolate ban risk from personal account. |

---

## 7. Scaling Strategy

### Phase 1: Single User (MVP)
- Single binary, single goroutine polling loop
- SQLite for both dedup and whatsapp session
- Config file with one search definition
- Runs on any Linux box, Raspberry Pi, or Docker container

### Phase 2: Multiple Users
- Multiple search configs in YAML, each with own recipients
- Concurrent polling per search (goroutines with shared rate limiter)
- Still single binary, still SQLite

### Phase 3: Multi-Marketplace
- Add new Fetcher adapters (e.g., AutoTrader, Facebook Marketplace)
- Each adapter implements the same `Fetcher` interface
- Search config specifies `source: "yad2"` or `source: "autotrader"`

### Phase 4: Full Scale (if ever needed)
- Replace SQLite with PostgreSQL
- Add Redis for hot dedup cache
- Deploy as a service with HTTP API for managing configs
- Add Telegram/email notifiers alongside WhatsApp

---

## 8. Testing Strategy

### Unit Tests
- **Filter engine:** Test all criteria combinations, edge cases (zero values = disabled filter)
- **Yad2 parser:** Parse saved `__NEXT_DATA__` JSON fixtures → assert correct struct mapping
- **Message formatter:** Assert output format for various listing states (missing fields, Hebrew text)
- **Dedup store:** HasSeen/MarkSeen/Prune with in-memory SQLite

### Integration Tests
- **Fetcher:** Record real Yad2 response (save HTML fixture), replay in test, verify parsing
- **Full pipeline:** Fetcher mock → filter → dedup (in-memory SQLite) → notifier mock → assert correct listings delivered

### Manual / Smoke Tests
- Run bot locally, observe first poll cycle, verify WhatsApp message received
- Test with a search that has zero results (no crash, no false notification)
- Kill and restart bot — verify no duplicate notifications

### Test Fixtures
- Save real Yad2 HTML responses as `testdata/*.html` for regression testing
- Anonymize any personal data in fixtures

---

## 9. Step-by-Step Implementation Plan

### Phase 1: MVP (Target: working bot for Mazda 3 alerts)

1. **Project scaffolding** — Go module, directory structure, Makefile
2. **Config loading** — YAML parsing, validation
3. **Yad2 fetcher** — HTTP client, HTML parsing, `__NEXT_DATA__` extraction
4. **Filter engine** — In-memory criteria matching
5. **Dedup store** — SQLite implementation
6. **WhatsApp notifier** — whatsmeow integration, QR auth
7. **Scheduler** — Ticker with jitter, active hours check
8. **Main orchestrator** — Wire everything together, graceful shutdown
9. **Docker setup** — Dockerfile, docker-compose, volume for data/

### Phase 2: Configurability
10. Multiple search configs per user
11. Multiple recipients per search
12. WhatsApp command interface (user sends "add search" to bot)
13. Runtime config reload (SIGHUP)

### Phase 3: Hardening
14. Structured logging (slog)
15. Metrics endpoint (Prometheus)
16. Health check endpoint
17. Proxy rotation support
18. Exponential backoff refinement
19. Alerting on persistent failures

---

## 10. WhatsApp Integration Comparison

| Factor | whatsmeow (Unofficial) | Cloud API (Meta) | Twilio |
|---|---|---|---|
| **Setup time** | < 1 hour | Days-weeks | 1-2 weeks |
| **Cost** | Free | Per-message | Per-message + markup |
| **Go SDK** | Excellent (tulir/whatsmeow) | Good (piusalfred/whatsapp) | Official (twilio-go) |
| **Rich messages** | Yes, no restrictions | Yes, requires templates | Yes, requires templates |
| **Ban risk** | Moderate (use secondary #) | None | None |
| **ToS compliant** | No | Yes | Yes |
| **Business verification** | No | Required | Required |
| **Best for** | Personal/small bots | Business at scale | Business needing Twilio ecosystem |

**MVP recommendation: whatsmeow** — Free, instant setup, no template approval. Use a secondary phone number.

**Production recommendation: WhatsApp Business Cloud API** — When reliability and compliance matter.

**Safest alternative: Telegram Bot API** — Free, first-class bot support, zero ban risk, excellent Go library (`go-telegram-bot-api`). Only downside: recipients must use Telegram.

---

## 11. Anti-Scraping Strategy for Yad2

### How to avoid being blocked
1. **Poll infrequently:** 10-20 minutes between requests (plenty fast for car listings)
2. **Add jitter:** ±2-5 minutes random variation on each cycle
3. **Realistic headers:** Full browser header set, rotated User-Agent pool
4. **Active hours only:** Poll 08:00-22:00 Israel time (mimics human behavior)
5. **Single page first:** Only fetch page 0 (newest 40 listings). Pagination only if needed.
6. **Exponential backoff:** On any error, double the interval (cap at 1 hour)
7. **Detect challenges:** Look for "Are you for real" text, back off immediately

### How to detect new listings efficiently
- Sort by newest first (`Order=1`)
- Only fetch page 0 (newest 40 listings)
- Compare tokens against seen set
- Stop pagination early if all tokens on a page are already seen

### Caching strategy
- Cache full response body for 5 minutes (avoid re-fetch on transient errors)
- Cache parsed listings in memory between cycles for diffing

### Future extensibility
- **Telegram:** Add `TelegramNotifier` implementing `Notifier` interface. Use `go-telegram-bot-api`.
- **Web dashboard:** Add HTTP server with REST API for managing searches, viewing history. Serve a simple React/HTMX frontend.

---

## 12. Deployment Strategy

### Local Development
```bash
go run ./cmd/bot -config config.yaml
# First run: scan QR code in terminal to authenticate WhatsApp
```

### Docker
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bot ./cmd/bot

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bot /bot
ENTRYPOINT ["/bot"]
```

```yaml
# docker-compose.yaml
services:
  bot:
    build: .
    volumes:
      - ./data:/data
      - ./config.yaml:/config.yaml:ro
    environment:
      - TZ=Asia/Jerusalem
    restart: unless-stopped
```

### Cloud-Ready
- Runs on any VPS (DigitalOcean $5/mo, Hetzner $4/mo)
- No external dependencies (SQLite is embedded)
- Persistent volume for `data/` directory
- Optional: systemd unit file for non-Docker deployments

---

## 13. Project Structure

```
carwatch/
├── cmd/
│   └── bot/
│       └── main.go              # Entry point, wiring
├── internal/
│   ├── config/
│   │   └── config.go            # YAML config loading & validation
│   ├── fetcher/
│   │   ├── fetcher.go           # Fetcher interface
│   │   └── yad2/
│   │       ├── client.go        # HTTP client with anti-bot headers
│   │       ├── parser.go        # __NEXT_DATA__ JSON parsing
│   │       └── yad2.go          # Yad2Fetcher adapter
│   ├── filter/
│   │   └── filter.go            # FilterEngine (pure functions)
│   ├── model/
│   │   └── listing.go           # Domain models
│   ├── notifier/
│   │   ├── notifier.go          # Notifier interface
│   │   ├── formatter.go         # Message formatting
│   │   └── whatsapp/
│   │       └── whatsapp.go      # whatsmeow adapter
│   ├── storage/
│   │   ├── dedup.go             # DedupStore interface
│   │   └── sqlite/
│   │       └── sqlite.go        # SQLite adapter
│   └── scheduler/
│       └── scheduler.go         # Polling loop with jitter
├── config.example.yaml
├── Dockerfile
├── docker-compose.yaml
├── Makefile
├── go.mod
├── go.sum
├── .gitignore
└── DESIGN.md
```
