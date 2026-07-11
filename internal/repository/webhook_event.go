package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TryInsertWebhookEvent 尝试记录一个 webhook 事件（幂等去重）。
// source 为触发来源（如 "feishu"），eventID 为该来源的事件唯一标识（如飞书 header.event_id）。
// 返回 true 表示本次是首次收到该事件（应继续处理）；返回 false 表示重复事件（应跳过业务处理）。
// 依赖 0011 迁移建立的 (source, event_id) UNIQUE 约束做 INSERT ... ON CONFLICT DO NOTHING。
func TryInsertWebhookEvent(ctx context.Context, pool *pgxpool.Pool, source, eventID string) (bool, error) {
	const q = `INSERT INTO webhook_events (source, event_id)
	           VALUES ($1, $2)
	           ON CONFLICT (source, event_id) DO NOTHING`
	tag, err := pool.Exec(ctx, q, source, eventID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
