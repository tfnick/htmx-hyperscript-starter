-- app/001_schema.sql: Application database baseline schema.

CREATE TABLE IF NOT EXISTS users (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    email          TEXT UNIQUE NOT NULL,
    password_hash  TEXT,
    email_verified INTEGER NOT NULL DEFAULT 0,
    is_active      INTEGER NOT NULL DEFAULT 1,
    is_admin       INTEGER NOT NULL DEFAULT 0,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    amount     INTEGER NOT NULL DEFAULT 0,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS order_items (
    id         TEXT PRIMARY KEY,
    order_id   TEXT NOT NULL,
    product_id TEXT NOT NULL,
    quantity   INTEGER NOT NULL DEFAULT 1,
    price      INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS products (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    price       INTEGER NOT NULL DEFAULT 0,
    stock       INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS password_resets (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  DATETIME NOT NULL,
    used_at     DATETIME,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS open_api_partners (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    account_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS open_api_keys (
    id           TEXT PRIMARY KEY,
    partner_id   TEXT NOT NULL REFERENCES open_api_partners(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    environment  TEXT NOT NULL DEFAULT 'prod',
    scopes       TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'active',
    revoked_at   DATETIME,
    expires_at   DATETIME,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS point_accounts (
    user_id    TEXT PRIMARY KEY,
    balance    INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS point_transactions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    order_id   TEXT NOT NULL,
    points     INTEGER NOT NULL,
    type       TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    UNIQUE(order_id, type)
);

CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    job_name       TEXT NOT NULL,
    schedule_type  TEXT NOT NULL,
    schedule_value TEXT NOT NULL,
    payload_json   TEXT NOT NULL DEFAULT '{}',
    enabled        INTEGER NOT NULL DEFAULT 1,
    next_run_at    DATETIME,
    last_run_at    DATETIME,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    CHECK (schedule_type IN ('cron', 'once_at'))
);

CREATE TRIGGER IF NOT EXISTS scheduled_tasks_updated_timestamp
AFTER UPDATE ON scheduled_tasks
BEGIN
    UPDATE scheduled_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS scheduled_task_executions (
    id            TEXT PRIMARY KEY,
    task_id       TEXT NOT NULL,
    job_name      TEXT NOT NULL,
    message_id    TEXT,
    status        TEXT NOT NULL,
    scheduled_at  DATETIME,
    started_at    DATETIME,
    finished_at   DATETIME,
    error_message TEXT,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    CHECK (status IN ('queued', 'running', 'succeeded', 'failed'))
);

CREATE TABLE IF NOT EXISTS domain_events (
    id             TEXT PRIMARY KEY,
    topic          TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id   TEXT NOT NULL,
    payload_json   TEXT NOT NULL,
    metadata_json  TEXT NOT NULL DEFAULT '{}',
    occurred_at    DATETIME NOT NULL,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS domain_event_deliveries (
    id            TEXT PRIMARY KEY,
    event_id      TEXT NOT NULL,
    subscriber    TEXT NOT NULL,
    message_id    TEXT,
    status        TEXT NOT NULL,
    attempts      INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(event_id, subscriber),
    FOREIGN KEY (event_id) REFERENCES domain_events(id) ON DELETE CASCADE,
    CHECK (status IN ('queued', 'running', 'succeeded', 'failed'))
);

CREATE TRIGGER IF NOT EXISTS domain_event_deliveries_updated_timestamp
AFTER UPDATE ON domain_event_deliveries
BEGIN
    UPDATE domain_event_deliveries SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS notifications (
    id                TEXT PRIMARY KEY,
    notification_type TEXT NOT NULL,
    source_type       TEXT NOT NULL DEFAULT '',
    source_id         TEXT NOT NULL DEFAULT '',
    user_id           TEXT NOT NULL DEFAULT '',
    recipient_email   TEXT NOT NULL DEFAULT '',
    recipient_phone   TEXT NOT NULL DEFAULT '',
    title             TEXT NOT NULL,
    summary           TEXT NOT NULL DEFAULT '',
    payload_json      TEXT NOT NULL DEFAULT '{}',
    status            TEXT NOT NULL DEFAULT 'pending',
    last_error        TEXT NOT NULL DEFAULT '',
    sent_at           DATETIME,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (status IN ('pending', 'sent', 'failed', 'skipped'))
);

CREATE TRIGGER IF NOT EXISTS notifications_updated_timestamp
AFTER UPDATE ON notifications
BEGIN
    UPDATE notifications SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS integration_credentials (
    id TEXT PRIMARY KEY,
    credential_type TEXT NOT NULL,
    ciphertext TEXT NOT NULL DEFAULT '',
    key_version TEXT NOT NULL DEFAULT '',
    masked_value TEXT NOT NULL DEFAULT '',
    value_text TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    rotated_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS integration_policies (
    id TEXT PRIMARY KEY,
    scenario TEXT NOT NULL,
    name TEXT NOT NULL,
    allowlist_json TEXT NOT NULL DEFAULT '{}',
    denylist_json TEXT NOT NULL DEFAULT '{}',
    rate_limit_json TEXT NOT NULL DEFAULT '{}',
    risk_rules_json TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS integration_channels (
    id TEXT PRIMARY KEY,
    scenario TEXT NOT NULL,
    channel_code TEXT NOT NULL,
    provider_code TEXT NOT NULL,
    adapter_key TEXT NOT NULL,
    environment TEXT NOT NULL DEFAULT 'production',
    enabled INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 100,
    credential_id TEXT NOT NULL,
    policy_id TEXT,
    callback_enabled INTEGER NOT NULL DEFAULT 0,
    config_json TEXT NOT NULL DEFAULT '{}',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (credential_id) REFERENCES integration_credentials(id),
    FOREIGN KEY (policy_id) REFERENCES integration_policies(id),
    UNIQUE (scenario, channel_code, environment)
);

CREATE TABLE IF NOT EXISTS integration_model_options (
    id TEXT PRIMARY KEY,
    scenario TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    model_code TEXT NOT NULL,
    provider_model_id TEXT NOT NULL,
    capabilities_json TEXT NOT NULL DEFAULT '{}',
    default_params_json TEXT NOT NULL DEFAULT '{}',
    cost_policy_json TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (channel_id) REFERENCES integration_channels(id),
    UNIQUE (scenario, channel_id, model_code)
);

CREATE TABLE IF NOT EXISTS integration_operation_configs (
    id TEXT PRIMARY KEY,
    scenario TEXT NOT NULL,
    operation TEXT NOT NULL,
    channel_code TEXT NOT NULL DEFAULT '',
    model_code TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (scenario, operation)
);

CREATE TABLE IF NOT EXISTS integration_invocations (
    id TEXT PRIMARY KEY,
    scenario TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    channel_code TEXT NOT NULL,
    provider_code TEXT NOT NULL,
    operation TEXT NOT NULL,
    idempotency_key TEXT NOT NULL DEFAULT '',
    model_code TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    error_category TEXT NOT NULL DEFAULT '',
    retryable INTEGER NOT NULL DEFAULT 0,
    usage_json TEXT NOT NULL DEFAULT '{}',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (channel_id) REFERENCES integration_channels(id)
);

CREATE TABLE IF NOT EXISTS integration_callback_receipts (
    id TEXT PRIMARY KEY,
    scenario TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    channel_code TEXT NOT NULL DEFAULT '',
    provider_code TEXT NOT NULL DEFAULT '',
    provider_event_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    payload_ciphertext TEXT NOT NULL,
    safe_snapshot_json TEXT NOT NULL DEFAULT '{}',
    headers_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error_code TEXT NOT NULL DEFAULT '',
    received_at DATETIME NOT NULL,
    processed_at DATETIME,
    message_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (channel_id) REFERENCES integration_channels(id),
    UNIQUE (scenario, idempotency_key)
);

CREATE TABLE IF NOT EXISTS variables (
    id TEXT PRIMARY KEY,
    variable_key TEXT NOT NULL,
    name TEXT NOT NULL,
    value_type TEXT NOT NULL,
    value_json TEXT NOT NULL DEFAULT 'null',
    enabled INTEGER NOT NULL DEFAULT 1,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (variable_key),
    CHECK (value_type IN ('string', 'number', 'boolean', 'json'))
);

CREATE TRIGGER IF NOT EXISTS variables_updated_timestamp
AFTER UPDATE ON variables
BEGIN
    UPDATE variables SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS dictionary_types (
    id TEXT PRIMARY KEY,
    type_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS dictionary_types_updated_timestamp
AFTER UPDATE ON dictionary_types
BEGIN
    UPDATE dictionary_types SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE TABLE IF NOT EXISTS dictionary_values (
    id TEXT PRIMARY KEY,
    dictionary_type_id TEXT NOT NULL,
    value_code TEXT NOT NULL,
    label TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 100,
    enabled INTEGER NOT NULL DEFAULT 1,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (dictionary_type_id) REFERENCES dictionary_types(id) ON DELETE CASCADE,
    UNIQUE(dictionary_type_id, value_code)
);

CREATE TRIGGER IF NOT EXISTS dictionary_values_updated_timestamp
AFTER UPDATE ON dictionary_values
BEGIN
    UPDATE dictionary_values SET updated_at = CURRENT_TIMESTAMP WHERE id = old.id;
END;

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
CREATE INDEX IF NOT EXISTS idx_password_resets_user_id ON password_resets(user_id);
CREATE INDEX IF NOT EXISTS idx_open_api_partners_account_id ON open_api_partners(account_id);
CREATE INDEX IF NOT EXISTS idx_open_api_keys_partner_id ON open_api_keys(partner_id);
CREATE INDEX IF NOT EXISTS idx_open_api_keys_status ON open_api_keys(status);
CREATE INDEX IF NOT EXISTS idx_point_transactions_user_id ON point_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_next_run ON scheduled_tasks (enabled, next_run_at);
CREATE INDEX IF NOT EXISTS idx_scheduled_task_executions_task_id ON scheduled_task_executions (task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_domain_events_topic_created ON domain_events (topic, created_at);
CREATE INDEX IF NOT EXISTS idx_domain_event_deliveries_event_id ON domain_event_deliveries (event_id);
CREATE INDEX IF NOT EXISTS idx_domain_event_deliveries_status ON domain_event_deliveries (status, updated_at);
CREATE INDEX IF NOT EXISTS idx_notifications_type_created ON notifications (notification_type, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_email_created ON notifications (recipient_email, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_phone_created ON notifications (recipient_phone, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_integration_channels_scenario_enabled ON integration_channels (scenario, enabled, priority, channel_code);
CREATE INDEX IF NOT EXISTS idx_integration_model_options_channel_enabled ON integration_model_options (channel_id, enabled, model_code);
CREATE INDEX IF NOT EXISTS idx_integration_operation_configs_lookup ON integration_operation_configs (scenario, operation, enabled);
CREATE INDEX IF NOT EXISTS idx_integration_invocations_created ON integration_invocations (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_integration_callback_receipts_created ON integration_callback_receipts (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_integration_callback_receipts_status ON integration_callback_receipts (status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_variables_enabled_key ON variables (enabled, variable_key);
CREATE INDEX IF NOT EXISTS idx_dictionary_values_type_enabled_order ON dictionary_values (dictionary_type_id, enabled, sort_order, value_code);
