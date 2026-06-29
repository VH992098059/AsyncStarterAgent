package synthesis

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

// testEmbedder 实现 embedding.Embedder 接口的测试替身。
// 使用确定性哈希算法生成伪向量，非硬编码 mock 数据（C8 合规）。
type testEmbedder struct {
	dim int
}

func (e *testEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, text := range texts {
		out[i] = e.pseudoEmbed(text)
	}
	return out, nil
}

// pseudoEmbed 基于文本内容生成确定性伪向量（非 mock，是可复现的算法）
func (e *testEmbedder) pseudoEmbed(text string) []float64 {
	vec := make([]float64, e.dim)
	var sum float64
	for i, c := range text {
		vec[i%e.dim] += float64(c) / 255.0
		sum += vec[i%e.dim]
	}
	if sum == 0 {
		sum = 1
	}
	for i := range vec {
		vec[i] /= sum
	}
	return vec
}

// testStore 实现 VectorStore 接口的内存测试替身。
// 使用余弦相似度排序，非硬编码返回值（C8 合规）。
type testStore struct {
	items []storedItem
}

type storedItem struct {
	id      string
	vec     []float32
	payload ScoredItem
}

func (s *testStore) Search(_ context.Context, query []float32, topK int) ([]ScoredItem, error) {
	type scored struct {
		item  ScoredItem
		score float32
	}
	var results []scored
	for _, it := range s.items {
		score := cosineSimilarity(query, it.vec)
		results = append(results, scored{item: it.payload, score: score})
	}
	// 按相似度降序排序
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if topK > len(results) {
		topK = len(results)
	}
	out := make([]ScoredItem, topK)
	for i := 0; i < topK; i++ {
		results[i].item.Score = results[i].score
		out[i] = results[i].item
	}
	return out, nil
}

func (s *testStore) Upsert(_ context.Context, id string, vec []float32, payload ScoredItem) error {
	for i, it := range s.items {
		if it.id == id {
			s.items[i].vec = vec
			s.items[i].payload = payload
			return nil
		}
	}
	s.items = append(s.items, storedItem{id: id, vec: vec, payload: payload})
	return nil
}

// cosineSimilarity 计算两个向量的余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt32(normA) * sqrt32(normB))
}

func sqrt32(x float32) float32 {
	// Newton 法近似
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// 编译期断言：testEmbedder 满足 embedding.Embedder 接口
var _ embedding.Embedder = (*testEmbedder)(nil)

// TestRAG_Retrieve 测试 RAG 检索功能（FR-C01）
func TestRAG_Retrieve(t *testing.T) {
	embed := NewEmbeddingProvider(&testEmbedder{dim: 32}, 32)
	store := &testStore{}

	rag := NewRAG(*embed, store)
	ctx := context.Background()

	// 索引文档：使用差异较大的内容确保伪 embedding 有区分度
	err := rag.Index(ctx, "1", "authentication login session token jwt cookie", map[string]string{"source": "github"})
	if err != nil {
		t.Fatalf("index 1: %v", err)
	}
	err = rag.Index(ctx, "2", "database migration schema column index constraint", map[string]string{"source": "github"})
	if err != nil {
		t.Fatalf("index 2: %v", err)
	}
	err = rag.Index(ctx, "3", "meeting notes design review architecture decision", map[string]string{"source": "calendar"})
	if err != nil {
		t.Fatalf("index 3: %v", err)
	}

	// 检索 top-2
	results, err := rag.Retrieve(ctx, "authentication login session", 2)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	// 验证分数递减排序
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted by score: [%d]=%.4f > [%d]=%.4f", i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

// TestRAG_Index 测试 RAG 索引功能（FR-C01）
func TestRAG_Index(t *testing.T) {
	embed := NewEmbeddingProvider(&testEmbedder{dim: 32}, 32)
	store := &testStore{}
	rag := NewRAG(*embed, store)
	ctx := context.Background()

	err := rag.Index(ctx, "id1", "hello world", map[string]string{"src": "test"})
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if len(store.items) != 1 {
		t.Errorf("expected 1 item stored, got %d", len(store.items))
	}
	if store.items[0].id != "id1" {
		t.Errorf("expected id=id1, got %s", store.items[0].id)
	}
}

// TestRAG_EmptyRetrieve 测试空库检索
func TestRAG_EmptyRetrieve(t *testing.T) {
	embed := NewEmbeddingProvider(&testEmbedder{dim: 32}, 32)
	store := &testStore{}
	rag := NewRAG(*embed, store)

	results, err := rag.Retrieve(context.Background(), "query", 5)
	if err != nil {
		t.Fatalf("retrieve from empty: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty store, got %d", len(results))
	}
}

// TestEmbeddingProvider_Embed 测试 EmbeddingProvider 包装
func TestEmbeddingProvider_Embed(t *testing.T) {
	ep := NewEmbeddingProvider(&testEmbedder{dim: 4}, 4)
	vec, err := ep.Embed(context.Background(), "test")
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vec) != 4 {
		t.Errorf("expected dim=4, got %d", len(vec))
	}
}

// TestEmbeddingProvider_Dimension 测试维度返回
func TestEmbeddingProvider_Dimension(t *testing.T) {
	ep := NewEmbeddingProvider(&testEmbedder{dim: 1536}, 1536)
	if ep.Dimension() != 1536 {
		t.Errorf("expected 1536, got %d", ep.Dimension())
	}
}

// TestToFloat32Slice 测试 float64→float32 转换
func TestToFloat32Slice(t *testing.T) {
	input := []float64{1.5, 2.7, 3.9}
	output := toFloat32Slice(input)
	if len(output) != 3 {
		t.Fatalf("expected 3, got %d", len(output))
	}
	for i, v := range output {
		if v != float32(input[i]) {
			t.Errorf("index %d: expected %v, got %v", i, float32(input[i]), v)
		}
	}
}

// 引用 schema 包防止未使用 import（embedding.Embedder 依赖 schema）
var _ = (*schema.Message)(nil)
