# Task 3 交接文档 — Service 层 GenerateDraftStream 完成

**完成时间：** 2026-06-29  
**Commit：** `8612997`  
**状态：** DONE ✅

---

## 已完成内容

### 修改文件

**`internal/synthesis/service.go`**

| 函数 | 变更 |
|---|---|
| `streamMarkdown(ctx, md string, w *A2UIWriter) error` | 新增私有 helper；按 **rune**（非字节）切割，避免中文乱码；每块写 `WriteDelta`，最后写 `WriteComplete` |
| `StreamDraft(ctx, runID string, w *A2UIWriter) error` | 签名从 `*SSEWriter` 改为 `*A2UIWriter`；内部调用 `streamMarkdown` |
| `GenerateDraftStream(ctx, runID, userID, taskType string, w *A2UIWriter) error` | 新增；注入 writer 到 ctx → 运行 workflow → 存库 → 写 `WriteComplete`；存库失败时先写 complete 再返回错误（降级策略） |

**`internal/synthesis/service_a2ui_test.go`** — 新建

| 测试 | 覆盖点 |
|---|---|
| `TestService_StreamMarkdown` | 基本 delta + complete 事件写入 |
| `TestService_StreamMarkdown_MultiByte` | 50 个汉字（150 字节）触发多块分割，验证 rune 切割正确性 |

### 测试结果
全部 synthesis 测试 PASS，build clean。

---

## 下一步：Task 4 — Handler 层替换

**目标文件：**
- `internal/handler/draft.go`（修改）
- `internal/handler/draft_test.go`（修改）

**当前状态：** `handler/draft.go` 仍引用旧的 `synthesis.NewSSEWriter` 和 `*SSEWriter`，`Service.StreamDraft` 签名已改为 `*A2UIWriter`，所以现在 handler 编译会报错——这是预期的，Task 4 解决。

### 需要做的修改

**`internal/handler/draft.go` 完整替换为：**

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

**`internal/handler/draft_test.go` 完整替换为：**

```go
package handler

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/asyncstarter/agent/internal/synthesis"
)

func TestDraftStreamHandler_A2UIUnsupported(t *testing.T) {
    w := httptest.NewRecorder()
    type noFlush struct{ http.ResponseWriter }
    _, err := synthesis.NewA2UIWriter(noFlush{ResponseWriter: w})
    if err == nil {
        t.Fatal("expected error for non-flusher writer")
    }
}
```

### 验证命令

```bash
cd k:/go_projects/AsyncStarterAgent
go build ./...             # 必须 PASS（全量编译）
go test ./internal/handler/... -v
go test ./internal/synthesis/... -v 2>&1 | tail -20
```

**注意：** 修改 handler 后整个项目应能编译通过（SSEWriter 调用已全部移除）。

### 完成后 commit

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/handler/draft.go internal/handler/draft_test.go
git commit -m "feat(handler): replace SSEWriter with A2UIWriter, wire GenerateDraftStream for live path"
```
