#!/bin/bash
set -e

# Проверяем переменные окружения
if [ -z "$PLANE_HOST" ] || [ -z "$PLANE_PORT" ]; then
    echo "Error: Missing required environment variables for remote DB"
    exit 1
fi

if [ -z "$REPLICATOR_USER" ] || [ -z "$REPLICATOR_PASSWORD" ] || [ -z "$POSTGRES_DB" ]; then
    echo "Error: Missing required environment variables for local DB"
    exit 1
fi

if [ -z "$PLANE_DB" ]; then
    echo "Error: Missing PLANE_DB environment variable for remote DB name"
    exit 1
fi

# Ждём запуска PostgreSQL
until PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c '\q'; do
  echo "Waiting for PostgreSQL to start..."
  sleep 2
done

# Проверяем, существует ли подписка
SUB_EXISTS=$(PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -tAc "SELECT 1 FROM pg_subscription WHERE subname = 'site_sub'")
if [ "$SUB_EXISTS" = "1" ]; then
  echo "Subscription 'site_sub' already exists. Skipping creation."
else
  # Удаляем старый слот на удалённой БД
  SLOT_EXISTS=$(PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d "$PLANE_DB" -tAc "SELECT 1 FROM pg_replication_slots WHERE slot_name = 'site_sub'")
  if [ "$SLOT_EXISTS" = "1" ]; then
    echo "Removing existing replication slot on remote DB: site_sub"
    PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d "$PLANE_DB" -c "SELECT pg_drop_replication_slot('site_sub');"
  fi

  # Создаём подписку
  CONN_STR="host=n8n.it4retail.tech port=$PLANE_PORT user=$REPLICATOR_USER password=$REPLICATOR_PASSWORD dbname=$PLANE_DB sslmode=require sslrootcert=/etc/ssl/postgresql/root.crt sslsni=0"

  echo "Creating subscription 'site_sub'..."
  PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "
    CREATE SUBSCRIPTION site_sub
    CONNECTION '$CONN_STR'
    PUBLICATION site_pub
    WITH (copy_data = true);
  "

  echo "Subscription created successfully."
fi