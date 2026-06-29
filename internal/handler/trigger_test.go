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

func TestTriggerHandler_BadRequest(t *testing.T) {
	// 决策 #2 扩 MVP：触发端点走 auth 中间件，user_id 从 JWT 注入。
	// 此处模拟有 user_id in context 的最小化 router。
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// nil svc 走前校验路径；只要不进 svc 就不 panic
	h := &handler.TriggerHandler{Svc: nil}
	r.POST("/trigger", func(c *gin.Context) {
		c.Set(auth.ContextUserIDKey, uuid.New().String())
		c.Set(auth.ContextUsernameKey, "tester")
		h.ManualTrigger(c)
	})

	body := bytes.NewBufferString(`{"text":""}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/trigger", body)
	req.Header.Set("Content-Type", "application/json")
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
