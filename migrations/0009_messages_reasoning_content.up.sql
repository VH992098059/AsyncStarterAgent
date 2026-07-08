-- 消息表增加推理内容字段（DeepSeek thinking）
-- 推理内容默认折叠显示，不占太多空间
ALTER TABLE agent_run_messages
    ADD COLUMN IF NOT EXISTS reasoning_content TEXT NOT NULL DEFAULT '';
