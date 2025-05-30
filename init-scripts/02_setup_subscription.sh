#!/bin/bash
set -e

if [ -z "$PLANE_HOST" ] || [ -z "$PLANE_PORT" ] || [ -z "$POSTGRES_USER" ] || [ -z "$POSTGRES_PASSWORD" ]; then
    echo "Error: Required environment variables are not set"
    exit 1
fi

until PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c '\q'; do
  echo "Waiting for PostgreSQL to start..."
  sleep 2
done

# Проверяем наличие подписки локально
SUB_EXISTS=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "SELECT 1 FROM pg_subscription WHERE subname = 'site_sub'")

if [ "$SUB_EXISTS" = "1" ]; then
  echo "Subscription 'site_sub' already exists. Skipping creation."
else
  # Проверяем, существует ли слот на удалённой БД
  SLOT_EXISTS=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d plane -tAc "SELECT 1 FROM pg_replication_slots WHERE slot_name = 'site_sub'")

  if [ "$SLOT_EXISTS" = "1" ]; then
    echo "Removing existing replication slot on remote DB: site_sub"
    PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d plane -c "SELECT pg_drop_replication_slot('site_sub');"
  fi

  echo "Creating subscription 'site_sub'..."
  CONN_STR="host=$PLANE_HOST port=$PLANE_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=plane"

  PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
    CREATE SUBSCRIPTION site_sub
    CONNECTION '$CONN_STR'
    PUBLICATION site_pub;
  "

  echo "Subscription created successfully."
fi