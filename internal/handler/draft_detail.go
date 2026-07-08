package handler

import (
	"errors"
	"net/http"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DraftDetailHandler 暴露单 run 草稿详情 / 更新 / mark 解决 / 交付历史端点。
// :id 参数统一为 agent_run_id（drafts 表通过 agent_run_id 关联）。
// 所有方法都需要 auth.Middleware。
type DraftDetailHandler struct {
	Pool *pgxpool.Pool
	Syn  *synthesis.Service
}

// DraftDetail 草稿详情展示项。
type DraftDetail struct {
	RunID        string                `json:"run_id"`
	Title        string                `json:"title"`
	Markdown     string                `json:"markdown"`
	Completeness float32               `json:"completeness"`
	Marks        []repository.MarkJSON `json:"marks"`
	Status       string                `json:"status"`
	UpdatedAt    string                `json:"updated_at"`
}

// DeliveryItem 交付历史展示项。
type DeliveryItem struct {
	ID           string `json:"id"`
	TargetType   string `json:"target_type"`
	TargetURL    string `json:"target_url"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
}

// GetDraft GET /api/v1/drafts/:id
// 返回草稿详情（markdown + marks + completeness + status）。
func (h *DraftDetailHandler) GetDraft(c *gin.Context) {
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
	draft, err := repository.GetDraftByRunID(c.Request.Context(), h.Pool, runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Fail(c, http.StatusNotFound, 4043, "draft not found")
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, 5001, "load draft")
		return
	}
	marks, _ := repository.UnmarshalMarks(draft.Marks)
	httpx.OK(c, DraftDetail{
		RunID:        draft.AgentRunID.String(),
		Title:        draft.Title,
		Markdown:     draft.MarkdownContent,
		Completeness: draft.Completeness,
		Marks:        marks,
		Status:       draft.Status,
		UpdatedAt:    draft.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

// updateDraftReq PUT /api/v1/drafts/:id 请求体。
type updateDraftReq struct {
	Markdown string `json:"markdown"`
}

// UpdateDraft PUT /api/v1/drafts/:id
// 更新草稿正文，同步重算 marks 与 completeness。
func (h *DraftDetailHandler) UpdateDraft(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	runID, ok := parseRunID(c)
	if !ok {
		return
	}
	var req updateDraftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid body")
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
	marks := synthesis.ExtractMarks(req.Markdown)
	comp := synthesis.Completeness(req.Markdown)
	marksJSON, err := synthesis.MarshalMarks(marks)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "marshal marks")
		return
	}
	if err := repository.UpdateDraftMarkdown(c.Request.Context(), h.Pool, runID, req.Markdown, marksJSON, comp); err != nil {
		if errors.Is(err, repository.ErrDraftNotFound) {
			httpx.Fail(c, http.StatusNotFound, 4043, "draft not found")
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, 5001, "update draft")
		return
	}
	httpx.OK(c, gin.H{"updated": runID.String()})
}

// resolveMarkReq POST /api/v1/drafts/:id/marks/:markID/resolve 请求体。
type resolveMarkReq struct {
	Value string `json:"value"`
}

// ResolveMark POST /api/v1/drafts/:id/marks/:markID/resolve
// 用 value 替换草稿中 markID 对应的 [待补充:xxx] 占位符，返回新 markdown。
func (h *DraftDetailHandler) ResolveMark(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	runID, ok := parseRunID(c)
	if !ok {
		return
	}
	markID := c.Param("markID")
	if markID == "" {
		httpx.Fail(c, http.StatusBadRequest, 4001, "missing mark id")
		return
	}
	var req resolveMarkReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid body")
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
	if _, err := repository.GetAgentRunByID(c.Request.Context(), h.Pool, uid, runID); err != nil {
		respondRunErr(c, err)
		return
	}
	newMd, err := h.Syn.ResolveMark(c.Request.Context(), runID.String(), markID, req.Value)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "resolve mark: "+err.Error())
		return
	}
	httpx.OK(c, gin.H{"markdown": newMd})
}

// ListDeliveries GET /api/v1/drafts/:id/deliveries
// 返回某 run 的交付历史列表。
func (h *DraftDetailHandler) ListDeliveries(c *gin.Context) {
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
	list, err := repository.ListDeliveriesByRun(c.Request.Context(), h.Pool, runID)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "list deliveries")
		return
	}
	out := make([]DeliveryItem, 0, len(list))
	for _, d := range list {
		out = append(out, DeliveryItem{
			ID:           d.ID.String(),
			TargetType:   d.TargetType,
			TargetURL:    d.TargetURL,
			Status:       d.Status,
			ErrorMessage: d.ErrorMessage,
			CreatedAt:    d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	httpx.OK(c, gin.H{"deliveries": out})
}
