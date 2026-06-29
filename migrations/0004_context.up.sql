-- T014: context_items 表（FR-B06 增量同步）+ sync_timestamps 表
CREATE TABLE context_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    agent_run_id    UUID,
    external_id     TEXT NOT NULL,        -- 数据源内唯一 ID
    source          VARCHAR(32) NOT NULL, -- github / google_calendar / feishu / obsidian
    type            VARCHAR(32) NOT NULL, -- commit / pr / meeting / message / note
    title           TEXT,
    content         TEXT,
    url             TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL,
    metadata        JSONB DEFAULT '{}'::jsonb,
    is_noise        BOOLEAN NOT NULL DEFAULT false,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, source, external_id)
);

CREATE INDEX idx_context_items_run ON context_items(agent_run_id);
CREATE INDEX idx_context_items_user_occurred ON context_items(user_id, occurred_at DESC);
CREATE INDEX idx_context_items_noise ON context_items(is_noise) WHERE is_noise = false;

-- sync_timestamps 表：记录每个数据源的最后同步时间（FR-B06）
-- 修复 0001 中 sync_timestamps 的 schema（UUID FK）与代码实现（TEXT 数据源 ID）不一致的问题
DROP TABLE IF EXISTS sync_timestamps;
CREATE TABLE sync_timestamps (
    data_source_id TEXT PRIMARY KEY,     -- 形如 "github:o/r" 或 "obsidian:/path"
    last_sync_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
