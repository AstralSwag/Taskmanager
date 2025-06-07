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

# Определяем имя подписки в зависимости от окружения
if [ "$POSTGRES_DB" = "replicator" ]; then
    SUB_NAME="site_sub_test"
else
    SUB_NAME="site_sub"
fi

# Проверяем, существует ли подписка
SUB_EXISTS=$(PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -tAc "SELECT 1 FROM pg_subscription WHERE subname = '$SUB_NAME'")
if [ "$SUB_EXISTS" = "1" ]; then
  echo "Subscription '$SUB_NAME' already exists. Skipping creation."
else
  # Удаляем старый слот на удалённой БД
  SLOT_EXISTS=$(PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d "$PLANE_DB" -tAc "SELECT 1 FROM pg_replication_slots WHERE slot_name = '$SUB_NAME'")
  if [ "$SLOT_EXISTS" = "1" ]; then
    echo "Removing existing replication slot on remote DB: $SUB_NAME"
    PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d "$PLANE_DB" -c "SELECT pg_drop_replication_slot('$SUB_NAME');"
  fi

  # Создаём подписку
  CONN_STR="host=$PLANE_HOST port=$PLANE_PORT user=$REPLICATOR_USER password=$REPLICATOR_PASSWORD dbname=$PLANE_DB sslmode=require sslrootcert=/etc/ssl/postgresql/root.crt"

  echo "Creating subscription '$SUB_NAME'..."
  PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "
    CREATE SUBSCRIPTION $SUB_NAME
    CONNECTION '$CONN_STR'
    PUBLICATION site_pub
    WITH (copy_data = true);
  "

  echo "Subscription created successfully."
fi