package handler

import (
	"errors"
	"net/http"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentRunHandler 暴露单个 AgentRun 的 CRUD 端点。
// 所有方法都需要 auth.Middleware。
type AgentRunHandler struct {
	Pool *pgxpool.Pool
}

// AgentRunDetail AgentRun 详情展示项。
type AgentRunDetail struct {
	ID            string  `json:"id"`
	TaskType      string  `json:"task_type"`
	Status        string  `json:"status"`
	CurrentStage  string  `json:"current_stage"`
	TriggerType   string  `json:"trigger_type"`
	TriggerSource string  `json:"trigger_source"`
	ErrorMessage  string  `json:"error_message"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	CompletedAt   *string `json:"completed_at"`
}

// GetRun GET /api/v1/agent-runs/:id
// 返回单个 run 详情（含归属校验）。
func (h *AgentRunHandler) GetRun(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	id, ok := parseRunID(c)
	if !ok {
		return
	}
	if h.Pool == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "pool not configured")
		return
	}
	run, err := repository.GetAgentRunByID(c.Request.Context(), h.Pool, uid, id)
	if err != nil {
		respondRunErr(c, err)
		return
	}
	httpx.OK(c, toAgentRunDetail(run))
}

// DeleteRun DELETE /api/v1/agent-runs/:id
// 删除 run（FK CASCADE 清 drafts/deliveries/messages）。
// 先做归属校验，再删除。
func (h *AgentRunHandler) DeleteRun(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	id, ok := parseRunID(c)
	if !ok {
		return
	}
	if h.Pool == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "pool not configured")
		return
	}
	// 先校验归属，避免删别人的 run
	if _, err := repository.GetAgentRunByID(c.Request.Context(), h.Pool, uid, id); err != nil {
		respondRunErr(c, err)
		return
	}
	if err := repository.DeleteAgentRun(c.Request.Context(), h.Pool, id); err != nil {
		respondRunErr(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": id.String()})
}

// CancelRun POST /api/v1/agent-runs/:id/cancel
// 将 run 状态置为 cancelled。仅当 run 属于当前用户时允许。
func (h *AgentRunHandler) CancelRun(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	id, ok := parseRunID(c)
	if !ok {
		return
	}
	if h.Pool == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "pool not configured")
		return
	}
	if _, err := repository.GetAgentRunByID(c.Request.Context(), h.Pool, uid, id); err != nil {
		respondRunErr(c, err)
		return
	}
	if err := repository.UpdateAgentRunStatus(c.Request.Context(), h.Pool, id, "cancelled", "", ""); err != nil {
		respondRunErr(c, err)
		return
	}
	httpx.OK(c, gin.H{"cancelled": id.String()})
}

// RetryRun POST /api/v1/agent-runs/:id/retry
// 基于原 run 创建新 run（同 user_id/task_type，trigger_type='manual'，trigger_source='retry:<原id>'）。
// 返回新 run_id。
func (h *AgentRunHandler) RetryRun(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	id, ok := parseRunID(c)
	if !ok {
		return
	}
	if h.Pool == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "pool not configured")
		return
	}
	orig, err := repository.GetAgentRunByID(c.Request.Context(), h.Pool, uid, id)
	if err != nil {
		respondRunErr(c, err)
		return
	}
	newRun := &repository.AgentRun{
		UserID:        orig.UserID,
		TaskType:      orig.TaskType,
		Status:        "pending",
		CurrentStage:  "ingestion",
		TriggerType:   "manual",
		TriggerSource: "retry:" + id.String(),
	}
	newID, err := repository.CreateAgentRun(c.Request.Context(), h.Pool, newRun)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "create retry run")
		return
	}
	httpx.OK(c, gin.H{"run_id": newID.String()})
}

// parseRunID 解析 :id 参数为 uuid。失败时已写入 400 响应，调用方直接 return。
func parseRunID(c *gin.Context) (uuid.UUID, bool) {
	s := c.Param("id")
	id, err := uuid.Parse(s)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid run id")
		return uuid.Nil, false
	}
	return id, true
}

// respondRunErr 把 repository 层的 run 错误映射到 HTTP 响应。
func respondRunErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrRunNotOwned):
		httpx.Fail(c, http.StatusForbidden, 4042, "run not owned by user")
	case errors.Is(err, repository.ErrRunNotFound):
		httpx.Fail(c, http.StatusNotFound, 4041, "run not found")
	case errors.Is(err, pgx.ErrNoRows):
		httpx.Fail(c, http.StatusNotFound, 4041, "run not found")
	default:
		httpx.Fail(c, http.StatusInternalServerError, 5001, "run repository error")
	}
}

// toAgentRunDetail 把 repository.AgentRun 转成展示项。
func toAgentRunDetail(r *repository.AgentRun) AgentRunDetail {
	d := AgentRunDetail{
		ID:            r.ID.String(),
		TaskType:      r.TaskType,
		Status:        r.Status,
		CurrentStage:  r.CurrentStage,
		TriggerType:   r.TriggerType,
		TriggerSource: r.TriggerSource,
		ErrorMessage:  r.ErrorMessage,
		CreatedAt:     r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if r.CompletedAt != nil {
		s := r.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
		d.CompletedAt = &s
	}
	return d
}
