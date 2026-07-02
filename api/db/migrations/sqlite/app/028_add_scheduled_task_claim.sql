ALTER TABLE scheduled_tasks ADD COLUMN claim_token TEXT NOT NULL DEFAULT '';
ALTER TABLE scheduled_tasks ADD COLUMN claim_expires_at DATETIME;

CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_due_claim
ON scheduled_tasks (enabled, next_run_at, claim_expires_at);
