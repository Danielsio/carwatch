# CarWatch — Logging & Data Flow Reference

> Complete observability map of all 4 services, their log output, and the data flow from startup through scrape cycle to notification delivery.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Oracle Cloud VM                              │
│                                                                     │
│  ┌──────────┐  ┌───────────┐  ┌───────────┐  ┌──────────────┐     │
│  │api-server│  │  scraper  │  │bot-poller │  │   notifier   │     │
│  │  :8080   │  │  :8081    │  │  :8082    │  │   :8083      │     │
│  └────┬─────┘  └─────┬─────┘  └─────┬─────┘  └──────┬───────┘     │
│       │              │              │               │              │
│       │    ┌─────────┴──────────────┴───────────────┘              │
│       │    │         Redis pub/sub (carwatch:logs)                  │
│       │    │         Redis stream  (carwatch:alerts)                │
│       ▼    ▼                                                        │
│  ┌─────────────┐              ┌────────────┐                       │
│  │ PostgreSQL  │              │   Redis    │                       │
│  │   :5432     │              │   :6379    │                       │
│  └─────────────┘              └────────────┘                       │
│                                                                     │
│  ┌─────────┐                                                        │
│  │  Caddy  │ ← HTTPS reverse proxy → api-server                   │
│  │ :80/443 │                                                        │
│  └─────────┘                                                        │
└─────────────────────────────────────────────────────────────────────┘
```

### Log Streaming Architecture

```
scraper  ──┐
bot-poller ┤── Redis pub/sub ──→ api-server ──→ SSE ──→ Admin LogsTab
notifier  ─┘                         ↑
                              local TeeHandler
                              (api-server's own logs)
```

All binaries use `slog` with a `TeeHandler` that captures logs by component name. The scraper/bot-poller/notifier publish to Redis pub/sub (`carwatch:logs`). The api-server subscribes and merges them with its own logs into the SSE stream for the admin dashboard.

---

## 1. Startup Sequence

### api-server

```
INFO  "config loaded"                    log_level=info log_format=auto version=9.5.1
INFO  "subscribed to cross-service log stream"  redis=carwatch-redis:6379
INFO  "telegram bot connected"           component=telegram username=carwatch_il_bot
INFO  "webpush notifier registered"
INFO  "dynamic catalog loaded"
INFO  "api-server started"               health=http://0.0.0.0:8080/healthz
```

### scraper

```
INFO  "config loaded"                    log_level=debug log_format=auto version=9.5.1
INFO  "redis publisher enabled"          addr=carwatch-redis:6379
INFO  "telegram bot connected"           component=telegram username=carwatch_il_bot
INFO  "dynamic catalog loaded"
INFO  "scraper started"                  health=http://0.0.0.0:8081/healthz
INFO  "scheduler started"               check_interval=15m0s jitter=5m0s
```

### bot-poller

```
INFO  "config loaded"                    log_level=debug log_format=auto version=9.5.1
INFO  "telegram bot connected"           component=telegram username=carwatch_il_bot
INFO  "dynamic catalog loaded"
INFO  "bot-poller started"              health=http://0.0.0.0:8082/healthz
INFO  "telegram bot polling loop starting"
```

### notifier

```
INFO  "config loaded"                    log_level=debug log_format=auto version=9.5.1
INFO  "telegram bot connected"           component=telegram username=carwatch_il_bot
INFO  "notifier worker started"          redis=carwatch-redis:6379 health=http://0.0.0.0:8083/healthz
```

---

## 2. Scraper Cycle (runs every ~15min)

This is the core data pipeline. Each step is numbered.

```
┌──────────────────────────────────────────────────────────────────────┐
│                     SCAN CYCLE #N                                    │
│                                                                      │
│  ① Check for listings ─→ ② Load searches ─→ ③ Prune old data       │
│       │                                                              │
│  ④ Load market cache ─→ ⑤ Fetch global feed ─→ ⑥ Catalog ingest    │
│       │                                                              │
│  ⑦ Prefill from DB ─→ ⑧ KM enrichment ─→ ⑨ Percolator match       │
│       │                                                              │
│  ⑩ Per match: dedup → price drop → pipeline → persist               │
│       │                                                              │
│  ⑪ Deliver notifications ─→ ⑫ Process digests                      │
│       │                                                              │
│  ⑬ Backfill unenriched ─→ ⑭ Write cycle log ─→ ⑮ Scan complete    │
└──────────────────────────────────────────────────────────────────────┘
```

### Step-by-step logs

#### ① Cycle start
```
INFO  "checking for new listings"        component=scheduler scan=5
```

#### ② Load active searches
```
INFO  "active searches loaded"           component=scheduler scan=5 searches=8 db_duration_ms=12
```

#### ③ Prune old data (every 24h)
```
INFO  "pruned old dedup entries"         component=scheduler count=150 retention=720h0m0s
INFO  "pruned old price history"         component=scheduler count=42 retention=2160h0m0s
INFO  "pruned old listing history"       component=scheduler count=89 retention=2160h0m0s
```

#### ④ Build market cache
```
DEBUG "market medians loaded"            component=scheduler rows=340 duration_ms=85
```
Or if cached:
```
DEBUG "reusing cached market data"       component=scheduler age=12m30s
```

#### ⑤ Fetch global Yad2 feed
```
DEBUG "fetching listings"                component=yad2 url=https://www.yad2.co.il/vehicles/cars?Order=1
INFO  "global feed fetched"              component=scheduler scan=5 source=yad2 listings=200 active_searches=8 duration_ms=3200
```

#### ⑥ Catalog ingest
*(No log — happens silently unless new manufacturers/models are discovered)*

#### ⑦ Prefill from DB
```
INFO  "prefilled from DB"               component=scheduler filled=122 looked_up=200 duration_ms=45
```

#### ⑧ KM enrichment (1s delay between items, max 100/cycle)
```
DEBUG "enriched listing"                 component=enricher token=dqrrj9yg km=69000 city=Tel Aviv-Yafo image=false
DEBUG "enriched listing"                 component=enricher token=d2xvt6p1 km=57000 city=Hod ha-Sharon image=false
  ... (up to 100 listings) ...
DEBUG "enrichment limit reached"         component=enricher enriched=100 attempts=111 remaining=15
INFO  "mileage data enriched"            component=scheduler enriched=100 total=200
```

#### ⑨ Percolator match
*(No log — matching is in-memory, produces MatchResult per listing×search pair)*

#### ⑩ Per-match processing

For each listing that matches a search:

**Dedup check:**
```
# (only logs on error)
ERROR "claim failed"                     component=scheduler token=abc123 search_id=26 chat_id=100 search_name=mazda-3 error=...
```

**Price drop detected:**
```
INFO  "price drop detected"              component=scheduler token=abc123 old_price=95000 new_price=89000 manufacturer=Mazda model=3 year=2021
```

**Search match summary:**
```
INFO  "search matched listings"          component=scheduler search_id=26 chat_id=100 search_name=mazda-3 new=3 price_drops=1
```

#### ⑪ Deliver notifications

**Instant delivery (via Redis → notifier):**
```
INFO  "delivering new listings to user"  component=scheduler count=3 search=mazda-3 user=100
```

**In the notifier worker:**
```
INFO  "alert delivered"                  component=broker-consumer id=1716571200000-0 chat_id=100 search_name=mazda-3
```

**In the Telegram notifier:**
```
INFO  "sent telegram message"            component=telegram chat_id=100 chunks=1
INFO  "sent telegram photo"              component=telegram chat_id=100
```

**If user blocked the bot:**
```
WARN  "user blocked bot, deactivating"   component=scheduler search_id=26 chat_id=100 search_name=mazda-3
```

#### ⑫ Digest processing
```
INFO  "digest sent"                      component=scheduler chat_id=200 items=5
INFO  "daily digest sent"                component=scheduler chat_id=300 searches=3
```

#### ⑬ Backfill unenriched (DB listings with km=0)
```
INFO  "backfill enrichment: fetching km for DB listings missing data"  component=scheduler candidates=42
INFO  "backfill enrichment complete"     component=scheduler enriched=35 candidates=42
```

#### ⑭ Write cycle log
*(Writes to cycle_log table — no log unless error)*

#### ⑮ Scan complete
```
INFO  "scan complete"                    component=scheduler scan=5 elapsed=183.17s searches=8 listings_checked=200 new_matches=3 notifications_sent=2
```

---

## 3. Notification Delivery Pipeline

```
Scheduler                    Redis                     Notifier                  Telegram
    │                          │                          │                          │
    │ Publish(Alert)           │                          │                          │
    │─────────────────────────→│ XADD carwatch:alerts     │                          │
    │                          │                          │                          │
    │                          │ XReadGroup ──────────────→│                          │
    │                          │                          │ rate.Limiter.Wait (30/s) │
    │                          │                          │                          │
    │                          │                          │ NotifyRaw ───────────────→│
    │                          │                          │                          │ sendMessage
    │                          │                          │                          │
    │                          │                          │ XACK ────────────────────→│ 200 OK
    │                          │                          │                          │
    │                          │                          │ INFO "alert delivered"    │
```

### Failure paths

```
# Rate limited by Telegram (429)
WARN  "rate limited by telegram, retrying"  component=telegram chat_id=100 attempt=1 wait=5s

# Delivery failed after retries
ERROR "deliver alert failed"                component=broker-consumer id=... chat_id=100 search_name=mazda-3 error=...
# (message stays in PEL, retried by reclaimPending every 30s)

# Max retries exceeded
WARN  "message dead-lettered after max retries"  component=broker-consumer id=... max_retries=3

# Orphaned messages from crashed consumer
INFO  "reclaiming orphaned messages"        component=broker-consumer count=3
```

---

## 4. Bot Command Flow

### User sends /start

```
INFO  "command received"                 component=bot chat_id=6025534247 username=danielsio command=/start
```

### User sends /watch → Wizard flow

```
INFO  "command received"                 component=bot chat_id=6025534247 username=danielsio command=/watch
DEBUG "wizard step"                      component=bot chat_id=6025534247 state=ask_manufacturer
DEBUG "wizard step"                      component=bot chat_id=6025534247 state=ask_model
DEBUG "wizard step"                      component=bot chat_id=6025534247 state=ask_year
DEBUG "wizard step"                      component=bot chat_id=6025534247 state=ask_price
DEBUG "wizard step"                      component=bot chat_id=6025534247 state=confirm
INFO  "search created"                   component=bot chat_id=6025534247 search_id=26 name=mazda-3
```

### User sends /list

```
INFO  "command received"                 component=bot chat_id=6025534247 username=danielsio command=/list
```

### User sends /stop 26

```
INFO  "command received"                 component=bot chat_id=6025534247 username=danielsio command=/stop
INFO  "search deleted"                   component=bot chat_id=6025534247 search_id=26
```

---

## 5. API Request Flow

Every HTTP request produces one access log entry:

```
INFO  "http_request"                     method=GET path=/api/v1/searches status=200 duration_ms=12
                                         request_id=a1b2c3d4 remote_addr=1.2.3.4 user_agent=Mozilla/5.0
                                         chat_id=6025534247
```

### Error responses

```
INFO  "http_request"                     method=POST path=/api/v1/searches status=409 duration_ms=8
                                         request_id=e5f6g7h8 error="search limit reached"
```

### Guest instant search

```
INFO  "http_request"                     method=POST path=/api/v1/guest/instant-search status=200 duration_ms=3200
INFO  "instant search complete"          component=api fetched=45 filtered=30 returned=30
```

---

## 6. Component Tags

Each log entry carries a `component` field for filtering in the admin LogsTab:

| Component | Binary | What it logs |
|-----------|--------|-------------|
| `scheduler` | scraper | Scan cycles, search matching, delivery decisions |
| `yad2` | scraper | Feed fetching, HTTP responses, parsing |
| `enricher` | scraper | Per-listing km/city enrichment from item pages |
| `circuit_breaker` | scraper | Yad2 outage detection, state transitions |
| `api-pricelist` | scraper/api | Base price lookups from Yad2 price list |
| `bot` | bot-poller | Telegram commands, wizard state, callbacks |
| `telegram` | bot-poller/notifier | Message delivery, rate limits, retries |
| `notifier` | notifier | Multi-notifier routing, channel resolution |
| `webpush` | notifier | WebPush VAPID delivery |
| `broker-consumer` | notifier | Redis stream consumption, orphan reclaim |
| `api` | api-server | HTTP access log (every request) |

---

## 7. Key Structured Fields

| Field | Type | Where used | Example |
|-------|------|-----------|---------|
| `scan` | int | scheduler cycle | `5` |
| `chat_id` | int64 | everywhere | `6025534247` |
| `search_id` | int64 | scheduler, API | `26` |
| `search_name` | string | scheduler, broker | `"mazda-3"` |
| `token` | string | listings | `"dqrrj9yg"` |
| `username` | string | bot commands | `"danielsio"` |
| `command` | string | bot commands | `"/watch"` |
| `request_id` | string | API requests | `"a1b2c3d4"` |
| `duration_ms` | int64 | fetches, DB queries | `3200` |
| `db_duration_ms` | int64 | scheduler DB calls | `12` |
| `count` | int | result counts | `200` |
| `error` | string | all error logs | `"connection refused"` |
| `retention` | string | prune operations | `"720h0m0s"` |

---

## 8. Log Levels Guide

| Level | When to use | Example |
|-------|------------|---------|
| **ERROR** | Operation failed, cannot recover automatically | DB write failed, notification delivery failed |
| **WARN** | Degraded but recovered, or expected user action | User blocked bot, rate limited, challenge detected |
| **INFO** | Key business events — one per operation boundary | Cycle complete, search created, alert delivered |
| **DEBUG** | Internal detail for troubleshooting | Per-listing enrichment, cache hits, wizard steps |
