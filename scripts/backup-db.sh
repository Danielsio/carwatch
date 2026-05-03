#!/usr/bin/env bash
#
# Online SQLite backup with daily rotation (keeps last N days).
#
# Usage:
#   backup-db.sh <db-path> [backup-dir] [retention-days]
#
# Defaults:
#   backup-dir:     /data/backups
#   retention-days: 7
#
# The script uses "sqlite3 .backup" for a safe online backup that
# does not interfere with writers (no need to stop the application).

set -euo pipefail

DB_PATH="${1:?Usage: backup-db.sh <db-path> [backup-dir] [retention-days]}"
BACKUP_DIR="${2:-/data/backups}"
RETENTION_DAYS="${3:-7}"

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP_FILE="${BACKUP_DIR}/carwatch-${TIMESTAMP}.db"

mkdir -p "${BACKUP_DIR}"

if [ ! -f "${DB_PATH}" ]; then
  echo "ERROR: database file not found: ${DB_PATH}" >&2
  exit 1
fi

if ! command -v sqlite3 &>/dev/null; then
  echo "ERROR: sqlite3 not found in PATH" >&2
  exit 1
fi

sqlite3 "${DB_PATH}" ".backup '${BACKUP_FILE}'"

SIZE=$(stat --printf='%s' "${BACKUP_FILE}" 2>/dev/null || stat -f '%z' "${BACKUP_FILE}")
echo "Backup created: ${BACKUP_FILE} ($(( SIZE / 1024 )) KB)"

PRUNED=0
while IFS= read -r old; do
  rm -f "${old}"
  PRUNED=$((PRUNED + 1))
done < <(find "${BACKUP_DIR}" -name 'carwatch-*.db' -mtime +"${RETENTION_DAYS}" -type f 2>/dev/null)

if [ "${PRUNED}" -gt 0 ]; then
  echo "Pruned ${PRUNED} backup(s) older than ${RETENTION_DAYS} days"
fi
