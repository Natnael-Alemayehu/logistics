#!/usr/bin/env bash
# Restores the database to a moment: a base backup from scripts/backup.sh,
# brought forward with the WAL scripts/ship-wal.sh kept, to the time given or
# as far as the WAL goes. It replaces the database; the app is stopped
# meanwhile. The app's files aren't touched (scripts/restore.sh has them).
#
#   scripts/restore-pitr.sh /var/backups/kiflet/kiflet_20261117_021500_base.tar.gz.age \
#     '2026-11-17 14:04:00+03'
#
# Give the time with its offset (+03 is Addis Ababa): without one it is UTC.
# Pick a moment just before the mistake to be undone. With no time, it
# restores everything the WAL holds.
#
# The WAL comes from $BACKUP_DIR/wal (WAL_DIR names another directory; on a
# new machine scripts/fetch-backup.sh fills it from the bucket), and from
# the wal-archive volume, which holds what isn't shipped yet. Encrypted files
# (.age) need AGE_IDENTITY, as for restore.sh, and are all decrypted before
# anything is replaced. COMPOSE_FILE and ENV_FILE choose the stack; it asks
# for the stack's name before replacing anything, and CONFIRM=yes skips the
# question.
set -euo pipefail
umask 077

cd "$(dirname "$0")/.."
if [ $# -lt 1 ] || [ $# -gt 2 ]; then
  echo "usage: $0 <kiflet_…_base.tar.gz[.age]> ['<YYYY-MM-DD HH:MM:SS+03>']" >&2
  exit 2
fi
base="$1"
target="${2:-}"
[ -r "$base" ] || { echo "cannot read $base" >&2; exit 1; }
export COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/kiflet}"
WAL_DIR="${WAL_DIR:-$BACKUP_DIR/wal}"
compose() { docker compose --env-file "$ENV_FILE" "$@"; }
pgdata=/var/lib/postgresql/data

# Beside the backup, which has room for it.
plain="$(mktemp -d "$(dirname "$base")/.restore-pitr.XXXXXX")"
trap 'rm -rf "$plain"' EXIT
open() {
  case "$1" in
    *.age)
      [ -n "${AGE_IDENTITY:-}" ] || { echo "$1 is encrypted: set AGE_IDENTITY to the file holding its private key." >&2; exit 2; }
      age -d -i "$AGE_IDENTITY" -o "$2" "$1" || { echo "Cannot decrypt $1 with $AGE_IDENTITY. Nothing changed." >&2; exit 1; }
      ;;
    *) cp "$1" "$2" ;;
  esac
}
if [[ "$base" == *.age ]]; then command -v age >/dev/null || { echo "The backup is encrypted, and age isn't installed (apt install age)." >&2; exit 1; }; fi
open "$base" "$plain/base.tar.gz"

# The first WAL segment the base backup needs: older ones are of no use.
label="$(gunzip -c "$plain/base.tar.gz" | tar -xOf - backup_label)" || { echo "$base isn't a base backup. Nothing changed." >&2; exit 1; }
first="$(sed -n 's/^START WAL LOCATION: .*(file \([0-9A-F]\{24\}\))$/\1/p' <<<"$label")"
[ -n "$first" ] || { echo "$base has no start in its backup_label. Nothing changed." >&2; exit 1; }

mkdir "$plain/pitr-wal"
count=0
if [ -d "$WAL_DIR" ]; then
  for f in "$WAL_DIR"/*; do
    [ -f "$f" ] || continue
    name="$(basename "${f%.age}")" # 000000010000000000000003.gz, 00000002.history.gz
    seg="${name%.gz}"
    case "$seg" in
      *.history) ;;
      *) [[ ! "${seg:0:24}" < "$first" ]] || continue ;;
    esac
    open "$f" "$plain/pitr-wal/$name"
    count=$((count + 1))
  done
fi
echo "The base backup starts at $first; $count WAL file(s) from $WAL_DIR."

name="$(compose config | sed -n 's/^name: //p')"
if [ "${CONFIRM:-}" != yes ]; then
  read -r -p "This replaces the database of \"$name\" with the backup brought to ${target:-the end of its WAL}. Type \"$name\" to go on: " answer
  [ "$answer" = "$name" ] || { echo "Nothing changed."; exit 1; }
fi

compose stop app
# What Postgres is still writing goes to the archive too: a switch to a new
# segment closes the current one, which a clean stop archives.
compose exec -T postgres psql -U logistics -d postgres -qtAc 'SELECT pg_switch_wal()' >/dev/null 2>&1 || true
compose stop postgres

# The data directory replaced by the base backup, with the WAL beside it
# and the settings that make Postgres replay it, from the WAL here first and
# then from what the archive holds.
restore_command="f=%f; if [ -f $pgdata/pitr-wal/\$f.gz ]; then gunzip -c $pgdata/pitr-wal/\$f.gz > %p; elif [ -f /wal-archive/\$f.gz ]; then gunzip -c /wal-archive/\$f.gz > %p; else exit 1; fi"
settings="restore_command = '$restore_command'
recovery_target_timeline = 'latest'"
if [ -n "$target" ]; then
  settings="$settings
recovery_target_time = '$target'
recovery_target_action = 'promote'"
fi
compose run --rm --no-deps -T --entrypoint sh postgres -c \
  "find $pgdata -mindepth 1 -delete && gunzip -c | tar -C $pgdata -xf -" <"$plain/base.tar.gz"
tar -C "$plain" -cf - pitr-wal | compose run --rm --no-deps -T --entrypoint sh postgres -c "
  tar -C $pgdata -xf - &&
  printf '%s\n' \"\$0\" >>$pgdata/postgresql.auto.conf &&
  touch $pgdata/recovery.signal &&
  chown -R postgres:postgres $pgdata && chmod 700 $pgdata" "$settings"

started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
compose start postgres
echo "Replaying the WAL…"
while :; do
  state="$(compose exec -T postgres psql -U logistics -d postgres -qtAc 'SELECT pg_is_in_recovery()' 2>/dev/null || true)"
  [ "$state" = f ] && break
  if compose logs --no-log-prefix --since "$started" postgres 2>&1 | grep -q 'FATAL:'; then
    compose logs --no-log-prefix --since "$started" postgres 2>&1 | grep -E 'FATAL|LOG:  (recovery|redo|last completed)' | tail -n 8 >&2
    compose stop postgres
    echo "The database couldn't be brought to ${target:-the end of the WAL}, and is stopped. Try an earlier time, a base backup from before it, or restore.sh with a nightly dump." >&2
    exit 1
  fi
  sleep 2
done

# Done: the recovery settings go, and the WAL that was copied in.
compose exec -T postgres psql -U logistics -d postgres -q -v ON_ERROR_STOP=1 \
  -c 'ALTER SYSTEM RESET restore_command' -c 'ALTER SYSTEM RESET recovery_target_timeline' \
  -c 'ALTER SYSTEM RESET recovery_target_time' -c 'ALTER SYSTEM RESET recovery_target_action' \
  -c 'SELECT pg_reload_conf()' >/dev/null
compose exec -T postgres rm -rf "$pgdata/pitr-wal"
until_="$(compose exec -T postgres psql -U logistics -d postgres -qtAc "SELECT to_char(pg_last_xact_replay_timestamp() AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS') || ' UTC'" 2>/dev/null || true)"
compose start app
echo "Restored${until_:+ to the last change before $until_}. The app is starting again."
echo "The database is on a new timeline: take a backup now (scripts/backup.sh), the base of what follows."
