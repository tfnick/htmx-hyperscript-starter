-- app/020_add_siliconflow_embedding_provider.sql: Use SiliconFlow embeddings for KB indexing by default.

INSERT OR IGNORE INTO integration_credentials (
    id, credential_type, ciphertext, key_version, masked_value, value_text, enabled
) VALUES (
    '019eda10-0001-7000-8000-000000000001',
    'api_key',
    '',
    '',
    '',
    '',
    1
);

INSERT OR IGNORE INTO integration_channels (
    id, scenario, channel_code, provider_code, adapter_key, environment, enabled,
    priority, credential_id, config_json, metadata_json
) VALUES (
    '019eda10-0002-7000-8000-000000000001',
    'embedding',
    'siliconflow-qwen3-embedding',
    'siliconflow',
    'embedding.siliconflow.openai_compatible',
    'production',
    1,
    1,
    '019eda10-0001-7000-8000-000000000001',
    '{"base_url":"https://api.siliconflow.cn","model":"Qwen/Qwen3-Embedding-0.6B","dimensions":64,"encoding_format":"float","endpoint_path":"/v1/embeddings"}',
    '{}'
);

INSERT OR IGNORE INTO integration_model_options (
    id, scenario, channel_id, model_code, provider_model_id, capabilities_json,
    default_params_json, cost_policy_json, enabled
) SELECT
    '019eda10-0003-7000-8000-000000000001',
    'embedding',
    id,
    'qwen3-embedding-0.6b',
    'Qwen/Qwen3-Embedding-0.6B',
    '{}',
    '{"dimensions":64,"encoding_format":"float"}',
    '{}',
    1
FROM integration_channels
WHERE scenario = 'embedding'
  AND channel_code = 'siliconflow-qwen3-embedding'
  AND adapter_key = 'embedding.siliconflow.openai_compatible'
ORDER BY priority ASC, created_at DESC
LIMIT 1;

UPDATE integration_channels
SET priority = 100,
    updated_at = CURRENT_TIMESTAMP
WHERE scenario = 'embedding'
  AND channel_code = 'local-hash-64'
  AND adapter_key = 'embedding.local_hash_64';

UPDATE integration_channels
SET enabled = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE scenario = 'embedding'
  AND adapter_key = 'embedding.deepseek.openai_compatible';

UPDATE integration_model_options
SET enabled = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE scenario = 'embedding'
  AND channel_id IN (
      SELECT id
      FROM integration_channels
      WHERE scenario = 'embedding'
        AND adapter_key = 'embedding.deepseek.openai_compatible'
  );

INSERT INTO integration_operation_configs (
    id, scenario, operation, channel_code, model_code, enabled, config_json
) VALUES (
    '019eda10-0004-7000-8000-000000000001',
    'embedding',
    'embedding_create',
    'siliconflow-qwen3-embedding',
    'qwen3-embedding-0.6b',
    1,
    '{}'
) ON CONFLICT(scenario, operation) DO UPDATE SET
    channel_code = 'siliconflow-qwen3-embedding',
    model_code = 'qwen3-embedding-0.6b',
    enabled = 1,
    config_json = '{}',
    updated_at = CURRENT_TIMESTAMP
WHERE integration_operation_configs.channel_code = ''
   OR integration_operation_configs.channel_code = 'local-hash-64'
   OR integration_operation_configs.channel_code IN (
       SELECT channel_code
       FROM integration_channels
       WHERE scenario = 'embedding'
         AND adapter_key = 'embedding.deepseek.openai_compatible'
   )
   OR integration_operation_configs.model_code IN ('local-hash-64', 'deepseek-embedding');
