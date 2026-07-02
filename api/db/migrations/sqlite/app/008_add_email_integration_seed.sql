-- app/008_add_email_integration_seed.sql: Seed email integration credential dictionary values.

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
) VALUES (
    '019ea0c1-0003-7000-8000-000000000017',
    '019ea0c1-0003-7000-8000-000000000009',
    'smtp_password',
    'SMTP Password',
    30,
    1,
    'SMTP username and password credential bundle'
);
