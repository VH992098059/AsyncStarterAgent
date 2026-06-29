// T015: PGVectorStore — 基于 pgvector 的向量存储实现（FR-C01）
package synthesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// PGVectorStore 基于 PostgreSQL + pgvector 的向量存储（FR-C01）
type PGVectorStore struct {
	pool *pgxpool.Pool
	dim  int
}

// NewPGVectorStore 创建 PGVectorStore 实例。
// dim 为向量维度，需与 embedding 模型输出一致（text-embedding-3-small 为 1536）。
func NewPGVectorStore(pool *pgxpool.Pool, dim int) *PGVectorStore {
	return &PGVectorStore{pool: pool, dim: dim}
}

// Search 使用余弦距离检索最相似的 topK 项（FR-C01）。
// SQL 使用 <=> 运算符（cosine distance），1 - distance 即为 cosine similarity。
func (s *PGVectorStore) Search(ctx context.Context, vec []float32, topK int) ([]ScoredItem, error) {
	v := pgvector.NewVector(vec)
	rows, err := s.pool.Query(ctx, `
		SELECT external_id, content, source, metadata,
		       1 - (embedding <=> $1) AS score
		FROM context_items
		WHERE embedding IS NOT NULL AND is_noise = false
		ORDER BY embedding <=> $1
		LIMIT $2
	`, v, topK)
	if err != nil {
		return nil, fmt.Errorf("pgvector search: %w", err)
	}
	defer rows.Close()

	var out []ScoredItem
	for rows.Next() {
		var it ScoredItem
		var metaBytes []byte
		if err := rows.Scan(&it.ID, &it.Content, &it.Source, &metaBytes, &it.Score); err != nil {
			return nil, fmt.Errorf("pgvector scan: %w", err)
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &it.Metadata)
		}
		out = append(out, it)
	}
	return out, nil
}

// Upsert 将向量写入 context_items.embedding 列（FR-C01）。
// 使用 ON CONFLICT DO UPDATE 实现 upsert 语义。
func (s *PGVectorStore) Upsert(ctx context.Context, id string, vec []float32, payload ScoredItem) error {
	v := pgvector.NewVector(vec)
	metaJSON, err := json.Marshal(payload.Metadata)
	if err != nil {
		return fmt.Errorf("pgvector marshal metadata: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE context_items
		SET embedding = $1, metadata = $2
		WHERE external_id = $3
	`, v, metaJSON, id)
	if err != nil {
		return fmt.Errorf("pgvector upsert: %w", err)
	}
	return nil
}
