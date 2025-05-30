#!/bin/bash
set -e

PGPASSWORD="$DB_PASSWORD" psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Создаем таблицу states
    CREATE TABLE IF NOT EXISTS states (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        color VARCHAR(50)
    );

    -- Создаем таблицу projects
    CREATE TABLE IF NOT EXISTS projects (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description TEXT,
        identifier VARCHAR(50) NOT NULL
    );

    -- Создаем таблицу estimates
    CREATE TABLE IF NOT EXISTS estimates (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description TEXT,
        type VARCHAR(50)
    );

    -- Создаем таблицу estimate_points
    CREATE TABLE IF NOT EXISTS estimate_points (
        id UUID PRIMARY KEY,
        estimate_id UUID REFERENCES estimates(id),
        value VARCHAR(50)
    );

    -- Создаем таблицу issues
    CREATE TABLE IF NOT EXISTS issues (
        id UUID PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        description_stripped TEXT,
        priority VARCHAR(50),
        state_id UUID REFERENCES states(id),
        point INTEGER,
        project_id UUID REFERENCES projects(id),
        sequence_id INTEGER,
        created_at TIMESTAMP WITH TIME ZONE,
        deleted_at TIMESTAMP WITH TIME ZONE,
        estimate_point_id UUID REFERENCES estimate_points(id)
    );

    -- Создаем таблицу issue_assignees
    CREATE TABLE IF NOT EXISTS issue_assignees (
        issue_id UUID REFERENCES issues(id),
        assignee_id UUID,
        created_at TIMESTAMP WITH TIME ZONE,
        deleted_at TIMESTAMP WITH TIME ZONE,
        PRIMARY KEY (issue_id, assignee_id)
    );
EOSQL 