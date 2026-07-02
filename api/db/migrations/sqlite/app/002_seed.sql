-- app/002_seed.sql: Application seed data with deterministic UUIDv7-style IDs.

INSERT OR IGNORE INTO users (
    id, name, email, password_hash, email_verified, is_active, is_admin
) VALUES
    ('019ea0c1-0001-7000-8000-000000000001', '张三', 'zhangsan@example.com', '$2a$12$qmXX.FmF8QrkCDV76NAVnO68DM9.XdrgKrrQ69zniIXCcP10kCoBC', 0, 1, 1),
    ('019ea0c1-0001-7000-8000-000000000002', '李四', 'lisi@example.com', '$2a$12$qmXX.FmF8QrkCDV76NAVnO68DM9.XdrgKrrQ69zniIXCcP10kCoBC', 0, 1, 0),
    ('019ea0c1-0001-7000-8000-000000000003', '王五', 'wangwu@example.com', '$2a$12$qmXX.FmF8QrkCDV76NAVnO68DM9.XdrgKrrQ69zniIXCcP10kCoBC', 0, 1, 0);

INSERT OR IGNORE INTO open_api_partners (
    id, name, account_id, status
) VALUES (
    '019ea0c1-0002-7000-8000-000000000001',
    'Demo Partner',
    '019ea0c1-0001-7000-8000-000000000001',
    'active'
);

-- Demo API key: demo-open-api-key
INSERT OR IGNORE INTO open_api_keys (
    id, partner_id, token_hash, environment, scopes, status
) VALUES (
    '019ea0c1-0002-7000-8000-000000000002',
    '019ea0c1-0002-7000-8000-000000000001',
    '9bacbd8eb733a8b5d226bc5b33fdb481dc9763b8927c840f781a6aa737513dc4',
    'dev',
    'account:read',
    'active'
);

INSERT OR IGNORE INTO products (id, name, description, price, stock) VALUES
    ('019ea0c1-0004-7000-8000-000000000001', 'iPhone 15', '苹果手机', 699900, 100),
    ('019ea0c1-0004-7000-8000-000000000002', 'MacBook Pro', '苹果笔记本电脑', 1299900, 50),
    ('019ea0c1-0004-7000-8000-000000000003', 'AirPods Pro', '苹果无线耳机', 189900, 200),
    ('019ea0c1-0004-7000-8000-000000000004', 'iPad Air', '苹果平板电脑', 479900, 80);

INSERT OR IGNORE INTO dictionary_types (
    id, type_key, name, enabled, description
) VALUES
    ('019ea0c1-0003-7000-8000-000000000001', 'product_category', 'Product category', 1, 'Default product category options'),
    ('019ea0c1-0003-7000-8000-000000000006', 'integration_environment', 'Integration environment', 1, 'Runtime environment options for external integrations'),
    ('019ea0c1-0003-7000-8000-000000000009', 'integration_credential_type', 'Integration credential type', 1, 'Credential storage formats for external integrations'),
    ('019ea0c1-0003-7000-8000-000000000012', 'notification_type', 'Notification type', 1, 'Business notification categories');

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
) VALUES
    ('019ea0c1-0003-7000-8000-000000000002', '019ea0c1-0003-7000-8000-000000000001', 'phone', 'Phone', 10, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000003', '019ea0c1-0003-7000-8000-000000000001', 'computer', 'Computer', 20, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000004', '019ea0c1-0003-7000-8000-000000000001', 'audio', 'Audio', 30, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000005', '019ea0c1-0003-7000-8000-000000000001', 'tablet', 'Tablet', 40, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000007', '019ea0c1-0003-7000-8000-000000000006', 'test', 'Test', 10, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000008', '019ea0c1-0003-7000-8000-000000000006', 'production', 'Production', 20, 1, ''),
    ('019ebcf0-0001-7000-8000-000000000001', '019ea0c1-0003-7000-8000-000000000009', 'none', 'None', 5, 1, 'No external credential required'),
    ('019ea0c1-0003-7000-8000-000000000010', '019ea0c1-0003-7000-8000-000000000009', 'payment_bundle', 'Payment Bundle', 10, 1, 'JSON object containing payment API key and webhook secret'),
    ('019ea0c1-0003-7000-8000-000000000011', '019ea0c1-0003-7000-8000-000000000009', 'api_key', 'API Key', 20, 1, 'Single API key credential value'),
    ('019ea0c1-0003-7000-8000-000000000017', '019ea0c1-0003-7000-8000-000000000009', 'smtp_password', 'SMTP Password', 30, 1, 'SMTP username and password credential bundle'),
    ('019ea0c1-0003-7000-8000-000000000018', '019ea0c1-0003-7000-8000-000000000009', 's3_access_key', 'S3 Access Key', 40, 1, 'S3-compatible access key credential bundle'),
    ('019ea0c1-0003-7000-8000-000000000013', '019ea0c1-0003-7000-8000-000000000012', 'realtime', 'Realtime', 10, 1, 'Realtime WebSocket notification'),
    ('019ea0c1-0003-7000-8000-000000000014', '019ea0c1-0003-7000-8000-000000000012', 'sms', 'SMS', 20, 1, 'SMS notification ledger entry'),
    ('019ea0c1-0003-7000-8000-000000000015', '019ea0c1-0003-7000-8000-000000000012', 'email', 'Email', 30, 1, 'Email notification ledger entry'),
    ('019ea0c1-0003-7000-8000-000000000016', '019ea0c1-0003-7000-8000-000000000012', 'wechat_official_account', 'WeChat Official Account', 40, 1, 'WeChat official account message ledger entry');
