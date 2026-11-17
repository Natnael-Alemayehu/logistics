#!/usr/bin/env bash
# Backs up a Kiflet deployment, in three files:
#
#   kiflet_<time>.dump              the database
#   kiflet_<time>_files.tar.gz      the app's volume: signatures and photos
#                                   stored on disk, and the app secret
#   kiflet_<time>_settings.tar.gz   this machine's settings: .env,
#                                   backup.env and deployments/secrets/
#
# and with WAL archiving on (WAL_ARCHIVE=on in .env), a fourth:
#
#   kiflet_<time>_base.tar.gz       the database's files as they are
#                                   (pg_basebackup), which the archived
#                                   WAL brings forward to any moment since:
#                                   see scripts/ship-wal.sh and
#                                   scripts/restore-pitr.sh
#
# With backup.env (backup.env.example is the template), each is encrypted
# with age as it is written, and named with .age at the end, then copied to a
# bucket off the machine. A backup leaves the machine only encrypted. Two
# weeks of backups stay here (KEEP_DAYS), 90 days in the bucket.
#
#   scripts/backup.sh                       # e.g. nightly from cron
#   BACKUP_DIR=/mnt/backups scripts/backup.sh
#
# It runs against deployments/docker-compose.prod.yml with .env. COMPOSE_FILE
# and ENV_FILE choose another stack, as for `docker compose`, and
# BACKUP_ENV_FILE another backup.env:
#
#   COMPOSE_FILE=deployments/docker-compose.prod.yml:deployments/staging/compose.yml \
#     ENV_FILE=deployments/staging/staging.env scripts/backup.sh
#
# Photos kept in S3 aren't included: keep that bucket with the provider's
# tools (versioning, replication). Backups hold the app secret and
# customers' details, so they are readable by their owner only.
set -euo pipefail
umask 077

cd "$(dirname "$0")/.."
export COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/kiflet}"
KEEP_DAYS="${KEEP_DAYS:-14}"
compose() { docker compose --env-file "$ENV_FILE" "$@"; }

# backup.env, if there is one. One named in BACKUP_ENV_FILE must exist.
settings_file="${BACKUP_ENV_FILE:-backup.env}"
if [ -n "${BACKUP_ENV_FILE:-}" ] || [ -f "$settings_file" ]; then
  set -a
  # shellcheck source=/dev/null
  . "$settings_file"
  set +a
fi
bucket="${BACKUP_S3_BUCKET:-}"
read -r -a recipients <<<"${BACKUP_AGE_RECIPIENTS:-}"
if [ -n "$bucket" ] && [ ${#recipients[@]} -eq 0 ]; then
  echo "backup.sh: BACKUP_S3_BUCKET is set and BACKUP_AGE_RECIPIENTS isn't. Backups leave the machine only encrypted." >&2
  exit 1
fi

age_args=()
for r in "${recipients[@]}"; do age_args+=(-r "$r"); done
ext=""
if [ ${#age_args[@]} -gt 0 ]; then
  command -v age >/dev/null || { echo "backup.sh: BACKUP_AGE_RECIPIENTS is set, and age isn't installed (apt install age)." >&2; exit 1; }
  # A key that isn't one stops the backup now, not after the dump.
  age "${age_args[@]}" </dev/null >/dev/null
  ext=".age"
fi
# Standard input to standard output, encrypted to the recipients if there
# are any: nothing unencrypted reaches the disk.
seal() { if [ -n "$ext" ]; then age "${age_args[@]}"; else cat; fi; }

mkdir -p "$BACKUP_DIR"
BACKUP_DIR="$(cd "$BACKUP_DIR" && pwd)"
# rclone in a container, with the backups at /backups, as their owner.
offsite() {
  COMPOSE_FILE="${BACKUP_COMPOSE_FILE:-deployments/docker-compose.backup.yml}" \
    docker compose --progress quiet --env-file /dev/null run --rm --no-deps -T \
    --user "$(id -u):$(id -g)" -v "$BACKUP_DIR:/backups" offsite "$@"
}

stamp="$(date +%Y%m%d_%H%M%S)"
db="$BACKUP_DIR/kiflet_${stamp}.dump$ext"
files="$BACKUP_DIR/kiflet_${stamp}_files.tar.gz$ext"
settings="$BACKUP_DIR/kiflet_${stamp}_settings.tar.gz$ext"
base=""
if [ "$(compose exec -T postgres psql -U logistics -d logistics -tAc 'SHOW archive_mode')" = on ]; then
  base="$BACKUP_DIR/kiflet_${stamp}_base.tar.gz$ext"
fi
# Written under a temporary name, so a failed run leaves nothing that looks
# like a backup.
trap 'rm -f "$db.partial" "$files.partial" "$settings.partial" ${base:+"$base.partial"}' EXIT

compose exec -T postgres pg_dump -U logistics -Fc logistics | seal >"$db.partial"
mv "$db.partial" "$db"
compose exec -T app tar -C /data -czf - . | seal >"$files.partial"
mv "$files.partial" "$files"
keep=("$ENV_FILE" deployments/secrets)
if [ -f "$settings_file" ]; then keep+=("$settings_file"); fi
tar -czf - "${keep[@]}" | seal >"$settings.partial"
mv "$settings.partial" "$settings"
if [ -n "$base" ]; then
  # One tar of the data directory with the WAL it needs to be consistent,
  # over the socket. A checkpoint first, so it starts at once.
  compose exec -T postgres pg_basebackup -U logistics -D - -Ft -X fetch -c fast | gzip | seal >"$base.partial"
  mv "$base.partial" "$base"
fi

if [ -n "$bucket" ]; then
  # rclone checks each file's size and checksum once it is in the bucket.
  offsite copy --include "${db##*/}" --include "${files##*/}" --include "${settings##*/}" \
    ${base:+--include "${base##*/}"} /backups "offsite:$bucket"
  keep_days="${BACKUP_S3_KEEP_DAYS:-90}"
  if [ "$keep_days" != 0 ]; then
    offsite delete --include 'kiflet_*' --min-age "${keep_days}d" "offsite:$bucket"
  fi
fi
find "$BACKUP_DIR" -name 'kiflet_*' -mtime "+${KEEP_DAYS}" -delete

echo "Database: $db"
echo "Files:    $files"
echo "Settings: $settings"
if [ -n "$base" ]; then echo "Base:     $base"; fi
if [ -n "$bucket" ]; then echo "Off-site: $bucket"; fi
echo "Restore with: ${ext:+AGE_IDENTITY=<private key file> }scripts/restore.sh $db $files"
