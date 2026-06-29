-- 用户级第三方服务配置（AI 参数、Notion、Obsidian）
-- 每个用户独立配置，空字符串表示未配置（fallback 到环境变量默认值）
CREATE TABLE user_settings (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- LLM 对话模型配置
    llm_api_key         TEXT NOT NULL DEFAULT '',
    llm_base_url        TEXT NOT NULL DEFAULT '',
    llm_model           TEXT NOT NULL DEFAULT '',
    llm_temperature     REAL NOT NULL DEFAULT 0.3,
    llm_max_tokens      INTEGER NOT NULL DEFAULT 0,
    -- Embedding 向量模型配置（空则复用 LLM 的 key/base_url）
    embed_api_key       TEXT NOT NULL DEFAULT '',
    embed_base_url      TEXT NOT NULL DEFAULT '',
    embed_model         TEXT NOT NULL DEFAULT '',
    -- Notion 配置
    notion_api_key      TEXT NOT NULL DEFAULT '',
    notion_parent_page  TEXT NOT NULL DEFAULT '',
    -- Obsidian 配置
    obsidian_vault_path TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
