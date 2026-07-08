package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DraftStreamHandler struct {
	Svc  *synthesis.Service
	Pool *pgxpool.Pool
}

// Stream 处理草稿流式响应：
//   - 草稿已存在 → 重放（不改变 run 状态）
//   - 草稿不存在 → 跑完整 GenerateDraftStream（service 层标 running/synthesis）
//     成功 → handler 标 completed/synthesis；失败 → handler 标 failed/synthesis
//
// 状态回写失败不阻断流式响应，仅记日志。
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
		genErr := h.Svc.GenerateDraftStream(ctx, runID, userID, taskType, a2ui)
		h.writeRunStatus(ctx, runID, genErr)
		if genErr != nil {
			_ = a2ui.WriteError(genErr.Error())
		}
		return
	}

	if err := h.Svc.StreamDraft(ctx, runID, a2ui); err != nil {
		_ = a2ui.WriteError(err.Error())
	}
}

// writeRunStatus 在 GenerateDraftStream 结束后回写 run 状态：
//   - genErr == nil → completed/synthesis
//   - genErr != nil → failed/synthesis + errMsg
//
// 仅当 Pool 已注入且 runID 是合法 UUID 时执行；回写失败仅记日志。
func (h *DraftStreamHandler) writeRunStatus(ctx context.Context, runID string, genErr error) {
	if h.Pool == nil {
		return
	}
	runUUID, parseErr := uuid.Parse(runID)
	if parseErr != nil {
		log.Printf("[draft] invalid run id %q: %v", runID, parseErr)
		return
	}
	status := "completed"
	stage := "synthesis"
	errMsg := ""
	if genErr != nil {
		status = "failed"
		errMsg = genErr.Error()
	}
	if err := repository.UpdateAgentRunStatus(ctx, h.Pool, runUUID, status, stage, errMsg); err != nil {
		log.Printf("[draft] update run status to %s/%s: %v", status, stage, err)
	}
}
