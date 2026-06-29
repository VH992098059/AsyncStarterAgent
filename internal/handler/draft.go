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

	exists, err := h.Svc.DraftExists(ctx, runID)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "check draft: "+err.Error())
		return
	}

	if !exists {
		var userID, taskType string
		err = h.Svc.QueryRunInfo(ctx, runID, &userID, &taskType)
		if err != nil {
			httpx.Fail(c, http.StatusNotFound, 4004, "run not found: "+err.Error())
			return
		}
		result, err := h.Svc.GenerateDraft(ctx, runID, userID, taskType)
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, 5001, err.Error())
			return
		}
		_ = result
	}

	sse, err := synthesis.NewSSEWriter(c.Writer)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "sse unsupported")
		return
	}
	if err := h.Svc.StreamDraft(ctx, runID, sse); err != nil {
		_ = sse.Write("error", map[string]interface{}{"message": err.Error()})
	}
}
