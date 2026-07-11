-- 用户级飞书自建应用凭证（取代全局 FEISHU_APP_ID/FEISHU_APP_SECRET）
-- app_secret 用 pgcrypto 加密存储，加密密钥来自 DB_ENCRYPTION_KEY（与 feishu_tokens 一致）
CREATE TABLE feishu_app_configs (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    app_id              TEXT NOT NULL,
    app_secret          BYTEA NOT NULL,       -- pgp_sym_encrypt 加密后的密文
    verification_token  BYTEA,                -- pgp_sym_encrypt 加密；阶段二 webhook 多租户校验用，本迁移先建列
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_feishu_app_configs_app_id ON feishu_app_configs(app_id);
