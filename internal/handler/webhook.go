package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookHandler struct {
	Secret string
	Svc    *trigger.Service
	Pool   *pgxpool.Pool  // 决策 #7: 查 feishu_tokens 表
	Cfg    *config.Config // 决策 #7: 读 FeishuVerificationToken
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
		// event_data.id 是 item id），用 uuid.Nil 占位表示 unowned run。Phase 2+ 引入 user
		// identity provider 后替换为真实 user_id。ProcessKeyword 内部已不再返回
		// "no rule matched"（无匹配时回退为 message 类型），故这里不再需要区分该错误。
		log.Printf("[webhook] creating unowned AgentRun (user_id=uuid.Nil) for Todoist event_id=%s", ev.EventID)
		uid := uuid.Nil
		if _, err := h.Svc.ProcessKeyword(c.Request.Context(), uid, content); err != nil {
			httpx.Fail(c, http.StatusInternalServerError, 5001, "create run")
			return
		}
	}

	httpx.OK(c, gin.H{"received": true, "event_id": ev.EventID, "matched": true})
}

func bindJSON(body []byte, v interface{}) error {
	return jsonUnmarshal(body, v)
}

// HandleFeishuWebhook 处理飞书事件订阅推送
// 路由: POST /api/v1/webhook/feishu
// 决策 #7: 飞书任务事件触发 AgentRun
func (h *WebhookHandler) HandleFeishuWebhook(c *gin.Context) {
	var payload struct {
		Challenge string `json:"challenge"` // URL 校验时飞书下发
		Token     string `json:"token"`     // 事件订阅 Verification Token（顶层兼容旧格式）
		Type      string `json:"type"`      // url_verification / event_callback
		Header    struct {
			EventID    string `json:"event_id"`
			EventType  string `json:"event_type"`
			Token      string `json:"token"`
			CreateTime string `json:"create_time"`
		} `json:"header"`
		Event map[string]interface{} `json:"event"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// 1. URL 校验：返回 challenge
	if payload.Type == "url_verification" {
		c.JSON(http.StatusOK, gin.H{"challenge": payload.Challenge})
		return
	}

	// 2. Token 校验（防伪造）。header.token 优先，回退顶层 token（飞书旧版格式）
	token := payload.Header.Token
	if token == "" {
		token = payload.Token
	}
	if h.Cfg != nil && h.Cfg.FeishuVerificationToken != "" && token != h.Cfg.FeishuVerificationToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid verification token"})
		return
	}

	// 3. 事件分发：只处理任务相关事件
	eventType := payload.Header.EventType
	switch eventType {
	case "task.v2.task.created", "task.v2.task.updated":
		h.handleFeishuTaskEvent(c, payload.Header.EventID, payload.Event)
	default:
		// 非任务事件，确认接收但不处理
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ignored"})
	}
}

// handleFeishuTaskEvent 处理飞书任务事件，创建 AgentRun
// MVP 限制：open_id → user_id 映射依赖 feishu_tokens 表的 open_id 字段。
// 若用户未授权过飞书（表里无此 open_id），事件被忽略（返回 200 + user not mapped），
// 避免飞书端因业务不匹配无限重试。
//
// eventID 幂等去重：基于 webhook_events 表（event_id 唯一约束），飞书重试推送同一
// event_id 时第二次插入会因唯一约束冲突而失败，据此判断为重复事件并跳过创建 AgentRun。
func (h *WebhookHandler) handleFeishuTaskEvent(c *gin.Context, eventID string, event map[string]interface{}) {
	summary, _ := event["summary"].(string)
	if summary == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "no summary"})
		return
	}

	// 提取任务 GUID（飞书 task.v2 事件 payload 顶层字段，用于后续回写评论）。
	// guid 缺失时回退到 summary 作为 trigger_source，主流程不阻断，仅不回写评论。
	guid, _ := event["guid"].(string)

	// 安全导航：operator_id 可能不存在或不是 map
	openID := ""
	if operator, ok := event["operator_id"].(map[string]interface{}); ok {
		openID, _ = operator["open_id"].(string)
	}
	if openID == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "no operator"})
		return
	}

	if h.Pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "db not configured"})
		return
	}

	// eventID 缺失时无法去重，跳过检查直接放行（不阻断主流程，飞书 header.event_id 理论上总是存在）
	if eventID != "" {
		first, err := repository.TryInsertWebhookEvent(c.Request.Context(), h.Pool, "feishu", eventID)
		if err != nil {
			log.Printf("[feishu-webhook] dedup check failed for event_id=%s: %v", eventID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "dedup check failed"})
			return
		}
		if !first {
			// 重复事件（飞书重试），已处理过，直接返回成功避免继续重试
			c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "duplicate event, skipped"})
			return
		}
	}

	// 通过 open_id 查找系统用户（feishu_tokens 表的 open_id 字段，F001 已建）
	userID, err := h.findUserByOpenID(c.Request.Context(), openID)
	if err != nil {
		// 头号"为什么没触发"原因：open_id 未在 feishu_tokens 表中映射到系统用户。
		// 200 + user not mapped 让飞书端停止重试（业务正常，非系统错误）。
		log.Printf("[feishu-webhook] user not mapped for open_id=%s", openID)
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "user not mapped"})
		return
	}

	// 创建 AgentRun。h.Svc 是 *trigger.Service。
	// 用 ProcessKeywordWithSource 指定 SourceFeishu 和精确 trigger_source，
	// 让后续 delivery.Service 能从 trigger_source 解析 guid 回写评论。
	if h.Svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trigger service not configured"})
		return
	}
	// 构造 trigger_source：有 guid 时用 "feishu:task:<guid>"（供 delivery 回写评论），
	// 无 guid 时回退到 summary（不回写评论，但不阻断主流程）
	triggerSource := summary
	if guid != "" {
		triggerSource = "feishu:task:" + guid
	}
	if _, err := h.Svc.ProcessKeywordWithSource(c.Request.Context(), userID, summary, trigger.SourceFeishu, triggerSource); err != nil {
		// 记录真实错误用于诊断，但对外只返回通用消息（不泄露内部错误细节，
		// 与 Todoist handler webhook.go:76 的 "create run" 风格一致）。
		log.Printf("[feishu-webhook] ProcessKeyword failed: open_id=%s guid=%s err=%v", openID, guid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// findUserByOpenID 通过飞书 open_id 查找系统用户
func (h *WebhookHandler) findUserByOpenID(ctx context.Context, openID string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := h.Pool.QueryRow(ctx, `SELECT user_id FROM feishu_tokens WHERE open_id = $1`, openID).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user not found for open_id %s: %w", openID, err)
	}
	return userID, nil
}
