package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/queue"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type TriggerHandler struct {
	Svc *trigger.Service
	// Queue 用于把草稿生成任务异步入队给 worker（问题 #11）。为 nil 时（未配置 Redis
	// 或 worker 未接入）静默跳过入队——DraftStreamHandler.Stream 的同步兜底逻辑
	// 仍会在前端打开 SSE 时生成草稿，保持向后兼容。
	Queue *queue.Client
}

type triggerRequest struct {
	// user_id 不再从 body 读取——由 auth.Middleware 从 JWT 注入 gin.Context。
	// 此处仅保留 text（FR-A04 手动触发）。
	Text string `json:"text" binding:"required"`
}

type triggerResponse struct {
	RunID string `json:"run_id"`
}

func (h *TriggerHandler) ManualTrigger(c *gin.Context) {
	var req triggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	runID, err := h.Svc.ProcessKeyword(c.Request.Context(), uid, req.Text)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, err.Error())
		return
	}
	// 问题 #11: 手动触发的草稿生成入队给 worker 异步处理。入队失败不阻断 trigger 主流程
	// （run 已创建），仅记日志——DraftStreamHandler.Stream 的同步兜底会在前端打开 SSE 时补跑。
	if h.Queue != nil {
		if err := h.Queue.Enqueue(c.Request.Context(), queue.TaskDraftGenerate, runID.String(), 5*time.Minute); err != nil {
			log.Printf("[trigger] enqueue draft:generate failed for run_id=%s: %v", runID, err)
		}
	}
	httpx.OK(c, triggerResponse{RunID: runID.String()})
}
