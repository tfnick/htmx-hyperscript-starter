ALTER TABLE notifications ADD COLUMN cleared_at TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_notifications_user_clear_created
ON notifications (user_id, cleared_at, created_at DESC, id DESC);
