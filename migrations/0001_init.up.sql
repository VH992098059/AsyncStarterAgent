-- 启用 pgvector 扩展（Phase 3 使用，Phase 0 预先装好）
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

-- AgentRun: 单次端到端执行实例（模块 A → D 编排状态）
CREATE TABLE agent_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    task_type       VARCHAR(32) NOT NULL,  -- weekly_report / summary / plan / meeting_minutes
    status          VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending / running / completed / failed / cancelled
    current_stage   VARCHAR(16) NOT NULL DEFAULT 'ingestion', -- ingestion / harvesting / synthesis / delivery
    trigger_type    VARCHAR(16) NOT NULL,  -- keyword / ddl / webhook / manual
    trigger_source  TEXT,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);
CREATE INDEX idx_agent_runs_user_status ON agent_runs(user_id, status);
CREATE INDEX idx_agent_runs_created_at ON agent_runs(created_at DESC);

-- Draft: LLM 生成的草稿
CREATE TABLE drafts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_run_id    UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    title           VARCHAR(255) NOT NULL,
    content         JSONB,                  -- 富文本内容
    markdown_content TEXT,                  -- Markdown 源
    completeness    REAL NOT NULL DEFAULT 0.0, -- 0.0 - 1.0
    marks           JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{id, hint, position, resolved}]
    status          VARCHAR(16) NOT NULL DEFAULT 'draft', -- draft / reviewed / delivered
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_drafts_run_id ON drafts(agent_run_id);
CREATE INDEX idx_drafts_status ON drafts(status);

-- DataSource: 第三方数据源配置
CREATE TABLE data_sources (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    type            VARCHAR(32) NOT NULL,  -- github / google_calendar / outlook / feishu / slack / notion / obsidian
    name            VARCHAR(128) NOT NULL,
    config          TEXT,                   -- 加密的 JSON
    status          VARCHAR(16) NOT NULL DEFAULT 'disconnected', -- disconnected / connected / error
    last_sync_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, type)
);
CREATE INDEX idx_data_sources_user ON data_sources(user_id);

-- Delivery: 草稿交付记录
CREATE TABLE deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id        UUID NOT NULL REFERENCES drafts(id) ON DELETE CASCADE,
    target_type     VARCHAR(16) NOT NULL,  -- notion / obsidian / feishu
    target_url      TEXT,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending / success / failed
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_deliveries_draft ON deliveries(draft_id);

-- Template: 文档模板
CREATE TABLE templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(128) NOT NULL,
    type            VARCHAR(32) NOT NULL,  -- weekly_report / summary / plan / meeting_minutes / general
    content_markdown TEXT NOT NULL,
    category        VARCHAR(64),
    is_default      BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_templates_type ON templates(type);

-- 同步时间戳表（FR-B06 增量同步）
CREATE TABLE sync_timestamps (
    data_source_id  UUID PRIMARY KEY REFERENCES data_sources(id) ON DELETE CASCADE,
    last_sync_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
