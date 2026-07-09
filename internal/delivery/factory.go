package delivery

import (
	"context"

	"github.com/google/uuid"
)

type AdapterFactory interface {
	GetNotionAdapter(ctx context.Context, userID uuid.UUID) (*NotionAdapter, error)
	GetObsidianAdapter(ctx context.Context, userID uuid.UUID) (*ObsidianAdapter, error)
	// 决策 #7: 飞书文档交付适配器
	GetFeishuAdapter(ctx context.Context, userID uuid.UUID) (*FeishuAdapter, error)
}
