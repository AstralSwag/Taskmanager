#!/bin/bash
set -e

echo "Downloading schema from remote DB..."

SCHEMA_FILE="/var/lib/postgresql/remote_schema.sql"
DATA_FILE="/var/lib/postgresql/remote_data.sql"

# Список только необходимых таблиц в правильном порядке
TABLES=(
    users
    workspaces
    states
    projects
    estimates
    estimate_points
    issues
    issue_assignees
)

echo "Tables to be copied: ${TABLES[*]}"

# Сначала дамп схемы
echo "Dumping schema..."
PGPASSWORD="$PLANE_PASSWORD" pg_dump -h "$PLANE_HOST" \
        -p "$PLANE_PORT" \
        -U "$PLANE_USER" \
        -d "$PLANE_DB" \
        --schema-only \
        $(printf -- "--table=%s " "${TABLES[@]}") \
        > "$SCHEMA_FILE"

echo "Schema dump completed. Size: $(du -h "$SCHEMA_FILE" | cut -f1)"

echo "Applying schema to local DB..."
PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -f "$SCHEMA_FILE"

# Проверяем, что таблицы созданы
echo "Verifying tables were created..."
PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "\dt"

# Отключаем триггеры и ограничения
echo "Disabling triggers and constraints..."
PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "SET session_replication_role = 'replica';"

# Теперь дамп данных
echo "Dumping data..."
PGPASSWORD="$PLANE_PASSWORD" pg_dump -h "$PLANE_HOST" \
        -p "$PLANE_PORT" \
        -U "$PLANE_USER" \
        -d "$PLANE_DB" \
        --data-only \
        --disable-triggers \
        $(printf -- "--table=%s " "${TABLES[@]}") \
        > "$DATA_FILE"

echo "Data dump completed. Size: $(du -h "$DATA_FILE" | cut -f1)"

echo "Applying data to local DB..."
PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -f "$DATA_FILE"

# Включаем триггеры и ограничения обратно
echo "Enabling triggers and constraints..."
PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "SET session_replication_role = 'origin';"

# Проверяем количество записей в основных таблицах
echo "Verifying data was copied..."
for table in "${TABLES[@]}"; do
    echo "Checking table $table..."
    PGPASSWORD="$REPLICATOR_PASSWORD" psql -U "$REPLICATOR_USER" -d "$POSTGRES_DB" -c "SELECT COUNT(*) FROM $table;"
done

echo "Schema and data import completed successfully." 