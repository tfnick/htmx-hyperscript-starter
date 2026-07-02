-- app/019_default_local_embedding_provider.sql: Use local 64-dimensional embeddings for KB indexing by default.

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
)
SELECT
    '019ebcf0-0001-7000-8000-000000000001',
    id,
    'none',
    'None',
    5,
    1,
    'No external credential required'
FROM dictionary_types
WHERE type_key = 'integration_credential_type';

INSERT OR IGNORE INTO integration_credentials (
    id, credential_type, ciphertext, key_version, masked_value, value_text, enabled
) VALUES (
    '019ebcf0-0002-7000-8000-000000000001',
    'none',
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
    '019ebcf0-0003-7000-8000-000000000001',
    'embedding',
    'local-hash-64',
    'local',
    'embedding.local_hash_64',
    'production',
    1,
    1,
    '019ebcf0-0002-7000-8000-000000000001',
    '{}',
    '{}'
);

INSERT OR IGNORE INTO integration_model_options (
    id, scenario, channel_id, model_code, provider_model_id, capabilities_json,
    default_params_json, cost_policy_json, enabled
) SELECT
    '019ebcf0-0004-7000-8000-000000000001',
    'embedding',
    id,
    'local-hash-64',
    'local-hash-64',
    '{}',
    '{"dimensions":64}',
    '{}',
    1
FROM integration_channels
WHERE scenario = 'embedding'
  AND channel_code = 'local-hash-64'
  AND adapter_key = 'embedding.local_hash_64'
ORDER BY priority ASC, created_at DESC
LIMIT 1;

INSERT INTO integration_operation_configs (
    id, scenario, operation, channel_code, model_code, enabled, config_json
) VALUES (
    '019ebcf0-0005-7000-8000-000000000001',
    'embedding',
    'embedding_create',
    'local-hash-64',
    'local-hash-64',
    1,
    '{}'
) ON CONFLICT(scenario, operation) DO UPDATE SET
    channel_code = 'local-hash-64',
    model_code = 'local-hash-64',
    enabled = 1,
    config_json = '{}',
    updated_at = CURRENT_TIMESTAMP
WHERE integration_operation_configs.channel_code = ''
   OR integration_operation_configs.channel_code IN (
       SELECT channel_code
       FROM integration_channels
       WHERE scenario = 'embedding'
         AND adapter_key = 'embedding.deepseek.openai_compatible'
   )
   OR integration_operation_configs.model_code = 'deepseek-embedding';
