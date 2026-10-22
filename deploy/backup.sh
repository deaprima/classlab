#!/bin/sh
# Daily backup of everything the app cannot rebuild: the database and the uploaded files (letters and
# check-out photos). Run from anywhere on the server; by cron once a day (deploy/README.md).
#
#   ./backup.sh                 -> backups/db-YYYYMMDD-HHMMSS.dump and backups/uploads-YYYYMMDD-HHMMSS.tar.gz
#   BACKUP_DIR=/mnt/usb ./backup.sh
#   RETENTION_DAYS=30 ./backup.sh
#
# A backup is only kept when it was read back successfully; a failure leaves the older ones alone
# and exits with an error, so cron mails it.
set -eu

cd "$(dirname "$0")"
COMPOSE="docker compose -f docker-compose.prod.yml"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
STAMP="$(date +%Y%m%d-%H%M%S)"

mkdir -p "$BACKUP_DIR"
DB_FILE="$BACKUP_DIR/db-$STAMP.dump"
UPLOADS_FILE="$BACKUP_DIR/uploads-$STAMP.tar.gz"

cleanup() { rm -f "$DB_FILE.part" "$UPLOADS_FILE.part"; }
trap cleanup EXIT

echo "[$(date -Iseconds)] dumping the database"
# Custom format: compressed, and pg_restore can pick what to restore.
$COMPOSE exec -T postgres pg_dump -U classlab -d classlab --format=custom > "$DB_FILE.part"
# Read the archive back: a truncated or empty dump fails here instead of at 2 a.m. on restore day.
$COMPOSE exec -T postgres pg_restore --list < "$DB_FILE.part" > /dev/null
mv "$DB_FILE.part" "$DB_FILE"

echo "[$(date -Iseconds)] archiving the uploads"
# The volume is read-only for this container, so a mistake here cannot touch the files.
docker run --rm -v classlab-prod_uploads:/data:ro alpine:3.21 tar -czf - -C /data . > "$UPLOADS_FILE.part"
tar -tzf "$UPLOADS_FILE.part" > /dev/null
mv "$UPLOADS_FILE.part" "$UPLOADS_FILE"

echo "[$(date -Iseconds)] removing backups older than $RETENTION_DAYS days"
find "$BACKUP_DIR" -maxdepth 1 \( -name 'db-*.dump' -o -name 'uploads-*.tar.gz' \) -mtime "+$RETENTION_DAYS" -print -delete

echo "[$(date -Iseconds)] done: $DB_FILE ($(wc -c < "$DB_FILE") bytes), $UPLOADS_FILE ($(wc -c < "$UPLOADS_FILE") bytes)"
