# Architecture

CarWatch follows a ports-and-adapters pattern. The scheduler orchestrates the pipeline, and each stage communicates through interfaces so implementations can be swapped independently.

## Component diagram

```
┌──────────────────────────────────────────────────────┐
│                     Scheduler                        │
│                                                      │
│  ┌─────────┐    ┌────────┐    ┌───────┐    ┌──────┐ │
│  │ Fetcher │───>│ Filter │───>│ Dedup │───>│Notify│ │
│  └────┬────┘    └────────┘    └───┬───┘    └──────┘ │
│       │                          │                   │
│  ┌────┴────┐               ┌─────┴─────┐            │
│  │  Yad2   │               │  SQLite   │            │
│  │ Adapter │               │  Store    │            │
│  └─────────┘               └───────────┘            │
└──────────────────────────────────────────────────────┘
```

## Interfaces

### Fetcher

```go
type Fetcher interface {
    Fetch(ctx context.Context, params config.SourceParams) ([]model.RawListing, error)
}
```

Retrieves raw listings from a marketplace. The only implementation is `yad2.Yad2Fetcher`. Error sentinels `ErrChallenge` and `ErrRateLimited` live in the `fetcher` package so the scheduler can react without importing concrete adapters.

### DedupStore

```go
type DedupStore interface {
    ClaimNew(ctx context.Context, token string, searchName string) (bool, error)
    ReleaseClaim(ctx context.Context, token string) error
    Prune(ctx context.Context, olderThan time.Duration) (int64, error)
    Close() error
}
```

`ClaimNew` uses `INSERT OR IGNORE` + `RowsAffected` for atomic deduplication -- no read-then-write race. If notification fails for all recipients, `ReleaseClaim` removes the token so it retries next cycle.

### Notifier

```go
type Notifier interface {
    Connect(ctx context.Context) error
    Notify(ctx context.Context, recipient string, listings []model.Listing) error
    Disconnect() error
}
```

Currently a WhatsApp stub (logs to stdout). Planned: Telegram.

## Data flow

A single poll cycle works like this:

```
1. Scheduler wakes up
   - Check active hours (skip if outside window)
   - Apply adaptive backoff multiplier to delay

2. For each search config:
   a. Fetch
      - HTTP GET to Yad2 with browser-like headers
      - Rotate User-Agent, support SOCKS5 proxy
      - Retry up to 3 times (2s/4s/8s exponential backoff)
      - Detect anti-bot challenges (doubles backoff multiplier)
      - Parse HTML, extract __NEXT_DATA__ JSON
      - Return []RawListing (~40 per page)

   b. Filter
      - Engine volume range (cc)
      - Max mileage, max ownership count
      - Required/excluded keywords (case-insensitive)
      - Zero values disable that filter
      - Pure function, no I/O

   c. Dedup
      - For each filtered listing: ClaimNew(token)
      - Atomic: INSERT OR IGNORE + check RowsAffected
      - If new -> add to notification batch
      - If seen -> skip

   d. Notify
      - Format listings with specs, emoji, link
      - Send to each recipient
      - If all recipients fail -> ReleaseClaim (retry next cycle)
      - If at least one succeeds -> keep claims

3. Housekeeping
   - Prune listings older than 30 days (once per 24h)
   - Adjust backoff: challenge -> 2x, success -> 0.5x
```

## Scheduler timing

```
Base delay:   interval * backoffMultiplier
Jitter:       +/- (jitter / 2), randomly distributed
Minimum:      1 minute floor
Active hours: if outside, sleep until window opens

Backoff multiplier:
  Challenge detected -> multiply by 2 (cap at 4x)
  Successful fetch   -> divide by 2 (floor at 1x)

Example with defaults (15m interval, 5m jitter, 1x backoff):
  Delay range: 12.5m to 17.5m
  After challenge: 25m to 35m (2x)
  After two challenges: 50m to 70m (4x, capped)
  After recovery: back to 12.5m-17.5m
```

## Yad2 adapter internals

### HTTP client (`client.go`)

- Clones `http.DefaultTransport`
- 30-second overall timeout
- Optional SOCKS5 proxy
- Realistic browser headers on every request:
  - User-Agent rotation from configured pool
  - `Accept-Language: he-IL,he;q=0.9,en-US;q=0.8`
  - `Sec-Fetch-*` metadata headers
  - `Accept-Encoding: gzip, deflate` (no brotli)

### Parser (`parser.go`)

Yad2 is a Next.js app. Listing data lives in a `<script id="__NEXT_DATA__">` tag as JSON.

```
HTML page
  -> goquery finds <script id="__NEXT_DATA__">
  -> extract JSON text
  -> unmarshal to nested structure
  -> navigate: props.pageProps.dehydratedState.queries[].state.data.data.feed.feed_items[]
  -> map each item to RawListing
  -> skip items with empty token
  -> prefer english_text over hebrew text for field values
```

### URL builder (`yad2.go`)

Uses `net/url.Values` for query parameter construction. Maps `SourceParams` fields to Yad2's query format (ranges use `min-max` syntax).

## Config loading pipeline

```
1. Read YAML file
2. os.ExpandEnv() on raw bytes (enables ${VAR} interpolation)
3. yaml.Unmarshal into Config struct
4. applyDefaults() -- interval, jitter, timezone, paths, log level
5. validate() -- required fields, format checks, unit sanity
```

Validation is strict and fails at startup:
- At least one search required
- Each search needs name, source, recipients
- Engine values < 100 are rejected (likely liters, not cc)
- Active hours must be `HH:MM` format
- Log level must be debug/info/warn/error

## SQLite schema

```sql
CREATE TABLE IF NOT EXISTS seen_listings (
    token         TEXT PRIMARY KEY,
    search_name   TEXT NOT NULL,
    first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Opened with WAL mode and 5-second busy timeout. Parent directories are created automatically.

## Security considerations

- Non-root Docker user (UID 1000)
- Phone numbers masked in logs (`+972***XXX`)
- Secrets via env var interpolation (never hardcoded in config)
- Response body capped at 10 MB (prevents OOM)
- Per-search 60-second timeout (prevents hung connections)
- No brotli advertised (only decompressors we ship)
