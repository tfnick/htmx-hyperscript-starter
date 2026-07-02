CREATE TABLE IF NOT EXISTS monitor_settings (
    id TEXT PRIMARY KEY,
    sample_interval_seconds INTEGER NOT NULL DEFAULT 10,
    latency_probe_enabled INTEGER NOT NULL DEFAULT 1,
    latency_probe_url TEXT NOT NULL DEFAULT 'https://example.com',
    latency_probe_timeout_ms INTEGER NOT NULL DEFAULT 2000,
    alert_bot_channel_id TEXT NOT NULL DEFAULT '',
    daily_alert_limit INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS monitor_settings_updated_timestamp
AFTER UPDATE ON monitor_settings
BEGIN
    UPDATE monitor_settings SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS monitor_alert_rules (
    id TEXT PRIMARY KEY,
    metric_key TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 0,
    threshold_value REAL NOT NULL DEFAULT 0,
    sustained_samples INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (metric_key IN ('cpu', 'memory', 'disk', 'latency')),
    CHECK (sustained_samples >= 1)
);

CREATE TRIGGER IF NOT EXISTS monitor_alert_rules_updated_timestamp
AFTER UPDATE ON monitor_alert_rules
BEGIN
    UPDATE monitor_alert_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

INSERT INTO monitor_settings (id)
VALUES ('default')
ON CONFLICT(id) DO NOTHING;

INSERT INTO monitor_alert_rules (id, metric_key, enabled, threshold_value, sustained_samples)
VALUES
    ('monitor-rule-cpu', 'cpu', 0, 90, 3),
    ('monitor-rule-memory', 'memory', 0, 90, 3),
    ('monitor-rule-disk', 'disk', 0, 90, 3),
    ('monitor-rule-latency', 'latency', 0, 1000, 3)
ON CONFLICT(metric_key) DO NOTHING;
