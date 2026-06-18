package handler

import (
	"io"
	"net/http"

	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	Secret string
}

func (h *WebhookHandler) Todoist(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "read body")
		return
	}
	sig := c.GetHeader("X-Todoist-HMAC-SHA256")
	if sig == "" || !trigger.VerifyHMAC(h.Secret, body, sig) {
		httpx.Fail(c, http.StatusForbidden, 4003, "invalid signature")
		return
	}
	var payload map[string]interface{}
	if err := bindJSON(body, &payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid json")
		return
	}
	ev := trigger.NormalizeTodoist(getStr(payload, "event_name"), getStr(payload, "event_id"), payload)
	// T009 整合时接入 EventBus / AgentRun 创建；此处先 200 返回
	httpx.OK(c, gin.H{"received": true, "event_id": ev.EventID})
}

func bindJSON(body []byte, v interface{}) error {
	return jsonUnmarshal(body, v)
}
