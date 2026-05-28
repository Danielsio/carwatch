#!/usr/bin/env bash
set -euo pipefail

# Post-deploy smoke test for CarWatch production.
# Verifies all services are healthy and core functionality works.
# Exit 0 on success, non-zero on any failure.

COMPOSE_DIR="${1:-/home/ubuntu/carwatch}"
COMPOSE="docker compose -f ${COMPOSE_DIR}/docker-compose.prod.yaml"

SERVICES=(api bot-poller scraper notifier enricher)
HEALTH_PORTS=(8080 8082 8081 8083 8084)

fail=0

echo "=== CarWatch Post-Deploy Smoke Test ==="

# 1. Check all containers are running
echo ""
echo "--- Container status ---"
for svc in "${SERVICES[@]}"; do
  container="carwatch-${svc}"
  if docker inspect --format='{{.State.Status}}' "$container" 2>/dev/null | grep -q running; then
    echo "  ✓ ${container} running"
  else
    echo "  ✗ ${container} NOT running"
    fail=1
  fi
done

# 2. Health endpoint checks
echo ""
echo "--- Health checks ---"
for i in "${!SERVICES[@]}"; do
  svc="${SERVICES[$i]}"
  port="${HEALTH_PORTS[$i]}"
  container="carwatch-${svc}"
  if docker exec "$container" wget -qO- --timeout=5 "http://localhost:${port}/healthz" >/dev/null 2>&1; then
    echo "  ✓ ${svc} :${port}/healthz OK"
  else
    echo "  ✗ ${svc} :${port}/healthz FAILED"
    fail=1
  fi
done

# 3. Database connectivity (via API catalog endpoint)
echo ""
echo "--- Database connectivity ---"
if docker exec carwatch-api wget -qO- --timeout=5 "http://localhost:8080/api/v1/catalog/manufacturers" | grep -q '"id"'; then
  echo "  ✓ API catalog returns data (DB connected)"
else
  echo "  ✗ API catalog empty or unreachable (DB issue?)"
  fail=1
fi

# 4. Redis connectivity (check stream exists)
echo ""
echo "--- Redis connectivity ---"
REDIS_PASS=$(grep -oP 'REDIS_PASSWORD=\K[^\s]+' "${COMPOSE_DIR}/.env" 2>/dev/null || echo "")
if [ -n "$REDIS_PASS" ]; then
  if docker exec carwatch-redis redis-cli -a "$REDIS_PASS" --no-auth-warning ping 2>/dev/null | grep -q PONG; then
    echo "  ✓ Redis PONG"
  else
    echo "  ✗ Redis not responding"
    fail=1
  fi
else
  echo "  - Redis password not found in .env, skipping"
fi

# 5. Version check
echo ""
echo "--- Service versions ---"
for svc in "${SERVICES[@]}"; do
  container="carwatch-${svc}"
  binary="${svc}"
  [ "$svc" = "api" ] && binary="api-server"
  version=$(docker exec "$container" "${binary}" -version 2>/dev/null | head -1 || echo "unknown")
  echo "  ${svc}: ${version}"
done

echo ""
if [ "$fail" -eq 0 ]; then
  echo "=== ALL SMOKE TESTS PASSED ==="
  exit 0
else
  echo "=== SMOKE TESTS FAILED ==="
  exit 1
fi
