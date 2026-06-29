package handler

import (
	"net/http"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/delivery"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type DeliveryHandler struct {
	Svc *delivery.Service
}

func (h *DeliveryHandler) Deliver(c *gin.Context) {
	if h.Svc == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 503, "delivery service not configured")
		return
	}

	runID := c.Param("id")
	if runID == "" {
		httpx.Fail(c, http.StatusBadRequest, 400, "run id required")
		return
	}

	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 401, "no user in context")
		return
	}

	var req struct {
		TargetType string `json:"target_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 400, "target_type required")
		return
	}

	result, err := h.Svc.Deliver(c.Request.Context(), uid.String(), runID, req.TargetType)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 500, err.Error())
		return
	}

	httpx.OK(c, result)
}
