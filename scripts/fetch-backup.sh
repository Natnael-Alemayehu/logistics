#!/usr/bin/env bash
# Fetches a backup from the bucket scripts/backup.sh copies them to, with
# backup.env's settings: the newest, or the one made at the time given. For
# restoring on a new machine, and for the restore drill (docs/deploy.md,
# "Backups").
#
#   scripts/fetch-backup.sh                    # the newest
#   scripts/fetch-backup.sh 20261104_020000    # kiflet_20261104_020000…
#
# With WAL archiving, it also fetches the base backup made at that time and
# the bucket's WAL (into BACKUP_DIR/wal), for scripts/restore-pitr.sh.
#
# The files go to /var/backups/kiflet (BACKUP_DIR), still encrypted. It
# needs neither .env nor a running stack, only Docker and backup.env
# (BACKUP_ENV_FILE names another).
set -euo pipefail
umask 077

cd "$(dirname "$0")/.."
if [ $# -gt 1 ]; then
  echo "usage: $0 [<time>]" >&2
  exit 2
fi
BACKUP_DIR="${BACKUP_DIR:-/var/backups/kiflet}"
settings_file="${BACKUP_ENV_FILE:-backup.env}"
set -a
# shellcheck source=/dev/null
. "$settings_file"
set +a
bucket="${BACKUP_S3_BUCKET:?set BACKUP_S3_BUCKET in $settings_file}"

mkdir -p "$BACKUP_DIR"
BACKUP_DIR="$(cd "$BACKUP_DIR" && pwd)"
# rclone in a container, with the backups at /backups, as their owner.
offsite() {
  COMPOSE_FILE="${BACKUP_COMPOSE_FILE:-deployments/docker-compose.backup.yml}" \
    docker compose --progress quiet --env-file /dev/null run --rm --no-deps -T \
    --user "$(id -u):$(id -g)" -v "$BACKUP_DIR:/backups" offsite "$@"
}

stamp="${1:-}"
if [ -z "$stamp" ]; then
  stamp="$(offsite lsf --files-only --include 'kiflet_*.dump.age' "offsite:$bucket" |
    sed -n 's/^kiflet_\(.*\)\.dump\.age$/\1/p' | sort | tail -n 1)"
  [ -n "$stamp" ] || { echo "There is no backup in $bucket." >&2; exit 1; }
fi
db="$BACKUP_DIR/kiflet_${stamp}.dump.age"
files="$BACKUP_DIR/kiflet_${stamp}_files.tar.gz.age"
settings="$BACKUP_DIR/kiflet_${stamp}_settings.tar.gz.age"
base="$BACKUP_DIR/kiflet_${stamp}_base.tar.gz.age"

offsite copy --include "${db##*/}" --include "${files##*/}" --include "${settings##*/}" \
  --include "${base##*/}" "offsite:$bucket" /backups
for f in "$db" "$files" "$settings"; do
  [ -f "$f" ] || { echo "$bucket has no $(basename "$f")." >&2; exit 1; }
done
chmod 600 "$db" "$files" "$settings"
if [ -f "$base" ]; then
  chmod 600 "$base"
  mkdir -p "$BACKUP_DIR/wal"
  offsite copy "offsite:$bucket/wal" /backups/wal
fi

echo "Database: $db"
echo "Files:    $files"
echo "Settings: $settings"
if [ -f "$base" ]; then
  echo "Base:     $base"
  echo "WAL:      $BACKUP_DIR/wal"
  echo "Or to a moment since: AGE_IDENTITY=<private key file> BACKUP_DIR=$BACKUP_DIR scripts/restore-pitr.sh $base '<time>'"
fi
echo "Restore with: AGE_IDENTITY=<private key file> scripts/restore.sh $db $files"
