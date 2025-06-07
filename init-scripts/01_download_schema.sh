#!/bin/bash
set -e

echo "Downloading schema from remote DB..."

OUTPUT_FILE="/var/lib/postgresql/remote_schema.sql"

# Список таблиц, которые нужно реплицировать
TABLES=(
  states
  projects
  estimates
  estimate_points
  issues
  issue_assignees
)

# Формируем команду pg_dump
PGPASSWORD="$REPLICATOR_PASSWORD" pg_dump -h "$PLANE_HOST" \
        -p "$PLANE_PORT" \
        -U "$REPLICATOR_USER" \
        -d "$PLANE_DB" \
        --schema-only \
        $(printf -- "--table=%s " "${TABLES[@]}") \
        > "$OUTPUT_FILE"

echo "Applying schema to local DB..."
PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -f "$OUTPUT_FILE"