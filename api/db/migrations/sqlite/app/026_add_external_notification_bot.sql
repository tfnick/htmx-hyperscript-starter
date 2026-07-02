-- app/026_add_external_notification_bot.sql: External notification bot configuration and delivery ledger.

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
) VALUES (
    '019eca2a-0001-7000-8000-000000000001',
    '019ea0c1-0003-7000-8000-000000000009',
    'webhook_url',
    'Webhook URL',
    50,
    1,
    'Webhook URL credential bundle for bot notifications'
);

CREATE TABLE IF NOT EXISTS external_notification_deliveries (
    id                     TEXT PRIMARY KEY,
    event_id               TEXT NOT NULL DEFAULT '',
    channel_id             TEXT NOT NULL DEFAULT '',
    provider_code          TEXT NOT NULL DEFAULT '',
    adapter_key            TEXT NOT NULL DEFAULT '',
    idempotency_key        TEXT NOT NULL,
    status                 TEXT NOT NULL,
    attempts               INTEGER NOT NULL DEFAULT 0,
    next_attempt_at        DATETIME,
    last_error_code        TEXT NOT NULL DEFAULT '',
    last_error_message     TEXT NOT NULL DEFAULT '',
    request_snapshot_json  TEXT NOT NULL DEFAULT '{}',
    response_snapshot_json TEXT NOT NULL DEFAULT '{}',
    note                   TEXT NOT NULL DEFAULT '',
    sent_at                DATETIME,
    message_id             TEXT NOT NULL DEFAULT '',
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(idempotency_key),
    CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'skipped'))
);

CREATE TRIGGER IF NOT EXISTS external_notification_deliveries_updated_timestamp
AFTER UPDATE ON external_notification_deliveries
BEGIN
    UPDATE external_notification_deliveries SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE INDEX IF NOT EXISTS idx_external_notification_deliveries_event
ON external_notification_deliveries (event_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_external_notification_deliveries_status
ON external_notification_deliveries (status, next_attempt_at, created_at ASC);
