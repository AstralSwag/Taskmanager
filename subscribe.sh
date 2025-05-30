#!/bin/bash
set -e

# Проверяем наличие необходимых переменных окружения
if [ -z "$PLANE_HOST" ] || [ -z "$PLANE_PORT" ] || [ -z "$REPLICATOR_USER" ] || [ -z "$REPLICATOR_PASSWORD" ]; then
    echo "Error: Required environment variables are not set"
    echo "PLANE_HOST: $PLANE_HOST"
    echo "PLANE_PORT: $PLANE_PORT"
    echo "REPLICATOR_USER: $REPLICATOR_USER"
    echo "REPLICATOR_PASSWORD: [hidden]"
    exit 1
fi

# Проверка доступности удаленной БД
until PGPASSWORD="${REPLICATOR_PASSWORD}" psql -h "${PLANE_HOST}" -p "${PLANE_PORT}" -U "${REPLICATOR_USER}" -d plane -c '\q'; do
    echo "Waiting for primary database at ${PLANE_HOST}:${PLANE_PORT}..."
    sleep 2
done

# Формирование строки подключения
CONN_STR="host=${PLANE_HOST} port=${PLANE_PORT} user=${REPLICATOR_USER} password=${REPLICATOR_PASSWORD} dbname=plane"

echo "Creating subscription with connection string: host=${PLANE_HOST} port=${PLANE_PORT} user=${REPLICATOR_USER} dbname=plane"

# Создание подписки
su postgres -c "PGPASSWORD='${REPLICATOR_PASSWORD}' psql -v ON_ERROR_STOP=1 -U '${REPLICATOR_USER}' -d replicator <<EOF
DO
\$do\$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_subscription WHERE subname = 'site_sub') THEN
        CREATE SUBSCRIPTION site_sub
        CONNECTION '${CONN_STR}'
        PUBLICATION site_pub;
    END IF;
END
\$do\$;
EOF" 