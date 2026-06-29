-- T015: 回滚 embedding 列
DROP INDEX IF EXISTS idx_context_items_embedding;
ALTER TABLE context_items DROP COLUMN IF EXISTS embedding;
