-- 用户任务表（DDL 触发源，FR-A02）
CREATE TABLE user_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    source          VARCHAR(16) NOT NULL,   -- todoist / feishu / notion
    external_id     VARCHAR(128) NOT NULL,
    title           TEXT NOT NULL,
    content         TEXT,
    deadline_at     TIMESTAMPTZ,
    priority        VARCHAR(16) DEFAULT 'normal',
    completed       BOOLEAN NOT NULL DEFAULT false,
    triggered_at    TIMESTAMPTZ,           -- FR-A02 重复触发去重标记
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source, external_id)
);

-- partial index：仅索引"未完成且未触发过"的任务，避免已完成/已触发的历史行膨胀索引。
-- 谓词与 RunDDLScheduler 的 SELECT 完全对齐，调度器走 index-only scan。
CREATE INDEX idx_user_tasks_deadline ON user_tasks(deadline_at)
    WHERE completed = false AND triggered_at IS NULL;
