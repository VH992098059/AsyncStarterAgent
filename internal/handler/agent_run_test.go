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

// newAgentRunRouter 构造最小化 router 用于 AgentRunHandler 前置校验测试。
// nil Pool 走前置校验路径，不进 repository。
func newAgentRunRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.AgentRunHandler{Pool: nil}
	r.GET("/api/v1/agent-runs/:id", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.GetRun(c)
	})
	r.DELETE("/api/v1/agent-runs/:id", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.DeleteRun(c)
	})
	r.POST("/api/v1/agent-runs/:id/cancel", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.CancelRun(c)
	})
	r.POST("/api/v1/agent-runs/:id/retry", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.RetryRun(c)
	})
	return r
}

func TestAgentRunHandler_PoolNil(t *testing.T) {
	r := newAgentRunRouter()
	cases := []struct {
		method string
		path   string
		code   int
		errc   int
	}{
		{"GET", "/api/v1/agent-runs/" + uuid.New().String(), http.StatusServiceUnavailable, 5001},
		{"DELETE", "/api/v1/agent-runs/" + uuid.New().String(), http.StatusServiceUnavailable, 5001},
		{"POST", "/api/v1/agent-runs/" + uuid.New().String() + "/cancel", http.StatusServiceUnavailable, 5001},
		{"POST", "/api/v1/agent-runs/" + uuid.New().String() + "/retry", http.StatusServiceUnavailable, 5001},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tc.method, tc.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("%s %s: expected %d, got %d", tc.method, tc.path, tc.code, w.Code)
		}
		var resp httpx.Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s %s: decode: %v", tc.method, tc.path, err)
		}
		if resp.Code != tc.errc {
			t.Fatalf("%s %s: expected code %d, got %d", tc.method, tc.path, tc.errc, resp.Code)
		}
	}
}

func TestAgentRunHandler_BadID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 非 nil Pool 但用 *pgxpool.Pool 的零值无法直接构造；这里只测 bad id 路径，
	// 用 nil Pool 也会先走 id 解析校验，所以能覆盖 bad id 分支。
	h := &handler.AgentRunHandler{Pool: nil}
	r.GET("/api/v1/agent-runs/:id", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.GetRun(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/agent-runs/not-a-uuid", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp httpx.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 4001 {
		t.Fatalf("expected code 4001, got %d", resp.Code)
	}
}
