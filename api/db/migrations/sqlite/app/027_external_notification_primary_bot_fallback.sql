-- app/027_external_notification_primary_bot_fallback.sql: normalize external notification primary bot state.

UPDATE integration_channels
SET is_primary = CASE
        WHEN id = (
            SELECT c.id
            FROM integration_channels c
            INNER JOIN integration_credentials cred ON cred.id = c.credential_id
            WHERE c.scenario = 'external_notification'
              AND c.enabled = 1
              AND cred.enabled = 1
            ORDER BY c.is_primary DESC, c.priority ASC, c.created_at DESC, c.id DESC
            LIMIT 1
        ) THEN 1
        ELSE 0
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE scenario = 'external_notification'
  AND EXISTS (
    SELECT c.id
    FROM integration_channels c
    INNER JOIN integration_credentials cred ON cred.id = c.credential_id
    WHERE c.scenario = 'external_notification'
      AND c.enabled = 1
      AND cred.enabled = 1
    LIMIT 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_integration_channels_one_primary_external_notification
ON integration_channels (scenario)
WHERE scenario = 'external_notification' AND is_primary = 1;
