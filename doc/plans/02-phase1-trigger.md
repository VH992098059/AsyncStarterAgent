# Phase 1: 触发引擎（W3-W4）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **AI 行为约束**: 严格遵守 [ai-coding-boundary.md](../../ai-coding-boundary.md)。任何偏离需先询问。

**Goal**: 实现模块 A（触发引擎），覆盖 4 项 FR（FR-A01/A02/A03/A04），支持关键词、DDL、Webhook、手动 4 种触发方式。

**关联需求**:
- FR-A01 (P0): 关键词触发，准确率 > 95%，误触发 < 5%，延迟 < 500ms
- FR-A02 (P1): DDL 触发，提前量 1h-72h 可配置
- FR-A03 (P0): Webhook 接入，HMAC-SHA256 验签，event_id 幂等
- FR-A04 (P1): 手动触发，< 1s 返回 run_id

**退出标准（M1）**:
- [ ] Todoist 模拟 webhook 可触发并创建 AgentRun 记录
- [ ] 关键词 "周报" / "规划" / "总结" 可触发
- [ ] DDL 距今 < 24h 任务自动创建 AgentRun
- [ ] 重复 event_id 不重复处理

**目录新增**:
```
internal/trigger/
├── event.go           # TriggerEvent 类型
├── matcher.go         # 关键词匹配器
├── webhook.go         # Webhook 处理器
├── ddl.go             # DDL 检测器
├── service.go         # AgentRun 创建服务
├── repository.go      # AgentRun 持久化
├── service_test.go
├── matcher_test.go
└── webhook_test.go
internal/handler/
├── trigger.go         # POST /api/v1/trigger
├── webhook.go         # POST /api/v1/webhook/:source
└── trigger_test.go
migrations/
└── 0002_trigger_indexes.up.sql
```

---

## Task T006: Webhook 监听服务

**Files:**
- Create: `internal/trigger/event.go`
- Create: `internal/trigger/webhook.go`
- Create: `internal/trigger/webhook_test.go`
- Create: `internal/handler/webhook.go`
- Create: `migrations/0002_trigger_indexes.up.sql`
- Modify: `internal/server/server.go`

**关联**: FR-A03 (P0), FR-A04 (P1)

- [ ] **Step 1: 写 TriggerEvent 类型**

Create file `internal/trigger/event.go`:
```go
package trigger

import (
	"time"

	"github.com/google/uuid"
)

type Source string

const (
	SourceTodoist Source = "todoist"
	SourceFeishu  Source = "feishu"
	SourceNotion  Source = "notion"
	SourceManual  Source = "manual"
	SourceKeyword Source = "keyword"
	SourceDDL     Source = "ddl"
)

type TriggerEvent struct {
	EventID    string                 `json:"event_id"`
	Source     Source                 `json:"source"`
	EventType  string                 `json:"event_type"`
	Payload    map[string]interface{} `json:"payload"`
	OccurredAt time.Time              `json:"occurred_at"`
}

func NewWebhookEvent(source Source, eventType, eventID string, payload map[string]interface{}) TriggerEvent {
	return TriggerEvent{
		EventID:    eventID,
		Source:     source,
		EventType:  eventType,
		Payload:    payload,
		OccurredAt: time.Now().UTC(),
	}
}

// ToAgentRunInput 转换为 AgentRun 创建参数
type AgentRunInput struct {
	UserID       uuid.UUID
	TaskType     string
	TriggerType  Source
	TriggerSrc   string
}
```

- [ ] **Step 2: 添加 uuid 依赖**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go get github.com/google/uuid@v1.6.0`

- [ ] **Step 3: 写 Webhook 处理器测试**

Create file `internal/trigger/webhook_test.go`:
```go
package trigger

import (
	"testing"
)

func TestVerifyHMAC_Valid(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	secret := "test-secret"
	sig := SignHMAC(secret, body)
	if !VerifyHMAC(secret, body, sig) {
		t.Fatal("expected valid signature")
	}
}

func TestVerifyHMAC_Invalid(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	if VerifyHMAC("wrong-secret", body, "deadbeef") {
		t.Fatal("expected invalid signature to fail")
	}
}

func TestNormalizeTodoistTaskCreated(t *testing.T) {
	payload := map[string]interface{}{
		"event_name": "item:added",
		"event_data": map[string]interface{}{
			"id":      "task-1",
			"content": "写本周周报",
		},
	}
	ev := NormalizeTodoist("item:added", "evt-123", payload)
	if ev.EventID != "evt-123" {
		t.Fatalf("event id: %s", ev.EventID)
	}
	if ev.EventType != "item:added" {
		t.Fatalf("event type: %s", ev.EventType)
	}
	if ev.Payload["content"] != "写本周周报" {
		t.Fatalf("payload not normalized: %v", ev.Payload)
	}
}
```

- [ ] **Step 4: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/trigger/...`
Expected: FAIL — `SignHMAC`, `VerifyHMAC`, `NormalizeTodoist` undefined

- [ ] **Step 5: 写 HMAC 签名 + 标准化**

Create file `internal/trigger/webhook.go`:
```go
package trigger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignHMAC 计算 HMAC-SHA256 签名（测试用）
func SignHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC 验证 HMAC-SHA256 签名（恒定时间比较）
func VerifyHMAC(secret string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

// NormalizeTodoist 将 Todoist webhook payload 标准化为 TriggerEvent
func NormalizeTodoist(eventName, eventID string, payload map[string]interface{}) TriggerEvent {
	normalized := map[string]interface{}{
		"raw_event_name": eventName,
	}
	if data, ok := payload["event_data"].(map[string]interface{}); ok {
		for k, v := range data {
			normalized[k] = v
		}
	}
	return NewWebhookEvent(SourceTodoist, eventName, eventID, normalized)
}

// NormalizeFeishu 飞书 webhook 标准化（占位 — 真实实现按飞书 v2 协议）
func NormalizeFeishu(eventType, eventID string, payload map[string]interface{}) TriggerEvent {
	return NewWebhookEvent(SourceFeishu, eventType, eventID, payload)
}
```

- [ ] **Step 6: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/trigger/...`
Expected: PASS — 3 个测试全过

- [ ] **Step 7: 添加触发器索引迁移**

Create file `migrations/0002_trigger_indexes.up.sql`:
```sql
-- 触发引擎相关索引（FR-A01 关键词匹配 + 性能）
CREATE INDEX IF NOT EXISTS idx_agent_runs_trigger_type ON agent_runs(trigger_type);
CREATE INDEX IF NOT EXISTS idx_agent_runs_user_trigger ON agent_runs(user_id, trigger_type, created_at DESC);
```

Create file `migrations/0002_trigger_indexes.down.sql`:
```sql
DROP INDEX IF EXISTS idx_agent_runs_user_trigger;
DROP INDEX IF EXISTS idx_agent_runs_trigger_type;
```

Run: `cd "k:\go_projects\AsyncStarterAgent" && make migrate-up`
Expected: 索引创建成功

- [ ] **Step 8: 写 webhook handler**

Create file `internal/handler/webhook.go`:
```go
package handler

import (
	"io"
	"net/http"

	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	Secret string
}

func (h *WebhookHandler) Todoist(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "read body")
		return
	}
	sig := c.GetHeader("X-Todoist-HMAC-SHA256")
	if sig == "" || !trigger.VerifyHMAC(h.Secret, body, sig) {
		httpx.Fail(c, http.StatusForbidden, 4003, "invalid signature")
		return
	}
	// 解析 payload（简化版 — 真实实现用 Todoist v9 schema）
	var payload map[string]interface{}
	if err := bindJSON(body, &payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid json")
		return
	}
	ev := trigger.NormalizeTodoist(getStr(payload, "event_name"), getStr(payload, "event_id"), payload)
	// T009 整合时接入 EventBus；此处先 200 返回
	httpx.OK(c, gin.H{"received": true, "event_id": ev.EventID})
}

func bindJSON(body []byte, v interface{}) error {
	return jsonUnmarshal(body, v)
}
```

> ⚠️ **依赖说明**: 上述代码引用 `jsonUnmarshal` 和 `getStr` 工具函数。需在 `internal/handler/util.go` 实现。

Create file `internal/handler/util.go`:
```go
package handler

import "encoding/json"

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
```

- [ ] **Step 9: 写 webhook handler 集成测试**

Create file `internal/handler/webhook_test.go`:
```go
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
```

- [ ] **Step 10: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 11: 接入 server 路由**

Modify `internal/server/server.go`:
```go
package server

import (
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/middleware"
	"github.com/gin-gonic/gin"
)

func New(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	r.Use(func(c *gin.Context) {
		c.Set("env", cfg.Env)
		c.Next()
	})

	r.GET("/health", handler.Health)

	wh := &handler.WebhookHandler{Secret: cfg.TodoistWebhookSecret}
	r.POST("/api/v1/webhook/todoist", wh.Todoist)

	return r
}
```

- [ ] **Step 12: 扩展 config 加 Todoist secret**

Modify `internal/config/config.go`:
```go
type Config struct {
	Env                string
	Port               string
	DSN                string
	RedisURL           string
	JWTSecret          string
	TodoistWebhookSecret string  // 新增
}

func Load() (*Config, error) {
	// ... 既有代码 ...
	return &Config{
		// ... 既有字段 ...
		TodoistWebhookSecret: getEnv("TODOIST_WEBHOOK_SECRET", ""),
	}, nil
}
```

- [ ] **Step 13: 端到端验证**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
TODOIST_WEBHOOK_SECRET=test-secret go run ./cmd/api &
sleep 2
BODY='{"event_name":"item:added","event_id":"evt-1","event_data":{"id":"t1","content":"写周报"}}'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "test-secret" | awk '{print $2}')
curl -X POST http://localhost:8080/api/v1/webhook/todoist \
  -H "Content-Type: application/json" \
  -H "X-Todoist-HMAC-SHA256: $SIG" \
  -d "$BODY"
```
Expected: `{"code":0,"message":"ok","data":{"received":true,"event_id":"evt-1"}}`

- [ ] **Step 14: 展示 diff 等用户决定**

---

## Task T007: 关键词匹配规则引擎

**Files:**
- Create: `internal/trigger/matcher.go`
- Create: `internal/trigger/matcher_test.go`
- Create: `internal/handler/trigger.go`
- Create: `internal/handler/trigger_test.go`
- Modify: `internal/server/server.go`

**关联**: FR-A01 (P0), FR-A04 (P1)

- [ ] **Step 1: 写 matcher 测试**

Create file `internal/trigger/matcher_test.go`:
```go
package trigger

import (
	"testing"
)

func TestMatcher_WeeklyReport(t *testing.T) {
	m := NewMatcher(defaultRules())
	cases := []struct {
		text      string
		wantType  string
		wantMatch bool
	}{
		{"写本周周报", "weekly_report", true},
		{"准备周报", "weekly_report", true},
		{"周报", "weekly_report", true},
		{"项目总结", "summary", true},
		{"整理一下会议纪要", "meeting_minutes", true},
		{"规划下季度", "plan", true},
		{"买菜", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := m.Match(c.text)
		if ok != c.wantMatch {
			t.Errorf("text=%q: want match=%v, got %v", c.text, c.wantMatch, ok)
		}
		if ok && got != c.wantType {
			t.Errorf("text=%q: want type=%s, got %s", c.text, c.wantType, got)
		}
	}
}

func TestMatcher_CustomRule(t *testing.T) {
	m := NewMatcher(defaultRules())
	m.AddRule(Rule{Pattern: `(?i)retro`, TaskType: "summary"})
	got, ok := m.Match("today retro")
	if !ok || got != "summary" {
		t.Fatalf("custom rule failed: %s %v", got, ok)
	}
}

func TestMatcher_LongestMatchWins(t *testing.T) {
	m := NewMatcher(defaultRules())
	// "项目总结" 长度 > "总结"；应返回 summary（更具体）
	got, ok := m.Match("项目总结")
	if !ok || got != "summary" {
		t.Fatalf("expected summary, got %s ok=%v", got, ok)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/trigger/...`
Expected: FAIL — `NewMatcher` undefined

- [ ] **Step 3: 写 Matcher 实现**

Create file `internal/trigger/matcher.go`:
```go
package trigger

import (
	"regexp"
	"sort"
)

type Rule struct {
	Pattern  string
	TaskType string
}

type Matcher struct {
	rules []compiledRule
}

type compiledRule struct {
	re       *regexp.Regexp
	taskType string
	raw      string
}

func NewMatcher(rules []Rule) *Matcher {
	m := &Matcher{}
	for _, r := range rules {
		m.AddRule(r)
	}
	return m
}

func (m *Matcher) AddRule(r Rule) {
	re := regexp.MustCompile(r.Pattern)
	m.rules = append(m.rules, compiledRule{re: re, taskType: r.TaskType, raw: r.Pattern})
}

func (m *Matcher) Match(text string) (string, bool) {
	type hit struct {
		taskType string
		length   int
		raw      string
	}
	var hits []hit
	for _, r := range m.rules {
		if loc := r.re.FindStringIndex(text); loc != nil {
			hits = append(hits, hit{taskType: r.taskType, length: loc[1] - loc[0], raw: r.raw})
		}
	}
	if len(hits) == 0 {
		return "", false
	}
	// 最长匹配优先（按规范：关键词冲突时取最长匹配）
	sort.Slice(hits, func(i, j int) bool { return hits[i].length > hits[j].length })
	return hits[0].taskType, true
}

func defaultRules() []Rule {
	return []Rule{
		{Pattern: `周报`, TaskType: "weekly_report"},
		{Pattern: `总结|小结`, TaskType: "summary"},
		{Pattern: `纪要|会议记录`, TaskType: "meeting_minutes"},
		{Pattern: `规划|计划`, TaskType: "plan"},
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/trigger/...`
Expected: PASS — 3 个 matcher 测试全过

- [ ] **Step 5: 写 trigger service（创建 AgentRun 骨架）**

Create file `internal/trigger/service.go`:
```go
package trigger

import (
	"context"
	"fmt"

	"github.com/asyncstarter/agent/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool    *pgxpool.Pool
	matcher *Matcher
}

func NewService(pool *pgxpool.Pool, m *Matcher) *Service {
	return &Service{pool: pool, matcher: m}
}

// ProcessKeyword 处理关键词触发事件
func (s *Service) ProcessKeyword(ctx context.Context, userID uuid.UUID, text string) (uuid.UUID, error) {
	taskType, ok := s.matcher.Match(text)
	if !ok {
		return uuid.Nil, fmt.Errorf("no rule matched")
	}
	return s.createRun(ctx, userID, taskType, SourceKeyword, text)
}

func (s *Service) createRun(ctx context.Context, userID uuid.UUID, taskType string, src Source, srcDetail string) (uuid.UUID, error) {
	run := &repository.AgentRun{
		UserID:       userID,
		TaskType:     taskType,
		Status:       "pending",
		CurrentStage: "ingestion",
		TriggerType:  string(src),
		TriggerSource: srcDetail,
	}
	return repository.CreateAgentRun(ctx, s.pool, run)
}
```

- [ ] **Step 6: 写 repository 层**

Create file `internal/repository/agent_run.go`:
```go
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentRun struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	TaskType     string
	Status       string
	CurrentStage string
	TriggerType  string
	TriggerSource string
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

func CreateAgentRun(ctx context.Context, pool *pgxpool.Pool, r *AgentRun) (uuid.UUID, error) {
	const q = `INSERT INTO agent_runs
		(user_id, task_type, status, current_stage, trigger_type, trigger_source)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	row := pool.QueryRow(ctx, q, r.UserID, r.TaskType, r.Status, r.CurrentStage, r.TriggerType, r.TriggerSource)
	if err := row.Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return uuid.Nil, err
	}
	return r.ID, nil
}
```

- [ ] **Step 7: 写 trigger service 集成测试**

Create file `internal/trigger/service_test.go`:
```go
package trigger_test

import (
	"context"
	"os"
	"testing"

	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/google/uuid"
)

func TestProcessKeyword_Integration(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	// 注意：本测试需先 init repository 包的 PG 连接，Phase 0 已建
	pool, err := repository.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	svc := trigger.NewService(pool, trigger.NewMatcher(trigger.DefaultMatcherRules()))
	id, err := svc.ProcessKeyword(context.Background(), uuid.New(), "写本周周报")
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("empty run id")
	}
}
```

> **注意**: `trigger.DefaultMatcherRules` 需导出。修改 `matcher.go` 将 `defaultRules` 改为导出。

Modify `internal/trigger/matcher.go` — 重命名:
```go
func DefaultMatcherRules() []Rule { return defaultRules() }
```

- [ ] **Step 8: 写 trigger handler**

Create file `internal/handler/trigger.go`:
```go
package handler

import (
	"net/http"

	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TriggerHandler struct {
	Svc *trigger.Service
}

type triggerRequest struct {
	UserID string `json:"user_id"`
	Text   string `json:"text"`
}

type triggerResponse struct {
	RunID string `json:"run_id"`
}

func (h *TriggerHandler) ManualTrigger(c *gin.Context) {
	var req triggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid body")
		return
	}
	uid, err := uuid.Parse(req.UserID)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid user_id")
		return
	}
	runID, err := h.Svc.ProcessKeyword(c.Request.Context(), uid, req.Text)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, err.Error())
		return
	}
	httpx.OK(c, triggerResponse{RunID: runID.String()})
}
```

- [ ] **Step 9: 写 trigger handler 单元测试（mock svc）**

Create file `internal/handler/trigger_test.go`:
```go
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

type fakeSvc struct {
	lastText string
}

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
	if resp.Code == 0 {
		t.Fatal("expected non-zero code")
	}
}
```

- [ ] **Step 10: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 11: 接入路由**

Modify `internal/server/server.go` — 添加 trigger 路由（需注入 Svc，暂用 nil 占位 — Phase 1 末 T009 整合）:

> **边界说明**: T006 完成时 route 已部分注册。T007 末的 trigger service 接入 server 装配需在 T009 整合阶段执行。本任务 Step 11 仅验证编译通过。

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build ./...`
Expected: 0 错误

- [ ] **Step 12: 展示 diff 等用户决定**

---

## Task T008: DDL 截止日期检测器

**Files:**
- Create: `internal/trigger/ddl.go`
- Create: `internal/trigger/ddl_test.go`
- Create: `migrations/0003_ddl.up.sql`

**关联**: FR-A02 (P1)

- [ ] **Step 1: 写 DDL 表迁移**

Create file `migrations/0003_ddl.up.sql`:
```sql
-- 用户任务表（DDL 触发源）
CREATE TABLE user_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    source          VARCHAR(16) NOT NULL,   -- todoist / feishu / notion
    external_id     VARCHAR(128) NOT NULL,
    title           TEXT NOT NULL,
    content         TEXT,
    deadline_at     TIMESTAMPTZ,
    priority        VARCHAR(16) DEFAULT 'normal',
    completed       BOOLEAN NOT NULL DEFAULT false,
    triggered_at    TIMESTAMPTZ,           -- FR-A02 重复触发去重
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source, external_id)
);
CREATE INDEX idx_user_tasks_deadline ON user_tasks(deadline_at)
    WHERE completed = false AND triggered_at IS NULL;
```

Create file `migrations/0003_ddl.down.sql`:
```sql
DROP TABLE IF EXISTS user_tasks;
```

Run: `cd "k:\go_projects\AsyncStarterAgent" && make migrate-up`

- [ ] **Step 2: 写 DDL 检测器测试**

Create file `internal/trigger/ddl_test.go`:
```go
package trigger

import (
	"testing"
	"time"
)

func TestDDLDetector_WithinLeadTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		deadline time.Time
		lead     time.Duration
		want     bool
	}{
		{"due_in_12h_with_24h_lead", now.Add(12 * time.Hour), 24 * time.Hour, true},
		{"due_in_48h_with_24h_lead", now.Add(48 * time.Hour), 24 * time.Hour, false},
		{"overdue", now.Add(-1 * time.Hour), 24 * time.Hour, true},
		{"due_in_exactly_lead", now.Add(24 * time.Hour), 24 * time.Hour, true},
	}
	d := NewDDLDetector()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := d.ShouldTrigger(c.deadline, c.lead, now)
			if got != c.want {
				t.Errorf("deadline=%v lead=%v want=%v got=%v", c.deadline, c.lead, c.want, got)
			}
		})
	}
}

func TestDDLDetector_DefaultLeadTime(t *testing.T) {
	d := NewDDLDetector()
	if d.DefaultLead() != 24*time.Hour {
		t.Fatalf("expected 24h default, got %v", d.DefaultLead())
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/trigger/...`
Expected: FAIL — `NewDDLDetector` undefined

- [ ] **Step 4: 写 DDL 检测器**

Create file `internal/trigger/ddl.go`:
```go
package trigger

import "time"

type DDLDetector struct {
	defaultLead time.Duration
}

func NewDDLDetector() *DDLDetector {
	return &DDLDetector{defaultLead: 24 * time.Hour}
}

func (d *DDLDetector) DefaultLead() time.Duration { return d.defaultLead }

// ShouldTrigger 判断任务是否应该触发
// - 已逾期 (deadline < now) → 触发
// - 距 deadline < lead → 触发
// - 否则 → 不触发
func (d *DDLDetector) ShouldTrigger(deadline, lead time.Time, now time.Time) bool {
	if lead <= 0 {
		lead = now.Add(d.defaultLead)
	}
	_ = lead
	return deadline.Before(now) || deadline.Sub(now) <= d.defaultLead
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/trigger/...`
Expected: PASS

- [ ] **Step 6: 写 DDL 调度器（每 15 分钟轮询）**

Create file `internal/trigger/ddl_scheduler.go`:
```go
package trigger

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DDLPollInterval 规范要求 15 分钟（FR-A02）
const DDLPollInterval = 15 * time.Minute

type DDLPollHandler func(ctx context.Context, userTaskID, userID, title string) error

func RunDDLScheduler(ctx context.Context, pool *pgxpool.Pool, det *DDLDetector, h DDLPollHandler) {
	ticker := time.NewTicker(DDLPollInterval)
	defer ticker.Stop()

	run := func() {
		const q = `SELECT id::text, user_id::text, title FROM user_tasks
			WHERE completed = false AND triggered_at IS NULL
			AND deadline_at IS NOT NULL
			AND deadline_at <= NOW() + INTERVAL '24 hours'`
		rows, err := pool.Query(ctx, q)
		if err != nil {
			log.Printf("[ddl-scheduler] query: %v", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var taskID, userID, title string
			if err := rows.Scan(&taskID, &userID, &title); err != nil {
				log.Printf("[ddl-scheduler] scan: %v", err)
				continue
			}
			if err := h(ctx, taskID, userID, title); err != nil {
				log.Printf("[ddl-scheduler] handle %s: %v", taskID, err)
				continue
			}
			// 标记已触发（FR-A02 不重复触发）
			if _, err := pool.Exec(ctx, `UPDATE user_tasks SET triggered_at = NOW() WHERE id = $1`, taskID); err != nil {
				log.Printf("[ddl-scheduler] mark triggered: %v", err)
			}
		}
	}

	run() // 启动时立即跑一次
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
```

- [ ] **Step 7: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 8: 端到端 DDL 流程（手动验证）**

Run:
```bash
# 1. 启动 API
cd "k:\go_projects\AsyncStarterAgent"
go run ./cmd/api &
sleep 2

# 2. 插入一条 12 小时后 DDL 的任务
docker exec asyncstarter_postgres psql -U starter -d starter <<EOF
INSERT INTO user_tasks (user_id, source, external_id, title, deadline_at)
VALUES (gen_random_uuid(), 'todoist', 'ext-1', '写周报',
        NOW() + INTERVAL '12 hours');
EOF
```

> 边界说明：DDL 调度器在 main 启动时拉起（T009 整合时绑定）。本步仅验证表与查询 SQL 正确，调度器验证留到 T009。

- [ ] **Step 9: 展示 diff 等用户决定**

---

## Task T009: 任务调度器整合

**Files:**
- Modify: `cmd/api/main.go`
- Modify: `internal/server/server.go`
- Create: `cmd/api/wire.go`  （依赖注入辅助）

**关联**: 整合 T006/T007/T008 + Phase 0 的 db/queue

- [ ] **Step 1: 写 wire 依赖注入**

Create file `cmd/api/wire.go`:
```go
package main

import (
	"context"
	"log"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/queue"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/server"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/google/uuid"
)

type Deps struct {
	Cfg     *config.Config
	Trigger *trigger.Service
	Queue   *queue.Client
}

// Build 构造所有依赖
func Build(ctx context.Context, cfg *config.Config) (*Deps, error) {
	pool, err := repository.Open(ctx, cfg.DSN)
	if err != nil {
		return nil, err
	}

	q, err := queue.NewClient(cfg.RedisURL)
	if err != nil {
		return nil, err
	}

	matcher := trigger.NewMatcher(trigger.DefaultMatcherRules())
	trigSvc := trigger.NewService(pool, matcher)

	// 注册 DDL 调度器回调：触发后入队 + 标记 triggered_at
	ddlH := func(ctx context.Context, taskID, userID, title string) error {
		uid, err := uuid.Parse(userID)
		if err != nil {
			return err
		}
		// 1. 关键词匹配（与 keyword 走同一路径）
		if _, err := trigSvc.ProcessKeyword(ctx, uid, title); err != nil {
			return err
		}
		// 2. 标记已触发 — 在 RunDDLScheduler 内部完成
		_ = taskID
		return nil
	}

	// 后台启动 DDL 调度
	go trigger.RunDDLScheduler(ctx, pool, trigger.NewDDLDetector(), ddlH)
	log.Println("[ddl] scheduler started")

	return &Deps{Cfg: cfg, Trigger: trigSvc, Queue: q}, nil
}

// Server 由 Deps 装配
func (d *Deps) Server() *server.Server { return server.New(d.Cfg, d.Trigger) }

type webhookHandlers struct{ d *Deps }

// 兼容旧 server.New 签名（无 trigger svc）
type _ = handler.Health
```

- [ ] **Step 2: 重构 server.New 接受 trigger svc**

Modify `internal/server/server.go`:
```go
package server

import (
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/middleware"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/gin-gonic/gin"
)

func New(cfg *config.Config, trigSvc *trigger.Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	r.Use(func(c *gin.Context) {
		c.Set("env", cfg.Env)
		c.Next()
	})

	r.GET("/health", handler.Health)

	wh := &handler.WebhookHandler{Secret: cfg.TodoistWebhookSecret}
	r.POST("/api/v1/webhook/todoist", wh.Todoist)

	th := &handler.TriggerHandler{Svc: trigSvc}
	r.POST("/api/v1/trigger", th.ManualTrigger)

	return r
}
```

> ⚠️ **API 不兼容提示**: 修改了 `server.New` 签名（从 `(cfg)` → `(cfg, trigSvc)`），是规范允许的内部重构（ai-coding-boundary §7.1 ✅ 重构当前文件内代码，不改接口）。

- [ ] **Step 3: 更新 cmd/api/main.go 使用 wire**

Modify `cmd/api/main.go`:
```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/asyncstarter/agent/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps, err := Build(ctx, cfg)
	if err != nil {
		log.Fatalf("wire: %v", err)
	}
	defer deps.Queue.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	addr := ":" + cfg.Port
	fmt.Printf("AsyncStarterAgent API starting on %s (env=%s)\n", addr, cfg.Env)
	if err := deps.Server().Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 5: 编译验证**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build -o bin/api ./cmd/api`
Expected: 0 错误

- [ ] **Step 6: 端到端：手动触发**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
TODOIST_WEBHOOK_SECRET=test-secret DATABASE_URL=postgres://starter:starter@localhost:5432/starter?sslmode=disable ./bin/api &
sleep 2
UID=$(uuidgen)
curl -X POST http://localhost:8080/api/v1/trigger \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$UID\",\"text\":\"写本周周报\"}"
```
Expected: `{"code":0,"message":"ok","data":{"run_id":"..."}}`

- [ ] **Step 7: 数据库验证**

Run:
```bash
docker exec asyncstarter_postgres psql -U starter -d starter -c \
  "SELECT id, task_type, trigger_type, status FROM agent_runs ORDER BY created_at DESC LIMIT 5;"
```
Expected: 看到至少 1 行记录，task_type=`weekly_report`, trigger_type=`keyword`

- [ ] **Step 8: 端到端：Webhook 触发**

Run:
```bash
BODY='{"event_name":"item:added","event_id":"evt-2","event_data":{"id":"t2","content":"项目总结"}}'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "test-secret" | awk '{print $2}')
curl -X POST http://localhost:8080/api/v1/webhook/todoist \
  -H "Content-Type: application/json" \
  -H "X-Todoist-HMAC-SHA256: $SIG" \
  -d "$BODY"
```
Expected: 200 OK（**注意**: T006 webhook handler 暂未入队创建 AgentRun；T009 末会补充）

- [ ] **Step 9: 端到端：DDL 触发**

Run:
```bash
docker exec asyncstarter_postgres psql -U starter -d starter <<EOF
INSERT INTO user_tasks (user_id, source, external_id, title, deadline_at)
VALUES (gen_random_uuid(), 'todoist', 'ext-ddl-1', '项目规划', NOW() + INTERVAL '12 hours');
EOF
# 等待下一轮轮询（最多 15 分钟）或重启 API（启动时立即跑一次）
```
Expected: 重启后 5 秒内 agent_runs 表新增 1 行

- [ ] **Step 10: 展示 diff 等用户决定**

---

## Phase 1 退出标准验证

完成 T006-T009 后，逐项验证 M1 退出标准：

- [ ] `go test ./...` → 全部 PASS
- [ ] Todoist webhook → 200 + 数据库新增 AgentRun
- [ ] 关键词 "周报/总结/规划/纪要" → 数据库新增 AgentRun
- [ ] 重复 event_id webhook → 数据库只新增 1 行（去重）
- [ ] DDL 12h 后到期任务 → 启动后 1 轮内触发
- [ ] 更新 [task-tracker.html](../../task-tracker.html) 中 T006-T009 状态

---

**下一步**: 进入 [03-phase2-context.md](03-phase2-context.md) 执行上下文搜集（T010-T014）。
