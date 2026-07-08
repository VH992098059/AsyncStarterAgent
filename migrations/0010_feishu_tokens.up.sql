-- 启用 pgcrypto 扩展（提供 pgp_sym_encrypt / pgp_sym_decrypt）
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 飞书用户 token 存储（决策 #7）
-- 独立于 user_settings 表，避免 token 频繁刷新污染 settings 缓存
-- access_token / refresh_token 用 BYTEA：pgp_sym_encrypt 返回 bytea，
-- 直接存 BYTEA 避免依赖 bytea→text 隐式转换（生产环境可能禁用）
CREATE TABLE feishu_tokens (
    user_id        UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_token   BYTEA NOT NULL,       -- pgp_sym_encrypt 加密后的密文
    refresh_token  BYTEA NOT NULL,       -- pgp_sym_encrypt 加密后的密文
    expires_at     TIMESTAMPTZ NOT NULL, -- access_token 过期时间
    open_id        TEXT,                 -- 飞书用户标识
    name           TEXT,                 -- 飞书用户名（展示用）
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_feishu_tokens_expires ON feishu_tokens(expires_at);
