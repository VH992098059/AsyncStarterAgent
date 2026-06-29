-- T015: 添加 embedding 列到 context_items（FR-C01 RAG 语义检索）
-- 需要 pgvector 扩展
CREATE EXTENSION IF NOT EXISTS vector;

-- 添加 embedding 向量列（text-embedding-3-small 输出 1536 维）
ALTER TABLE context_items ADD COLUMN embedding vector(1536);

-- 向量相似度搜索索引（HNSW，cosine 距离）
CREATE INDEX idx_context_items_embedding ON context_items
    USING hnsw (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL;
