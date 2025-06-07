-- Создаем таблицу для хранения статусов посещения офиса
CREATE TABLE IF NOT EXISTS attendance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    is_office BOOLEAN NOT NULL DEFAULT 0,
    is_today BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    date_part DATE NOT NULL,
    UNIQUE(user_id, date_part, is_today)
);

CREATE INDEX IF NOT EXISTS idx_attendance_user_created ON attendance(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attendance_date_part ON attendance(date_part);

-- Создание таблицы для хранения планов
CREATE TABLE IF NOT EXISTS plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    date DATE NOT NULL,
    plan TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, date)
);

-- Создание индексов для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_plans_user_created ON plans(user_id, created_at DESC); 