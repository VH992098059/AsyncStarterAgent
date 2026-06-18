package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestWebhook_Todoist_ValidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	// Svc=nil 走"无 trigger svc 注入"路径，验证 200 + matched=false 行为。
	// （M-1 修复后，nil 分支加 log 警告并返回 matched=false，避免外部 webhook 误以为匹配成功。）
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
	data := decodeData(t, w.Body.Bytes())
	// Svc=nil 路径 → log 警告 + matched=false（避免外部 webhook 误以为匹配成功）
	if matched, ok := data["matched"].(bool); !ok || matched {
		t.Fatalf("expected matched=false, got %v", data["matched"])
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

// TestWebhook_Todoist_Matched 验证：注入 Svc + 匹配关键词 → 200 + matched=true。
// 需要 DATABASE_URL 才能跑（参考 service_test.go 的 t.Skip 风格）。
func TestWebhook_Todoist_Matched(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	gin.SetMode(gin.TestMode)
	secret := "test-secret"

	pool, err := repository.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	svc := trigger.NewService(pool, trigger.NewMatcher(trigger.DefaultMatcherRules()))
	h := &handler.WebhookHandler{Secret: secret, Svc: svc}

	body := []byte(`{"event_name":"item:added","event_id":"evt-match-` + uuid.New().String() +
		`","event_data":{"id":"t-match","content":"写本周周报"}}`)
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
	data := decodeData(t, w.Body.Bytes())
	if matched, ok := data["matched"].(bool); !ok || !matched {
		t.Fatalf("expected matched=true, got %v", data["matched"])
	}
}

// TestWebhook_Todoist_NoMatch 验证：注入 Svc + 不匹配文本 → 200 + matched=false。
// （"no rule matched" 是业务预期，外部 webhook 不应因此重试。）
func TestWebhook_Todoist_NoMatch(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	gin.SetMode(gin.TestMode)
	secret := "test-secret"

	pool, err := repository.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	svc := trigger.NewService(pool, trigger.NewMatcher(trigger.DefaultMatcherRules()))
	h := &handler.WebhookHandler{Secret: secret, Svc: svc}

	body := []byte(`{"event_name":"item:added","event_id":"evt-nomatch-` + uuid.New().String() +
		`","event_data":{"id":"t-nomatch","content":"今天去超市买菜"}}`)
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
	data := decodeData(t, w.Body.Bytes())
	if matched, ok := data["matched"].(bool); !ok || matched {
		t.Fatalf("expected matched=false, got %v", data["matched"])
	}
}

// decodeData 解码 httpx.Response 标准包络并返回 data 字段（map 形式）。
func decodeData(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var resp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, string(body))
	}
	if resp.Data == nil {
		t.Fatalf("nil data: %s", string(body))
	}
	return resp.Data
}
