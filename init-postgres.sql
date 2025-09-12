-- Создание таблицы для планов пользователей
CREATE TABLE IF NOT EXISTS planfact_plans (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    date DATE NOT NULL,
    plan TEXT NOT NULL,
    created_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
        UNIQUE (user_id, date)
);

-- Создание индексов для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_planfact_plans_user_date ON planfact_plans (user_id, date);

CREATE INDEX IF NOT EXISTS idx_planfact_plans_created_at ON planfact_plans (created_at DESC);

-- Таблица attendance создается автоматически в коде приложения
-- но добавим её сюда для полноты
CREATE TABLE IF NOT EXISTS planfact_attendance (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    is_office BOOLEAN NOT NULL DEFAULT false,
    is_today BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
        date_part DATE GENERATED ALWAYS AS (
            DATE(
                created_at AT TIME ZONE 'Europe/Moscow'
            )
        ) STORED,
        UNIQUE (user_id, date_part, is_today)
);

-- Индексы для таблицы attendance
CREATE INDEX IF NOT EXISTS idx_planfact_attendance_user_created ON planfact_attendance (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_planfact_attendance_date_part ON planfact_attendance (date_part);