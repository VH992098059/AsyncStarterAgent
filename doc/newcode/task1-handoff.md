# Task 1 交接文档 — A2UIWriter 定义完成

**完成时间：** 2026-06-29  
**Commit：** `c938d5e`  
**状态：** DONE ✅

---

## 已完成内容

### 新增文件

**`internal/synthesis/a2ui.go`**

定义了 `A2UIWriter` 类型及所有写方法，以及 context 注入函数：

| 函数 / 类型 | 说明 |
|---|---|
| `A2UIWriter` struct | 包含 `w http.ResponseWriter` + `flusher http.Flusher`；not safe for concurrent use |
| `NewA2UIWriter(w) (*A2UIWriter, error)` | 非 Flusher 返回 error；设置 SSE headers（Content-Type/Cache-Control/Connection）|
| `WriteProgress(phase, status string) error` | 发送 `{"type":"progress","phase":...,"status":...}` |
| `WriteDelta(text string) error` | 发送 `{"type":"delta","text":...}` |
| `WriteComplete(marks []Mark, completeness float32) error` | 发送 `{"type":"complete","marks":...,"completeness":...}` |
| `WriteError(message string) error` | 发送 `{"type":"error","message":...}` |
| `WithA2UIWriter(ctx, w) context.Context` | 将 writer 注入 context |
| `writerFromCtx(ctx) *A2UIWriter` | 从 context 取 writer；无则返回 nil（静默模式）|

**`internal/synthesis/a2ui_test.go`**

6 个测试全部 PASS：
- `TestA2UIWriter_Headers`
- `TestA2UIWriter_WriteProgress`
- `TestA2UIWriter_WriteDelta`
- `TestA2UIWriter_WriteComplete`
- `TestA2UIWriter_WriteError`
- `TestA2UIWriter_NotSupported`

---

## 下一步：Task 2 — workflow 注入 writer

**目标文件：** `internal/synthesis/workflow.go`（修改，不新建）

**需要做的事：**

1. `wfRetrieveNode`：在 `rag.Retrieve` 调用前写 `WriteProgress("retrieve","running")`，调用后写 `WriteProgress("retrieve","done")`
2. `wfTemplateNode`：在模板渲染前写 `WriteProgress("template","running")`，渲染后写 `WriteProgress("template","done")`
3. `wfLLMNode`：
   - 进入时写 `WriteProgress("llm","running")`
   - 在 `for c := range ch` 循环里，每个 token 除了 `s.Draft += c.Content` 之外，也调用 `writer.WriteDelta(c.Content)`（`c.Content != ""` 时）
   - 退出时写 `WriteProgress("llm","done")`
4. 所有节点都通过 `writerFromCtx(ctx)` 取 writer，nil 时静默跳过（不报错）

**关键约束：**
- `WithA2UIWriter` 和 `writerFromCtx` 已在 `a2ui.go` 里定义，直接调用即可
- 不要修改 `workflow.go` 以外的任何文件
- 不要新建文件
- 编译通过后运行 `go build ./internal/synthesis/...` 验证

**参考代码（完整修改后的三个函数）：**

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

**验证命令：**
```bash
cd k:/go_projects/AsyncStarterAgent
go build ./internal/synthesis/...
go test ./internal/synthesis/... -v 2>&1 | tail -20
```

预期：编译无错，所有 synthesis 测试 PASS（包括已有的 TestSSEWriter*、TestA2UI* 等）。

**完成后 commit：**
```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/synthesis/workflow.go
git commit -m "feat(synthesis): inject A2UIWriter into workflow via context, emit progress + delta tokens"
```
