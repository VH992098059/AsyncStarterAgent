package handler

import (
	"net/http"

	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TriggerHandler struct {
	Svc *trigger.Service
}

type triggerRequest struct {
	UserID string `json:"user_id"`
	Text   string `json:"text"`
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
	uid, err := uuid.Parse(req.UserID)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid user_id")
		return
	}
	runID, err := h.Svc.ProcessKeyword(c.Request.Context(), uid, req.Text)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, err.Error())
		return
	}
	httpx.OK(c, triggerResponse{RunID: runID.String()})
}
