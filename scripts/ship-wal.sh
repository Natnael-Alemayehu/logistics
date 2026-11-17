#!/usr/bin/env bash
# Takes the database's archived WAL off the machine: the record of every
# change, which with a base backup from scripts/backup.sh restores the
# database to any moment since (scripts/restore-pitr.sh). It needs
# WAL_ARCHIVE=on in .env, and is run every few minutes from cron:
#
#   */5 * * * *  cd /srv/kiflet && scripts/ship-wal.sh
#
# Postgres writes each closed segment, compressed, to the wal-archive
# volume. This script encrypts each with age, as backup.sh does its files,
# keeps it in $BACKUP_DIR/wal, copies it to the bucket under wal/, and only
# then takes it out of the volume. With no bucket, the copies here are all
# there is. WAL stays two weeks here (KEEP_DAYS) and two weeks in the bucket
# (BACKUP_WAL_KEEP_DAYS), which is how far back a restore can reach.
#
# COMPOSE_FILE, ENV_FILE and BACKUP_ENV_FILE choose the stack and the
# settings, as for backup.sh.
set -euo pipefail
umask 077

cd "$(dirname "$0")/.."
export COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/kiflet}"
KEEP_DAYS="${KEEP_DAYS:-14}"
compose() { docker compose --env-file "$ENV_FILE" "$@"; }

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
  echo "ship-wal.sh: BACKUP_S3_BUCKET is set and BACKUP_AGE_RECIPIENTS isn't. WAL leaves the machine only encrypted." >&2
  exit 1
fi
age_args=()
for r in "${recipients[@]}"; do age_args+=(-r "$r"); done
ext=""
if [ ${#age_args[@]} -gt 0 ]; then
  command -v age >/dev/null || { echo "ship-wal.sh: BACKUP_AGE_RECIPIENTS is set, and age isn't installed (apt install age)." >&2; exit 1; }
  ext=".age"
fi
seal() { if [ -n "$ext" ]; then age "${age_args[@]}"; else cat; fi; }

mkdir -p "$BACKUP_DIR/wal"
BACKUP_DIR="$(cd "$BACKUP_DIR" && pwd)"
# One run at a time: cron starts the next whether or not this one is done.
exec 9>"$BACKUP_DIR/.ship-wal.lock"
flock -n 9 || { echo "ship-wal.sh: another run is still shipping." >&2; exit 0; }

offsite() {
  COMPOSE_FILE="${BACKUP_COMPOSE_FILE:-deployments/docker-compose.backup.yml}" \
    docker compose --progress quiet --env-file /dev/null run --rm --no-deps -T \
    --user "$(id -u):$(id -g)" -v "$BACKUP_DIR:/backups" offsite "$@"
}

# The closed segments, and the history file each new timeline starts with.
# A .part is one Postgres is still writing.
mapfile -t waiting < <(compose exec -T postgres sh -c 'cd /wal-archive && ls -1 | grep -v "\.part$"' || true)
shipped=()
for name in "${waiting[@]}"; do
  [ -n "$name" ] || continue
  out="$BACKUP_DIR/wal/$name$ext"
  if [ ! -f "$out" ]; then
    compose exec -T postgres cat "/wal-archive/$name" | seal >"$out.partial"
    mv "$out.partial" "$out"
  fi
  shipped+=("$name")
done

if [ -n "$bucket" ] && [ ${#shipped[@]} -gt 0 ]; then
  # rclone checks each file once it is in the bucket; what is there
  # already is skipped.
  offsite copy /backups/wal "offsite:$bucket/wal"
fi
# Out of the volume only once it is kept here and, with a bucket, there.
if [ ${#shipped[@]} -gt 0 ]; then
  printf '%s\n' "${shipped[@]}" | compose exec -T postgres sh -c 'cd /wal-archive && xargs rm -f --'
fi

if [ -n "$bucket" ]; then
  keep_days="${BACKUP_WAL_KEEP_DAYS:-14}"
  if [ "$keep_days" != 0 ]; then
    offsite delete --min-age "${keep_days}d" "offsite:$bucket/wal"
  fi
fi
find "$BACKUP_DIR/wal" -type f -mtime "+${KEEP_DAYS}" -delete

echo "Shipped ${#shipped[@]} WAL file(s)${bucket:+ to $bucket/wal}."
