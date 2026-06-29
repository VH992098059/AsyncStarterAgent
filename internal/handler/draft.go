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
