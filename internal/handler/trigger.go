package handler

import (
	"net/http"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type TriggerHandler struct {
	Svc *trigger.Service
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
	httpx.OK(c, triggerResponse{RunID: runID.String()})
}
