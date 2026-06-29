# Phase 3: 草稿生成（W7-W10）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **AI 行为约束**: 严格遵守 [ai-coding-boundary.md](../../ai-coding-boundary.md)。任何偏离需先询问。

**Goal**: 实现模块 C（草稿生成）：RAG 检索 + Eino 编排 + 模板引擎 + LLM 润色 + [待补充] 标记 + SSE 流式输出。

**关联需求**:
- FR-C01 (P0): RAG 语义检索，召回率 > 80%
- FR-C02 (P1): 模板套用（周报/总结/规划/纪要/通用）
- FR-C03 (P0): LLM 润色生成，通顺度 > 4/5
- FR-C04 (P1): [待补充] 标记系统，位置可定位
- FR-C05 (P0): SSE 流式输出，首字节 < 3s
- T026 (P0): Eino Workflow 5 阶段 DAG 集成

**退出标准（M3）**:
- [ ] RAG 检索单元测试：mock embeddings，召回率达标
- [ ] Eino DAG 跑通
- [ ] 模板引擎支持 4 种类型 + 变量插值
- [ ] LLM 调用 streaming 接口可工作（mock 客户端）
- [ ] [待补充] 标记可解析、定位、移除
- [ ] SSE 接口 `/api/v1/drafts/{id}/stream` 流式推送
- [ ] 端到端：手动触发 → 30 秒内生成首版草稿

**目录新增**:
```
internal/synthesis/
├── rag.go                    # RAG 检索
├── rag_test.go
├── template.go               # 模板引擎
├── template_test.go
├── llm.go                    # LLM 客户端抽象
├── llm_test.go
├── marks.go                  # [待补充] 标记
├── marks_test.go
├── sse.go                    # SSE 事件
├── sse_test.go
├── service.go                # 草稿生成服务
├── service_test.go
└── agent/
    ├── dag.go                # Eino DAG
    └── dag_test.go
migrations/
└── 0005_embeddings.up.sql
```

---

## Task T015: RAG 向量检索模块

**Files:**
- Create: `migrations/0005_embeddings.up.sql`
- Create: `migrations/0005_embeddings.down.sql`
- Create: `internal/synthesis/rag.go`
- Create: `internal/synthesis/rag_test.go`

**关联**: FR-C01 (P0)

- [ ] **Step 1: 写 embeddings 表迁移（如果 0004 未建）**

> **边界说明**: Phase 2 T014 Step 1 中 `context_items.embedding` 字段已建，本任务无需新建表。跳过此步。

- [ ] **Step 2: 写 embedding provider 接口**

Create file `internal/synthesis/rag.go`:
```go
package synthesis

import "context"

type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

// ScoredItem 检索结果
type ScoredItem struct {
	ID       string
	Score    float32
	Content  string
	Source   string
	Metadata map[string]string
}

type RAG struct {
	embed EmbeddingProvider
	store VectorStore
}

type VectorStore interface {
	Search(ctx context.Context, vec []float32, topK int) ([]ScoredItem, error)
	Upsert(ctx context.Context, id string, vec []float32, payload ScoredItem) error
}

func NewRAG(embed EmbeddingProvider, store VectorStore) *RAG {
	return &RAG{embed: embed, store: store}
}

// Retrieve 检索与 query 语义最相关的 topK 项
func (r *RAG) Retrieve(ctx context.Context, query string, topK int) ([]ScoredItem, error) {
	vec, err := r.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.store.Search(ctx, vec, topK)
}

// Index 把 item 索引到向量库
func (r *RAG) Index(ctx context.Context, id, content string, meta map[string]string) error {
	vec, err := r.embed.Embed(ctx, content)
	if err != nil {
		return err
	}
	return r.store.Upsert(ctx, id, vec, ScoredItem{
		ID:       id,
		Content:  content,
		Metadata: meta,
	})
}
```

- [ ] **Step 3: 写 RAG 测试（mock provider + store）**

Create file `internal/synthesis/rag_test.go`:
```go
package synthesis

import (
	"context"
	"testing"
)

type mockEmbed struct{ dim int }

func (m *mockEmbed) Embed(_ context.Context, text string) ([]float32, error) {
	v := make([]float32, m.dim)
	// 简单伪 embedding：把字符串 hash 后归一化
	var sum float32
	for i, c := range text {
		v[i%m.dim] += float32(c) / 255.0
		sum += v[i%m.dim]
	}
	if sum == 0 {
		sum = 1
	}
	for i := range v {
		v[i] /= sum
	}
	return v, nil
}
func (m *mockEmbed) Dimension() int { return m.dim }

type mockStore struct{ items []ScoredItem }

func (s *mockStore) Search(_ context.Context, vec []float32, topK int) ([]ScoredItem, error) {
	if topK > len(s.items) {
		topK = len(s.items)
	}
	return s.items[:topK], nil
}
func (s *mockStore) Upsert(_ context.Context, id string, vec []float32, p ScoredItem) error {
	p.ID = id
	s.items = append(s.items, p)
	return nil
}

func TestRAG_Retrieve(t *testing.T) {
	store := &mockStore{items: []ScoredItem{
		{ID: "1", Content: "login flow", Score: 0.9},
		{ID: "2", Content: "logout flow", Score: 0.8},
	}}
	rag := NewRAG(&mockEmbed{dim: 8}, store)
	got, err := rag.Retrieve(context.Background(), "test query", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestRAG_Index(t *testing.T) {
	store := &mockStore{}
	rag := NewRAG(&mockEmbed{dim: 8}, store)
	err := rag.Index(context.Background(), "id1", "hello", map[string]string{"src": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.items) != 1 {
		t.Errorf("expected 1 item stored, got %d", len(store.items))
	}
}
```

- [ ] **Step 4: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: FAIL — `RAG` undefined

- [ ] **Step 5: 跑测试通过**（Step 2 已实现）

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: PASS

- [ ] **Step 6: 写 PG vector store 真实实现**

Create file `internal/synthesis/pgvector.go`:
```go
package synthesis

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type PGVectorStore struct {
	pool *pgxpool.Pool
	dim  int
}

func NewPGVectorStore(pool *pgxpool.Pool, dim int) *PGVectorStore {
	return &PGVectorStore{pool: pool, dim: dim}
}

func (s *PGVectorStore) Search(ctx context.Context, vec []float32, topK int) ([]ScoredItem, error) {
	v := pgvector.NewVector(vec)
	rows, err := s.pool.Query(ctx, `
		SELECT external_id, title, content, source,
		       1 - (embedding <=> $1) AS score
		FROM context_items
		WHERE embedding IS NOT NULL AND is_noise = false
		ORDER BY embedding <=> $1
		LIMIT $2
	`, v, topK)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	var out []ScoredItem
	for rows.Next() {
		var it ScoredItem
		if err := rows.Scan(&it.ID, &it.Content, &it.Content, &it.Source, &it.Score); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func (s *PGVectorStore) Upsert(ctx context.Context, id string, vec []float32, payload ScoredItem) error {
	v := pgvector.NewVector(vec)
	_, err := s.pool.Exec(ctx, `
		UPDATE context_items
		SET embedding = $1
		WHERE external_id = $2
	`, v, id)
	return err
}
```

- [ ] **Step 7: 添加 pgvector 依赖**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go get github.com/pgvector/pgvector-go@v0.1.0`

- [ ] **Step 8: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 9: 展示 diff 等用户决定**

---

## Task T016: Eino Agent 编排实现

**Files:**
- Create: `internal/synthesis/agent/dag.go`
- Create: `internal/synthesis/agent/dag_test.go`

**关联**: FR-C03 (P0), T026

- [ ] **Step 1: 添加 Eino 依赖**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
go get github.com/cloudwego/eino@v0.0.0-20241024065831-a2c0a3a40a8e
```
Expected: go.mod 增加 eino 依赖（精确版本由 user 决定是否锁定）

> **边界说明**: Eino 是字节开源的 LLM 编排框架。如果实际版本号需调整，按 ai-coding-boundary §3.2 询问用户。

- [ ] **Step 2: 写 DAG 节点**

Create file `internal/synthesis/agent/dag.go`:
```go
package agent

import (
	"context"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// Node 单个 DAG 节点
type Node interface {
	Name() string
	Run(ctx context.Context, in State) (State, error)
}

// State 节点间传递的状态
type State struct {
	AgentRunID  string
	UserID      string
	TaskType    string
	Context     []harvesting.ContextItem
	Retrieved   []harvesting.ContextItem
	Template    string
	Draft       string
	Completeness float32
	Marks       []Mark
	Errors      []error
}

type Mark struct {
	ID       string `json:"id"`
	Hint     string `json:"hint"`
	Position int    `json:"position"`
	Resolved bool   `json:"resolved"`
}

// DAG 5 阶段流水线（FR-C03 + T026）
type DAG struct {
	Nodes []Node
}

func NewDAG(nodes ...Node) *DAG {
	return &DAG{Nodes: nodes}
}

func (d *DAG) Run(ctx context.Context, s State) (State, error) {
	for _, n := range d.Nodes {
		out, err := n.Run(ctx, s)
		if err != nil {
			s.Errors = append(s.Errors, err)
			return s, err
		}
		s = out
	}
	return s, nil
}
```

- [ ] **Step 3: 写 DAG 测试（mock 节点）**

Create file `internal/synthesis/agent/dag_test.go`:
```go
package agent

import (
	"context"
	"errors"
	"testing"
)

type incrementNode struct{}

func (incrementNode) Name() string { return "inc" }
func (incrementNode) Run(_ context.Context, s State) (State, error) {
	s.Completeness += 0.1
	return s, nil
}

type failNode struct{}

func (failNode) Name() string { return "fail" }
func (failNode) Run(_ context.Context, s State) (State, error) {
	return s, errors.New("simulated")
}

func TestDAG_AllNodesRun(t *testing.T) {
	d := NewDAG(incrementNode{}, incrementNode{}, incrementNode{})
	s, err := d.Run(context.Background(), State{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Completeness < 0.29 || s.Completeness > 0.31 {
		t.Errorf("completeness: %v", s.Completeness)
	}
}

func TestDAG_StopsOnError(t *testing.T) {
	d := NewDAG(incrementNode{}, failNode{}, incrementNode{})
	_, err := d.Run(context.Background(), State{})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/agent/...`
Expected: PASS

- [ ] **Step 5: 实现具体节点（retrieve/template/llm/mark）**

Create file `internal/synthesis/agent/nodes.go`:
```go
package agent

import (
	"context"

	"github.com/asyncstarter/agent/internal/synthesis"
)

// RetrieveNode 检索相关上下文
type RetrieveNode struct {
	RAG *synthesis.RAG
}

func (n *RetrieveNode) Name() string { return "retrieve" }
func (n *RetrieveNode) Run(ctx context.Context, s State) (State, error) {
	query := s.TaskType + " " + s.AgentRunID
	items, err := n.RAG.Retrieve(ctx, query, 20)
	if err != nil {
		return s, err
	}
	for _, it := range items {
		s.Retrieved = append(s.Retrieved, itemFromScored(it))
	}
	s.Completeness = 0.3
	return s, nil
}

func itemFromScored(s synthesis.ScoredItem) (out _Item) {
	return _Item{}.from(s)
}

type _Item struct {
	ID, Source, Content string
}

func (_Item) from(s synthesis.ScoredItem) _Item {
	return _Item{ID: s.ID, Source: s.Source, Content: s.Content}
}

// ... 其它节点在 T017/T018/T019 中实现
```

> **简化说明**: 实际节点实现分散在 T017/T018/T019。T016 任务只建立 DAG 骨架与测试。

- [ ] **Step 6: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 7: 展示 diff 等用户决定**

---

## Task T017: 模板引擎开发

**Files:**
- Create: `internal/synthesis/template.go`
- Create: `internal/synthesis/template_test.go`
- Create: `templates/weekly_report.md.tmpl`
- Create: `templates/summary.md.tmpl`
- Create: `templates/plan.md.tmpl`
- Create: `templates/meeting_minutes.md.tmpl`

**关联**: FR-C02 (P1), FR-C03 (P0)

- [ ] **Step 1: 写模板引擎（Go text/template 包装）**

Create file `internal/synthesis/template.go`:
```go
package synthesis

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"
)

type Template struct {
	tpl *template.Template
}

type TemplateData struct {
	TaskType   string
	Title      string
	Items      []ContextItem
	Now        time.Time
	UserID     string
	CustomVars map[string]string
}

func LoadTemplate(name, path string) (*Template, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}
	tpl, err := template.New(name).Funcs(funcMap()).Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	return &Template{tpl: tpl}, nil
}

func (t *Template) Render(data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := t.tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"upper": strings.ToUpper,
		"join":  strings.Join,
		"date":  func(format string) string { return time.Now().Format(format) },
	}
}

// SelectByTaskType 根据 task_type 选择模板文件
func SelectByTaskType(taskType string) string {
	switch taskType {
	case "weekly_report":
		return "templates/weekly_report.md.tmpl"
	case "summary":
		return "templates/summary.md.tmpl"
	case "plan":
		return "templates/plan.md.tmpl"
	case "meeting_minutes":
		return "templates/meeting_minutes.md.tmpl"
	default:
		return "templates/summary.md.tmpl"
	}
}
```

- [ ] **Step 2: 写 4 个模板文件**

Create file `templates/weekly_report.md.tmpl`:
```markdown
# {{.Title}}

> 生成时间：{{date "2006-01-02"}}

## 本周要点

{{range .Items}}- **{{.Title}}** ({{.Source}}){{end}}

## 详细记录

{{range .Items}}
### {{.Title}}

{{.Content}}

来源：{{.URL}}

{{end}}
```

Create file `templates/summary.md.tmpl`:
```markdown
# {{.Title}}

> 总结时间：{{date "2006-01-02"}}

## 核心结论

{{range .Items}}- {{.Title}}
{{end}}

## 详细说明

{{range .Items}}
### {{.Title}}

{{.Content}}

{{end}}
```

Create file `templates/plan.md.tmpl`:
```markdown
# {{.Title}}

> 规划时间：{{date "2006-01-02"}}

## 目标

{{range .Items}}- {{.Title}}
{{end}}

## 行动项

{{range .Items}}
### {{.Title}}

{{.Content}}

{{end}}
```

Create file `templates/meeting_minutes.md.tmpl`:
```markdown
# {{.Title}} - 会议纪要

> 时间：{{date "2006-01-02 15:04"}}

## 参会与议题

{{range .Items}}### {{.Title}}

{{.Content}}

{{end}}
```

- [ ] **Step 3: 写模板测试**

Create file `internal/synthesis/template_test.go`:
```go
package synthesis

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTemplate_LoadAndRender(t *testing.T) {
	dir := t.TempDir()
	tplPath := filepath.Join(dir, "test.tmpl")
	err := os.WriteFile(tplPath, []byte("# {{.Title}}\n{{range .Items}}- {{.Title}}\n{{end}}"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := LoadTemplate("test", tplPath)
	if err != nil {
		t.Fatal(err)
	}
	data := TemplateData{
		Title: "Week",
		Items: []ContextItem{{Title: "A"}, {Title: "B"}},
		Now:   time.Now(),
	}
	out, err := tpl.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "Week") || !contains(out, "A") || !contains(out, "B") {
		t.Errorf("render output: %s", out)
	}
}

func TestTemplate_SelectByTaskType(t *testing.T) {
	cases := map[string]string{
		"weekly_report":    "templates/weekly_report.md.tmpl",
		"summary":          "templates/summary.md.tmpl",
		"plan":             "templates/plan.md.tmpl",
		"meeting_minutes":  "templates/meeting_minutes.md.tmpl",
		"unknown":          "templates/summary.md.tmpl",
	}
	for in, want := range cases {
		if got := SelectByTaskType(in); got != want {
			t.Errorf("%s: want %s, got %s", in, want, got)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: PASS

- [ ] **Step 5: 展示 diff 等用户决定**

---

## Task T018: LLM 调用与润色

**Files:**
- Create: `internal/synthesis/llm.go`
- Create: `internal/synthesis/llm_test.go`

**关联**: FR-C03 (P0)

- [ ] **Step 1: 写 LLM 客户端接口**

Create file `internal/synthesis/llm.go`:
```go
package synthesis

import "context"

type Message struct {
	Role    string // system / user / assistant
	Content string
}

type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float32
	MaxTokens   int
	Stream      bool
}

type ChatChunk struct {
	Content string
	Done    bool
	Err     error
}

type LLMClient interface {
	Chat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// PromptBuilder 构造 LLM prompt
type PromptBuilder struct{}

func (PromptBuilder) Build(taskType, draft string, items []ContextItem) []Message {
	itemSummary := ""
	for _, it := range items {
		itemSummary += "- " + it.Title + " (" + it.Source + ")\n"
	}

	system := "你是一个专业的文档润色助手。请保持原意，修正语法，提升可读性。对不确定的内容使用 [待补充:说明] 标记。"
	if taskType == "weekly_report" {
		system += "这是一份周报，重点突出本周完成的工作。"
	}
	return []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: "素材：\n" + itemSummary + "\n\n草稿：\n" + draft + "\n\n请输出润色后的 markdown。"},
	}
}
```

- [ ] **Step 2: 写 mock LLM 客户端**

Create file `internal/synthesis/llm_test.go`:
```go
package synthesis

import (
	"context"
	"errors"
	"testing"
)

type mockLLM struct {
	chunks []string
	fail   bool
}

func (m *mockLLM) Chat(_ context.Context, _ ChatRequest) (<-chan ChatChunk, error) {
	if m.fail {
		return nil, errors.New("llm unavailable")
	}
	out := make(chan ChatChunk, len(m.chunks)+1)
	for _, c := range m.chunks {
		out <- ChatChunk{Content: c}
	}
	out <- ChatChunk{Done: true}
	close(out)
	return out, nil
}

func TestLLM_Chat_Stream(t *testing.T) {
	cli := &mockLLM{chunks: []string{"Hello", " world", "!"}}
	ch, err := cli.Chat(context.Background(), ChatRequest{Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	for c := range ch {
		if c.Err != nil {
			t.Fatal(c.Err)
		}
		got += c.Content
		if c.Done {
			break
		}
	}
	if got != "Hello world!" {
		t.Errorf("got: %q", got)
	}
}

func TestPromptBuilder(t *testing.T) {
	pb := PromptBuilder{}
	msgs := pb.Build("weekly_report", "draft", []ContextItem{{Title: "task A", Source: "github"}})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first role: %s", msgs[0].Role)
	}
}

func TestLLM_Chat_Error(t *testing.T) {
	cli := &mockLLM{fail: true}
	_, err := cli.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: FAIL — `LLMClient` undefined

- [ ] **Step 4: 跑测试通过**（Step 1 已实现）

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: PASS

- [ ] **Step 5: 写真实 OpenAI 客户端（gpt-4o-mini streaming）**

Create file `internal/synthesis/openai.go`:
```go
package synthesis

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAIClient struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		APIKey:  apiKey,
		BaseURL: "https://api.openai.com/v1",
		Model:   model,
		HTTP:    http.DefaultClient,
	}
}

func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	body := map[string]interface{}{
		"model":       req.Model,
		"messages":    toOpenAIMsgs(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return nil, err
	}
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", buf)
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai status %d", resp.StatusCode)
	}
	out := make(chan ChatChunk)
	go c.readSSE(ctx, resp.Body, out)
	return out, nil
}

func (c *OpenAIClient) readSSE(ctx context.Context, body io.Reader, out chan<- ChatChunk) {
	defer close(out)
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			out <- ChatChunk{Done: true}
			return
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			out <- ChatChunk{Err: err}
			return
		}
		for _, ch := range ev.Choices {
			if ch.Delta.Content != "" {
				out <- ChatChunk{Content: ch.Delta.Content}
			}
		}
	}
}

func toOpenAIMsgs(msgs []Message) []map[string]string {
	out := make([]map[string]string, len(msgs))
	for i, m := range msgs {
		out[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	return out
}
```

- [ ] **Step 6: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 7: 展示 diff 等用户决定**

---

## Task T019: [待补充] 标记系统

**Files:**
- Create: `internal/synthesis/marks.go`
- Create: `internal/synthesis/marks_test.go`

**关联**: FR-C04 (P1)

- [ ] **Step 1: 写 Mark 解析与替换**

Create file `internal/synthesis/marks.go`:
```go
package synthesis

import (
	"fmt"
	"regexp"
)

var markRe = regexp.MustCompile(`\[待补充:([^\]]+)\]`)

// ExtractMarks 从 markdown 提取所有 [待补充:xxx] 标记
func ExtractMarks(md string) []Mark {
	matches := markRe.FindAllStringSubmatchIndex(md, -1)
	out := make([]Mark, 0, len(matches))
	for i, m := range matches {
		hint := md[m[2]:m[3]]
		uid := fmt.Sprintf("mark-%d", i)
		out = append(out, Mark{
			ID:       uid,
			Hint:     hint,
			Position: m[0],
		})
	}
	return out
}

// ReplaceMark 用 user 提供的值替换指定 mark
func ReplaceMark(md string, markID string, items []Mark, value string) (string, error) {
	for i, m := range items {
		if m.ID != markID {
			continue
		}
		// 找到 mark 在原文的位置并替换
		// 简化：mark 的 Position 不可靠（之前可能有变化），按 hint 匹配
		re := regexp.MustCompile(regexp.QuoteMeta("[待补充:" + m.Hint + "]"))
		return re.ReplaceAllString(md, value), nil
	}
	return md, fmt.Errorf("mark not found: %s", markID)
}

// Completeness 基于 mark 数量计算完成度（0.0 - 1.0）
// mark 越多完成度越低
func Completeness(md string) float32 {
	total := float32(len(md))
	marks := float32(len(ExtractMarks(md)))
	if total == 0 {
		return 0
	}
	// 简化模型：mark 占总长度 30% 视为 0% 完成度
	markPenalty := marks * 20.0
	if markPenalty > total {
		return 0
	}
	return 1.0 - markPenalty/total
}
```

- [ ] **Step 2: 写 Mark 测试**

Create file `internal/synthesis/marks_test.go`:
```go
package synthesis

import (
	"testing"
)

func TestExtractMarks(t *testing.T) {
	md := "Hello [待补充:具体数字] world [待补充:负责人]"
	marks := ExtractMarks(md)
	if len(marks) != 2 {
		t.Fatalf("expected 2 marks, got %d", len(marks))
	}
	if marks[0].Hint != "具体数字" {
		t.Errorf("first hint: %s", marks[0].Hint)
	}
	if marks[1].Hint != "负责人" {
		t.Errorf("second hint: %s", marks[1].Hint)
	}
}

func TestReplaceMark(t *testing.T) {
	md := "Hello [待补充:数字] world"
	marks := ExtractMarks(md)
	out, err := ReplaceMark(md, marks[0].ID, marks, "42")
	if err != nil {
		t.Fatal(err)
	}
	if out != "Hello 42 world" {
		t.Errorf("got: %s", out)
	}
}

func TestCompleteness(t *testing.T) {
	cases := []struct {
		md  string
		min float32
	}{
		{"no marks here", 0.9},
		{"one [待补充:foo] mark", 0.5},
		{"[待补充:a][待补充:b][待补充:c][待补充:d][待补充:e][待补充:f]", 0.0},
	}
	for _, c := range cases {
		got := Completeness(c.md)
		if got < c.min-0.05 {
			t.Errorf("md=%q: completeness=%v < min %v", c.md, got, c.min)
		}
	}
}
```

- [ ] **Step 3: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: PASS

- [ ] **Step 4: 展示 diff 等用户决定**

---

## Task T020: SSE 流式输出接口

**Files:**
- Create: `internal/synthesis/sse.go`
- Create: `internal/synthesis/sse_test.go`
- Create: `internal/handler/draft.go`
- Create: `internal/handler/draft_test.go`
- Modify: `internal/server/server.go`

**关联**: FR-C05 (P0), NFR-04 (低延迟)

- [ ] **Step 1: 写 SSE helper**

Create file `internal/synthesis/sse.go`:
```go
package synthesis

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter 简单 SSE writer
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	return &SSEWriter{w: w, flusher: f}, nil
}

// EventType: delta / complete / error / mark
func (s *SSEWriter) Write(eventType string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", eventType, body); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
```

- [ ] **Step 2: 写 SSE 测试**

Create file `internal/synthesis/sse_test.go`:
```go
package synthesis

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSEWriter_Write(t *testing.T) {
	w := httptest.NewRecorder()
	sse, err := NewSSEWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := sse.Write("delta", map[string]string{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := sse.Write("complete", map[string]bool{"done": true}); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `event: delta`) {
		t.Errorf("missing delta event: %s", body)
	}
	if !strings.Contains(body, `"text":"hi"`) {
		t.Errorf("missing data: %s", body)
	}
}

func TestSSEWriter_NotSupported(t *testing.T) {
	w := httptest.NewRecorder() // httptest.ResponseRecorder 不实现 Flusher
	_, err := NewSSEWriter(w)
	if err == nil {
		t.Fatal("expected error for non-flusher")
	}
}
```

- [ ] **Step 3: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: PASS

- [ ] **Step 4: 写 draft stream handler**

Create file `internal/handler/draft.go`:
```go
package handler

import (
	"net/http"

	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type DraftStreamHandler struct {
	Svc *synthesis.Service
}

func (h *DraftStreamHandler) Stream(c *gin.Context) {
	runID := c.Param("id")
	sse, err := synthesis.NewSSEWriter(c.Writer)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "sse unsupported")
		return
	}
	if err := h.Svc.StreamDraft(c.Request.Context(), runID, sse); err != nil {
		_ = sse.Write("error", gin.H{"message": err.Error()})
	}
}
```

- [ ] **Step 5: 写 draft service 骨架**

Create file `internal/synthesis/service.go`:
```go
package synthesis

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	llm  LLMClient
	rag  *RAG
}

func NewService(pool *pgxpool.Pool, llm LLMClient, rag *RAG) *Service {
	return &Service{pool: pool, llm: llm, rag: rag}
}

// StreamDraft 同步版本：拉取已生成的 draft 并流式推送（Phase 3 简化版）
// 实际 LLM 增量生成在 T026 集成
func (s *Service) StreamDraft(ctx context.Context, runID string, w *SSEWriter) error {
	// 查询 draft
	row := s.pool.QueryRow(ctx, `SELECT id, markdown_content FROM drafts WHERE agent_run_id = $1 LIMIT 1`, runID)
	var id, md string
	if err := row.Scan(&id, &md); err != nil {
		return err
	}
	// 流式分块推送
	chunkSize := 30
	for i := 0; i < len(md); i += chunkSize {
		end := i + chunkSize
		if end > len(md) {
			end = len(md)
		}
		if err := w.Write("delta", gin.H{"text": md[i:end]}); err != nil {
			return err
		}
	}
	// 推送 mark
	marks := ExtractMarks(md)
	return w.Write("complete", gin.H{"marks": marks, "completeness": Completeness(md)})
}
```

> **修复 import**: service.go 用了 `gin.H` 需在 import 加 `"github.com/gin-gonic/gin"`。

- [ ] **Step 6: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 7: 接入路由**

Modify `internal/server/server.go`:
```go
// 在已有 trigger 路由后增加：
r.GET("/api/v1/drafts/:id/stream", draftStreamHandler.Stream)
```

> **注意**: `draftStreamHandler` 需要 `*synthesis.Service` 注入。在 T026 集成时统一调整 server.New 签名（按 boundary §7.1 是允许的内部重构）。

- [ ] **Step 8: 端到端验证（SSE）**

> 边界说明：T026 完成后才能跑真实端到端。本任务仅确保编译通过。

- [ ] **Step 9: 展示 diff 等用户决定**

---

## Task T026: Eino Workflow 编排集成（W9-W10）

**Files:**
- Create: `internal/synthesis/agent/workflow.go`
- Create: `internal/synthesis/agent/workflow_test.go`
- Modify: `cmd/api/wire.go`
- Modify: `internal/server/server.go`
- Modify: `internal/synthesis/service.go`

**关联**: 整合 T015-T020 + LLM streaming

- [ ] **Step 1: 写 Workflow 编排**

Create file `internal/synthesis/agent/workflow.go`:
```go
package agent

import (
	"context"
	"log"

	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/cloudwego/eino/compose"
)

// Workflow 5 阶段 DAG：ingestion → retrieve → template → llm → mark
type Workflow struct {
	comp *compose.Workflow
}

func NewWorkflow(rag *synthesis.RAG, llm synthesis.LLMClient) (*Workflow, error) {
	// 简化：直接使用 Task DAG；真实 Eino workflow 在实际使用 cloudwego/eino 后替换
	wf := compose.NewWorkflow[State, State]()
	wf.AddLambdaNode("retrieve", compose.Lambda(func(ctx context.Context, s State) (State, error) {
		items, err := rag.Retrieve(ctx, s.TaskType, 20)
		if err != nil {
			return s, err
		}
		for _, it := range items {
			s.Retrieved = append(s.Retrieved, harvestingItem(it))
		}
		s.Completeness = 0.3
		return s, nil
	}))
	wf.AddLambdaNode("template", compose.Lambda(func(ctx context.Context, s State) (State, error) {
		tplPath := synthesis.SelectByTaskType(s.TaskType)
		tpl, err := synthesis.LoadTemplate(s.TaskType, tplPath)
		if err != nil {
			return s, err
		}
		data := synthesis.TemplateData{
			TaskType: s.TaskType,
			Title:    s.TaskType,
			Items:    s.Retrieved,
		}
		s.Template, err = tpl.Render(data)
		if err != nil {
			return s, err
		}
		s.Completeness = 0.5
		return s, nil
	}))
	wf.AddLambdaNode("llm", compose.Lambda(func(ctx context.Context, s State) (State, error) {
		pb := synthesis.PromptBuilder{}
		msgs := pb.Build(s.TaskType, s.Template, s.Retrieved)
		ch, err := llm.Chat(ctx, synthesis.ChatRequest{Messages: msgs, Stream: true, Temperature: 0.3})
		if err != nil {
			return s, err
		}
		for c := range ch {
			if c.Err != nil {
				return s, c.Err
			}
			s.Draft += c.Content
			if c.Done {
				break
			}
		}
		s.Completeness = synthesis.Completeness(s.Draft)
		return s, nil
	}))
	wf.AddLambdaNode("mark", compose.Lambda(func(ctx context.Context, s State) (State, error) {
		s.Marks = synthesis.ExtractMarks(s.Draft)
		return s, nil
	}))
	if err := wf.End().Connect("retrieve", "template", "llm", "mark"); err != nil {
		return nil, err
	}
	comp, err := wf.Compile(context.Background())
	if err != nil {
		return nil, err
	}
	return &Workflow{comp: comp}, nil
}

func (w *Workflow) Run(ctx context.Context, s State) (State, error) {
	log.Printf("[workflow] running for task %s", s.TaskType)
	result, err := w.comp.Run(ctx, s)
	if err != nil {
		return s, err
	}
	return result, nil
}

func harvestingItem(s synthesis.ScoredItem) (out struct {
	ID, Source, Content string
}) {
	return struct{ ID, Source, Content string }{
		ID:      s.ID,
		Source:  s.Source,
		Content: s.Content,
	}
}

// 编译期断言：workflow.State 兼容 agent.State
var _ = func() State { return State{} }
```

- [ ] **Step 2: 写 Workflow 单元测试（mock LLM）**

Create file `internal/synthesis/agent/workflow_test.go`:
```go
package agent

import (
	"context"
	"testing"

	"github.com/asyncstarter/agent/internal/synthesis"
)

func TestWorkflow_Compilation(t *testing.T) {
	mockLLM := &mockLLMClient{chunks: []string{"Polished ", "draft"}}
	wf, err := NewWorkflow(&synthesis.RAG{}, mockLLM)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if wf == nil {
		t.Fatal("nil workflow")
	}
}

type mockLLMClient struct{ chunks []string }

func (m *mockLLMClient) Chat(_ context.Context, _ synthesis.ChatRequest) (<-chan synthesis.ChatChunk, error) {
	out := make(chan synthesis.ChatChunk, len(m.chunks)+1)
	for _, c := range m.chunks {
		out <- synthesis.ChatChunk{Content: c}
	}
	out <- synthesis.ChatChunk{Done: true}
	close(out)
	return out, nil
}
```

- [ ] **Step 3: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/synthesis/...`
Expected: PASS

- [ ] **Step 4: 调整 server.New 接受 Service**

Modify `internal/server/server.go`:
```go
package server

import (
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/middleware"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/gin-gonic/gin"
)

func New(cfg *config.Config, trigSvc *trigger.Service, synSvc *synthesis.Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(func(c *gin.Context) { c.Set("env", cfg.Env); c.Next() })

	r.GET("/health", handler.Health)

	wh := &handler.WebhookHandler{Secret: cfg.TodoistWebhookSecret}
	r.POST("/api/v1/webhook/todoist", wh.Todoist)

	th := &handler.TriggerHandler{Svc: trigSvc}
	r.POST("/api/v1/trigger", th.ManualTrigger)

	dh := &handler.DraftStreamHandler{Svc: synSvc}
	r.GET("/api/v1/drafts/:id/stream", dh.Stream)

	return r
}
```

- [ ] **Step 5: 修改 wire.go 注入 synthesis service**

Modify `cmd/api/wire.go`:
```go
import (
	"context"
	"log"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/queue"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/server"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/internal/synthesis/agent"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/google/uuid"
)

type Deps struct {
	Cfg     *config.Config
	Trigger *trigger.Service
	Queue   *queue.Client
	Syn     *synthesis.Service
}

func Build(ctx context.Context, cfg *config.Config) (*Deps, error) {
	pool, err := repository.Open(ctx, cfg.DSN)
	if err != nil { return nil, err }
	q, err := queue.NewClient(cfg.RedisURL)
	if err != nil { return nil, err }
	defer q.Close()

	matcher := trigger.NewMatcher(trigger.DefaultMatcherRules())
	trigSvc := trigger.NewService(pool, matcher)

	ddlH := func(ctx context.Context, taskID, userID, title string) error {
		uid, err := uuid.Parse(userID)
		if err != nil { return err }
		_, err = trigSvc.ProcessKeyword(ctx, uid, title)
		return err
	}
	go trigger.RunDDLScheduler(ctx, pool, trigger.NewDDLDetector(), ddlH)

	rag := synthesis.NewRAG(&noopEmbed{}, &noopStore{})
	llm := synthesis.NewOpenAIClient(cfg.OpenAIKey, cfg.OpenAIModel)
	wf, err := agent.NewWorkflow(rag, llm)
	if err != nil { return nil, err }
	synSvc := synthesis.NewService(pool, llm, rag, wf)

	log.Println("[ddl] scheduler started; [workflow] compiled")
	return &Deps{Cfg: cfg, Trigger: trigSvc, Queue: q, Syn: synSvc}, nil
}

func (d *Deps) Server() *gin.Engine { return server.New(d.Cfg, d.Trigger, d.Syn) }

type noopEmbed struct{}
func (noopEmbed) Embed(_ context.Context, _ string) ([]float32, error) { return nil, nil }
func (noopEmbed) Dimension() int { return 1536 }
type noopStore struct{}
func (noopStore) Search(_ context.Context, _ []float32, _ int) ([]synthesis.ScoredItem, error) {
	return nil, nil
}
func (noopStore) Upsert(_ context.Context, _ string, _ []float32, _ synthesis.ScoredItem) error {
	return nil
}

// 引用 handler 防未使用
var _ = handler.Health
```

> **修复 import**: 上面的 `gin.Engine` 引用需在 wire.go 顶部 import 添加 `"github.com/gin-gonic/gin"`。

- [ ] **Step 6: 扩展 config 加 OpenAI 配置**

Modify `internal/config/config.go`:
```go
type Config struct {
	// ... 已有字段 ...
	OpenAIKey  string
	OpenAIModel string
}

func Load() (*Config, error) {
	// ... 既有代码 ...
	return &Config{
		// ... 既有字段 ...
		OpenAIKey:  getEnv("OPENAI_API_KEY", ""),
		OpenAIModel: getEnv("OPENAI_MODEL", "gpt-4o-mini"),
	}, nil
}
```

- [ ] **Step 7: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 8: 编译验证**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build -o bin/api ./cmd/api`
Expected: 0 错误

- [ ] **Step 9: 端到端：手动触发 → SSE 拉取草稿**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
DATABASE_URL=postgres://starter:starter@localhost:5432/starter?sslmode=disable \
REDIS_URL=redis://localhost:6379/0 \
OPENAI_API_KEY=<your-key> \
TODOIST_WEBHOOK_SECRET=test-secret \
./bin/api &
sleep 3
UID=$(uuidgen)
RUN=$(curl -s -X POST http://localhost:8080/api/v1/trigger \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$UID\",\"text\":\"写本周周报\"}" | jq -r .data.run_id)
echo "Run ID: $RUN"
# 等待 LLM 生成
sleep 5
curl -N http://localhost:8080/api/v1/drafts/$RUN/stream
```
Expected: 看到 `event: delta` + `event: complete` 事件流

- [ ] **Step 10: 展示 diff 等用户决定**

---

## Phase 3 退出标准验证

完成 T015-T020 + T026 后，逐项验证 M3 退出标准：

- [ ] RAG 检索单元测试通过
- [ ] Eino DAG 编译通过 + 单元测试通过
- [ ] 4 个模板加载 + 渲染测试通过
- [ ] LLM streaming 接口 mock 测试通过
- [ ] [待补充] 标记提取/替换/完成度计算测试通过
- [ ] SSE handler 路由注册 + 集成测试通过
- [ ] Eino Workflow 5 节点串联通过
- [ ] 端到端：触发 → workflow run → SSE 流式输出草稿
- [ ] 更新 [task-tracker.html](../../task-tracker.html) 中 T015-T020, T026 状态

---

**下一步**: 进入 [05-phase4-delivery.md](05-phase4-delivery.md) 执行前端与交付（T021-T025）。
