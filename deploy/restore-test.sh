#!/bin/sh
# Proves that a backup can be restored, without touching the running app: the dump is loaded into a
# throwaway Postgres container and checked, then the container is removed. Run it after a backup, and
# on a schedule (the runbook says monthly): a backup nobody has restored is only a hope.
#
#   ./restore-test.sh                       # newest dump in ./backups
#   ./restore-test.sh backups/db-2026....dump
set -eu

cd "$(dirname "$0")"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
DUMP="${1:-}"
if [ -z "$DUMP" ]; then
  # shellcheck disable=SC2012
  DUMP="$(ls -1t "$BACKUP_DIR"/db-*.dump 2>/dev/null | head -n 1 || true)"
fi
[ -n "$DUMP" ] && [ -s "$DUMP" ] || { echo "no dump found (looked in $BACKUP_DIR)" >&2; exit 1; }

NAME="classlab-restore-test-$$"
cleanup() { docker rm -f "$NAME" > /dev/null 2>&1 || true; }
trap cleanup EXIT

echo "restoring $DUMP into a throwaway database"
docker run -d --name "$NAME" -e POSTGRES_PASSWORD=restore-test -e POSTGRES_DB=restore_test \
  postgres:16.14-alpine > /dev/null

i=0
until docker exec "$NAME" pg_isready -U postgres -d restore_test > /dev/null 2>&1; do
  i=$((i + 1))
  [ "$i" -le 30 ] || { echo "the test database did not start" >&2; exit 1; }
  sleep 1
done
# The dump was taken as user "classlab": it names that role as owner, so the role must exist.
docker exec "$NAME" psql -U postgres -d restore_test -q -c "CREATE ROLE classlab LOGIN" > /dev/null

docker exec -i "$NAME" pg_restore -U postgres -d restore_test --exit-on-error --no-owner < "$DUMP"

psql_value() { docker exec "$NAME" psql -U postgres -d restore_test -tA -c "$1"; }

# The migration table says how far the schema is; the app refuses nothing, but a restore that lost
# it would make the next `migrate up` try to rebuild the schema over the data.
VERSION="$(psql_value "SELECT version FROM schema_migrations")"
DIRTY="$(psql_value "SELECT dirty FROM schema_migrations")"
[ "$DIRTY" = "f" ] || { echo "schema_migrations is dirty after the restore" >&2; exit 1; }

echo "restored. migration version $VERSION; rows per table:"
for table in users rooms bookings attachments audit_logs; do
  printf '  %-12s %s\n' "$table" "$(psql_value "SELECT count(*) FROM $table")"
done
echo "OK: $DUMP can be restored."
