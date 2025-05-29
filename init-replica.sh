#!/bin/bash
set -e

# Создаем пользователя для репликации
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD '${DB_PASSWORD}';
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

# Создаем подписку
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE SUBSCRIPTION site_sub 
    CONNECTION 'host=${DB_HOST} port=${DB_PORT} user=replicator password=${DB_PASSWORD} dbname=${DB_NAME}' 
    PUBLICATION site_pub;
EOSQL
EOF

chmod +x /docker-entrypoint-initdb.d/subscribe.sh 