package handler

import (
	"io"
	"log"
	"net/http"

	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WebhookHandler struct {
	Secret string
	Svc    *trigger.Service
}

// noRuleMatchedErr 是 trigger.Service 返回的"无匹配规则"错误 sentinel。
// 与 internal/trigger/service.go:25 的 fmt.Errorf("no rule matched") 字符串保持一致。
// webhook 场景下视为正常业务流（webhook 来了但不命中任何关键词），返回 200 + matched=false，
// 避免外部 webhook 端因"业务不匹配"无限重试。
const noRuleMatchedErr = "no rule matched"

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

	// T009 整合：webhook → 关键词匹配 → 创建 AgentRun。
	// payload.content 优先（部分 payload 把 content 平铺在顶层），
	// 否则从 event_data.content 取。匹配走 trigger.Service.ProcessKeyword 路径。
	content := getStr(payload, "content")
	if content == "" {
		if data, ok := payload["event_data"].(map[string]interface{}); ok {
			content = getStr(data, "content")
		}
	}
	if content != "" {
		if h.Svc == nil {
			// 理论上不会发生（server.New 总是注入 trigSvc），但 nil 路径返回 200 会让外部
			// webhook 端误以为匹配成功。加 log 警告便于诊断 misuse。
			log.Printf("[webhook] Svc=nil, skipping ProcessKeyword for event_id=%s", ev.EventID)
			httpx.OK(c, gin.H{"received": true, "event_id": ev.EventID, "matched": false})
			return
		}
		// T009 范围内 webhook payload 暂无 user 关联字段（Todoist payload 只有
		// event_data.id 是 item id），用 uuid.Nil 占位。Phase 2+ 引入 user
		// identity provider 后替换为真实 user_id。
		uid := uuid.Nil
		_, err := h.Svc.ProcessKeyword(c.Request.Context(), uid, content)
		if err != nil {
			if err.Error() == noRuleMatchedErr {
				httpx.OK(c, gin.H{"received": true, "event_id": ev.EventID, "matched": false})
				return
			}
			httpx.Fail(c, http.StatusInternalServerError, 5001, "create run")
			return
		}
	}

	httpx.OK(c, gin.H{"received": true, "event_id": ev.EventID, "matched": true})
}

func bindJSON(body []byte, v interface{}) error {
	return jsonUnmarshal(body, v)
}
