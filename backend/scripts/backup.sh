#!/usr/bin/env bash
# Dumps the production database and keeps two weeks of backups.
#
#   scripts/backup.sh                 # from backend/, e.g. nightly from cron
#   BACKUP_DIR=/mnt/backups scripts/backup.sh
#
# Signatures and photos live in the app-data volume (or S3); back that up
# alongside, e.g. with `docker run --rm -v <project>_app-data:/data ...`.
set -euo pipefail

cd "$(dirname "$0")/.."
BACKUP_DIR="${BACKUP_DIR:-/var/backups/menged}"
KEEP_DAYS="${KEEP_DAYS:-14}"
FILE="${BACKUP_DIR}/menged_$(date +%Y%m%d_%H%M%S).dump"

mkdir -p "$BACKUP_DIR"
docker compose -f deployments/docker-compose.prod.yml exec -T postgres \
  pg_dump -U logistics -Fc logistics >"$FILE"
find "$BACKUP_DIR" -name 'menged_*.dump' -mtime "+${KEEP_DAYS}" -delete

echo "Backup written to $FILE"
echo "Restore with: pg_restore -U logistics -d logistics --clean $FILE"
