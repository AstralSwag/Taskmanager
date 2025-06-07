CREATE TABLE IF NOT EXISTS plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    content TEXT NOT NULL,
    UNIQUE(user_id, created_at)
);
CREATE INDEX IF NOT EXISTS idx_plans_user_created ON plans(user_id, created_at DESC); 