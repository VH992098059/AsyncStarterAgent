# A2UI Protocol 替换 SSE 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有的 SSE 伪流改为 A2UI 真实流——LLM token 逐个穿透到前端，同时每个 workflow 阶段发送进度事件；两条路径（新生成 + 回放）都改为 A2UI 格式。

**Architecture:** 新增 `A2UIWriter` 替换 `SSEWriter`，定义统一事件类型（`progress / delta / complete / error`）；`workflow.go` 的 LLM 节点通过 context 取出 writer 实时写 delta；`Service.GenerateDraftStream` 负责串联 writer + workflow + 存库；`StreamDraft` 回放路径也改为写 A2UI delta。handler 层只需替换 writer 类型。

**Tech Stack:** Go 1.22+, Eino compose.Workflow, gin, pgx/v5, net/http SSE

---

## 文件结构

| 操作 | 路径 | 职责 |
|------|------|------|
| 新建 | `internal/synthesis/a2ui.go` | `A2UIWriter` + 所有事件类型常量 |
| 新建 | `internal/synthesis/a2ui_test.go` | A2UIWriter 单元测试 |
| 修改 | `internal/synthesis/workflow.go` | LLM 节点从 ctx 取 writer，阶段开始/结束写 progress |
| 修改 | `internal/synthesis/workflow_test.go` | 补充 writer 注入测试 |
| 修改 | `internal/synthesis/service.go` | 新增 `GenerateDraftStream`；`StreamDraft` 改为写 A2UI |
| 修改 | `internal/synthesis/service_test.go` | 测试两条路径 |
| 修改 | `internal/handler/draft.go` | 使用 `A2UIWriter`；分支调用 `GenerateDraftStream` |
| 修改 | `internal/handler/draft_test.go` | 测试 A2UI handler |

---

### Task 1: 定义 A2UIWriter

**Files:**
- Create: `internal/synthesis/a2ui.go`
- Create: `internal/synthesis/a2ui_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/synthesis/a2ui_test.go
package synthesis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestA2UIWriter_Headers(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	_, err := NewA2UIWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	h := w.Header()
	if h.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", h.Get("Content-Type"))
	}
}

func TestA2UIWriter_WriteProgress(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, _ := NewA2UIWriter(w)
	if err := aw.WriteProgress("retrieve", "running"); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"progress"`) {
		t.Errorf("missing type:progress in: %s", body)
	}
	if !strings.Contains(body, `"phase":"retrieve"`) {
		t.Errorf("missing phase:retrieve in: %s", body)
	}
	if !strings.Contains(body, `"status":"running"`) {
		t.Errorf("missing status:running in: %s", body)
	}
}

func TestA2UIWriter_WriteDelta(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, _ := NewA2UIWriter(w)
	if err := aw.WriteDelta("hello"); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"delta"`) {
		t.Errorf("missing type:delta in: %s", body)
	}
	if !strings.Contains(body, `"text":"hello"`) {
		t.Errorf("missing text:hello in: %s", body)
	}
}

func TestA2UIWriter_WriteComplete(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, _ := NewA2UIWriter(w)
	marks := []Mark{{ID: "m1", Hint: "check", Position: 5, Resolved: false}}
	if err := aw.WriteComplete(marks, 0.9); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"complete"`) {
		t.Errorf("missing type:complete in: %s", body)
	}
	if !strings.Contains(body, `"completeness":0.9`) {
		t.Errorf("missing completeness in: %s", body)
	}
}

func TestA2UIWriter_WriteError(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, _ := NewA2UIWriter(w)
	if err := aw.WriteError("something failed"); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"error"`) {
		t.Errorf("missing type:error in: %s", body)
	}
}

func TestA2UIWriter_NotSupported(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &nonFlushWriter{ResponseWriter: rec}
	_, err := NewA2UIWriter(w)
	if err == nil {
		t.Fatal("expected error for non-flusher")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd k:/go_projects/AsyncStarterAgent
go test ./internal/synthesis/... -run TestA2UI -v 2>&1 | head -30
```

预期：`FAIL` — `NewA2UIWriter undefined`

- [ ] **Step 3: 实现 A2UIWriter**

```go
// internal/synthesis/a2ui.go
package synthesis

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type A2UIWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewA2UIWriter(w http.ResponseWriter) (*A2UIWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	return &A2UIWriter{w: w, flusher: f}, nil
}

func (a *A2UIWriter) write(payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.w, "data: %s\n\n", body); err != nil {
		return err
	}
	a.flusher.Flush()
	return nil
}

func (a *A2UIWriter) WriteProgress(phase, status string) error {
	return a.write(map[string]string{
		"type":   "progress",
		"phase":  phase,
		"status": status,
	})
}

func (a *A2UIWriter) WriteDelta(text string) error {
	return a.write(map[string]string{
		"type": "delta",
		"text": text,
	})
}

func (a *A2UIWriter) WriteComplete(marks []Mark, completeness float32) error {
	return a.write(map[string]interface{}{
		"type":         "complete",
		"marks":        marks,
		"completeness": completeness,
	})
}

func (a *A2UIWriter) WriteError(message string) error {
	return a.write(map[string]string{
		"type":    "error",
		"message": message,
	})
}
```

- [ ] **Step 4: 运行测试，确认通过**

```bash
cd k:/go_projects/AsyncStarterAgent
go test ./internal/synthesis/... -run TestA2UI -v
```

预期：所有 `TestA2UI*` 测试 PASS

- [ ] **Step 5: Commit**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/synthesis/a2ui.go internal/synthesis/a2ui_test.go
git commit -m "feat(synthesis): add A2UIWriter with progress/delta/complete/error events"
```

---

### Task 2: workflow 注入 writer——进度事件 + LLM token 穿透

**Files:**
- Modify: `internal/synthesis/workflow.go`

**背景：** `wfLLMNode` 里已有 `for c := range ch { s.Draft += c.Content }` 循环。改造为：从 ctx 取 `*A2UIWriter`（可为 nil，nil 时静默），每收到 token 调用 `writer.WriteDelta(c.Content)`；每个节点进入时写 `progress running`，退出时写 `progress done`。

- [ ] **Step 1: 定义 context key**

在 `internal/synthesis/a2ui.go` 末尾追加（不新建文件）：

```go
type a2uiCtxKey struct{}

// WithA2UIWriter 把 writer 注入 context，供 workflow 节点使用。
func WithA2UIWriter(ctx context.Context, w *A2UIWriter) context.Context {
	return context.WithValue(ctx, a2uiCtxKey{}, w)
}

// writerFromCtx 从 context 取 writer；无则返回 nil（静默模式）。
func writerFromCtx(ctx context.Context) *A2UIWriter {
	v, _ := ctx.Value(a2uiCtxKey{}).(*A2UIWriter)
	return v
}
```

记得在 `a2ui.go` 的 import 里加 `"context"`。

- [ ] **Step 2: 修改 wfRetrieveNode 写 progress**

在 `internal/synthesis/workflow.go` 的 `wfRetrieveNode` 函数里，在 `Retrieve` 调用前后写 progress：

```go
func wfRetrieveNode(rag *RAG) func(ctx context.Context, s workflowState) (workflowState, error) {
	return func(ctx context.Context, s workflowState) (workflowState, error) {
		if w := writerFromCtx(ctx); w != nil {
			_ = w.WriteProgress("retrieve", "running")
		}
		items, err := rag.Retrieve(ctx, s.TaskType, 20)
		if err != nil {
			return s, fmt.Errorf("retrieve: %w", err)
		}
		for _, it := range items {
			s.Retrieved = append(s.Retrieved, scoredItemToContextItem(it))
		}
		s.Completeness = 0.3
		if w := writerFromCtx(ctx); w != nil {
			_ = w.WriteProgress("retrieve", "done")
		}
		return s, nil
	}
}
```

- [ ] **Step 3: 修改 wfTemplateNode 写 progress**

```go
func wfTemplateNode(templateDir string) func(ctx context.Context, s workflowState) (workflowState, error) {
	return func(ctx context.Context, s workflowState) (workflowState, error) {
		if w := writerFromCtx(ctx); w != nil {
			_ = w.WriteProgress("template", "running")
		}
		tplRelPath := SelectByTaskType(s.TaskType)
		tplPath := tplRelPath
		if templateDir != "" {
			tplPath = templateDir + "/" + tplRelPath
		}
		tpl, err := LoadTemplate(s.TaskType, tplPath)
		if err != nil {
			return s, fmt.Errorf("template load: %w", err)
		}
		data := TemplateData{
			TaskType: s.TaskType,
			Title:    s.TaskType,
			Items:    s.Retrieved,
		}
		s.Template, err = tpl.Render(data)
		if err != nil {
			return s, fmt.Errorf("template render: %w", err)
		}
		s.Completeness = 0.5
		if w := writerFromCtx(ctx); w != nil {
			_ = w.WriteProgress("template", "done")
		}
		return s, nil
	}
}
```

- [ ] **Step 4: 修改 wfLLMNode——token 实时穿透**

```go
func wfLLMNode(llm LLMClient, temperature float32, maxTokens int) func(ctx context.Context, s workflowState) (workflowState, error) {
	return func(ctx context.Context, s workflowState) (workflowState, error) {
		writer := writerFromCtx(ctx)
		if writer != nil {
			_ = writer.WriteProgress("llm", "running")
		}
		pb := PromptBuilder{}
		msgs := pb.Build(s.TaskType, s.Template, s.Retrieved)
		req := ChatRequest{
			Messages:    msgs,
			Temperature: temperature,
		}
		if maxTokens > 0 {
			req.MaxTokens = maxTokens
		}
		ch, err := llm.Chat(ctx, req)
		if err != nil {
			return s, fmt.Errorf("llm chat: %w", err)
		}
		for c := range ch {
			if c.Err != nil {
				return s, fmt.Errorf("llm stream: %w", c.Err)
			}
			s.Draft += c.Content
			if writer != nil && c.Content != "" {
				_ = writer.WriteDelta(c.Content)
			}
			if c.Done {
				break
			}
		}
		s.Completeness = Completeness(s.Draft)
		if writer != nil {
			_ = writer.WriteProgress("llm", "done")
		}
		return s, nil
	}
}
```

- [ ] **Step 5: 编译检查**

```bash
cd k:/go_projects/AsyncStarterAgent
go build ./internal/synthesis/...
```

预期：无编译错误

- [ ] **Step 6: Commit**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/synthesis/a2ui.go internal/synthesis/workflow.go
git commit -m "feat(synthesis): inject A2UIWriter into workflow via context, emit progress + delta tokens"
```

---

### Task 3: Service 层——GenerateDraftStream + StreamDraft 改为 A2UI

**Files:**
- Modify: `internal/synthesis/service.go`

**说明：**
- 新增 `GenerateDraftStream(ctx, runID, userID, taskType, writer)` —— 把 writer 注入 ctx，调用 workflow，存库，最后写 `complete`
- `StreamDraft` 改为接收 `*A2UIWriter`，写 `delta` + `complete`（格式不变，类型换掉）

- [ ] **Step 1: 写失败测试**

新建 `internal/synthesis/service_a2ui_test.go`：

```go
package synthesis

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestService_StreamDraft_A2UI(t *testing.T) {
	// StreamDraft 现在接收 *A2UIWriter，验证它能写出 delta + complete
	rec := &flushRecorder{httptest.NewRecorder()}
	aw, err := NewA2UIWriter(rec)
	if err != nil {
		t.Fatal(err)
	}

	svc := &Service{pool: nil}
	// 直接测试 streamMarkdown helper（从 markdown 模拟回放）
	md := "这是测试内容"
	if err := svc.streamMarkdown(context.Background(), md, aw); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"delta"`) {
		t.Errorf("missing delta events in: %s", body)
	}
	if !strings.Contains(body, `"type":"complete"`) {
		t.Errorf("missing complete event in: %s", body)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd k:/go_projects/AsyncStarterAgent
go test ./internal/synthesis/... -run TestService_StreamDraft_A2UI -v 2>&1 | head -20
```

预期：`FAIL` — `streamMarkdown undefined`

- [ ] **Step 3: 重构 service.go**

将 `service.go` 中以下内容修改：

**3a. 把 `StreamDraft` 里的逻辑提成私有 helper `streamMarkdown`：**

```go
// streamMarkdown 将已有 markdown 分块写为 A2UI delta + complete。
func (s *Service) streamMarkdown(ctx context.Context, md string, w *A2UIWriter) error {
	const chunkSize = 30
	for i := 0; i < len(md); i += chunkSize {
		end := i + chunkSize
		if end > len(md) {
			end = len(md)
		}
		if err := w.WriteDelta(md[i:end]); err != nil {
			return err
		}
	}
	marks := ExtractMarks(md)
	return w.WriteComplete(marks, Completeness(md))
}
```

**3b. 修改 `StreamDraft` 签名，改为接收 `*A2UIWriter`：**

```go
func (s *Service) StreamDraft(ctx context.Context, runID string, w *A2UIWriter) error {
	row := s.pool.QueryRow(ctx,
		"SELECT markdown_content FROM drafts WHERE agent_run_id = $1 LIMIT 1",
		runID,
	)
	var md string
	if err := row.Scan(&md); err != nil {
		return fmt.Errorf("draft not found: %w", err)
	}
	return s.streamMarkdown(ctx, md, w)
}
```

**3c. 新增 `GenerateDraftStream`：**

```go
// GenerateDraftStream 执行完整 workflow 并将 LLM token 实时写入 w，
// workflow 结束后将结果存库并写 complete 事件。
func (s *Service) GenerateDraftStream(ctx context.Context, runID, userID, taskType string, w *A2UIWriter) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	llm, err := s.factory.GetLLM(ctx, uid)
	if err != nil {
		return err
	}
	embedder, err := s.factory.GetEmbedder(ctx, uid)
	if err != nil {
		return err
	}

	rag := NewRAG(*embedder, s.vecStore)
	temp := s.factory.GetLLMTemperature(ctx, uid)
	maxTokens := s.factory.GetLLMMaxTokens(ctx, uid)

	wfCfg := workflowConfig{
		TemplateDir: s.templateDir,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}
	wf, err := newDraftWorkflow(ctx, rag, llm, wfCfg)
	if err != nil {
		return fmt.Errorf("build workflow: %w", err)
	}

	// 把 writer 注入 context，workflow 节点会从中取出进行实时写入
	streamCtx := WithA2UIWriter(ctx, w)
	result, err := wf.generate(streamCtx, runID, userID, taskType)
	if err != nil {
		return fmt.Errorf("generate draft: %w", err)
	}

	if s.pool != nil {
		marksJSON, _ := marshalMarks(result.Marks)
		_, err = s.pool.Exec(ctx,
			`INSERT INTO drafts (agent_run_id, title, markdown_content, completeness, marks, status)
			 VALUES ($1, $2, $3, $4, $5, 'draft')
			 ON CONFLICT (agent_run_id) DO UPDATE SET markdown_content = $3, completeness = $4, marks = $5, updated_at = NOW()`,
			runID, taskType, result.Draft, result.Completeness, marksJSON,
		)
		if err != nil {
			return fmt.Errorf("save draft: %w", err)
		}
	}

	return w.WriteComplete(result.Marks, result.Completeness)
}
```

- [ ] **Step 4: 运行测试**

```bash
cd k:/go_projects/AsyncStarterAgent
go test ./internal/synthesis/... -v 2>&1 | tail -20
```

预期：所有 synthesis 测试 PASS（包括新的 A2UI 测试）

- [ ] **Step 5: Commit**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/synthesis/service.go internal/synthesis/service_a2ui_test.go
git commit -m "feat(synthesis): add GenerateDraftStream + migrate StreamDraft to A2UIWriter"
```

---

### Task 4: Handler 层替换为 A2UIWriter

**Files:**
- Modify: `internal/handler/draft.go`
- Modify: `internal/handler/draft_test.go`

- [ ] **Step 1: 修改 draft.go**

将 `handler/draft.go` 改为：

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
	if h.Svc == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5002, "draft service not configured")
		return
	}
	runID := c.Param("id")
	ctx := c.Request.Context()

	a2ui, err := synthesis.NewA2UIWriter(c.Writer)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "streaming not supported")
		return
	}

	exists, err := h.Svc.DraftExists(ctx, runID)
	if err != nil {
		_ = a2ui.WriteError("check draft: " + err.Error())
		return
	}

	if !exists {
		var userID, taskType string
		if err = h.Svc.QueryRunInfo(ctx, runID, &userID, &taskType); err != nil {
			_ = a2ui.WriteError("run not found: " + err.Error())
			return
		}
		if err = h.Svc.GenerateDraftStream(ctx, runID, userID, taskType, a2ui); err != nil {
			_ = a2ui.WriteError(err.Error())
		}
		return
	}

	if err := h.Svc.StreamDraft(ctx, runID, a2ui); err != nil {
		_ = a2ui.WriteError(err.Error())
	}
}
```

- [ ] **Step 2: 更新 draft_test.go**

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/synthesis"
)

func TestDraftStreamHandler_A2UIUnsupported(t *testing.T) {
	// 当底层 writer 不支持 Flusher 时，NewA2UIWriter 应返回错误。
	w := httptest.NewRecorder()
	type noFlush struct{ http.ResponseWriter }
	_, err := synthesis.NewA2UIWriter(noFlush{ResponseWriter: w})
	if err == nil {
		t.Fatal("expected error for non-flusher writer")
	}
}
```

- [ ] **Step 3: 编译 + 运行全量测试**

```bash
cd k:/go_projects/AsyncStarterAgent
go build ./...
go test ./internal/handler/... -v
```

预期：全部 PASS，无编译错误

- [ ] **Step 4: Commit**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/handler/draft.go internal/handler/draft_test.go
git commit -m "feat(handler): replace SSEWriter with A2UIWriter, wire GenerateDraftStream for live path"
```

---

### Task 5: 全量验证

- [ ] **Step 1: 运行全量测试**

```bash
cd k:/go_projects/AsyncStarterAgent
go test ./... 2>&1
```

预期：所有测试 PASS，零编译错误

- [ ] **Step 2: 确认旧 SSEWriter 仍编译（未删除）**

`SSEWriter` 暂时保留，以免有其他调用方依赖。检查是否还有引用：

```bash
cd k:/go_projects/AsyncStarterAgent
grep -r "SSEWriter\|NewSSEWriter" --include="*.go" .
```

若 `handler/draft.go` 已不再引用，说明替换干净；`sse.go` 本身还在但无害。

- [ ] **Step 3: 验证 A2UI 事件格式**

```bash
cd k:/go_projects/AsyncStarterAgent
go test ./internal/synthesis/... -run TestA2UI -v
```

预期：所有 `TestA2UI*` PASS

- [ ] **Step 4: 最终 Commit（若有遗漏文件）**

```bash
cd k:/go_projects/AsyncStarterAgent
git status
# 若有未提交文件
git add .
git commit -m "chore: finalize A2UI protocol rollout"
```

---

## A2UI 事件格式参考

```
// 进度事件
data: {"type":"progress","phase":"retrieve","status":"running"}
data: {"type":"progress","phase":"retrieve","status":"done"}
data: {"type":"progress","phase":"template","status":"running"}
data: {"type":"progress","phase":"template","status":"done"}
data: {"type":"progress","phase":"llm","status":"running"}

// LLM token 流（每个 token 一条）
data: {"type":"delta","text":"今"}
data: {"type":"delta","text":"天"}

// 完成
data: {"type":"complete","marks":[...],"completeness":0.92}

// 错误
data: {"type":"error","message":"..."}
```

前端监听 `onmessage`，按 `data.type` 分发处理，无需 `event:` 字段解析。
