CREATE TABLE IF NOT EXISTS forum_categories (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 100,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS forum_categories_updated_timestamp
AFTER UPDATE ON forum_categories
BEGIN
    UPDATE forum_categories SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS forum_threads (
    id                TEXT PRIMARY KEY,
    category_id       TEXT NOT NULL,
    author_id         TEXT NOT NULL,
    title             TEXT NOT NULL,
    body              TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'published',
    is_pinned         INTEGER NOT NULL DEFAULT 0,
    is_locked         INTEGER NOT NULL DEFAULT 0,
    view_count        INTEGER NOT NULL DEFAULT 0,
    reply_count       INTEGER NOT NULL DEFAULT 0,
    last_post_id      TEXT NOT NULL DEFAULT '',
    last_post_user_id TEXT NOT NULL DEFAULT '',
    last_post_at      DATETIME,
    deleted_at        DATETIME,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (category_id) REFERENCES forum_categories(id),
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE,
    CHECK (status IN ('published', 'deleted'))
);

CREATE TRIGGER IF NOT EXISTS forum_threads_updated_timestamp
AFTER UPDATE ON forum_threads
BEGIN
    UPDATE forum_threads SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS forum_posts (
    id         TEXT PRIMARY KEY,
    thread_id  TEXT NOT NULL,
    author_id  TEXT NOT NULL,
    body       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'published',
    deleted_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES forum_threads(id) ON DELETE CASCADE,
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE,
    CHECK (status IN ('published', 'deleted'))
);

CREATE TRIGGER IF NOT EXISTS forum_posts_updated_timestamp
AFTER UPDATE ON forum_posts
BEGIN
    UPDATE forum_posts SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE INDEX IF NOT EXISTS idx_forum_categories_enabled_order ON forum_categories (enabled, sort_order, slug);
CREATE INDEX IF NOT EXISTS idx_forum_threads_category_activity ON forum_threads (category_id, status, is_pinned DESC, last_post_at DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_threads_author_created ON forum_threads (author_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_threads_created ON forum_threads (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_forum_posts_thread_created ON forum_posts (thread_id, status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_forum_posts_author_created ON forum_posts (author_id, created_at DESC);

INSERT INTO forum_categories (id, slug, name, description, sort_order, enabled)
VALUES ('forum-category-daily', 'daily', 'Daily', 'Daily discussion and community chatter.', 10, 1)
ON CONFLICT(slug) DO NOTHING;
