-- 修复 drafts 表 ON CONFLICT (agent_run_id) 缺 UNIQUE 约束的潜在 bug
-- synthesis/service.go 的 GenerateDraftStream 用 INSERT ... ON CONFLICT (agent_run_id) DO UPDATE，
-- 但 0001_init 只建了普通 INDEX idx_drafts_run_id，PostgreSQL 要求 ON CONFLICT 目标列必须有 unique index
CREATE UNIQUE INDEX IF NOT EXISTS uq_drafts_agent_run_id ON drafts(agent_run_id);

-- memory 第 15 条 Loop 预留：drafts 增加 iteration/quality/validation 字段
-- MVP 阶段不写值，纯预留；V1.5 Loop Engineering 接入时填
ALTER TABLE drafts
    ADD COLUMN IF NOT EXISTS iteration_count   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS quality_score     REAL    NOT NULL DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS validation_issues JSONB   NOT NULL DEFAULT '[]'::jsonb;

-- 任务对话消息表：支持 ChatPanel 与 Agent 续跑对话
-- 每条消息独立记录，user/assistant 双向，streaming 状态用于断电恢复
CREATE TABLE agent_run_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_run_id    UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,
    role            VARCHAR(16) NOT NULL,  -- user / assistant / system
    content         TEXT NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'sent', -- sent / streaming / done / error
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_agent_run_messages_run  ON agent_run_messages(agent_run_id, created_at);
CREATE INDEX idx_agent_run_messages_user ON agent_run_messages(user_id, created_at DESC);
