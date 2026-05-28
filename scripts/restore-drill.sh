#!/usr/bin/env bash
set -euo pipefail

# Restore drill: download backup, restore to temp container, verify, cleanup.
# Can use either a local backup or OCI Object Storage.
#
# Usage:
#   ./restore-drill.sh                     # use latest local backup
#   ./restore-drill.sh /path/to/backup.sql.gz  # use specific file

BACKUP_DIR="${BACKUP_DIR:-$HOME/carwatch/backups}"
DRILL_CONTAINER="carwatch-restore-drill"
DRILL_PORT=5433
PG_USER="carwatch"
PG_DB="carwatch"

cleanup() {
  echo "Cleaning up..."
  docker rm -f "$DRILL_CONTAINER" 2>/dev/null || true
}
trap cleanup EXIT

if [ $# -gt 0 ]; then
  BACKUP_FILE="$1"
else
  BACKUP_FILE=$(ls -t "${BACKUP_DIR}"/carwatch-backup-*.sql.gz 2>/dev/null | head -1)
fi

if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
  echo "No backup file found. Provide a path or ensure backups exist in ${BACKUP_DIR}"
  exit 1
fi

echo "=== Restore Drill ==="
echo "Backup: ${BACKUP_FILE}"
echo ""

echo "--- Starting temporary Postgres ---"
docker run -d --name "$DRILL_CONTAINER" \
  -e POSTGRES_USER="$PG_USER" \
  -e POSTGRES_PASSWORD=drill \
  -e POSTGRES_DB="$PG_DB" \
  -p "${DRILL_PORT}:5432" \
  postgres:17-alpine

echo "Waiting for Postgres to be ready..."
for i in $(seq 1 30); do
  if docker exec "$DRILL_CONTAINER" pg_isready -U "$PG_USER" -q 2>/dev/null; then
    break
  fi
  sleep 1
done

echo ""
echo "--- Restoring backup ---"
gunzip -c "$BACKUP_FILE" | docker exec -i "$DRILL_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -q

echo ""
echo "--- Verification queries ---"

verify() {
  local desc="$1"
  local query="$2"
  result=$(docker exec "$DRILL_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -tAc "$query" 2>/dev/null)
  echo "  ${desc}: ${result}"
}

verify "Users" "SELECT count(*) FROM users"
verify "Searches" "SELECT count(*) FROM searches"
verify "Listings" "SELECT count(*) FROM listing_history"
verify "Price records" "SELECT count(*) FROM price_history"
verify "Push subs" "SELECT count(*) FROM push_subscriptions"
verify "Latest listing" "SELECT first_seen_at FROM listing_history ORDER BY first_seen_at DESC LIMIT 1"

echo ""
echo "=== RESTORE DRILL PASSED ==="
