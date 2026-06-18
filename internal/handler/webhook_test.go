package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/gin-gonic/gin"
)

func TestWebhook_Todoist_ValidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	h := &handler.WebhookHandler{Secret: secret}

	body := []byte(`{"event_name":"item:added","event_id":"evt-1","event_data":{"id":"t1","content":"写周报"}}`)
	sig := trigger.SignHMAC(secret, body)

	r := gin.New()
	r.POST("/webhook/todoist", h.Todoist)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook/todoist", bytes.NewReader(body))
	req.Header.Set("X-Todoist-HMAC-SHA256", sig)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestWebhook_Todoist_InvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{Secret: "real-secret"}

	body := []byte(`{"event_name":"item:added","event_id":"evt-1"}`)

	r := gin.New()
	r.POST("/webhook/todoist", h.Todoist)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/webhook/todoist", bytes.NewReader(body))
	req.Header.Set("X-Todoist-HMAC-SHA256", "deadbeef")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
