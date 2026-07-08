package handler

import (
	"log"
	"net/http"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatHandler 暴露 AgentRun 对话端点：历史消息列表 + 流式聊天。
// 所有方法都需要 auth.Middleware。
type ChatHandler struct {
	Pool *pgxpool.Pool
	Syn  *synthesis.Service
}

// ChatMessageItem 消息展示项。role 为 "user" / "assistant"。
type ChatMessageItem struct {
	ID               string `json:"id"`
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"error_message"`
	CreatedAt        string `json:"created_at"`
}

// ListMessages GET /api/v1/agent-runs/:id/messages
// 返回某 run 的历史消息列表（按 created_at ASC，默认最近 50 条）。
func (h *ChatHandler) ListMessages(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	runID, ok := parseRunID(c)
	if !ok {
		return
	}
	if h.Pool == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "pool not configured")
		return
	}
	if _, err := repository.GetAgentRunByID(c.Request.Context(), h.Pool, uid, runID); err != nil {
		respondRunErr(c, err)
		return
	}

	// 清理脏数据：之前 UpdateMessageStatus SQL bug 导致 assistant 消息卡在 streaming + 空内容
	// 每次加载历史前自动清理，修了 SQL 后新消息正常，但旧消息需要清理
	cleaned, err := repository.CleanStaleStreamingMessages(c.Request.Context(), h.Pool, runID)
	if err != nil {
		log.Printf("[chat] clean stale messages run=%s: %v", runID, err)
	} else if cleaned > 0 {
		log.Printf("[chat] cleaned %d stale streaming messages for run=%s", cleaned, runID)
	}

	list, err := repository.ListMessagesByRun(c.Request.Context(), h.Pool, runID, 50)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "list messages")
		return
	}
	out := make([]ChatMessageItem, 0, len(list))
	for _, m := range list {
		out = append(out, ChatMessageItem{
			ID:               m.ID.String(),
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
			Status:           m.Status,
			ErrorMessage:     m.ErrorMessage,
			CreatedAt:        m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	httpx.OK(c, gin.H{"messages": out})
}

// chatReq POST /api/v1/agent-runs/:id/chat 请求体。
type chatReq struct {
	Message string `json:"message"`
}

// Chat POST /api/v1/agent-runs/:id/chat
// 流式 LLM 对话（单轮，不跑完整 RAG workflow）。
// 鉴权用 Authorization header（前端用 fetch 不用 EventSource，POST 不支持 EventSource）。
//
// 归属校验 + body 解析必须在 NewA2UIWriter 之前：一旦写入 200 流式响应头，
// 就无法再回退到 httpx.Fail 的 4xx JSON。
func (h *ChatHandler) Chat(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	runID, ok := parseRunID(c)
	if !ok {
		return
	}
	var req chatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	if req.Message == "" {
		httpx.Fail(c, http.StatusBadRequest, 4001, "message is required")
		return
	}
	if h.Pool == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "pool not configured")
		return
	}
	if h.Syn == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "synthesis service not configured")
		return
	}
	// 归属校验（在流式响应之前，否则无法写 4xx）
	if _, err := repository.GetAgentRunByID(c.Request.Context(), h.Pool, uid, runID); err != nil {
		respondRunErr(c, err)
		return
	}

	a2ui, err := synthesis.NewA2UIWriter(c.Writer)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "streaming not supported")
		return
	}

	// ChatWithRun 内部已通过 WriteChatError 把错误写入流；handler 仅记日志。
	if chatErr := h.Syn.ChatWithRun(c.Request.Context(), runID.String(), uid.String(), req.Message, a2ui); chatErr != nil {
		log.Printf("[chat] ChatWithRun run=%s: %v", runID, chatErr)
	}
}
