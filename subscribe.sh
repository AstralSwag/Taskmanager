#!/bin/bash
set -e

# Проверка доступности удаленной БД
until PGPASSWORD="${REPLICATOR_PASSWORD}" psql -h "${PLANE_HOST}" -p "${PLANE_PORT}" -U "${REPLICATOR_USER}" -d plane -c '\q'; do
    echo "Waiting for primary database at ${PLANE_HOST}:${PLANE_PORT}..."
    sleep 2
done

# Формирование строки подключения
CONN_STR="host=${PLANE_HOST} port=${PLANE_PORT} user=${REPLICATOR_USER} password=${REPLICATOR_PASSWORD} dbname=plane"

# Создание подписки
PGPASSWORD="${REPLICATOR_PASSWORD}" psql -v ON_ERROR_STOP=1 -U "${REPLICATOR_USER}" -d replicator <<EOF
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
EOF 