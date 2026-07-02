-- app/014_add_oauth_login.sql: OAuth identities and one-time login results.

CREATE TABLE IF NOT EXISTS oauth_identities (
    id               TEXT PRIMARY KEY,
    provider         TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email            TEXT NOT NULL DEFAULT '',
    email_verified   INTEGER NOT NULL DEFAULT 0,
    display_name     TEXT NOT NULL DEFAULT '',
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_identities_user_id ON oauth_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_identities_email ON oauth_identities(email);

CREATE TABLE IF NOT EXISTS oauth_states (
    id            TEXT PRIMARY KEY,
    state_hash    TEXT NOT NULL UNIQUE,
    provider      TEXT NOT NULL,
    redirect_path TEXT NOT NULL DEFAULT '/',
    expires_at    DATETIME NOT NULL,
    used_at       DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_oauth_states_expires_at ON oauth_states(expires_at);

CREATE TABLE IF NOT EXISTS oauth_login_results (
    id            TEXT PRIMARY KEY,
    token_hash    TEXT NOT NULL UNIQUE,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_path TEXT NOT NULL DEFAULT '/',
    expires_at    DATETIME NOT NULL,
    used_at       DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_oauth_login_results_user_id ON oauth_login_results(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_login_results_expires_at ON oauth_login_results(expires_at);
