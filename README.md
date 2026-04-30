# CarWatch

A self-hosted bot that monitors Israeli car listing sites (starting with Yad2) and sends notifications when new listings match your search criteria.

## What it does

1. Polls Yad2 on a schedule (every 10-20 minutes)
2. Filters results by engine size, mileage, ownership count, keywords
3. Deduplicates using SQLite so you only get notified once per listing
4. Sends formatted notifications with specs, price, and a direct link

## Quick start

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your search criteria and recipients

make build
./bot -config config.yaml
```

### Docker

```bash
docker compose up -d
```

Mount your config and a data volume for persistence:

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
```

## Configuration

See [`config.example.yaml`](config.example.yaml) for all options. Key sections:

**Polling** -- how often and when to check:

```yaml
polling:
  interval: 15m
  jitter: 5m
  active_hours:
    start: "08:00"
    end: "22:00"
  timezone: "Asia/Jerusalem"
```

**Searches** -- what to look for:

```yaml
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
      engine_min_cc: 1800    # Always in cc, not liters
      engine_max_cc: 2100
      max_km: 150000
      max_hand: 3
      keywords: []
      exclude_keys: []
    recipients:
      - "+972XXXXXXXXX"
```

**Secrets** -- use environment variables for sensitive values:

```yaml
http:
  proxy: "${PROXY_URL}"
```

## Architecture

See [docs/architecture.md](docs/architecture.md) for the full breakdown.

```
Scheduler (interval + jitter + adaptive backoff)
    |
    v
Fetcher -----> Parser (HTML -> JSON -> RawListing)
    |
    v
Filter (engine, km, hand, keywords)
    |
    v
Dedup Store (SQLite: atomic claim)
    |
    v
Notifier (WhatsApp stub / future: Telegram)
    |
    v
User
```

All components communicate through interfaces, making each layer independently testable and swappable.

## Project structure

```
carwatch/
├── cmd/bot/main.go              # Entry point, wiring
├── internal/
│   ├── api/                     # REST API (listings, searches, bookmarks)
│   ├── bot/                     # Telegram bot handlers, wizards, callbacks
│   ├── catalog/                 # Dynamic car catalog (make/model/submodel)
│   ├── config/                  # YAML loading, validation, defaults
│   ├── fetcher/                 # Fetcher interface + circuit breaker, caching
│   │   ├── yad2/                # Yad2 adapter (client, parser, fetcher)
│   │   └── winwin/              # WinWin adapter
│   ├── filter/                  # Stateless listing filter
│   ├── format/                  # Number/price formatting
│   ├── health/                  # Health check handler
│   ├── locale/                  # Hebrew locale strings
│   ├── model/                   # RawListing, Listing
│   ├── notifier/                # Multi-notifier dispatch
│   │   └── telegram/            # Telegram adapter
│   ├── scheduler/               # Polling loop, retry, backoff
│   ├── scoring/                 # Listing quality scoring
│   ├── spa/                     # SPA file server (serves web/dist)
│   └── storage/                 # Store interfaces
│       └── sqlite/              # SQLite adapter
├── web/                         # React/Vite/TypeScript frontend (SPA)
│   ├── src/pages/               # Login, Listings, Searches, Saved, Admin...
│   └── public/                  # Icons, manifest
├── testdata/                    # HTML/JSON fixtures
├── docs/                        # Architecture & design docs
├── config.example.yaml
├── Dockerfile                   # Multi-stage: frontend build → Go build → runtime
├── docker-compose.dev.yaml      # Local development
├── docker-compose.prod.yaml     # Production (carwatch + Caddy + Watchtower)
└── Makefile                     # Build, test, lint, VM management
```

## Development

```bash
make test          # Run tests with coverage
make lint          # Run golangci-lint
make ci            # Lint + test (what CI runs)
make test-cover    # Generate HTML coverage report
```

Requires Go 1.24+ and a C compiler (for SQLite via cgo).

## Current status

All core pipeline components are implemented and tested. The WhatsApp notifier is currently a stub that logs to stdout. See the [project board](https://github.com/users/Danielsio/projects/1) for remaining work.

## License

Private project.
