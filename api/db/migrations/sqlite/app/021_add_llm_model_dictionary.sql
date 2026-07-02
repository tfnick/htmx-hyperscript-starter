-- app/021_add_llm_model_dictionary.sql: Seed llm_model dictionary type for LLM/Embedding model selection.

INSERT OR IGNORE INTO dictionary_types (
    id, type_key, name, enabled, description
) VALUES (
    '019ea0c1-0003-7000-8000-000000000021', 'llm_model', 'LLM Model', 1, 'Available LLM and embedding models'
);

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
) VALUES
    ('019ea0c1-0003-7000-8000-000000000022', '019ea0c1-0003-7000-8000-000000000021', 'pro', 'Pro', 10, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000023', '019ea0c1-0003-7000-8000-000000000021', 'minimaxai', 'MiniMaxAI', 20, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000024', '019ea0c1-0003-7000-8000-000000000021', 'minimax-m2.7', 'MiniMax-M2.7', 30, 1, '');
