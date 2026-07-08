package handler_test

import (
	"bytes"
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

// newDraftDetailRouter 构造最小化 router 用于 DraftDetailHandler 前置校验测试。
// nil Pool 走前置校验路径，不进 repository。
func newDraftDetailRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.DraftDetailHandler{Pool: nil, Syn: nil}
	r.GET("/api/v1/drafts/:id", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.GetDraft(c)
	})
	r.PUT("/api/v1/drafts/:id", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.UpdateDraft(c)
	})
	r.POST("/api/v1/drafts/:id/marks/:markID/resolve", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.ResolveMark(c)
	})
	r.GET("/api/v1/drafts/:id/deliveries", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.ListDeliveries(c)
	})
	return r
}

func TestDraftDetailHandler_PoolNil(t *testing.T) {
	r := newDraftDetailRouter()
	cases := []struct {
		method string
		path   string
		body   string
		code   int
		errc   int
	}{
		{"GET", "/api/v1/drafts/" + uuid.New().String(), "", http.StatusServiceUnavailable, 5001},
		{"PUT", "/api/v1/drafts/" + uuid.New().String(), `{"markdown":"x"}`, http.StatusServiceUnavailable, 5001},
		{"POST", "/api/v1/drafts/" + uuid.New().String() + "/marks/m1/resolve", `{"value":"v"}`, http.StatusServiceUnavailable, 5001},
		{"GET", "/api/v1/drafts/" + uuid.New().String() + "/deliveries", "", http.StatusServiceUnavailable, 5001},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("%s %s: expected %d, got %d body=%s", tc.method, tc.path, tc.code, w.Code, w.Body.String())
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

func TestDraftDetailHandler_BadID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.DraftDetailHandler{Pool: nil}
	r.GET("/api/v1/drafts/:id", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.GetDraft(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/drafts/not-a-uuid", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDraftDetailHandler_UpdateBadBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.DraftDetailHandler{Pool: nil}
	r.PUT("/api/v1/drafts/:id", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.UpdateDraft(c)
	})

	w := httptest.NewRecorder()
	// bad json
	req, _ := http.NewRequest("PUT", "/api/v1/drafts/"+uuid.New().String(), bytes.NewBufferString(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDraftDetailHandler_ResolveBadBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &handler.DraftDetailHandler{Pool: nil}
	r.POST("/api/v1/drafts/:id/marks/:markID/resolve", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		h.ResolveMark(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/drafts/"+uuid.New().String()+"/marks/m1/resolve", bytes.NewBufferString(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
