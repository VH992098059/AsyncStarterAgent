# Task 2 交接文档 — workflow A2UIWriter 注入完成

**完成时间：** 2026-06-29  
**Commit：** `4384ec7`  
**状态：** DONE ✅

---

## 已完成内容

### 修改文件

**`internal/synthesis/workflow.go`**

三个 workflow 节点均已注入 A2UIWriter 进度/流式事件：

| 节点 | 新增行为 |
|---|---|
| `wfRetrieveNode` | 调用 RAG 前写 `progress{retrieve,running}`，结束后写 `progress{retrieve,done}` |
| `wfTemplateNode` | 模板渲染前写 `progress{template,running}`，渲染后写 `progress{template,done}` |
| `wfLLMNode` | 进入写 `progress{llm,running}`；每个 token（`c.Content != ""`）写 `delta{text}`；完成写 `progress{llm,done}` |

所有节点：
- 通过 `writer := writerFromCtx(ctx)` 在闭包顶部 cache 一次（风格统一）
- writer 为 nil 时静默跳过（不报错）
- 写失败用 `_ = writer.Write...` 忽略（SSE 断开属正常）

### 测试结果
所有 synthesis 测试 14/14 PASS，build clean。

---

## 下一步：Task 3 — Service 层

**目标文件：** `internal/synthesis/service.go`（修改）  
**新增测试文件：** `internal/synthesis/service_a2ui_test.go`（新建）

**需要做的两件事：**

### 3a. 新增 `streamMarkdown` 私有 helper

将已有 `StreamDraft` 里的分块逻辑提取出来：

```go
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

### 3b. 修改 `StreamDraft` 签名，接收 `*A2UIWriter`

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

**注意：** 原来的 `StreamDraft` 接收 `*SSEWriter`，改签名后 handler 层会编译失败，但 Task 4 会修 handler，所以此时编译失败是预期的——Task 3 只需要 synthesis 包自身能编译即可。

### 3c. 新增 `GenerateDraftStream`

```go
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

### 3d. 新建测试文件 `internal/synthesis/service_a2ui_test.go`

```go
package synthesis

import (
    "context"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestService_StreamMarkdown(t *testing.T) {
    rec := &flushRecorder{httptest.NewRecorder()}
    aw, err := NewA2UIWriter(rec)
    if err != nil {
        t.Fatal(err)
    }

    svc := &Service{pool: nil}
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

**验证：**
```bash
cd k:/go_projects/AsyncStarterAgent
go build ./internal/synthesis/...  # 必须 PASS
go test ./internal/synthesis/... -run TestService_StreamMarkdown -v
```

**注意事项：**
- `StreamDraft` 签名从 `*SSEWriter` 改为 `*A2UIWriter` 会导致 `internal/handler/draft.go` 编译报错，这是预期的
- Task 3 只需要 `synthesis` 包自身能编译 + 测试通过
- `factory.GetLLMTemperature` 和 `factory.GetLLMMaxTokens` 是现有接口方法，直接调用即可

**完成后 commit：**
```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/synthesis/service.go internal/synthesis/service_a2ui_test.go
git commit -m "feat(synthesis): add GenerateDraftStream + migrate StreamDraft to A2UIWriter"
```
