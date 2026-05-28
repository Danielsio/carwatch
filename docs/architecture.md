# CarWatch Architecture Document

**Date:** 2026-05-28
**Status:** Living document — updated for multi-service production topology

---

## 1. System Overview

CarWatch is a **multi-tenant vehicle listing aggregator** for the Israeli used-car market. It continuously scrapes Yad2, deduplicates and scores listings against user-defined search criteria, and delivers real-time notifications via Telegram and Web Push. A React SPA provides a web dashboard for managing searches, browsing results, and viewing market analytics.

### Key Stats
- **Language:** Go 1.25 (backend), TypeScript/React (frontend)
- **Entry points:** 6 binaries (`api-server`, `bot-poller`, `scraper`, `notifier`, `enricher`, `catalog-gen`)
- **Internal packages:** 26
- **Database:** PostgreSQL 17 + Redis 7 (stream-based message broker)
- **Schema migrations:** 15 (versioned, up/down)
- **Reverse proxy:** Caddy 2 (automatic TLS)
- **Orchestration:** Docker Compose (9 containers in production)

---

## 2. High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLIENTS                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────────┐  │
│  │ Telegram Bot │  │  Web SPA     │  │  Admin Dashboard (SPA)    │  │
│  │  (long-poll) │  │ (React/Vite) │  │  (embedded in SPA)       │  │
│  └──────┬───────┘  └──────┬───────┘  └────────────┬──────────────┘  │
└─────────┼─────────────────┼───────────────────────┼─────────────────┘
          │                 │                       │
          │ Telegram API    │ REST + SSE            │ REST
          ▼                 ▼                       ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     APPLICATION LAYER                                │
│                                                                     │
│  ┌──────────────┐  ┌──────────────────────────────────────────────┐ │
│  │   Bot        │  │              API Server                      │ │
│  │  (Telegram   │  │  ┌────────┐ ┌──────────┐ ┌────────────────┐ │ │
│  │   handlers,  │  │  │Listings│ │ Searches │ │ Instant Search │ │ │
│  │   wizard,    │  │  ├────────┤ ├──────────┤ ├────────────────┤ │ │
│  │   callbacks) │  │  │Bookmarks│ │ Catalog │ │ Notifications  │ │ │
│  │             │  │  ├────────┤ ├──────────┤ ├────────────────┤ │ │
│  │             │  │  │ Push   │ │ Admin   │ │ Log Stream(SSE)│ │ │
│  │             │  │  ├────────┤ ├──────────┤ ├────────────────┤ │ │
│  │             │  │  │Firebase│ │Rate Limit│ │  Metrics       │ │ │
│  │             │  │  │  Auth  │ │(per user)│ │  (Vitals)      │ │ │
│  └──────┬──────┘  │  └────────┘ └──────────┘ └────────────────┘ │ │
│         │         └──────────────────┬───────────────────────────┘ │
│         │                            │                             │
│         ▼                            ▼                             │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    SCHEDULER                                 │   │
│  │  ┌──────────┐  ┌────────────┐  ┌──────────┐  ┌──────────┐  │   │
│  │  │ Polling  │  │ Processing │  │ Delivery │  │Maintenance│  │   │
│  │  │ Loop     │  │ (pipeline) │  │ (notify) │  │ (prune,  │  │   │
│  │  │ (jitter, │  │            │  │          │  │  vacuum,  │  │   │
│  │  │ backoff) │  │            │  │          │  │  market)  │  │   │
│  │  └──────────┘  └────────────┘  └──────────┘  └──────────┘  │   │
│  └─────────────────────────┬───────────────────────────────────┘   │
└────────────────────────────┼───────────────────────────────────────┘
                             │
┌────────────────────────────┼───────────────────────────────────────┐
│                    DOMAIN / BUSINESS LOGIC                          │
│                            │                                       │
│  ┌─────────────────────────▼─────────────────────────────────────┐ │
│  │                  Listing Pipeline                              │ │
│  │  1. Prefill from DB  →  2. Fitness Score  →  3. Deal Score    │ │
│  │  4. Suspicious Detection  →  5. Base Price Enrichment         │ │
│  │  6. Build ListingRecords                                      │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                    │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────┐  │
│  │  Filter   │  │  Scoring  │  │ Pricelist │  │   Catalog     │  │
│  │ (criteria │  │(market    │  │ (Yad2 base│  │ (mfr/model    │  │
│  │  matching)│  │ cache,    │  │  price    │  │  directory,   │  │
│  │           │  │ fitness,  │  │  lookup,  │  │  fuzzy search,│  │
│  │           │  │ suspicious│  │  7-day    │  │  static +     │  │
│  │           │  │ detection)│  │  cache)   │  │  dynamic)     │  │
│  └───────────┘  └───────────┘  └───────────┘  └───────────────┘  │
│                                                                    │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐                     │
│  │  Locale   │  │  Format   │  │  TimeUtil │                     │
│  │ (HE/EN   │  │ (numbers, │  │ (timezone,│                     │
│  │  i18n)    │  │  markdown)│  │  duration)│                     │
│  └───────────┘  └───────────┘  └───────────┘                     │
└────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────┐
│                    INFRASTRUCTURE / ADAPTERS                        │
│                                                                    │
│  ┌─── Fetchers ─────────────────────────────────────────────────┐  │
│  │  ┌────────────────────────────────────────────────────────┐  │  │
│  │  │              Fetcher Factory (registry)                │  │  │
│  │  │  "yad2" → CircuitBreaker → Cache → Paginator → Yad2   │  │  │
│  │  └────────────────────────────────────────────────────────┘  │  │
│  │                                                              │  │
│  │  ┌──── Yad2 Adapter ──────┐                                  │  │
│  │  │ Client (HTTP, UA rot., │                                  │  │
│  │  │   SOCKS5 proxy pool,   │                                  │  │
│  │  │   cookie isolation)    │                                  │  │
│  │  │ Parser (__NEXT_DATA__) │                                  │  │
│  │  │ ItemParser (specs)     │                                  │  │
│  │  │ Enricher (km/city)     │                                  │  │
│  │  │ URL builder (ranges)   │                                  │  │
│  │  └────────────────────────┘                                  │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                    │
│  ┌─── Notifiers ────────────────────────────────────────────────┐  │
│  │  ┌────────────────────────────────────────────────────────┐  │  │
│  │  │          MultiNotifier (fan-out to channels)           │  │  │
│  │  │  ┌───────────────────┐  ┌────────────────────────┐    │  │  │
│  │  │  │ Telegram Notifier │  │ WebPush Notifier       │    │  │  │
│  │  │  │ (rate-limited,    │  │ (VAPID auth,           │    │  │  │
│  │  │  │  blocked-user     │  │  endpoint validation)  │    │  │  │
│  │  │  │  detection)       │  │                        │    │  │  │
│  │  │  └───────────────────┘  └────────────────────────┘    │  │  │
│  │  └────────────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                    │
│  ┌─── Storage ──────────────────────────────────────────────────┐  │
│  │  ┌──────────────────────────────────────────────────────┐    │  │
│  │  │           Store Interface (composed)                  │    │  │
│  │  │  UserStore + SearchStore + ListingStore + DedupStore  │    │  │
│  │  │  + SavedListingStore + HiddenListingStore             │    │  │
│  │  │  + NotificationQueue + PriceTracker + DigestStore     │    │  │
│  │  │  + PriceListStore + MarketStore + DailyDigestStore    │    │  │
│  │  │  + LinkTokenStore + PushSubscriptionStore + AdminStore│    │  │
│  │  │  + NotificationStore                                  │    │  │
│  │  └──────────────────────────────┬──────────────────────────┘    │  │
│  │                                │                               │  │
│  │                      ┌─────────▼─────────────┐                │  │
│  │                      │      PostgreSQL        │                │  │
│  │                      │  (pgx/v5, conn pool,   │                │  │
│  │                      │   migrations)          │                │  │
│  │                      └───────────────────────┘                │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                    │
│  ┌─── Supporting ───────────────────────────────────────────────┐  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │  │
│  │  │   Health     │  │  LogStream   │  │   SPA Server     │   │  │
│  │  │ (uptime,     │  │ (pub-sub hub,│  │ (go:embed,       │   │  │
│  │  │  cycle count,│  │  ring buffer,│  │  CSP headers,    │   │  │
│  │  │  degraded    │  │  SSE handler)│  │  history-mode    │   │  │
│  │  │  detection)  │  │              │  │  fallback)       │   │  │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘   │  │
│  └──────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────┐
│                    EXTERNAL SERVICES                                │
│  ┌──────────┐  ┌─────────────┐  ┌────────────┐                    │
│  │ Yad2.co.il│  │ Firebase    │  │  Telegram  │                    │
│  │ (listings,│  │ Auth        │  │  Bot API   │                    │
│  │  catalog, │  │             │  │            │                    │
│  │  pricing) │  │             │  │            │                    │
│  └──────────┘  └─────────────┘  └────────────┘                    │
└────────────────────────────────────────────────────────────────────┘
```

---

## 3. Component Deep Dives

### 3.1 Entry Points (`cmd/`)

| Binary | Purpose | Lifecycle |
|--------|---------|-----------|
| `cmd/bot/main.go` | Primary application: wires all components, runs scheduler + bot + API server | Long-running daemon |
| `cmd/catalog-gen/main.go` | Scrapes Yad2 manufacturer/model catalog into JSON | One-shot CLI utility |

**`cmd/bot/main.go` initialization order:**
1. Parse flags & load YAML config (with `${ENV_VAR}` interpolation)
2. Open storage (PostgreSQL)
3. Build fetcher chain: `Yad2 → Paginator → Cache(5m TTL) → CircuitBreaker`
4. Load dynamic catalog from Yad2 HTML
5. Initialize health tracker
6. Build Telegram bot + multi-notifier (Telegram + optional WebPush)
7. Build API server with Firebase auth
8. Build HTTP server (mux: `/healthz`, `/api/v1/*`, `/` → SPA)
9. Start Telegram long-polling goroutine (with exponential backoff on disconnect)
10. Run scheduler (blocking)

---

### 3.2 Scheduler

The scheduler is the core engine. It runs a continuous polling loop that fetches, filters, deduplicates, scores, and delivers listings.

**Polling cycle:**
```
┌─────────────────────────────────────────────────────────┐
│                    Scheduler.Run()                       │
│                                                         │
│  ┌─── Every interval ± jitter ───────────────────────┐  │
│  │                                                    │  │
│  │  1. Check active hours (08:00-22:00 Asia/Jerusalem)│  │
│  │  2. List all active searches                       │  │
│  │  3. For each search (up to N concurrent):          │  │
│  │     ┌────────────────────────────────────────────┐  │  │
│  │     │ a. Fetch raw listings (via fetcher chain)  │  │  │
│  │     │ b. Filter by search criteria               │  │  │
│  │     │ c. Dedup (INSERT OR IGNORE + RowsAffected) │  │  │
│  │     │ d. Run listing pipeline (score & enrich)   │  │  │
│  │     │ e. Save to DB                              │  │  │
│  │     │ f. Enqueue notifications                   │  │  │
│  │     └────────────────────────────────────────────┘  │  │
│  │  4. Deliver notifications (worker pool)            │  │
│  │  5. Flush catalog updates                          │  │
│  │  6. Update observer (health metrics)               │  │
│  │                                                    │  │
│  │  Adaptive backoff:                                 │  │
│  │    On challenge/rate-limit: multiplier *= 2 (max 4)│  │
│  │    On success: multiplier *= 0.5 (floor 1)         │  │
│  └────────────────────────────────────────────────────┘  │
│                                                         │
│  ┌─── Every 24h ─────────────────────────────────────┐  │
│  │  Maintenance:                                      │  │
│  │  - Prune listings > 90 days                        │  │
│  │  - Prune price history > 90 days                   │  │
│  │  - Prune notifications > 48 hours                  │  │
│  │  - Rebuild market cache                            │  │
│  │  - Sync user active status                         │  │
│  │  - Vacuum DB                                       │  │
│  └────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**Key parameters:**
- Default interval: 15 minutes
- Default jitter: +/-5 minutes
- Fetch timeout: 60 seconds per search
- KM enrichment timeout: 25 minutes
- Max concurrent fetches: 4
- Market cache TTL: 30 minutes

---

### 3.3 Listing Pipeline

Shared by both the scheduler and the API instant-search endpoint, ensuring consistent scoring regardless of entry point.

```
Raw Listings (from fetcher)
        │
        ▼
┌─────────────────────────────────────────────┐
│  Step 1: Prefill from DB                     │
│  - LookupEnrichmentData(tokens)              │
│  - Fill missing km, city, imageURL           │
│  - Skip if ListingStore is nil               │
└─────────────────────┬───────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│  Step 2: Fitness Score (per listing)          │
│  - Weighted dimensions:                      │
│    price, km, hand, year, engine             │
│  - Each dimension: how close to ideal?       │
│  - Uses market median if available           │
│  - Output: 0.0 - 1.0 score + breakdown      │
└─────────────────────┬───────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│  Step 3: Deal Score (if market cache avail)   │
│  - Lookup: (manufacturer, model, year)       │
│    → median price, median km, cohort size    │
│  - Minimum cohort: 10 listings               │
│  - Price floor: 5000 NIS (filter junk)       │
│  - ScoreWithKm: price + km vs medians        │
│  - Output: integer score + market context    │
└─────────────────────┬───────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│  Step 4: Suspicious Detection                │
│  - Price outlier vs market median            │
│  - Unusual spec combinations                 │
│  - Output: []string reasons                  │
└─────────────────────┬───────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│  Step 5: Base Price Enrichment               │
│  - Lookup via Pricelist service              │
│  - Requires subModelID + year               │
│  - Cache: 7 days, rate-limited (20/cycle)    │
│  - Source: Yad2 pricing API                  │
│  - Output: base (catalog) price             │
└─────────────────────┬───────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────┐
│  Step 6: Build ListingRecord                 │
│  - Map Listing → storage.ListingRecord      │
│  - Attach scores, timestamps, metadata      │
└─────────────────────────────────────────────┘
```

---

### 3.4 Fetcher Chain

Each marketplace source is wrapped in a composable middleware chain:

```
                    Fetcher Interface
                    Fetch(ctx, SourceParams) → ([]RawListing, error)
                           │
                  ┌────────▼────────┐
                  │ CircuitBreaker   │
                  │ (5 failures →    │
                  │  10min cooldown) │
                  └────────┬────────┘
                           │
                  ┌────────▼────────┐
                  │ CachingFetcher   │
                  │ (5-min TTL,      │
                  │  in-memory)      │
                  └────────┬────────┘
                           │
                  ┌────────▼────────┐
                  │ PaginatingFetcher│
                  │ (maxPages config,│
                  │  per-page timeout)│
                  └────────┬────────┘
                           │
                  ┌────────▼────────┐
                  │   Yad2Fetcher    │
                  │                  │
                  │ Client:          │
                  │  - UA rotation   │
                  │  - SOCKS5 proxy  │
                  │  - Proxy pool    │
                  │  - Cookie jar    │
                  │                  │
                  │ Parser:          │
                  │  - __NEXT_DATA__ │
                  │  - JSON extract  │
                  │                  │
                  │ Enricher:        │
                  │  - Item page     │
                  │  - km/city fill  │
                  └──────────────────┘
```

**Error sentinels:**
- `ErrChallenge`: Anti-bot challenge detected (triggers backoff)
- `ErrRateLimited`: Too many requests (triggers backoff)

---

### 3.5 Storage Layer

#### Interface Composition

The `Store` interface composes 16 sub-interfaces, each representing a bounded persistence concern:

```
Store (composed interface)
├── UserStore           — user CRUD, auth, tier management
├── SearchStore         — search CRUD, activation, sharing
├── ListingStore        — listing persistence, querying, stats
├── SavedListingStore   — bookmarks
├── HiddenListingStore  — user-hidden listings
├── DedupStore          — atomic deduplication
├── NotificationQueue   — pending notification queue
├── PriceTracker        — price time-series (record, revert, prune)
├── DigestStore         — digest mode config & buffering
├── PriceListStore      — vehicle base price cache
├── MarketStore         — market listings for cohort scoring
├── DailyDigestStore    — daily digest scheduling
├── LinkTokenStore      — web-to-Telegram linking tokens
├── PushSubscriptionStore — WebPush subscriptions
├── AdminStore          — admin reporting, purging, metrics
├── NotificationStore   — per-user listing seen tracking
├── Close() error       — lifecycle
└── Migrate() error     — schema migration
```

#### Database Schema (PostgreSQL)

```sql
┌─────────────────────────────────────────────────────────┐
│                    users                                 │
│  chat_id (PK) | username | state | state_data           │
│  created_at | active | language | tier | tier_expires    │
│  trial_used | channel | channel_id | last_seen_at       │
└────────┬────────────────────────────────────────────────┘
         │ 1:N
         ▼
┌─────────────────────────────────────────────────────────┐
│                    searches                              │
│  id (PK) | chat_id (FK→users) | user_seq | name        │
│  source | manufacturer | model | year_min | year_max     │
│  price_min | price_max | engine_min_cc | max_km | max_hand│
│  keywords | exclude_keys | seller_filter | gearbox      │
│  price_only | photo_only | active | created_at           │
│  share_token                                             │
└────────┬────────────────────────────────────────────────┘
         │ 1:N
         ▼
┌─────────────────────────────────────────────────────────┐
│                  listing_history                         │
│  token (PK) | chat_id (FK) | search_id (FK)             │
│  search_name | manufacturer | model | sub_model          │
│  sub_model_id | year | price | km | hand | city          │
│  page_link | image_url | engine_volume | horse_power     │
│  engine_type | gearbox | description | is_commercial     │
│  fitness_score | median_price | cohort_size | deal_score  │
│  base_price | first_seen_at | removed_at                 │
└─────────────────────────────────────────────────────────┘

┌──────────────────────────┐  ┌──────────────────────────┐
│      seen_listings       │  │   listing_user_seen      │
│  token | chat_id         │  │  chat_id | token          │
│  search_id | first_seen  │  │  seen_at                 │
│  (dedup: UNIQUE)         │  │  (per-user "new" feed)   │
└──────────────────────────┘  └──────────────────────────┘

┌──────────────────────────┐  ┌──────────────────────────┐
│     price_history        │  │   pending_notifications  │
│  id | token | price      │  │  id | recipient           │
│  observed_at             │  │  search_name | payload    │
│  (time-series)           │  │  created_at              │
└──────────────────────────┘  └──────────────────────────┘

┌──────────────────────────┐  ┌──────────────────────────┐
│     price_list_cache     │  │   push_subscriptions     │
│  sub_model_id | year     │  │  id | chat_id             │
│  base_price | title      │  │  endpoint | p256dh | auth │
│  fetched_at              │  │  created_at              │
└──────────────────────────┘  └──────────────────────────┘
```

---

### 3.6 Notification System

```
┌───────────────────────────────────────────────────────┐
│                MultiNotifier                           │
│  - Registers named channels                           │
│  - Fan-out: sends to all channels where the user      │
│    has an active subscription                          │
│  - Connect/Disconnect lifecycle                       │
│                                                       │
│    ┌─────────────────┐    ┌─────────────────────┐     │
│    │ Telegram         │    │ WebPush              │     │
│    │ - Formats listing│    │ - VAPID key signing  │     │
│    │   as rich message│    │ - Endpoint validation│     │
│    │ - Rate limiting  │    │ - JSON payload       │     │
│    │ - ErrRecipient-  │    │ - PushSubscription-  │     │
│    │   Blocked detect │    │   Store integration  │     │
│    │ - Markdown escape│    │                      │     │
│    └─────────────────┘    └─────────────────────┘     │
└───────────────────────────────────────────────────────┘
```

**Notification flow:**
1. Scheduler finds new listings for a user's search
2. Formats listing into notification payload
3. MultiNotifier iterates registered channels
4. Each channel sends via its transport (Telegram API / WebPush)
5. On blocked user: marks user inactive, stops future notifications

---

### 3.7 API Server

**Authentication:** Firebase ID tokens via `Authorization: Bearer <token>`.
Web users are created/linked via `UpsertWebUser(firebaseUID, email)`.

**Rate limiting (token bucket):**
- Authenticated users: 10 req/s
- Per-IP (unauthenticated): 50 req/s
- Guest: 5 req/s

**Endpoints (under `/api/v1/`):**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Status + metrics |
| GET | `/listings` | Paginated, filtered, sorted listings |
| GET | `/listings/:token` | Single listing detail |
| GET | `/searches` | List user's searches |
| POST | `/searches` | Create search |
| PUT | `/searches/:id` | Update search |
| DELETE | `/searches/:id` | Delete search |
| GET | `/catalog/manufacturers` | Fuzzy search manufacturers |
| GET | `/catalog/models` | Models for a manufacturer |
| POST | `/instant-search` | Live preview (runs full pipeline) |
| GET | `/bookmarks` | List saved listings |
| POST | `/bookmarks` | Save a listing |
| DELETE | `/bookmarks/:token` | Remove bookmark |
| GET | `/notifications/center` | Notification history |
| POST | `/notifications/seen` | Mark listing as seen |
| POST | `/push/subscribe` | Register WebPush subscription |
| DELETE | `/push/subscribe` | Unregister |
| GET | `/logs/stream` | Real-time log stream (SSE) |
| GET | `/admin/*` | Admin dashboard data |
| GET | `/vitals` | Web vitals reporting |

---

### 3.8 Telegram Bot

**Architecture:** Long-polling with exponential backoff on disconnect (1s → 30s cap).

**Command set:**
- `/start` — Welcome + language selection
- `/watch` — Interactive search wizard (state machine)
- `/list` — Show active searches
- `/stop` — Deactivate/delete a search
- `/saved` — View bookmarked listings
- `/help` — Usage guide
- `/settings` — User preferences (language, digest mode)
- `/link` — Generate token to link Telegram-to-Web accounts

**Wizard state machine:**
```
idle → ask_source → ask_manufacturer → ask_model → ask_year_min
→ ask_year_max → ask_price_min → ask_price_max → ask_max_km
→ ask_max_hand → ask_gearbox → ask_keywords → confirm
```
Each state supports inline keyboards for selection, free-text input for ranges, and "skip" to use defaults.

---

### 3.9 Web Frontend (React SPA)

**Stack:** React 18 + TypeScript + Vite + Tailwind CSS

**Pages:**
| Page | Description |
|------|-------------|
| `LandingPage` | Marketing/onboarding |
| `LoginPage` / `SignupPage` / `AuthPage` | Firebase authentication |
| `SearchesPage` | List/manage active searches |
| `NewSearchPage` / `EditSearchPage` | Search wizard form |
| `TrySearchPage` | Instant preview without saving |
| `ListingsPage` | Browse results for a search |
| `ListingDetailPage` | Full listing with price chart |
| `SavedPage` | Bookmarked listings |
| `NotificationsPage` | Notification center |
| `SettingsPage` | User preferences |
| `HistoryPage` | Historical listings |
| `AdminPage` | Admin dashboard (logs, metrics) |

**Key hooks:**
- `useListings` / `useInfiniteListings` — paginated listing fetches
- `useSearches` — search CRUD
- `useCatalog` — manufacturer/model lookup
- `useBookmarks` — saved listings
- `useNotifications` — notification center
- `usePushSubscription` — WebPush registration
- `useAdmin` — admin data
- `useLogStream` — SSE log streaming
- `useHealthCheck` — health polling

**Contexts:**
- `AuthContext` — Firebase auth state, token management
- `ThemeContext` — Light/dark mode

**Serving:** Go `embed.FS` via `spa.Handler()` with CSP headers and history-mode fallback routing.

---

## 4. Data Flow Diagrams

### 4.1 New User Registration (Telegram)

```
User sends /start to Bot
        │
        ▼
Bot.DefaultHandler()
        │
        ▼
UpsertUser(chatID, username)  ── creates user in DB
        │
        ▼
Send welcome message with language keyboard
        │
        ▼
User selects language → SetUserLanguage(chatID, lang)
```

### 4.2 Search Creation (Web)

```
User fills search form → POST /api/v1/searches
        │
        ▼
Firebase auth middleware → verify token → get chatID
        │
        ▼
Validate fields → CreateSearch(ctx, Search{...})
        │
        ▼
Search stored in DB → scheduler picks it up on next cycle
```

### 4.3 Listing Discovery Cycle

```
Scheduler tick (every 15m +/- 5m)
        │
        ▼
ListAllActiveSearches() → iterate each
        │
        ▼
FetcherFactory.Get(source) → CircuitBreaker → Cache → Source
        │
        ▼
filter.Apply(rawListings, search.FilterCriteria)
        │
        ▼
For each listing: ClaimNew(token, chatID, searchID)
  │ new=true                          │ new=false
  ▼                                   ▼
Pipeline.Process()                  (skip, already seen)
  │
  ▼
RecordPrice(token, price)
  │ changed=true                      │ changed=false
  │ (price drop detected)             │
  ▼                                   ▼
SaveListing(record)               SaveListing(record)
  │
  ▼
EnqueueNotification(chatID, payload)
  │
  ▼
DeliveryWorker → MultiNotifier.Notify(chatID, listings, lang)
  │
  ├──→ Telegram: formatted message with specs
  └──→ WebPush: JSON push notification
```

### 4.4 Instant Search (Web)

```
User fills form → POST /api/v1/instant-search
        │
        ▼
Validate → build SourceParams from form
        │
        ▼
FetcherFactory.Get(source).Fetch(ctx, params)
        │
        ▼
filter.Apply(rawListings, criteria)
        │
        ▼
Pipeline.Process(ctx, filtered, ProcessParams{...})
        │
        ▼
Return scored listings as JSON (not saved to DB)
```

---

## 5. Configuration

```yaml
polling:
  interval: 15m                    # base polling interval
  jitter: 5m                       # random +/- jitter
  timezone: Asia/Jerusalem
  max_concurrent_fetches: 4
  active_hours:
    start: "08:00"
    end: "22:00"

telegram:
  token: ${TELEGRAM_BOT_TOKEN}
  admin_chat_id: 123456789
  max_searches: 10
  bot_username: carwatch_bot

storage:
  driver: postgres
  dsn: ${DATABASE_URL}
  migrations_path: ./migrations
  prune_after: 720h                # 30 days

http:
  bind: 0.0.0.0:8080
  user_agents: [...]               # rotated per request
  proxies: [socks5://...]          # SOCKS5 proxy pool
  max_pages: 5

api:
  cors_origins: [https://carwatch.app]
  trust_forwarded_for: true        # behind reverse proxy
  admin_email: admin@example.com

firebase:
  project_id: carwatch-prod
  credentials_json: ${FIREBASE_CREDS}

push:
  vapid_public_key: ${VAPID_PUBLIC}
  vapid_private_key: ${VAPID_PRIVATE}
  vapid_subject: mailto:admin@carwatch.app

log_level: info                    # debug | info | warn | error
log_format: auto                   # auto | json | pretty
```

---

## 6. Deployment Architecture

```
┌──────────────────────────────────────────────────────┐
│               Oracle Cloud VM                         │
│                                                      │
│  ┌─────────────┐  ┌───────────────┐                  │
│  │   Caddy      │  │  CarWatch     │                  │
│  │ (HTTPS       │──│  (single      │                  │
│  │  termination,│  │   binary,     │                  │
│  │  reverse     │  │   port 8080)  │                  │
│  │  proxy)      │  │               │                  │
│  └─────────────┘  └───────┬───────┘                  │
│                           │                          │
│                  ┌────────▼────────┐                  │
│                  │   PostgreSQL    │                  │
│                  │   (local)       │                  │
│                  └─────────────────┘                  │
│                                                      │
│  Managed via Makefile targets:                        │
│  - make deploy    (build + scp + restart)             │
│  - make ssh       (interactive shell)                 │
│  - make logs      (journalctl follow)                 │
│  - make backup    (pg_dump + download)                │
└──────────────────────────────────────────────────────┘
```

**Build pipeline:**
- Docker multi-stage build (Go compile → minimal image)
- Build flags inject version, commit, timestamp
- CI runs: lint (`golangci-lint`), test, build, secret scanning

---

## 7. Scoring Algorithms

### 7.1 Fitness Score

Multi-dimensional weighted score (0.0 to 1.0) measuring how well a listing matches the user's search criteria:

| Dimension | Weight | What it measures |
|-----------|--------|------------------|
| Price | High | Distance from budget cap (or market median if available) |
| Km | Medium | Mileage relative to max_km |
| Hand | Medium | Ownership count relative to max_hand |
| Year | Medium | Year within min-max range |
| Engine | Low | Engine CC relative to minimum |

### 7.2 Deal Score

Market-relative score combining price and mileage position vs. cohort:

1. Build market cache: `(manufacturer, model, year)` → `(median_price, median_km, cohort_size)`
2. Minimum cohort: 10 listings, price floor: 5,000 NIS
3. `ScoreWithKm(price, km, medianPrice, medianKm)` → integer score
4. Higher = better deal relative to comparable vehicles

### 7.3 Suspicious Detection

Flags listings with unusual characteristics:
- Price significantly below market median
- Unusual spec combinations vs. cohort norms

---

## 8. Resilience Patterns

| Pattern | Implementation | Where |
|---------|---------------|-------|
| Circuit Breaker | 5 consecutive failures → 10-min cooldown | Per-source fetcher |
| Adaptive Backoff | Multiplier 1x→4x on challenge, halves on success | Scheduler polling |
| Response Cache | 5-minute TTL, in-memory | Per-source fetcher |
| Proxy Pool | SOCKS5 rotation across requests | Yad2 client |
| User-Agent Rotation | Random selection per request | HTTP clients |
| Cookie Isolation | Separate jar for item-page fetches | Yad2 client pool |
| Price Revert | Undo RecordPrice on downstream failure | PriceTracker |
| Telegram Reconnect | Exponential backoff (1s→30s) on poll disconnect | Bot goroutine |
| Rate Limiting | Token bucket (per-user, per-IP, guest tiers) | API middleware |
| Active Hours | Skip polling outside configured window | Scheduler |
| Dedup Atomicity | `INSERT OR IGNORE` + `RowsAffected` check | DedupStore |
| Singleflight | Prevents thundering herd on market cache rebuild | MarketCache |

---

## 9. Observability

| Signal | Implementation |
|--------|---------------|
| Health endpoint | `/healthz` — status (ok/starting/degraded), uptime, cycle count, user/search counts, DB size |
| Structured logging | `slog` with JSON (prod) or colored (dev) output |
| Log streaming | SSE endpoint `/api/v1/logs/stream` — filtered by component (yad2, scheduler, enricher, bot, telegram, notifier, circuit_breaker, api-pricelist) |
| Web Vitals | Client-side CLS/FID/LCP reporting to `/api/v1/vitals` |
| Admin metrics | Table sizes, activity stats (new listings, price drops, new users per day), DB pool stats |
| Access logging | HTTP request logging with method, path, status, duration |

---

## 10. Security

| Area | Implementation |
|------|---------------|
| Authentication (web) | Firebase ID token verification |
| Authentication (bot) | Telegram chat_id (implicit, platform-enforced) |
| Authorization | Per-user data isolation via `chatID` in all queries |
| CORS | Configurable origins, scheme+host only (no path/query) |
| CSP | Content-Security-Policy headers on SPA responses |
| Secret management | Environment variable interpolation (`${VAR}`), hardcoded-secret warning at startup |
| Rate limiting | Token bucket at API layer (per-user + per-IP + guest) |
| Input validation | Config validation at startup, search filter bounds at API layer |
| Proxy authentication | SOCKS5 for scraping traffic |
| Account linking | One-time tokens with expiration for Telegram-to-Web linking |
| Docker | Non-root user (UID 1000) |
| Response limits | Body capped at 10 MB (prevents OOM) |
| Timeouts | Per-search 60s fetch timeout, HTTP server read/write/idle timeouts |

---

## 11. Key Design Decisions

| Decision | Rationale | Trade-off |
|----------|-----------|-----------|
| Hexagonal architecture (ports & adapters) | Decouples domain from infra; easy to add sources/notifiers | More interfaces and indirection |
| Embedded SPA (`go:embed`) | Single binary deployment, no separate web server needed | Larger binary, requires rebuild on frontend changes |
| Polling over WebSocket (for scraping) | Marketplaces don't offer real-time APIs | 15-minute latency on new listings |
| Multi-notifier fan-out | Users can receive on multiple channels simultaneously | Complexity in delivery tracking |
| Shared listing pipeline | Consistent scoring between scheduler and API instant-search | Pipeline changes affect both paths |
| In-memory market cache | Fast scoring without DB queries per listing | Stale for up to 30 minutes |
| Per-user dedup | Same listing can match multiple users' searches independently | Storage overhead for per-user seen set |
| Singleflight for market lookups | Prevents thundering herd on cache rebuild | Serializes initial lookups |
| Firebase Auth for web | Offloads auth complexity (email/password, OAuth) to managed service | Vendor dependency, requires Google Cloud project |
| Single-process monolith | Simple deployment and operations for a personal project | All components share one failure domain |
| Conventional commits for versioning | CI auto-bumps version on merge; no manual version management | Requires discipline in commit message format |

---

## 12. Dependency Map

### External Dependencies (Go)
| Dependency | Purpose |
|-----------|---------|
| `firebase.google.com/go/v4` | Firebase Auth token verification |
| `github.com/go-telegram/bot` | Telegram Bot API client |
| `github.com/jackc/pgx/v5` | PostgreSQL driver (pure Go) |
| `github.com/golang-migrate/migrate/v4` | Schema migration runner |
| `github.com/SherClockHolmes/webpush-go` | Web Push (VAPID) |
| `github.com/lmittmann/tint` | Colored log output |
| `golang.org/x/net` | HTML parsing (goquery) |
| `golang.org/x/sync` | errgroup, singleflight |
| `gopkg.in/yaml.v3` | Config parsing |

### Frontend Dependencies
| Dependency | Purpose |
|-----------|---------|
| React 18 | UI framework |
| Vite | Build tool + dev server |
| Tailwind CSS | Utility-first styling |
| Firebase SDK | Authentication |
| TypeScript | Type safety |

---

## 13. Areas for Reviewer Consideration

These are areas where the reviewer's feedback would be most valuable:

1. **Store interface granularity** — 16 sub-interfaces composed into one `Store`. Is this the right decomposition, or should some be merged/split differently?

2. **Scraping resilience** — Circuit breaker + cache + proxy rotation + UA rotation + cookie isolation. Are there gaps in the anti-detection strategy?

3. **Single-binary monolith** — Everything in one process (scheduler, bot, API, SPA). At what scale or failure mode would this need to be split?

4. **Market cache staleness** — 30-minute TTL for the market cache used in deal scoring. What is the impact on scoring accuracy vs. database load?

5. **Notification delivery guarantees** — Current queue is DB-backed with ack/prune. Is this sufficient, or should a proper message broker be considered?

6. **Frontend architecture** — Custom hooks + contexts vs. a state management library (e.g., Zustand, TanStack Query). Scaling implications as the app grows?

7. **Testing strategy** — Unit + integration tests exist. Missing: API contract tests, load/stress tests for the scheduler, end-to-end browser tests.

8. **Monitoring gaps** — No metrics export to external systems (Prometheus/Grafana), no structured alerting beyond the health endpoint. Sufficient for current scale?

9. **Data retention** — 90-day automatic prune for listings and price history. Is this the right retention period for users who want long-term market trend analysis?
