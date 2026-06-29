// Package synthesis 实现草稿生成模块（Phase 3: FR-C01~C05）。
// T015: RAG 向量检索（FR-C01 P0）
package synthesis

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
)

// EmbeddingProvider 包装 Eino Embedder 接口，提供单文本 embedding 便利方法。
// 按用户决策，全程使用 Eino 框架，不手写 OpenAI HTTP 客户端。
type EmbeddingProvider struct {
	embed embedding.Embedder
	dim   int
}

// NewEmbeddingProvider 创建 EmbeddingProvider。
// dim 为向量维度（text-embedding-3-small 为 1536）。
func NewEmbeddingProvider(embed embedding.Embedder, dim int) *EmbeddingProvider {
	return &EmbeddingProvider{embed: embed, dim: dim}
}

// Embed 对单段文本生成向量（FR-C01）。
func (p *EmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := p.embed.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("embedding: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("embedding: empty result")
	}
	return toFloat32Slice(vecs[0]), nil
}

// Dimension 返回向量维度。
func (p *EmbeddingProvider) Dimension() int { return p.dim }

// ScoredItem 检索结果（FR-C01）
type ScoredItem struct {
	ID       string            `json:"id"`
	Score    float32           `json:"score"`
	Content  string            `json:"content"`
	Source   string            `json:"source"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// VectorStore 向量存储接口（FR-C01）
type VectorStore interface {
	Search(ctx context.Context, vec []float32, topK int) ([]ScoredItem, error)
	Upsert(ctx context.Context, id string, vec []float32, payload ScoredItem) error
}

// RAG 检索增强生成核心（FR-C01 P0）
type RAG struct {
	embed EmbeddingProvider
	store VectorStore
}

// NewRAG 创建 RAG 实例。
func NewRAG(embed EmbeddingProvider, store VectorStore) *RAG {
	return &RAG{embed: embed, store: store}
}

// Retrieve 检索与 query 语义最相关的 topK 项（FR-C01）。
func (r *RAG) Retrieve(ctx context.Context, query string, topK int) ([]ScoredItem, error) {
	vec, err := r.embed.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("rag retrieve embed: %w", err)
	}
	return r.store.Search(ctx, vec, topK)
}

// Index 把 item 索引到向量库（FR-C01）。
func (r *RAG) Index(ctx context.Context, id, content string, meta map[string]string) error {
	vec, err := r.embed.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("rag index embed: %w", err)
	}
	return r.store.Upsert(ctx, id, vec, ScoredItem{
		ID:       id,
		Content:  content,
		Metadata: meta,
	})
}

// toFloat32Slice 将 []float64 转为 []float32。
// Eino EmbedStrings 返回 [][]float64，但 pgvector 使用 []float32。
func toFloat32Slice(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}
