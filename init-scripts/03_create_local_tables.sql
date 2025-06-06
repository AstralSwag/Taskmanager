-- Создаем таблицу для хранения статусов посещения офиса
CREATE TABLE IF NOT EXISTS attendance (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    is_office BOOLEAN NOT NULL DEFAULT false,
    is_today BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    date_part DATE GENERATED ALWAYS AS (DATE(created_at AT TIME ZONE 'Europe/Moscow')) STORED,
    UNIQUE(user_id, is_today)
);
CREATE INDEX IF NOT EXISTS idx_attendance_user_created ON attendance(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attendance_date_part ON attendance(date_part);

-- Создание таблицы для хранения планов
CREATE TABLE IF NOT EXISTS plans (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    date DATE NOT NULL,
    plan TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, date)
);

-- Создание индексов для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_plans_user_created ON plans(user_id, created_at DESC); 