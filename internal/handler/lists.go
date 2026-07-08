package handler

import (
	"net/http"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ListHandler 暴露 keywords / datasources / agent-runs 三个 GET 列表端点。
// 都需要 auth.Middleware。
type ListHandler struct {
	Pool    *pgxpool.Pool
	Matcher *trigger.Matcher
}

// KeywordItem 关键词规则展示项。
type KeywordItem struct {
	Pattern  string `json:"pattern"`
	TaskType string `json:"task_type"`
}

// GetKeywords GET /api/v1/keywords
// 展示 trigger.Matcher 已注册的关键词规则。
func (h *ListHandler) GetKeywords(c *gin.Context) {
	if h.Matcher == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "matcher not configured")
		return
	}
	rules := h.Matcher.Rules()
	out := make([]KeywordItem, 0, len(rules))
	for _, r := range rules {
		out = append(out, KeywordItem{Pattern: r.Pattern, TaskType: r.TaskType})
	}
	httpx.OK(c, out)
}

// DataSourceItem 数据源展示项。
type DataSourceItem struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	LastSyncAt *string `json:"last_sync_at"`
}

// GetDataSources GET /api/v1/datasources
// 拉取当前用户的数据源列表（按 type 排序）。
func (h *ListHandler) GetDataSources(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	ds, err := repository.ListDataSources(c.Request.Context(), h.Pool, uid)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "list data sources")
		return
	}
	out := make([]DataSourceItem, 0, len(ds))
	for _, d := range ds {
		item := DataSourceItem{
			ID:     d.ID.String(),
			Type:   d.Type,
			Name:   d.Name,
			Status: d.Status,
		}
		if d.LastSyncAt != nil {
			s := d.LastSyncAt.UTC().Format("2006-01-02T15:04:05Z")
			item.LastSyncAt = &s
		}
		out = append(out, item)
	}
	httpx.OK(c, out)
}

// AgentRunItem AgentRun 展示项。
type AgentRunItem struct {
	ID            string  `json:"id"`
	TaskType      string  `json:"task_type"`
	Status        string  `json:"status"`
	CurrentStage  string  `json:"current_stage"`
	TriggerType   string  `json:"trigger_type"`
	TriggerSource string  `json:"trigger_source"`
	ErrorMessage  string  `json:"error_message"`
	CreatedAt     string  `json:"created_at"`
	CompletedAt   *string `json:"completed_at"`
}

// AgentRunStats 统计信息。
type AgentRunStats struct {
	Total map[string]int `json:"total"`
	Today map[string]int `json:"today"`
}

// GetAgentRuns GET /api/v1/agent-runs?limit=20&before=<RFC3339>
// before 用于游标分页：仅返回 created_at 严格早于 before 的记录。
// before 解析失败时按未传处理（宽松降级，不返回 400）。
func (h *ListHandler) GetAgentRuns(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	if h.Pool == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5001, "pool not configured")
		return
	}
	limit := 20
	if l := c.Query("limit"); l != "" {
		var n int
		_, err := parseInt(l, &n)
		if err == nil && n > 0 {
			limit = n
		}
	}
	var before *time.Time
	if b := c.Query("before"); b != "" {
		if t, err := time.Parse(time.RFC3339, b); err == nil {
			before = &t
		}
	}
	runs, err := repository.ListAgentRuns(c.Request.Context(), h.Pool, uid, limit, before)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "list agent runs")
		return
	}
	total, today, err := repository.CountAgentRunsByStatus(c.Request.Context(), h.Pool, uid)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "count agent runs")
		return
	}
	items := make([]AgentRunItem, 0, len(runs))
	for _, r := range runs {
		item := AgentRunItem{
			ID:            r.ID.String(),
			TaskType:      r.TaskType,
			Status:        r.Status,
			CurrentStage:  r.CurrentStage,
			TriggerType:   r.TriggerType,
			TriggerSource: r.TriggerSource,
			ErrorMessage:  r.ErrorMessage,
			CreatedAt:     r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if r.CompletedAt != nil {
			s := r.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
			item.CompletedAt = &s
		}
		items = append(items, item)
	}
	httpx.OK(c, gin.H{
		"runs":  items,
		"stats": AgentRunStats{Total: total, Today: today},
	})
}

// parseInt 小工具：把 query string 转 int。
func parseInt(s string, out *int) (int, error) {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, http.ErrBodyNotAllowed
		}
		n = n*10 + int(s[i]-'0')
	}
	*out = n
	return n, nil
}
