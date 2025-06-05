-- Создаем таблицу для хранения статусов посещения офиса
CREATE TABLE IF NOT EXISTS attendance (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    is_office BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_attendance_user_created ON attendance(user_id, created_at DESC);

-- Создание таблицы для хранения планов
CREATE TABLE IF NOT EXISTS plans (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, created_at)
);

-- Создание индексов для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_plans_user_created ON plans(user_id, created_at DESC); 