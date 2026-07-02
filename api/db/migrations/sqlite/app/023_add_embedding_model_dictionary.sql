-- app/023_add_embedding_model_dictionary.sql: Seed provider-specific Embedding model dictionary values.

INSERT OR IGNORE INTO dictionary_types (
    id, type_key, name, enabled, description
) VALUES (
    '019ea0c1-0003-7000-8000-000000000037',
    'embedding_model_siliconflow',
    'SiliconFlow Embedding Model',
    1,
    'SiliconFlow embedding models'
);

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
) VALUES
    ('019ea0c1-0003-7000-8000-000000000038', '019ea0c1-0003-7000-8000-000000000037', 'Qwen/Qwen3-Embedding-0.6B', 'Qwen3 Embedding 0.6B', 10, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000039', '019ea0c1-0003-7000-8000-000000000037', 'Qwen/Qwen3-Embedding-4B', 'Qwen3 Embedding 4B', 20, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000040', '019ea0c1-0003-7000-8000-000000000037', 'Qwen/Qwen3-Embedding-8B', 'Qwen3 Embedding 8B', 30, 1, '');
