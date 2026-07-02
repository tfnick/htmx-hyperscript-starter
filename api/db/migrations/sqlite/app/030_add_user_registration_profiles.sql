CREATE TABLE IF NOT EXISTS user_registration_profiles (
    id                      TEXT PRIMARY KEY,
    user_id                 TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    registration_ip          TEXT NOT NULL DEFAULT '',
    registration_country     TEXT NOT NULL DEFAULT '',
    registration_region      TEXT NOT NULL DEFAULT '',
    registration_user_agent  TEXT NOT NULL DEFAULT '',
    created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_registration_profiles_country ON user_registration_profiles(registration_country);
CREATE INDEX IF NOT EXISTS idx_user_registration_profiles_region ON user_registration_profiles(registration_region);
