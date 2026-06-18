package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

func TestTriggerHandler_BadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// nil svc 走前校验路径；只要不进 svc 就不 panic
	h := &handler.TriggerHandler{Svc: nil}
	r.POST("/trigger", h.ManualTrigger)

	body := bytes.NewBufferString(`{"user_id":"not-uuid","text":"x"}`)
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
