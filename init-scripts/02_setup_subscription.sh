#!/bin/bash
set -e

# Проверяем переменные окружения
if [ -z "$PLANE_HOST" ] || [ -z "$PLANE_PORT" ] || [ -z "$POSTGRES_USER" ] || [ -z "$POSTGRES_PASSWORD" ]; then
    echo "Error: Required environment variables are not set"
    exit 1
fi

# Ждем, пока PostgreSQL будет готов
until PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c '\q'; do
  echo "Waiting for PostgreSQL to start..."
  sleep 2
done

# Создаем подписку
CONN_STR="host=$PLANE_HOST port=$PLANE_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=plane"

echo "Creating subscription..."
PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<EOF
DO
\$do\$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_subscription WHERE subname = 'site_sub') THEN
        CREATE SUBSCRIPTION site_sub
        CONNECTION '$CONN_STR'
        PUBLICATION site_pub;
    END IF;
END
\$do\$;
EOF