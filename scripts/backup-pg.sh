#!/usr/bin/env bash
#
# Automated PostgreSQL backup with daily rotation.
#
# Usage:
#   backup-pg.sh [backup-dir] [retention-days]
#
# Defaults:
#   backup-dir:     ~/carwatch/backups
#   retention-days: 7
#
# The script uses pg_dump via the carwatch-pg Docker container
# to create a compressed SQL backup. Output goes to stdout for
# systemd journal capture.

set -euo pipefail

BACKUP_DIR="${1:-${HOME}/carwatch/backups}"
RETENTION_DAYS="${2:-7}"

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP_FILE="${BACKUP_DIR}/carwatch-backup-${TIMESTAMP}.sql.gz"

mkdir -p "${BACKUP_DIR}"

# Verify the PostgreSQL container is running.
if ! docker inspect carwatch-pg >/dev/null 2>&1; then
  echo "ERROR: carwatch-pg container not found or not running" >&2
  exit 1
fi

echo "Starting backup: ${BACKUP_FILE}"

# Dump and compress in a single pipeline.
docker exec carwatch-pg pg_dump -U carwatch carwatch | gzip > "${BACKUP_FILE}"

SIZE=$(stat --printf='%s' "${BACKUP_FILE}" 2>/dev/null || stat -f '%z' "${BACKUP_FILE}")
echo "Backup created: ${BACKUP_FILE} ($((SIZE / 1024)) KB)"

# Prune old backups.
PRUNED=0
while IFS= read -r old; do
  rm -f "${old}"
  PRUNED=$((PRUNED + 1))
done < <(find "${BACKUP_DIR}" -name 'carwatch-backup-*.sql.gz' -mtime +"${RETENTION_DAYS}" -type f 2>/dev/null)

if [ "${PRUNED}" -gt 0 ]; then
  echo "Pruned ${PRUNED} backup(s) older than ${RETENTION_DAYS} days"
fi

echo "Backup complete"
