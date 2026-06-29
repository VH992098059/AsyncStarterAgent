// T015: EinoEmbedder — 基于 Eino 框架的 OpenAI Embedding 实现（FR-C01）
// 按用户决策，全程使用 Eino 框架，不手写 OpenAI HTTP 客户端。
package synthesis

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

// EinoEmbedderConfig 配置 Eino OpenAI Embedder
type EinoEmbedderConfig struct {
	APIKey  string // OPENAI_API_KEY
	Model   string // 默认 text-embedding-3-small
	BaseURL string // 可选，Azure 或代理地址
	Dim     int    // 向量维度，默认 1536
}

// NewEinoEmbedder 创建基于 Eino 的 OpenAI Embedder（FR-C01）。
// 使用 eino-ext/components/embedding/openai 真实调用 OpenAI API。
func NewEinoEmbedder(ctx context.Context, cfg EinoEmbedderConfig) (*EmbeddingProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("eino embedder: APIKey is required")
	}
	if cfg.Model == "" {
		cfg.Model = "text-embedding-3-small"
	}
	if cfg.Dim == 0 {
		cfg.Dim = 1536
	}

	einoCfg := &openai.EmbeddingConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.BaseURL != "" {
		einoCfg.BaseURL = cfg.BaseURL
	}

	embedder, err := openai.NewEmbedder(ctx, einoCfg)
	if err != nil {
		return nil, fmt.Errorf("eino embedder init: %w", err)
	}

	return NewEmbeddingProvider(embedder, cfg.Dim), nil
}
