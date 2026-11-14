#!/usr/bin/env bash
# Restores a backup made by scripts/backup.sh: the database, and the app's
# volume when the files archive is given. Both replace what is there now. The
# app is stopped meanwhile.
#
#   scripts/restore.sh /var/backups/menged/menged_20261104_020000.dump \
#     /var/backups/menged/menged_20261104_020000_files.tar.gz
#
# COMPOSE_FILE and ENV_FILE choose the stack, as for backup.sh. It asks for
# the stack's name before replacing anything; CONFIRM=yes skips the question.
set -euo pipefail

cd "$(dirname "$0")/.."
if [ $# -lt 1 ] || [ $# -gt 2 ]; then
  echo "usage: $0 <database.dump> [<files.tar.gz>]" >&2
  exit 2
fi
db="$1"
files="${2:-}"
for f in "$db" ${files:+"$files"}; do
  [ -r "$f" ] || { echo "cannot read $f" >&2; exit 1; }
done
export COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env}"
compose() { docker compose --env-file "$ENV_FILE" "$@"; }

name="$(compose config | sed -n 's/^name: //p')"
if [ "${CONFIRM:-}" != yes ]; then
  what="the database"
  [ -n "$files" ] && what="the database and the app's files"
  read -r -p "This replaces $what of \"$name\" with the backup. Type \"$name\" to go on: " answer
  [ "$answer" = "$name" ] || { echo "Nothing changed."; exit 1; }
fi

compose stop app
# Into a new database, so nothing of the current one stays behind.
compose exec -T postgres psql -U logistics -d postgres -q -v ON_ERROR_STOP=1 \
  -c 'DROP DATABASE IF EXISTS logistics WITH (FORCE)' \
  -c 'CREATE DATABASE logistics OWNER logistics'
compose exec -T postgres pg_restore -U logistics -d logistics --no-owner --exit-on-error <"$db"
if [ -n "$files" ]; then
  # The app's own container has a read-only root; a one-off container of
  # its image writes to the volume.
  compose run --rm --no-deps -T --entrypoint sh app \
    -c 'find /data -mindepth 1 -delete && tar -C /data -xzf -' <"$files"
fi
compose start app
echo "Restored. The app is starting again: \`docker compose ps\` shows it healthy once it is."
