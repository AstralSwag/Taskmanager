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

# Настраиваем доступ для репликации и локальных подключений
cat > /var/lib/postgresql/data/pg_hba.conf <<EOF
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all            postgres                                peer
local   all            all                                     md5
host    all            all             127.0.0.1/32           md5
host    all            all             ::1/128                 md5
host    all            all             0.0.0.0/0              md5
host    replication    replicator      0.0.0.0/0              md5
EOF

# Создаем скрипт для подписки на публикацию
cat > /var/lib/postgresql/subscribe.sh <<EOF
#!/bin/bash
set -e

# Проверяем и устанавливаем значения по умолчанию для переменных окружения
DB_HOST=\${DB_HOST:-"postgres"}
DB_PORT=\${DB_PORT:-"5432"}
DB_PASSWORD=\${DB_PASSWORD:-"replicator"}
DB_NAME=\${DB_NAME:-"replicator"}
POSTGRES_USER=\${POSTGRES_USER:-"replicator"}

echo "Using connection parameters:"
echo "Host: \$DB_HOST"
echo "Port: \$DB_PORT"
echo "Database: \$DB_NAME"
echo "User: \$POSTGRES_USER"

# Ждем, пока база данных будет готова
until PGPASSWORD=\$DB_PASSWORD pg_isready -h "\$DB_HOST" -p "\$DB_PORT" -U "\$POSTGRES_USER"; do
    echo "Waiting for primary database at \$DB_HOST:\$DB_PORT..."
    sleep 2
done

# Создаем подписку, если её нет
psql -v ON_ERROR_STOP=1 --username "\$POSTGRES_USER" --dbname "\$DB_NAME" <<-EOSQL
    DO
    \$do\$
    BEGIN
        IF NOT EXISTS (SELECT 1 FROM pg_subscription WHERE subname = 'site_sub') THEN
            CREATE SUBSCRIPTION site_sub 
            CONNECTION 'host=\$DB_HOST port=\$DB_PORT user=\$POSTGRES_USER password=\$DB_PASSWORD dbname=\$DB_NAME' 
            PUBLICATION site_pub;
        END IF;
    END
    \$do\$;
EOSQL
EOF

# Устанавливаем правильные права доступа
chmod 755 /var/lib/postgresql/subscribe.sh
chown postgres:postgres /var/lib/postgresql/subscribe.sh

# Запускаем скрипт подписки
su postgres -c "/var/lib/postgresql/subscribe.sh"
