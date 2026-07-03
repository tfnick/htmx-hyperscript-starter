ALTER TABLE forum_threads
ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private'));

CREATE INDEX IF NOT EXISTS idx_forum_threads_public_activity
ON forum_threads (visibility, status, is_pinned DESC, last_post_at DESC, created_at DESC);
