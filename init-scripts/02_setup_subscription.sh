#!/bin/bash
set -e

# Проверяем, заданы ли переменные
if [ -z "$PLANE_HOST" ] || [ -z "$PLANE_PORT" ] || [ -z "$POSTGRES_USER" ] || [ -z "$POSTGRES_PASSWORD" ]; then
    echo "Error: Required environment variables are not set"
    exit 1
fi

# Ждем, пока PostgreSQL запустится
until PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c '\q'; do
  echo "Waiting for PostgreSQL to start..."
  sleep 2
done

# Проверяем, существует ли подписка
SUB_EXISTS=$(PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "SELECT 1 FROM pg_subscription WHERE subname = 'site_sub'")

if [ "$SUB_EXISTS" = "1" ]; then
  echo "Subscription 'site_sub' already exists. Skipping creation."
else
  echo "Creating subscription 'site_sub'..."
  CONN_STR="host=$PLANE_HOST port=$PLANE_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=plane"

  # Создаем подписку напрямую (без блока DO)
  PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
    CREATE SUBSCRIPTION site_sub
    CONNECTION '$CONN_STR'
    PUBLICATION site_pub;
  "

  echo "Subscription created successfully."
fi