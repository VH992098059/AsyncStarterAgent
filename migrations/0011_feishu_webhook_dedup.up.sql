-- 飞书 webhook 事件幂等去重表（决策：event_id 去重表 + TTL 清理任务）
-- 飞书事件订阅在网络异常/响应慢时会重试推送相同 event_id，需要在创建 AgentRun 前
-- 记录已处理过的 event_id，重复事件直接跳过，避免产生重复 AgentRun。
CREATE TABLE feishu_webhook_events (
    event_id    TEXT PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- TTL 清理任务按 received_at 批量删除过期记录，此索引支撑该查询
CREATE INDEX idx_feishu_webhook_events_received_at ON feishu_webhook_events(received_at);
