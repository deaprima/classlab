#!/bin/sh
# Puts a backup back on the server, REPLACING the current database and uploads. Meant for the day the
# server or its disk is lost, or a bad change has to be undone. Try ./restore-test.sh first.
#
#   ./restore.sh backups/db-20261022-020000.dump backups/uploads-20261022-020000.tar.gz
#
# The app is stopped while this runs and started again afterwards. It asks before it deletes anything;
# pass --yes to skip the question (for scripts).
set -eu

cd "$(dirname "$0")"
COMPOSE="docker compose -f docker-compose.prod.yml"

YES=0
if [ "${1:-}" = "--yes" ]; then YES=1; shift; fi
DUMP="${1:-}"
UPLOADS="${2:-}"
[ -n "$DUMP" ] && [ -s "$DUMP" ] || { echo "usage: $0 [--yes] DB_DUMP [UPLOADS_TAR_GZ]" >&2; exit 2; }
[ -z "$UPLOADS" ] || [ -s "$UPLOADS" ] || { echo "uploads archive not found: $UPLOADS" >&2; exit 2; }

echo "This REPLACES the live database with $DUMP${UPLOADS:+ and the uploads with $UPLOADS}."
if [ "$YES" -ne 1 ]; then
  printf 'Type "restore" to continue: '
  read -r answer
  [ "$answer" = "restore" ] || { echo "cancelled"; exit 1; }
fi

# Verify the archives before anything is deleted.
$COMPOSE exec -T postgres pg_restore --list < "$DUMP" > /dev/null
[ -z "$UPLOADS" ] || tar -tzf "$UPLOADS" > /dev/null

echo "stopping the app"
$COMPOSE stop caddy api

echo "restoring the database"
$COMPOSE exec -T postgres psql -U classlab -d postgres -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS classlab WITH (FORCE)" -c "CREATE DATABASE classlab OWNER classlab"
$COMPOSE exec -T postgres pg_restore -U classlab -d classlab --exit-on-error --no-owner < "$DUMP"

if [ -n "$UPLOADS" ]; then
  echo "restoring the uploads"
  docker run --rm -i -v classlab-prod_uploads:/data alpine:3.21 \
    sh -c 'find /data -mindepth 1 -delete && tar -xzf - -C /data && chown -R 10001:10001 /data' < "$UPLOADS"
fi

echo "starting the app"
$COMPOSE up -d
echo "done. Sign in and check a booking and a photo before telling anyone."
