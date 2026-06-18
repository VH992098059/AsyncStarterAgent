-- 触发引擎相关索引
CREATE INDEX IF NOT EXISTS idx_agent_runs_trigger_type ON agent_runs(trigger_type);
CREATE INDEX IF NOT EXISTS idx_agent_runs_user_trigger ON agent_runs(user_id, trigger_type, created_at DESC);
