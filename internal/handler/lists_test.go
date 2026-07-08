package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// newListRouter 构造最小化 router 用于 ListHandler.GetAgentRuns 前置校验测试。
// nil Pool 走前置校验路径（auth 通过后立即返回 503），不进 repository。
func newListRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.ListHandler{Pool: nil}
	r.GET("/api/v1/agent-runs", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.GetAgentRuns(c)
	})
	return r
}

// TestListHandler_GetAgentRuns_PoolNil 验证 nil Pool 时返回 503/5001，
// 同时覆盖 limit / before 两个 query 参数的解析路径不会因新增 before 解析而崩溃。
func TestListHandler_GetAgentRuns_PoolNil(t *testing.T) {
	r := newListRouter()
	cases := []struct {
		name string
		path string
	}{
		{"no_query", "/api/v1/agent-runs"},
		{"limit_only", "/api/v1/agent-runs?limit=10"},
		{"before_rfc3339", "/api/v1/agent-runs?before=2026-07-05T12:00:00Z"},
		{"before_invalid", "/api/v1/agent-runs?before=not-a-time"},
		{"before_empty", "/api/v1/agent-runs?before="},
		{"limit_and_before", "/api/v1/agent-runs?limit=20&before=2026-07-05T12:00:00Z"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", tc.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: expected 503, got %d (body=%s)", tc.name, w.Code, w.Body.String())
		}
		var resp httpx.Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		if resp.Code != 5001 {
			t.Fatalf("%s: expected code 5001, got %d", tc.name, resp.Code)
		}
	}
}

// TestListHandler_GetAgentRuns_NoUser 验证未带 user context 时返回 401/4001。
func TestListHandler_GetAgentRuns_NoUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.ListHandler{Pool: nil}
	r.GET("/api/v1/agent-runs", h.GetAgentRuns)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/agent-runs?before=2026-07-05T12:00:00Z", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var resp httpx.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 4001 {
		t.Fatalf("expected code 4001, got %d", resp.Code)
	}
}
