package synthesis

import (
	"context"

	"github.com/google/uuid"
)

type ClientFactory interface {
	GetLLM(ctx context.Context, userID uuid.UUID) (LLMClient, error)
	GetEmbedder(ctx context.Context, userID uuid.UUID) (*EmbeddingProvider, error)
	GetLLMTemperature(ctx context.Context, userID uuid.UUID) float32
	GetLLMMaxTokens(ctx context.Context, userID uuid.UUID) int
}
