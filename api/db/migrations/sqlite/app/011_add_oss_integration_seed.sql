-- app/011_add_oss_integration_seed.sql: Seed OSS integration credential dictionary values.

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
) VALUES (
    '019ea0c1-0003-7000-8000-000000000018',
    '019ea0c1-0003-7000-8000-000000000009',
    's3_access_key',
    'S3 Access Key',
    40,
    1,
    'S3-compatible access key credential bundle'
);
