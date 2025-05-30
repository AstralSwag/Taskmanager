#!/bin/bash
set -e

# Настраиваем параметры репликации
cat >> /var/lib/postgresql/data/postgresql.conf <<EOF
wal_level = replica
max_wal_senders = 10
max_replication_slots = 10
hot_standby = on
EOF

# Настраиваем доступ для репликации и локальных подключений
cat > /var/lib/postgresql/data/pg_hba.conf <<EOF
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all            all                                     trust
host    all            all             127.0.0.1/32           trust
host    all            all             ::1/128                 trust
host    all            all             0.0.0.0/0              trust
host    ${POSTGRES_DB} ${POSTGRES_USER}    0.0.0.0/0        trust
EOF

# Перезапускаем PostgreSQL для применения изменений
pg_ctl -D /var/lib/postgresql/data restart

# Ждем, пока PostgreSQL будет готов
until pg_isready; do
    echo "Waiting for PostgreSQL to be ready..."
    sleep 2
done

# Создаем пользователя для репликации
PGPASSWORD="${POSTGRES_PASSWORD}" psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" <<-EOSQL
    DO
    \$do\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '${REPLICATOR_USER}') THEN
            CREATE USER ${REPLICATOR_USER} WITH REPLICATION ENCRYPTED PASSWORD '${REPLICATOR_PASSWORD}';
        END IF;
    END
    \$do\$;
EOSQL

# Создаем скрипт для подписки на публикацию
cat > /var/lib/postgresql/subscribe.sh <<'EOF'
#!/bin/bash
set -e

# Проверка доступности БД
until pg_isready; do
    echo "Waiting for primary database at $DB_HOST:$DB_PORT..."
    sleep 2
done

# Формирование строки подключения
CONN_STR="host=${DB_HOST} port=${DB_PORT} user=${POSTGRES_USER} password=${DB_PASSWORD} dbname=plane"

# Создание подписки
PGPASSWORD="$DB_PASSWORD" psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d replicator <<EOF
CREATE SUBSCRIPTION site_sub
CONNECTION '${CONN_STR}'
PUBLICATION site_pub;
EOF

# Устанавливаем правильные права доступа
chmod 755 /var/lib/postgresql/subscribe.sh
chown postgres:postgres /var/lib/postgresql/subscribe.sh

var/lib/postgresql/subscribe.sh
