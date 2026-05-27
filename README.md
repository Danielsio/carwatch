# CarWatch

A self-hosted bot that monitors Israeli car listing sites (Yad2) and sends notifications when new listings match your search criteria.

## What it does

1. Polls Yad2 on a schedule (every 10-20 minutes)
2. Filters results by engine size, mileage, ownership count, keywords
3. Deduplicates using PostgreSQL so you only get notified once per listing
4. Sends formatted notifications with specs, price, and a direct link via Telegram and Web Push

## Quick start

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your Telegram bot token and PostgreSQL DSN

make build
./bot -config config.yaml
```

### Docker

```bash
docker compose -f docker-compose.prod.yaml up -d
```

For production compose, set `REDIS_PASSWORD` in `.env` and configure Redis in `config.yaml`:

```yaml
redis:
  addr: "redis:6379"
  password: "${REDIS_PASSWORD}"
  db: 0
```

Also set `TELEMETRY_AUTH_TOKEN` in `.env` and configure:

```yaml
telemetry:
  auth_token: "${TELEMETRY_AUTH_TOKEN}"
```

This token protects `/metrics` when binding the API on a non-localhost address.
`POST /api/v1/vitals` uses normal API authentication (Firebase in production, `api.auth_token` in local dev mode).

Set an immutable application image tag in `.env` and use release tags for deploys:

```dotenv
CARWATCH_IMAGE_TAG=9.16.11
```

### Production deploy verification

After deployment, verify every runtime service is healthy:

```bash
docker exec carwatch-api wget -q --spider http://localhost:8080/healthz
docker exec carwatch-bot-poller wget -q --spider http://localhost:8082/healthz
docker exec carwatch-scraper wget -q --spider http://localhost:8081/healthz
docker exec carwatch-notifier wget -q --spider http://localhost:8083/healthz
docker exec carwatch-enricher wget -q --spider http://localhost:8084/healthz
```

If any check fails, rollback deterministically to the last known-good image tag:

```bash
cd ~/carwatch
set -a; source .env; set +a
PREV_TAG=$(cat prev_image_tag)
if grep -q '^CARWATCH_IMAGE_TAG=' .env; then
  sed -i "s/^CARWATCH_IMAGE_TAG=.*/CARWATCH_IMAGE_TAG=${PREV_TAG}/" .env
else
  echo "CARWATCH_IMAGE_TAG=${PREV_TAG}" >> .env
fi
CARWATCH_IMAGE_TAG="${PREV_TAG}" docker compose -f docker-compose.prod.yaml pull postgres redis api bot-poller scraper notifier enricher
CARWATCH_IMAGE_TAG="${PREV_TAG}" docker compose -f docker-compose.prod.yaml up -d postgres redis api bot-poller scraper notifier enricher

# verify rollback health
docker exec carwatch-api wget -q --spider http://localhost:8080/healthz
docker exec carwatch-bot-poller wget -q --spider http://localhost:8082/healthz
docker exec carwatch-scraper wget -q --spider http://localhost:8081/healthz
docker exec carwatch-notifier wget -q --spider http://localhost:8083/healthz
docker exec carwatch-enricher wget -q --spider http://localhost:8084/healthz
```

## Configuration

See [`config.example.yaml`](config.example.yaml) for all options.

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
Dedup Store (PostgreSQL: atomic claim)
    |
    v
Pipeline (fitness score, deal score, base price)
    |
    v
Notifier (Telegram + WebPush)
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
│   │   └── yad2/                # Yad2 adapter (client, parser, fetcher)
│   ├── filter/                  # Stateless listing filter
│   ├── format/                  # Number/price formatting
│   ├── health/                  # Health check handler
│   ├── locale/                  # Hebrew locale strings
│   ├── model/                   # RawListing, Listing
│   ├── notifier/                # Multi-notifier dispatch
│   │   ├── telegram/            # Telegram adapter
│   │   └── webpush/             # WebPush adapter (VAPID)
│   ├── pricelist/               # Vehicle base price lookup
│   ├── scheduler/               # Polling loop, retry, backoff, pipeline
│   ├── scoring/                 # Market analysis, fitness & deal scoring
│   ├── spa/                     # SPA file server (serves web/dist)
│   └── storage/                 # Store interfaces
│       └── postgres/            # PostgreSQL adapter
├── web/                         # React/Vite/TypeScript frontend (SPA)
│   ├── src/pages/               # Login, Listings, Searches, Saved, Admin...
│   └── public/                  # Icons, manifest
├── migrations/                  # PostgreSQL schema migrations
├── docs/                        # Architecture & design docs
├── config.example.yaml
├── Dockerfile                   # Multi-stage: frontend build -> Go build -> runtime
├── docker-compose.dev.yaml      # Local development
├── docker-compose.prod.yaml     # Production (carwatch + Caddy + PostgreSQL)
└── Makefile                     # Build, test, lint, VM management
```

## Development

### Local setup

Prerequisites: Go 1.25+, Docker (for PostgreSQL).

```bash
make dev-db        # Start PostgreSQL via Docker
make dev           # Build + run with local PostgreSQL
make dev-stop      # Stop PostgreSQL
make dev-reset     # Wipe database and start fresh
make dev-pg-shell  # Open psql shell to local database
```

Set `TELEGRAM_BOT_TOKEN` in your environment before running `make dev`.

### Testing

```bash
make test          # Run tests with coverage
make lint          # Run golangci-lint
make ci            # Lint + test (what CI runs)
make test-cover    # Generate HTML coverage report
```

## License

Private project.
