-- 决策 #2: auth 模块 - users 表（JWT 登录 + 黑名单）
-- 扩 MVP 范围（用户已在 AskUserQuestion 明确同意加 auth）
-- 仅用户身份认证；多用户协作仍属 OUT 范围（不在此表引入 organization/role）

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(64) NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_username ON users(username);
