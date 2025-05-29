#!/bin/bash
set -e

# Создаем пользователя для репликации, если его нет
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    DO
    \$do\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'replicator') THEN
            CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD '${DB_PASSWORD}';
        END IF;
    END
    \$do\$;
EOSQL

# Настраиваем параметры репликации
cat >> /var/lib/postgresql/data/postgresql.conf <<EOF
wal_level = replica
max_wal_senders = 10
max_replication_slots = 10
hot_standby = on
EOF

# Настраиваем доступ для репликации
cat >> /var/lib/postgresql/data/pg_hba.conf <<EOF
host replication replicator all md5
EOF

# Создаем скрипт для подписки на публикацию
cat > /docker-entrypoint-initdb.d/subscribe.sh <<EOF
#!/bin/bash
set -e

# Ждем, пока база данных будет готова
until pg_isready -h ${DB_HOST} -p ${DB_PORT} -U replicator; do
    echo "Waiting for primary database..."
    sleep 2
done

# Создаем подписку, если её нет
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    DO
    \$do\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_subscription WHERE subname = 'site_sub') THEN
            CREATE SUBSCRIPTION site_sub 
            CONNECTION 'host=${DB_HOST} port=${DB_PORT} user=replicator password=${DB_PASSWORD} dbname=${DB_NAME}' 
            PUBLICATION site_pub;
        END IF;
    END
    \$do\$;
EOSQL
EOF

chmod +x /docker-entrypoint-initdb.d/subscribe.sh 