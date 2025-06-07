-- Удаляем существующую таблицу
DROP TABLE IF EXISTS attendance;

-- Создаем таблицу заново с обновленными ограничениями
CREATE TABLE IF NOT EXISTS attendance (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    is_office BOOLEAN NOT NULL DEFAULT false,
    is_today BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    date_part DATE GENERATED ALWAYS AS (DATE(created_at AT TIME ZONE 'Europe/Moscow')) STORED,
    UNIQUE(user_id, is_today)
);

-- Создаем индексы
CREATE INDEX IF NOT EXISTS idx_attendance_user_created ON attendance(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attendance_date_part ON attendance(date_part); 