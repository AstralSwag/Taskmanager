#!/bin/bash
set -e
set -x

echo "DEBUG: Environment variables:"
echo "-----------------------------"
printenv | grep -E 'POSTGRES_DB|REPLICATOR_USER|REPLICATOR_PASSWORD|PLANE_DB|PLANE_HOST|PLANE_PORT'
echo "-----------------------------"

# Проверяем необходимые переменные окружения
if [ -z "$PLANE_HOST" ] || [ -z "$PLANE_PORT" ] || [ -z "$REPLICATOR_USER" ] || [ -z "$REPLICATOR_PASSWORD" ] || [ -z "$POSTGRES_DB" ] || [ -z "$PLANE_DB" ]; then
  echo "Error: Required environment variables are not set"
  exit 1
fi

# Ждём, пока локальная БД запустится
until PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c '\q'; do
  echo "Waiting for PostgreSQL to start..."
  sleep 1
done

echo "PostgreSQL is up - executing command"

# Проверяем, существует ли подписка на локальной БД
SUB_EXISTS=$(PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -tAc "SELECT 1 FROM pg_subscription WHERE subname = 'site_sub'")

if [ "$SUB_EXISTS" = "1" ]; then
  echo "Subscription 'site_sub' already exists. Skipping creation."
  exit 0
fi

# Проверяем, существует ли слот на удалённой БД
SLOT_EXISTS=$(PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d "$PLANE_DB" -tAc "SELECT 1 FROM pg_replication_slots WHERE slot_name = 'site_sub'")

if [ "$SLOT_EXISTS" = "1" ]; then
  echo "Removing existing replication slot on remote DB: site_sub"
  PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -h "$PLANE_HOST" -p "$PLANE_PORT" -d "$PLANE_DB" -c "SELECT pg_drop_replication_slot('site_sub');"
fi

# Создаём подписку на локальной БД
echo "Creating subscription 'site_sub'..."
CONN_STR="host=$PLANE_HOST port=$PLANE_PORT user=$REPLICATOR_USER password=$REPLICATOR_PASSWORD dbname=$PLANE_DB"

PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "
    CREATE SUBSCRIPTION site_sub
    CONNECTION '$CONN_STR'
    PUBLICATION site_pub
    WITH (copy_data = true, create_slot = true);
"

# Modify tables to handle conflicts
PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "
    ALTER TABLE issue_assignees REPLICA IDENTITY FULL;
    ALTER TABLE issues REPLICA IDENTITY FULL;
    ALTER TABLE projects REPLICA IDENTITY FULL;
    ALTER TABLE states REPLICA IDENTITY FULL;
    ALTER TABLE estimates REPLICA IDENTITY FULL;
    ALTER TABLE estimate_points REPLICA IDENTITY FULL;
"

echo "Subscription created successfully."