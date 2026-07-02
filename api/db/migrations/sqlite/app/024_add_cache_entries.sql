CREATE TABLE IF NOT EXISTS cache_entries (
    namespace  TEXT NOT NULL,
    cache_key  TEXT NOT NULL,
    value_blob BLOB NOT NULL,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (namespace, cache_key)
);

CREATE INDEX IF NOT EXISTS idx_cache_entries_expires_at
ON cache_entries(expires_at);
