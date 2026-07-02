CREATE UNIQUE INDEX IF NOT EXISTS idx_integration_channels_one_primary_llm
ON integration_channels (scenario)
WHERE scenario = 'llm' AND is_primary = 1;

CREATE UNIQUE INDEX IF NOT EXISTS idx_integration_channels_one_primary_embedding
ON integration_channels (scenario)
WHERE scenario = 'embedding' AND is_primary = 1;
