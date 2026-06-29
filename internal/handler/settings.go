package handler

import (
	"net/http"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/settings"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	Repo *settings.Repo
	Fac  *settings.Factory
	Syn  *synthesis.Service
}

func (h *SettingsHandler) Get(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	s, err := h.Repo.Get(c.Request.Context(), uid)
	if err != nil {
		s = settings.DefaultSettings()
	}
	httpx.OK(c, s.Masked())
}

func (h *SettingsHandler) Update(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	var req settings.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid body: "+err.Error())
		return
	}
	s, err := h.Repo.Upsert(c.Request.Context(), uid, &req)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, err.Error())
		return
	}
	h.Fac.Invalidate(uid)
	httpx.OK(c, s.Masked())
}

func (h *SettingsHandler) TestLLM(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	if err := h.Syn.TestLLM(c.Request.Context(), uid.String()); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, err.Error())
		return
	}
	httpx.OK(c, gin.H{"status": "ok"})
}
