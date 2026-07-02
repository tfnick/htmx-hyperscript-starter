ALTER TABLE async_tasks ADD COLUMN cleared_at TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_async_tasks_user_clear_created
ON async_tasks (user_id, cleared_at, created_at DESC, id DESC);
