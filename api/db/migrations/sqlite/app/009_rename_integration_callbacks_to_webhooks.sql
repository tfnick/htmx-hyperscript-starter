-- app/009_rename_integration_callbacks_to_webhooks.sql: Rename external integration callback terms to webhook.

DROP INDEX IF EXISTS idx_integration_callback_receipts_created;
DROP INDEX IF EXISTS idx_integration_callback_receipts_status;

ALTER TABLE integration_channels RENAME COLUMN callback_enabled TO webhook_enabled;
ALTER TABLE integration_callback_receipts RENAME TO integration_webhook_receipts;

UPDATE goqite
SET queue = 'integration-webhooks'
WHERE queue = 'integration-callbacks';

CREATE INDEX IF NOT EXISTS idx_integration_webhook_receipts_created
ON integration_webhook_receipts (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_integration_webhook_receipts_status
ON integration_webhook_receipts (status, created_at ASC);
