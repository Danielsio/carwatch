# CarWatch — System Design (v3): Multi-Tenant Bot-as-a-Service

## Overview

CarWatch evolves from a single-binary, config-file bot into a **cloud-hosted Telegram bot** where users onboard and manage car search alerts entirely through chat. No laptop needed — users interact from their phones.

**End state:** Users message the bot on Telegram, set up car search criteria via guided conversation, and receive alerts when new matching listings appear on Yad2 (and eventually other Israeli marketplaces).

**Key constraints:**
- Zero hosting cost (Oracle Cloud Always Free Tier)
- Telegram first, WhatsApp Cloud API later (~$0.005/msg)
- Stay in Go — existing scraping/parsing/scheduling is proven and more sophisticated than any open-source alternative found

---

## 1. What Changes from v2

| Aspect | v2 (Current) | v3 (Target) |
|---|---|---|
| **Config source** | YAML file | SQLite database (user-managed via Telegram) |
| **User management** | Static recipients in config | Self-service via Telegram bot commands |
| **Scraping strategy** | One loop per SearchConfig | Shared scraping with per-user fan-out |
| **Notification channel** | WhatsApp stub / Telegram HTTP | Telegram Bot API (interactive) |
| **Deployment** | Local Docker / any Linux box | Oracle Cloud ARM VM (free tier) |
| **Telegram library** | `go-telegram-bot-api` (unmaintained) | `go-telegram/bot` v1.20+ (actively maintained) |

**What stays the same:**
- Go language, single binary
- Yad2 fetcher with regex parsing, adaptive backoff, retry logic
- SQLite for storage (dedup, users, searches)
- Ports & adapters architecture (Fetcher, Notifier interfaces)
- Filter engine (pure function)
- Atomic dedup (INSERT OR IGNORE)
- Notify-then-mark semantics

---

## 2. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Telegram Bot API                                 │
│               (Long Polling — single bot token)                      │
└────────────┬────────────────────────────────────────────────────────┘
             │ Updates (commands, callbacks, messages)
             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Bot Handler Layer                               │
│    ┌─────────────┐  ┌──────────────┐  ┌───────────────┐            │
│    │ Command      │  │ Conversation │  │ Callback      │            │
│    │ Router       │  │ FSM          │  │ Handler       │            │
│    │ /start       │  │ (onboarding  │  │ (inline       │            │
│    │ /watch       │  │  wizard)     │  │  keyboards)   │            │
│    │ /list        │  │              │  │               │            │
│    │ /stop        │  │              │  │               │            │
│    └──────┬──────┘  └──────┬───────┘  └───────┬───────┘            │
└───────────┼────────────────┼──────────────────┼────────────────────┘
            │                │                  │
            ▼                ▼                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Service Layer                                  │
│    ┌─────────────────┐  ┌──────────────────┐                        │
│    │ User Service     │  │ Search Service    │                       │
│    │ (CRUD users)     │  │ (CRUD searches,   │                       │
│    │                  │  │  merge into        │                       │
│    │                  │  │  canonical groups) │                       │
│    └────────┬─────────┘  └────────┬──────────┘                      │
└─────────────┼─────────────────────┼─────────────────────────────────┘
              │                     │
              ▼                     ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     SQLite Database                                   │
│    ┌──────────┐ ┌───────────┐ ┌──────────────┐ ┌───────────────┐   │
│    │ users    │ │ searches  │ │ seen_listings│ │ price_history │   │
│    └──────────┘ └───────────┘ └──────────────┘ └───────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Scheduler (unchanged core)                        │
│    ┌──────────────┐                                                  │
│    │ Canonical     │ For each unique {manufacturer, model}:          │
│    │ Search Groups │ 1. Fetch once (widest params across all users)  │
│    │               │ 2. Fan out: filter per user's criteria          │
│    │               │ 3. Dedup per user                               │
│    │               │ 4. Notify each user individually                │
│    └──────────────┘                                                  │
│    Adaptive backoff, active hours, retry — all preserved             │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. User Experience (Telegram)

### Onboarding Flow

```
User: /start
Bot:  Welcome to CarWatch! I'll help you find your next car.
      Use /watch to set up a new car search.

User: /watch
Bot:  What manufacturer are you looking for?
      [Mazda] [Toyota] [Hyundai] [Honda] [Kia] [Other...]

User: taps [Mazda]
Bot:  Which Mazda model?
      [2] [3] [6] [CX-5] [CX-30] [Other...]

User: taps [3]
Bot:  Year range? (e.g. 2018-2024)

User: 2018-2024
Bot:  Max price in NIS? (e.g. 150000)

User: 150000
Bot:  Any engine preference?
      [Any] [1.5L+] [2.0L+]

User: taps [2.0L+]
Bot:  Got it! Here's your search:
      Mazda 3, 2018-2024, up to 150,000 NIS, 2.0L+
      [Confirm] [Edit] [Cancel]

User: taps [Confirm]
Bot:  Search saved! I'll check Yad2 every 15 minutes and
      send you new listings. Use /list to see your searches.
```

### Commands

| Command | Description |
|---|---|
| `/start` | Register + welcome message |
| `/watch` | Start new search wizard |
| `/list` | Show active searches |
| `/stop <id>` | Pause/delete a search |
| `/settings` | Quiet hours, notification preferences |
| `/help` | Command reference |

### Inline Keyboards for Known Values

Manufacturer and model IDs are fixed on Yad2. The bot stores a lookup table of `{name -> yad2_id}` and presents inline keyboard buttons. This eliminates typos and makes the flow phone-friendly.

For numeric inputs (year, price, mileage), free text is fine — the bot validates and re-prompts on bad input.

---

## 4. Database Schema

```sql
-- User registration
CREATE TABLE users (
    chat_id     INTEGER PRIMARY KEY,
    username    TEXT,
    state       TEXT NOT NULL DEFAULT 'idle',
    state_data  TEXT DEFAULT '{}',  -- JSON: wizard in-progress data
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    active      BOOLEAN DEFAULT true
);

-- Per-user saved searches
CREATE TABLE searches (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id      INTEGER NOT NULL REFERENCES users(chat_id),
    name         TEXT NOT NULL,
    manufacturer INTEGER NOT NULL,    -- Yad2 manufacturer ID
    model        INTEGER NOT NULL,    -- Yad2 model ID
    year_min     INTEGER DEFAULT 2000,
    year_max     INTEGER DEFAULT 2030,
    price_max    INTEGER DEFAULT 9999999,
    engine_min_cc INTEGER DEFAULT 0,
    max_km       INTEGER DEFAULT 0,
    max_hand     INTEGER DEFAULT 0,
    active       BOOLEAN DEFAULT true,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chat_id, manufacturer, model)
);

-- Dedup: per-user seen listings
CREATE TABLE seen_listings (
    token       TEXT NOT NULL,
    chat_id     INTEGER NOT NULL,
    search_id   INTEGER NOT NULL,
    first_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (token, chat_id)
);

-- Price tracking (shared across users)
CREATE TABLE price_history (
    token       TEXT NOT NULL,
    price       INTEGER NOT NULL,
    observed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (token, price)
);
```

Key change: dedup is now **per-user**. User A seeing a listing doesn't prevent User B from getting it.

---

## 5. Shared Scraping with Per-User Fan-Out

### The Problem

If 50 users all watch Mazda 3, we don't want to hit Yad2 50 times per cycle.

### The Solution: Canonical Search Groups

```
Step 1: Group overlapping searches
  User A: Mazda 3, 2018-2024, price < 150k, engine > 2.0L
  User B: Mazda 3, 2020-2026, price < 200k, any engine
  User C: Mazda 3, 2019-2025, price < 180k, engine > 1.5L
                    ↓
  Canonical group: Mazda 3, 2018-2026, price < 200k (union of all ranges)

Step 2: Fetch once with the widest parameters

Step 3: Fan out — apply each user's individual filters
  User A: filter(engine >= 2000cc, year >= 2018, price <= 150000)
  User B: filter(year >= 2020, price <= 200000)
  User C: filter(engine >= 1500cc, year >= 2019, price <= 180000)

Step 4: Per-user dedup + notify
```

This reduces Yad2 requests from N-per-user to N-per-unique-car-model. For a small user base (< 100 users), this is likely 5-20 unique scrapes per cycle regardless of user count.

---

## 6. Conversation State Machine

```go
type UserState string

const (
    StateIdle             UserState = "idle"
    StateAskManufacturer  UserState = "ask_manufacturer"
    StateAskModel         UserState = "ask_model"
    StateAskYearRange     UserState = "ask_year"
    StateAskPriceMax      UserState = "ask_price"
    StateAskEngine        UserState = "ask_engine"
    StateConfirm          UserState = "ask_confirm"
)
```

State is stored in the `users.state` column with wizard data in `users.state_data` (JSON). On bot restart, in-progress wizards resume from where they left off.

A `/cancel` command from any state returns to `idle` and clears `state_data`.

Sessions abandoned for >30 minutes are auto-expired back to `idle`.

---

## 7. Technology Decisions

| Decision | Choice | Rationale |
|---|---|---|
| **Language** | Go | Existing codebase, single binary, low memory (10-20 MB), proven scraping logic |
| **Telegram library** | `go-telegram/bot` v1.20+ | Actively maintained (Mar 2026), 1:1 Bot API mapping, official Telegram samples page listing. Replaces unmaintained `go-telegram-bot-api` |
| **FSM library** | Hand-rolled or `go-telegram/fsm` | Simple onboarding flow (7 states). No need for heavy framework |
| **Database** | SQLite (WAL mode) | Already in use, zero config, adequate for <1000 users |
| **Hosting** | Oracle Cloud Always Free (ARM A1) | 4 OCPUs, 24 GB RAM, 200 GB storage, free for life |
| **Bot mode** | Long polling | Simpler than webhooks, works behind NAT, single-instance deployment |
| **WhatsApp (future)** | Official Cloud API | ~$0.005/msg, zero ban risk. Avoid whatsmeow — high ban risk in 2026 |

### Why Not Python?

| Factor | Go | Python |
|---|---|---|
| Existing codebase | Keep everything | Rewrite from scratch |
| Memory footprint | 10-20 MB | 50-100 MB |
| Deployment | Single binary, no deps | virtualenv, pip, runtime |
| Long-running process | Native fit | GIL limitations, asyncio complexity |
| Telegram bot DX | Good (`go-telegram/bot`) | Slightly better (`aiogram` FSM) |
| **Verdict** | **Stay** | Not worth the rewrite cost |

### Why Not whatsmeow for WhatsApp?

Research shows whatsmeow users report "account at risk" warnings (GitHub issue #810, May 2025) and Baileys users report mass bans (issue #1869). Meta's 2026 policy further restricts unofficial automation. The official WhatsApp Cloud API costs ~$1-3/month for a small user base and provides reliable, ban-free delivery.

---

## 8. Hosting: Oracle Cloud Always Free

### Resources (free for life)

- **Compute:** Up to 4 ARM OCPUs + 24 GB RAM (Ampere A1)
- **Storage:** 200 GB block storage + 20 GB object storage
- **Network:** 10 TB/month outbound
- **Database:** 2 Autonomous Database instances (if we outgrow SQLite)

### Deployment

```
Oracle Cloud VM (ARM A1, 1 OCPU, 6 GB RAM)
├── Docker + Docker Compose
├── carwatch container
│   ├── Telegram bot (long polling)
│   ├── Scheduler (shared scraping loop)
│   ├── Health endpoint (:8080/healthz)
│   └── React SPA + REST API (:8080/ and :8080/api/v1/)
├── SQLite database (./data/ volume)
└── systemd service (auto-restart)
```

### Backup

- Automated: `sqlite3 .backup` daily to Oracle Object Storage (free 20 GB)
- The bot is stateless beyond the DB — container rebuild from git is trivial

### Caveats

- Popular ARM regions may have quota issues — use Phoenix or Osaka
- Convert to pay-as-you-go (stays free within limits) to prevent idle reclamation
- Keep light CPU activity (the bot's polling loop naturally does this)

---

## 9. WhatsApp Roadmap (Phase 3+)

### Official WhatsApp Cloud API

| Step | Details |
|---|---|
| **Setup** | Meta Business account + phone number verification |
| **Templates** | Pre-approved message templates for car alerts (Utility category) |
| **Cost** | ~$0.005/msg (Israel, Utility). 10 alerts/day to 10 users = ~$1.50/month |
| **Integration** | Webhook-based. Receives user messages, sends template responses |

### Dual-Channel Architecture

Users choose their notification channel during `/start`:
```
Bot: How would you like to receive alerts?
     [Telegram] [WhatsApp]
```

If WhatsApp: collect phone number, send verification via WhatsApp Cloud API, link to Telegram chat_id.

The Notifier interface already supports this — just a new adapter implementing `notifier.Notifier`.

---

## 10. Implementation Plan

### Phase 1: Multi-Tenant Telegram Bot (MVP)

**Goal:** Users onboard and receive alerts via Telegram. No config file needed.

1. Migrate Telegram library: `go-telegram-bot-api` → `go-telegram/bot`
2. Add `users` and `searches` tables to SQLite
3. Implement bot command handler (`/start`, `/watch`, `/list`, `/stop`)
4. Implement conversation FSM for search wizard (inline keyboards)
5. Build Yad2 manufacturer/model lookup table (ID → name mapping)
6. Refactor scheduler: load searches from DB instead of config
7. Implement canonical search grouping (shared scraping)
8. Per-user dedup and notification
9. Admin commands: `/stats`, `/broadcast` (for bot owner only)
10. Deploy to Oracle Cloud ARM VM

### Phase 2: Polish + Reliability

11. Price drop alerts per user (configurable threshold)
12. Notification digest mode (batch alerts every N hours)
13. User rate limiting (max 3 active searches per user)
14. Search pause/resume
15. SQLite backup to Oracle Object Storage
16. Monitoring: health endpoint + simple uptime check

### Phase 3: WhatsApp Channel

17. WhatsApp Cloud API integration
18. Dual-channel user preference
19. Template message approval flow
20. Phone number verification

### Phase 4: Growth

21. Additional marketplaces (AutoTrader IL, Facebook Marketplace)
22. Inline search results (Telegram Mini App for complex filters)
23. Search sharing ("send this search to a friend")
24. Move to PostgreSQL if SQLite becomes a bottleneck

---

## 11. Project Structure (v3)

```
carwatch/
├── cmd/
│   └── bot/
│       └── main.go                 # Entry point, wiring
├── internal/
│   ├── bot/
│   │   ├── bot.go                  # Telegram bot setup, update handler
│   │   ├── commands.go             # /start, /list, /stop, /help, /settings
│   │   ├── wizard.go               # Search creation FSM
│   │   ├── keyboards.go            # Inline keyboard builders
│   │   └── middleware.go           # User registration, rate limiting
│   ├── config/
│   │   └── config.go               # App config (bot token, DB path, polling)
│   ├── fetcher/
│   │   ├── fetcher.go              # Interface + error sentinels + factory
│   │   ├── cache.go                # Caching fetcher wrapper
│   │   ├── proxy.go                # Proxy pool with health tracking
│   │   └── yad2/
│   │       ├── yad2.go             # Yad2 adapter
│   │       ├── client.go           # HTTP client
│   │       ├── parser.go           # __NEXT_DATA__ regex extraction
│   │       └── catalog.go          # Manufacturer/model ID lookup table
│   ├── filter/
│   │   └── filter.go               # Pure filter function
│   ├── model/
│   │   └── listing.go              # Domain models
│   ├── notifier/
│   │   ├── notifier.go             # Interface
│   │   ├── formatter.go            # Message formatting
│   │   └── telegram/
│   │       └── telegram.go         # Telegram send adapter
│   ├── scheduler/
│   │   ├── scheduler.go            # Shared scraping loop
│   │   └── grouper.go              # Canonical search grouping
│   ├── service/
│   │   ├── user.go                 # User CRUD
│   │   └── search.go               # Search CRUD + grouping logic
│   ├── storage/
│   │   ├── interfaces.go           # All storage interfaces
│   │   └── sqlite/
│   │       └── sqlite.go           # SQLite adapter (users, searches, dedup)
│   ├── health/
│   │   └── health.go               # Health check endpoint
│   └── spa/
│       └── spa.go                  # Embedded React SPA (see web/)
├── testdata/
├── config.example.yaml              # Minimal: bot_token, db_path, polling
├── Dockerfile
├── docker-compose.dev.yaml
├── docker-compose.prod.yaml
├── Makefile
└── go.mod
```

---

## 12. Minimal Config (v3)

The config file shrinks dramatically. User searches live in the database.

```yaml
# Bot configuration (the only config file needed)
bot_token: "${TELEGRAM_BOT_TOKEN}"
admin_chat_id: 123456789          # your Telegram user ID

polling:
  interval: 15m
  jitter: 5m
  active_hours:
    start: "08:00"
    end: "22:00"
  timezone: "Asia/Jerusalem"

storage:
  db_path: "./data/carwatch.db"

http:
  user_agents:
    - "Mozilla/5.0 ..."
  # proxy: "socks5://..."
  # proxies: [...]

log_level: info
```

---

## 13. Industry Context

### How CarWatch compares to existing projects

| Project | Language | Architecture | CarWatch Advantage |
|---|---|---|---|
| yad2-scraper | Node.js | GitHub Actions cron, JSON file dedup | Real-time polling, SQLite dedup, adaptive backoff |
| CarScoutBot | Python | Simple poll+notify, in-memory state | Atomic dedup, notify-then-mark, multi-user |
| yad2bot | Python | Telegram bot + DB, single user | Shared scraping, price tracking, conversation wizard |
| autoscout24_bot | Python | Single marketplace scraper | Multi-marketplace factory, proxy rotation |

CarWatch is already more architecturally robust than any open-source car listing bot found. The v3 evolution adds the user-facing layer these projects have (Telegram bot) while keeping the proven backend.

### Patterns borrowed from CamelCamelCamel-style systems

- **Inverted index**: Map `{manufacturer, model}` → `[]user_criteria` for efficient fan-out
- **Shared ingest, per-user filter**: Scrape once per canonical group, apply N filter sets
- **Notification debouncing**: Don't re-alert on the same listing within 24 hours
- **Price change detection**: Track `(token, price)` history, alert on drops

---

## 14. Decision Log (v3 additions)

| Decision | Rationale | Alternative Considered |
|---|---|---|
| Oracle Cloud Free Tier | 4 ARM CPUs + 24 GB RAM, free for life, persistent storage | Fly.io ($5/mo), Render (spins down), Railway ($5/mo) |
| `go-telegram/bot` library | Actively maintained (v1.20, Mar 2026), 1:1 API, official listing | `go-telegram-bot-api` (unmaintained since 2021) |
| Telegram first, WhatsApp later | Free, zero ban risk, official bot API. WhatsApp Cloud API as Phase 3 | whatsmeow (high ban risk 2026, GitHub #810) |
| SQLite for multi-tenant | Adequate for <1000 users, zero config, already in use | PostgreSQL (overkill for Phase 1) |
| Canonical search grouping | Reduces Yad2 requests from N-per-user to N-per-model | Per-user scraping (hammers Yad2, gets rate-limited) |
| Hand-rolled FSM over library | Only 7 states in the wizard, not worth a dependency | `go-telegram/fsm`, `fsm-telebot` (overkill) |
| Long polling over webhooks | Single instance, behind NAT, simpler | Webhooks (needs public URL, TLS) |
| Stay in Go over Python rewrite | Working codebase, single binary, 10 MB memory | Python aiogram (better FSM, but full rewrite) |
