#!/usr/bin/env bash
#
# Validates the production docker-compose contract. These checks previously
# lived inline in .github/workflows/ci.yml; extracting them here lets you run
# them locally:
#
#   ./scripts/validate-compose.sh [compose-file]   # default: docker-compose.prod.yaml
#
# Verifies that the file renders with the required env vars, that each required
# secret is mandatory (rendering fails without it), and that every app service
# pins CARWATCH_IMAGE_TAG (never :latest), defines a healthcheck, and carries no
# watchtower auto-update label.
set -euo pipefail

COMPOSE_FILE="${1:-docker-compose.prod.yaml}"
APP_SERVICES="api bot-poller scraper notifier"

# render runs `docker compose config` with the full set of required env vars.
render() {
  CARWATCH_IMAGE_TAG=1.2.3 \
  POSTGRES_PASSWORD=test-postgres \
  REDIS_PASSWORD=test-redis \
  TELEMETRY_AUTH_TOKEN=test-telemetry \
  CARWATCH_DOMAIN=example.com \
  TELEGRAM_BOT_TOKEN=test-token \
  docker compose -f "$COMPOSE_FILE" "$@"
}

echo "==> compose config renders with full env"
render config >/dev/null

# require_var asserts that `docker compose config` FAILS when $1 is unset.
# Remaining required vars are passed through the environment.
require_var() {
  local name="$1"; shift
  local rc=0
  env "$@" docker compose -f "$COMPOSE_FILE" config >/dev/null 2>&1 || rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "Expected docker compose config to fail without ${name}"
    exit 1
  fi
  echo "==> ${name} is required ✓"
}

require_var CARWATCH_IMAGE_TAG \
  POSTGRES_PASSWORD=test-postgres REDIS_PASSWORD=test-redis \
  TELEMETRY_AUTH_TOKEN=test-telemetry CARWATCH_DOMAIN=example.com \
  TELEGRAM_BOT_TOKEN=test-token

require_var REDIS_PASSWORD \
  CARWATCH_IMAGE_TAG=1.2.3 POSTGRES_PASSWORD=test-postgres \
  TELEMETRY_AUTH_TOKEN=test-telemetry CARWATCH_DOMAIN=example.com \
  TELEGRAM_BOT_TOKEN=test-token

require_var TELEMETRY_AUTH_TOKEN \
  CARWATCH_IMAGE_TAG=1.2.3 POSTGRES_PASSWORD=test-postgres \
  REDIS_PASSWORD=test-redis CARWATCH_DOMAIN=example.com \
  TELEGRAM_BOT_TOKEN=test-token

echo "==> app services must not pin :latest"
if grep -n "ghcr.io/danielsio/carwatch:latest" "$COMPOSE_FILE"; then
  echo "${COMPOSE_FILE} must not pin app services to :latest"
  exit 1
fi

echo "==> watchtower auto-update labels must be absent"
if grep -n "com.centurylinklabs.watchtower.enable=true" "$COMPOSE_FILE"; then
  echo "watchtower auto-update labels must be removed from app services"
  exit 1
fi

echo "==> all app services use CARWATCH_IMAGE_TAG"
count=$(grep -c "ghcr.io/danielsio/carwatch:\${CARWATCH_IMAGE_TAG" "$COMPOSE_FILE" || true)
if [ "$count" -ne 4 ]; then
  echo "Expected api/bot-poller/scraper/notifier to use CARWATCH_IMAGE_TAG (found $count)"
  exit 1
fi

echo "==> compose topology has expected app services"
render config --services > /tmp/compose-services.txt
for svc in $APP_SERVICES; do
  if ! grep -q "^${svc}$" /tmp/compose-services.txt; then
    echo "::error::Missing expected app service: ${svc}"
    exit 1
  fi
done
echo "All expected app services present ✓"

echo "==> all app services define healthchecks"
render config > /tmp/compose-full.yaml
for svc in $APP_SERVICES; do
  if ! grep -A40 "^  ${svc}:" /tmp/compose-full.yaml | grep -q "healthcheck:"; then
    echo "::error::Service ${svc} is missing a healthcheck"
    exit 1
  fi
done
echo "All app services have healthchecks ✓"

echo "All production compose contract checks passed ✓"
