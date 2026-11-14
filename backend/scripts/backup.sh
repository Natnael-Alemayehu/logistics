#!/usr/bin/env bash
# Backs up a Menged deployment: the database, and the app's volume (the
# signatures and photos stored on disk, and the app secret). Keeps two weeks
# of backups.
#
#   scripts/backup.sh                       # from backend/, e.g. nightly from cron
#   BACKUP_DIR=/mnt/backups scripts/backup.sh
#
# It runs against deployments/docker-compose.prod.yml with .env. COMPOSE_FILE
# and ENV_FILE choose another stack, as for `docker compose`:
#
#   COMPOSE_FILE=deployments/docker-compose.prod.yml:deployments/staging/compose.yml \
#     ENV_FILE=deployments/staging/staging.env scripts/backup.sh
#
# Photos kept in S3 aren't included: keep the bucket with the provider's
# tools (versioning, replication). Copy the backups off the machine too. They
# hold the app secret and customers' details, so they are readable by their
# owner only.
set -euo pipefail
umask 077

cd "$(dirname "$0")/.."
export COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/menged}"
KEEP_DAYS="${KEEP_DAYS:-14}"
compose() { docker compose --env-file "$ENV_FILE" "$@"; }

stamp="$(date +%Y%m%d_%H%M%S)"
db="$BACKUP_DIR/menged_${stamp}.dump"
files="$BACKUP_DIR/menged_${stamp}_files.tar.gz"
# Written under a temporary name, so a failed run leaves nothing that looks
# like a backup.
trap 'rm -f "$db.partial" "$files.partial"' EXIT

mkdir -p "$BACKUP_DIR"
compose exec -T postgres pg_dump -U logistics -Fc logistics >"$db.partial"
mv "$db.partial" "$db"
compose exec -T app tar -C /data -czf - . >"$files.partial"
mv "$files.partial" "$files"
find "$BACKUP_DIR" -name 'menged_*' -mtime "+${KEEP_DAYS}" -delete

echo "Database: $db"
echo "Files:    $files"
echo "Restore with: scripts/restore.sh $db $files"
