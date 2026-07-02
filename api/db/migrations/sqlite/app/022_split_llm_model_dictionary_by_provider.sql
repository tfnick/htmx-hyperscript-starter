-- app/022_split_llm_model_dictionary_by_provider.sql: Split LLM model dictionaries by provider.

INSERT OR IGNORE INTO dictionary_types (
    id, type_key, name, enabled, description
) VALUES
    ('019ea0c1-0003-7000-8000-000000000025', 'llm_model_deepseek', 'DeepSeek LLM Model', 1, 'DeepSeek API LLM models'),
    ('019ea0c1-0003-7000-8000-000000000026', 'llm_model_siliconflow', 'SiliconFlow LLM Model', 1, 'SiliconFlow chat completion models');

INSERT OR IGNORE INTO dictionary_values (
    id, dictionary_type_id, value_code, label, sort_order, enabled, description
) VALUES
    ('019ea0c1-0003-7000-8000-000000000027', '019ea0c1-0003-7000-8000-000000000025', 'deepseek-v4-flash', 'DeepSeek V4 Flash', 10, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000028', '019ea0c1-0003-7000-8000-000000000025', 'deepseek-v4-pro', 'DeepSeek V4 Pro', 20, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000029', '019ea0c1-0003-7000-8000-000000000025', 'deepseek-chat', 'DeepSeek Chat (legacy)', 90, 1, 'Deprecated by DeepSeek after 2026-07-24; retained for existing compatibility'),
    ('019ea0c1-0003-7000-8000-000000000030', '019ea0c1-0003-7000-8000-000000000025', 'deepseek-reasoner', 'DeepSeek Reasoner (legacy)', 100, 1, 'Deprecated by DeepSeek after 2026-07-24; retained for existing compatibility'),
    ('019ea0c1-0003-7000-8000-000000000031', '019ea0c1-0003-7000-8000-000000000026', 'Pro/zai-org/GLM-4.7', 'GLM 4.7 Pro', 10, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000032', '019ea0c1-0003-7000-8000-000000000026', 'Qwen/Qwen3-32B', 'Qwen3 32B', 20, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000033', '019ea0c1-0003-7000-8000-000000000026', 'Qwen/Qwen3-235B-A22B', 'Qwen3 235B A22B', 30, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000034', '019ea0c1-0003-7000-8000-000000000026', 'deepseek-ai/DeepSeek-R1', 'DeepSeek R1', 40, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000035', '019ea0c1-0003-7000-8000-000000000026', 'deepseek-ai/DeepSeek-V3', 'DeepSeek V3', 50, 1, ''),
    ('019ea0c1-0003-7000-8000-000000000036', '019ea0c1-0003-7000-8000-000000000026', 'moonshotai/Kimi-K2-Instruct-0905', 'Kimi K2 Instruct', 60, 1, '');
