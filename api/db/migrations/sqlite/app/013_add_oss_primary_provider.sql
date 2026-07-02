ALTER TABLE integration_channels ADD COLUMN is_primary INTEGER NOT NULL DEFAULT 0;

UPDATE integration_channels
SET is_primary = 0
WHERE scenario <> 'oss';

CREATE UNIQUE INDEX IF NOT EXISTS idx_integration_channels_one_primary_oss
ON integration_channels (scenario)
WHERE scenario = 'oss' AND is_primary = 1;
