package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/asyncstarter/agent/internal/config"
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

// 飞书 webhook 测试（决策 #7）

func TestWebhook_Feishu_Challenge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{
		Cfg: &config.Config{FeishuVerificationToken: "tok"},
	}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	body := `{"challenge":"ajls384kdjx98XX","type":"url_verification","token":"tok"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp["challenge"] != "ajls384kdjx98XX" {
		t.Errorf("challenge: %s", resp["challenge"])
	}
}

func TestWebhook_Feishu_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{
		Cfg: &config.Config{FeishuVerificationToken: "expected-token"},
	}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	body := `{"type":"event_callback","header":{"event_type":"task.v2.task.created","token":"wrong-token"},"event":{}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestWebhook_Feishu_NoOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Cfg.FeishuVerificationToken 为空 → 跳过 token 校验
	h := &handler.WebhookHandler{}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	// 事件无 operator_id 字段 → 走"no operator"分支
	body := `{"type":"event_callback","header":{"event_type":"task.v2.task.created","token":""},"event":{"summary":"写周报"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no operator") {
		t.Errorf("expected 'no operator' msg, got %s", w.Body.String())
	}
}

func TestWebhook_Feishu_NonTaskEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	// 非任务事件 → "ignored"
	body := `{"type":"event_callback","header":{"event_type":"im.message.receive_v1","token":""},"event":{}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ignored") {
		t.Errorf("expected 'ignored' msg, got %s", w.Body.String())
	}
}

// TestWebhook_Feishu_EmptySummary 验证：task 事件 summary 为空 → 200 + "no summary"。
// 覆盖 handleFeishuTaskEvent 的早返回分支（无需 DB）。
func TestWebhook_Feishu_EmptySummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	// summary 为空 → "no summary" 早返回
	body := `{"type":"event_callback","header":{"event_type":"task.v2.task.created","token":""},"event":{"summary":""}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no summary") {
		t.Errorf("expected 'no summary' msg, got %s", w.Body.String())
	}
}

// TestWebhook_Feishu_TaskEvent_Matched 验证 happy path：
// 合法 task.v2.task.created 事件 + 已映射 open_id → ProcessKeyword 被调用 → 200 + "ok"。
// 镜像 TestWebhook_Todoist_Matched 模式，gated on DATABASE_URL。
// 需先 seed users + feishu_tokens 行（feishu_tokens.user_id REFERENCES users(id)）。
func TestWebhook_Feishu_TaskEvent_Matched(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	gin.SetMode(gin.TestMode)

	pool, err := repository.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	// Seed user（users 表：id/username/password_hash，见 migrations/0006_auth.up.sql）。
	// username 唯一随机，避免并发或重跑冲突。
	openID := "ou_test_feishu_" + uuid.New().String()
	username := "feishu_test_" + uuid.New().String()
	var userID uuid.UUID
	err = pool.QueryRow(context.Background(),
		`INSERT INTO users (username, password_hash) VALUES ($1, 'dummy_hash') RETURNING id`,
		username).Scan(&userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// cleanup 顺序：先 agent_runs（无 FK 到 users），再 users（CASCADE 到 feishu_tokens）。
	// 用 t.Logf 记录 cleanup 错误，不掩盖测试失败（不调用 t.Errorf）。
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM agent_runs WHERE user_id = $1`, userID); err != nil {
			t.Logf("cleanup agent_runs for user_id=%s: %v", userID, err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Logf("cleanup users id=%s: %v", userID, err)
		}
	})

	// Seed feishu_tokens（FK user_id REFERENCES users(id) ON DELETE CASCADE）。
	// access_token/refresh_token 是 BYTEA NOT NULL，但本路径不解密，直接塞任意字节即可。
	// open_id 随机以避免与 M-4（open_id 无 UNIQUE 约束）残留行冲突。
	_, err = pool.Exec(context.Background(),
		`INSERT INTO feishu_tokens (user_id, open_id, access_token, refresh_token, expires_at)
		 VALUES ($1, $2, $3, $4, NOW() + interval '2 hours')`,
		userID, openID, []byte("dummy_access"), []byte("dummy_refresh"))
	if err != nil {
		t.Fatalf("seed feishu_tokens: %v", err)
	}

	svc := trigger.NewService(pool, trigger.NewMatcher(trigger.DefaultMatcherRules()))
	// Cfg 为空 config → FeishuVerificationToken 为空 → 跳过 token 校验
	h := &handler.WebhookHandler{Svc: svc, Pool: pool, Cfg: &config.Config{}}

	// task.v2.task.created 事件：summary 含"周报"关键词（DefaultMatcherRules 匹配），
	// operator_id.open_id 指向 seed 行
	body := `{"type":"event_callback","header":{"event_type":"task.v2.task.created","event_id":"evt-feishu-` +
		uuid.New().String() + `","token":""},"event":{"summary":"写本周周报","operator_id":{"open_id":"` + openID + `"}}}`

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	// feishu 返回 raw {"code":0,"msg":"ok"}，不走 httpx 包络，按 raw map 解析
	// （参考 TestWebhook_Feishu_Challenge 的解析方式，不使用 decodeData）
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp["msg"] != "ok" {
		t.Errorf("expected msg=ok, got %q body=%s", resp["msg"], w.Body.String())
	}
}
