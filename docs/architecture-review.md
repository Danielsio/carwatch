# CarWatch — Architecture Review

Current architecture analysis, scalability assessment, and proposed improvements.

---

## Current Architecture

```
┌─────────────────────────────────────────────────┐
│                 cmd/bot/main.go                  │
│              (wiring + lifecycle)                 │
└────────────┬────────────┬───────────────────┬────┘
             │            │                   │
    ┌────────▼──────┐  ┌──▼──────────┐  ┌─────▼─────┐
    │   Scheduler   │  │  Telegram   │  │   HTTP     │
    │  (poll loop)  │  │  Bot (cmds) │  │  /healthz  │
    │               │  │             │  │  /dashboard │
    └───┬───────────┘  └──────┬──────┘  └────────────┘
        │                     │
        │  ┌──────────────────┘
        │  │
   ┌────▼──▼────────────────────────────────────────┐
   │              SQLite (WAL mode)                  │
   │  users | searches | seen_listings |             │
   │  price_history | listing_history |              │
   │  pending_notifications | pending_digest |       │
   │  catalog_cache                                  │
   └────────────────────────────────────────────────┘
        │
   ┌────▼──────────────────────────────────────┐
   │         Fetcher Pipeline (per source)      │
   │                                            │
   │  Yad2Fetcher ──► Paginating ──► Cache      │
   │       │              │            │        │
   │       └──────────────┴────────────┘        │
   │              ──► CircuitBreaker            │
   │                                            │
   │  WinWinFetcher ──► (same stack)            │
   └───────────────────────────┬────────────────┘
                               │
                       ┌───────▼───────┐
                       │   Yad2 / WW   │
                       │   HTTP APIs   │
                       └───────────────┘
```

**Data flow per cycle:**
1. Scheduler wakes up (interval + jitter)
2. Loads all active searches from SQLite
3. Groups by (source, manufacturer, model) → N groups
4. For each group (sequentially, 2-5s delay):
   a. Fetch via pipeline (retry 3x, timeout 60s)
   b. For each search in group:
      - Filter (engine, km, hand, keywords)
      - Check year/price bounds
      - Dedup via ClaimNew (INSERT OR IGNORE)
      - RecordPrice → detect drops
      - Deliver (instant or digest)
      - Save to listing_history
5. Prune old entries every 24h
6. Flush pending digests

---

## Scalability Assessment

### What scales well

| Component | Why it works | Limit |
|-----------|-------------|-------|
| Search grouping | N users watching same car = 1 API call | ~infinite users per group |
| Dedup (INSERT OR IGNORE) | O(1) per listing, no locks | SQLite write throughput |
| Filter (stateless) | Pure function, no allocations | CPU-bound, very fast |
| Fetcher decorators | Composable, independent | Memory for cache entries |
| Per-user isolation | Separate dedup, digest, state | Storage linear in users |

### What doesn't scale

| Component | Bottleneck | Breaking point |
|-----------|-----------|---------------|
| Sequential group fetching | 2-5s delay × N groups | 30 groups = 1.5-2.5 min per cycle |
| SQLite single-writer | WAL helps reads, writes still serial | ~500 writes/sec, enough for ~200 users |
| MaxOpenConns=2 | Artificial DB connection limit | Immediate; should be 4-8 |
| No connection pooling | Every DB call acquires from 2-conn pool | Contention under load |
| HTML parsing in-band | Parser runs in fetch goroutine | CPU-bound for large pages |
| Telegram long-polling | Single connection, one update at a time | Bot command latency under load |

### Scaling scenarios

**10 users, 20 searches:** Current architecture is fine. Cycle completes in under 30 seconds. No bottlenecks.

**50 users, 100 searches:** Groups compress to ~30-40 unique (source, mfr, model) pairs. Sequential fetching takes 60-120 seconds per cycle. SQLite handles the write load (200-400 writes per cycle). Users start noticing 2+ minute latency between listing appearance on Yad2 and bot notification.

**200 users, 500 searches:** Groups compress to ~80-100 pairs. Sequential fetching takes 4-5 minutes. With a 15-minute interval, the bot is fetching for 1/3 of each cycle. SQLite write contention starts showing. At this point: parallelize fetching or accept the latency.

**1000+ users:** SQLite is no longer viable as the primary store. Need Postgres or similar. Telegram bot needs webhook mode. Fetching needs distributed workers.

---

## Proposed Future Architecture

### Phase 1: Quick wins (current user base)

No architecture changes needed. Just fixes:

```
Current architecture + these changes:
- Parallel group fetching (bounded goroutine pool)
- MaxOpenConns → 8
- Fix listing_history per-user keying
- Add image support to notifications
```

### Phase 2: Growth architecture (50-500 users)

```
┌─────────────────────────────────────────────────┐
│                    main.go                       │
└────────┬────────────┬───────────────────────┬────┘
         │            │                       │
┌────────▼──────┐  ┌──▼──────────┐  ┌─────────▼──────┐
│   Scheduler   │  │  Telegram   │  │     HTTP        │
│ (coordinator) │  │  Bot (wh)   │  │  /healthz       │
│               │  │  webhook    │  │  /dashboard     │
│               │  │  mode       │  │  /api/v1/...    │
└───┬───────────┘  └──────┬──────┘  └────────────────┘
    │                     │
    │  ┌──────────────────┘
    │  │
┌───▼──▼─────────────────────────────────────────┐
│              SQLite → PostgreSQL                │
│  (same interfaces, swap implementation)         │
│  + Connection pool (pgxpool)                    │
│  + Read replicas for history/stats queries      │
└────────────────────────────────────────────────┘
    │
┌───▼──────────────────────────────────────────┐
│        Fetch Worker Pool (N goroutines)        │
│                                                │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐         │
│  │ W1   │ │ W2   │ │ W3   │ │ W4   │         │
│  │ Yad2 │ │ Yad2 │ │ WW   │ │ Yad2 │         │
│  └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘         │
│     └────────┴────────┴────────┘              │
│              Results channel                   │
└──────────────────────────────────────────────┘
    │
┌───▼──────────────────────────────────────────┐
│         Notification Fan-out                   │
│                                                │
│  ┌────────────┐  ┌────────────┐               │
│  │  Telegram   │  │  WhatsApp  │               │
│  │  Notifier   │  │  Notifier  │               │
│  └─────────────┘  └────────────┘               │
└──────────────────────────────────────────────┘
```

Key changes:
- **Fetch worker pool:** 4-8 concurrent fetchers with per-source rate limiting
- **Results channel:** Decouple fetching from processing
- **Telegram webhook mode:** Eliminates long-polling overhead
- **Notification fan-out:** Multiple notifier adapters (Telegram + WhatsApp)
- **PostgreSQL migration path:** Same interfaces, better concurrency

### Phase 3: Scale architecture (1000+ users)

```
┌──────────────────────────────────────────────────────────┐
│                     API Gateway                           │
│              (Telegram webhook + REST API)                │
└────────────┬──────────────────────────┬──────────────────┘
             │                          │
    ┌────────▼──────────┐     ┌─────────▼────────────┐
    │  Command Service   │     │   Notification Svc   │
    │  (bot commands,    │     │   (format, deliver,   │
    │   wizard, search   │     │    retry, fan-out)    │
    │   CRUD)            │     │                       │
    └────────┬───────────┘     └─────────▲─────────────┘
             │                           │
    ┌────────▼──────────┐     ┌──────────┴─────────────┐
    │    PostgreSQL      │     │    Message Queue        │
    │  (users, searches, │     │  (Redis Streams or      │
    │   listings, dedup) │     │   NATS)                 │
    └────────┬───────────┘     └──────────▲─────────────┘
             │                            │
    ┌────────▼────────────────────────────┴─────────────┐
    │              Fetch Scheduler                       │
    │                                                    │
    │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
    │  │ Worker 1  │  │ Worker 2  │  │ Worker N  │        │
    │  │ (Yad2)    │  │ (WinWin)  │  │ (NewSrc)  │        │
    │  └──────────┘  └──────────┘  └──────────┘        │
    │                                                    │
    │  Publishes new listings to queue                   │
    └────────────────────────────────────────────────────┘
```

This is YAGNI for now. Don't build this until you actually have 500+ users.

---

## WhatsApp Integration Design

The architecture is already 80% ready for WhatsApp thanks to the `Notifier` interface:

```go
type Notifier interface {
    Notify(ctx context.Context, chatID string, listings []model.Listing) error
    NotifyRaw(ctx context.Context, chatID string, message string) error
}
```

### What needs to change:

1. **User identity:** Currently `chatID int64` (Telegram chat ID). For WhatsApp, identity is phone number (string). The `Notifier` interface already takes `chatID string`, but the storage layer uses `int64`. Need to abstract user identity.

2. **Notification preferences:** Users need to choose which channel(s) receive alerts. New table:
```sql
CREATE TABLE notification_channels (
    user_id INTEGER NOT NULL,
    channel TEXT NOT NULL,  -- 'telegram', 'whatsapp'
    channel_id TEXT NOT NULL,  -- chat_id or phone number
    active BOOLEAN DEFAULT true,
    PRIMARY KEY (user_id, channel)
);
```

3. **Message formatting:** WhatsApp has different formatting rules (no Markdown, uses WhatsApp-specific formatting). The `FormatListing` function needs a `format` parameter or separate formatters per channel.

4. **WhatsApp API adapter:**
```go
type WhatsAppNotifier struct {
    client *whatsapp.Client  // Twilio or Meta Business API
}

func (w *WhatsAppNotifier) Notify(ctx context.Context, phoneNumber string, listings []model.Listing) error {
    // Format for WhatsApp
    // Send via API
    // Handle delivery receipts
}
```

5. **Onboarding:** User sends message to WhatsApp business number → bot responds with setup wizard. Or: user links WhatsApp number via Telegram bot command.

### What doesn't need to change:
- Fetcher pipeline (source-agnostic)
- Scheduler loop (channel-agnostic)
- Filter logic (pure function)
- Dedup store (keyed on token + user_id, not channel)
- Price tracker (listing-level, not user-level)

---

## Database Schema Improvements

### Current issues:
1. `listing_history.token` as PK — should be `(token, chat_id)`
2. `listing_history` JOIN on `seen_listings` — breaks after prune
3. `price_history` never pruned
4. `pending_notifications` never expires
5. `catalog_cache` full-wipe on update

### Proposed schema evolution:

```sql
-- v2: Fix listing_history per-user
ALTER TABLE listing_history ADD COLUMN chat_id INTEGER;
CREATE INDEX idx_listing_history_chatid ON listing_history(chat_id, first_seen_at DESC);
-- Migrate: UPDATE listing_history SET chat_id = (SELECT chat_id FROM seen_listings WHERE seen_listings.token = listing_history.token LIMIT 1);
-- Drop old PK, create new one on (token, chat_id)

-- v3: Add pending_notifications expiry
ALTER TABLE pending_notifications ADD COLUMN retry_count INTEGER DEFAULT 0;
ALTER TABLE pending_notifications ADD COLUMN expires_at TIMESTAMP;

-- v4: Add user tier for monetization
ALTER TABLE users ADD COLUMN tier TEXT NOT NULL DEFAULT 'free';
ALTER TABLE users ADD COLUMN tier_expires_at TIMESTAMP;

-- v5: Add notification channels
CREATE TABLE notification_channels (
    user_id INTEGER NOT NULL,
    channel TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    active BOOLEAN DEFAULT true,
    PRIMARY KEY (user_id, channel)
);
```

---

## Caching Strategy

### Current:
- 5-minute TTL per (manufacturer, model, year, price, page)
- Max 100 entries, evict entries > 2x TTL
- Graceful fallback to stale cache on non-critical errors
- In-memory only (lost on restart)

### Improvements:

1. **Cache key should include source.** Currently, a Yad2 cache entry could theoretically be returned for a WinWin request if they share the same manufacturer/model/year/price/page params. In practice this doesn't happen because fetchers are separate instances, but the abstraction is leaky.

2. **Warm cache on startup.** After restart, the first cycle has 0% cache hit rate and hammers Yad2 with all groups at once. Consider persisting cache to SQLite or a file.

3. **Adaptive TTL.** During active hours with no challenges, use shorter TTL (3 min) for fresher data. During backoff periods, extend TTL (15 min) to reduce load.

4. **Cache metrics.** Track hit rate, miss rate, eviction rate. Add to `/healthz` and `/stats`.

---

## Rate Limiting Strategy

### Current:
- Circuit breaker: 5 failures → 30-min cooldown
- Proxy rotation: round-robin with health tracking
- Adaptive backoff: 1x-4x multiplier on challenge detection
- Inter-group delay: 2-5 seconds (scheduler.go:378-379)

### Improvements:

1. **Per-source rate limiter.** Use `golang.org/x/time/rate` to enforce explicit requests-per-minute limits per source. Currently rate limiting is implicit (delay between groups + backoff). Explicit limits are more predictable.

2. **Proxy quality scoring.** Instead of binary healthy/unhealthy, track success rate per proxy over a rolling window. Prefer high-success proxies. Retire proxies that consistently trigger challenges.

3. **Request fingerprint rotation.** Current anti-bot evasion uses randomized User-Agent. Add: randomized Accept-Language ordering, randomized header ordering (within Chrome-realistic bounds), randomized viewport hints in Sec-CH-UA headers.

4. **Backoff per source, not global.** Currently, a Yad2 challenge backs off the entire scheduler. If WinWin is healthy, it should continue at normal speed.

---

## Observability Recommendations

### Current state:
- Structured JSON logging (slog)
- Health endpoint with basic counters
- Admin-only /stats command

### What's missing:

1. **Metrics endpoint (Prometheus).** Export:
   - `carwatch_fetch_duration_seconds{source}` — histogram
   - `carwatch_fetch_errors_total{source,error_type}` — counter
   - `carwatch_listings_found_total{source}` — counter
   - `carwatch_notifications_sent_total{channel}` — counter
   - `carwatch_active_users` — gauge
   - `carwatch_active_searches` — gauge
   - `carwatch_cache_hit_ratio` — gauge
   - `carwatch_cycle_duration_seconds` — histogram

2. **Alerting.** Watchtower handles deployment, but there's no alerting for:
   - Health status degraded > 30 minutes
   - Zero listings found for 3+ consecutive cycles
   - Error rate > 50%
   - Database size > threshold

3. **Request tracing.** Add a cycle ID to all log entries within a cycle. Currently, interleaved log entries from concurrent operations are hard to trace back to a specific cycle.

4. **DB query logging.** At debug level, log query execution time for all DB operations. Identifies slow queries before they become bottlenecks.
