#!/usr/bin/env bash
set -euo pipefail

# Upload the latest local backup to OCI Object Storage.
# Prerequisites: OCI CLI configured with ~/.oci/config
#
# Required env vars:
#   OCI_BUCKET_NAME   - Object Storage bucket name
#   OCI_NAMESPACE      - Object Storage namespace (tenancy)
#
# Optional:
#   BACKUP_DIR        - local backup directory (default: ~/carwatch/backups)
#   OCI_COMPARTMENT   - compartment OCID (uses default if unset)

BACKUP_DIR="${BACKUP_DIR:-$HOME/carwatch/backups}"
BUCKET="${OCI_BUCKET_NAME:?OCI_BUCKET_NAME must be set}"
NAMESPACE="${OCI_NAMESPACE:?OCI_NAMESPACE must be set}"

latest=$(ls -t "${BACKUP_DIR}"/carwatch-backup-*.sql.gz 2>/dev/null | head -1)
if [ -z "$latest" ]; then
  echo "No backups found in ${BACKUP_DIR}"
  exit 1
fi

filename=$(basename "$latest")
echo "Uploading ${filename} to OCI Object Storage..."

oci os object put \
  --bucket-name "$BUCKET" \
  --namespace "$NAMESPACE" \
  --file "$latest" \
  --name "carwatch-backups/${filename}" \
  --force

echo "Upload complete: carwatch-backups/${filename}"
