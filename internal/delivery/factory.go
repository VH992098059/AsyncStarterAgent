package delivery

import (
	"context"

	"github.com/google/uuid"
)

type AdapterFactory interface {
	GetNotionAdapter(ctx context.Context, userID uuid.UUID) (*NotionAdapter, error)
	GetObsidianAdapter(ctx context.Context, userID uuid.UUID) (*ObsidianAdapter, error)
}
