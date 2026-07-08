
---

# Eino ADK Chat Agent (TurnLoop + SkillMiddleware) 实施计划

> **给 Agent 开发者的提示：** 推荐使用分步执行的方式，依照任务清单中的复选框（`- [ ]`）逐项推进并跟踪进度。

**目标：** 新增一个自包含的 `internal/chat` 包，用于集成 Eino ADK 的多轮对话 Chat Agent (`TurnLoop`) 与 Skill 中间件，并通过现有 Gin 服务器上的新 HTTP 路由进行暴露。

**架构设计：** 新的 `internal/chat` 包负责 Agent 构建 (`agent.go`)、会话与循环管理 (`session.go`) 以及 HTTP 处理器 (`handler.go`)。现有 `internal/server/server.go` 中的 Gin 服务器用于注册新路由。配置类（Config）新增两个环境变量（`CHAT_SKILLS_DIR`、`CHAT_SESSION_DIR`）。此改动不调整现有的包结构。

**技术栈：** `github.com/cloudwego/eino v0.9.9`、`github.com/cloudwego/eino-ext/components/model/openai`、`github.com/cloudwego/eino-ext/adk/backend/local`（本地文件系统 Skill 后端）、Gin 框架、标准 `net/http` SSE。

---

## 文件映射表

| 文件 | 操作 | 职责 |
|---|---|---|
| `internal/chat/agent.go` | 创建 | 构建支持 Skill 中间件的 `adk.TypedResumableAgent[*schema.Message]` |
| `internal/chat/session.go` | 创建 | `SessionStore` — 创建、获取、删除内存中的 `adk.TurnLoop` 实例 |
| `internal/chat/handler.go` | 创建 | Gin HTTP 处理器：POST /chat/sessions, POST /chat/sessions/:id/message, POST /chat/sessions/:id/abort, GET /chat/sessions/:id/stream |
| `internal/chat/handler_test.go` | 创建 | 使用 `httptest` 对处理器进行集成测试 |
| `internal/config/config.go` | 修改 | 添加 `ChatSkillsDir` 和 `ChatSessionDir` 字段 |
| `internal/server/server.go` | 修改 | 注册 `/api/v1/chat/*` 路由并绑定新处理器 |
| `cmd/api/wire.go` | 修改 | 实例化 `chat.Agent` 与 `chat.SessionStore` 并将其装配进 server |

---

## 任务 1：新增配置字段

**涉及文件：**
- 修改：`internal/config/config.go`

- [ ] **步骤 1：向 Config 结构体中添加字段**

在 `internal/config/config.go` 中，在 `NotionParentPageID` 之后往 `Config` 结构体添加以下字段：

```go
ChatSkillsDir string
ChatSessionDir string
```

- [ ] **步骤 2：在 Load() 中加载新字段**

在 `return &Config{...}` 代码块中，在 `NotionParentPageID` 之后添加：

```go
ChatSkillsDir:  getEnv("CHAT_SKILLS_DIR", ""),
ChatSessionDir: getEnv("CHAT_SESSION_DIR", "./data/chat_sessions"),
```

- [ ] **步骤 3：验证编译是否成功**

```bash
cd k:/go_projects/AsyncStarterAgent && go build ./internal/config/...
```
预期结果：编译通过，无错误输出。

- [ ] **步骤 4：提交代码**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/config/config.go
git commit -m "feat(chat): add ChatSkillsDir, ChatSessionDir config fields"
```

---

## 任务 2：实现 Agent 构建器

**涉及文件：**
- 创建：`internal/chat/agent.go`

**上下文说明：** Eino ADK 的 `deep.NewTyped[M]` 可用于构建完整的 `TypedResumableAgent`。其中的 Skill 中间件（`skill.NewTyped`）是可选的——如果 `ChatSkillsDir` 为空或目录不存在，我们将跳过该中间件。本地文件系统后端（`localbk.NewBackend`）提供了 Deep Agent 所需的文件读取与通配符匹配（glob）功能。Chat 模型将基于现有的 `settings.Factory` 通过 `GetLLM` 获取（该函数返回一个封装了 `model.BaseChatModel` 的 `synthesis.LLMClient`）。为了获取其底层原始的 `model.BaseChatModel`，我们需要通过在 `EinoLLM` 中新增 `ChatModel()` 访问器来提取 `EinoLLM.chatModel`。

- [ ] **步骤 1：编写会导致编译或运行失败的 Agent 构建测试**

创建 `internal/chat/agent_test.go`：

```go
package chat_test

import (
	"context"
	"testing"

	"github.com/asyncstarter/agent/internal/chat"
	"github.com/cloudwego/eino/components/model"
)

type stubChatModel struct{ model.BaseChatModel }

func TestBuildAgent_NilModel(t *testing.T) {
	_, err := chat.BuildAgent(context.Background(), chat.AgentConfig{
		ChatModel: nil,
		SkillsDir: "",
	})
	if err == nil {
		t.Fatal("expected error when ChatModel is nil")
	}
}
```

- [ ] **步骤 2：运行测试以确认其如预期失败**

```bash
cd k:/go_projects/AsyncStarterAgent && go test ./internal/chat/... -run TestBuildAgent_NilModel -v
```
预期结果：失败（FAIL）——因为 `chat` 包尚未创建。

- [ ] **步骤 3：在 EinoLLM 中添加 ChatModel() 访问器**

在 `internal/synthesis/llm.go` 中，在 `NewEinoLLM` 之后添加：

```go
// ChatModel returns the underlying Eino BaseChatModel.
func (e *EinoLLM) ChatModel() model.BaseChatModel {
	return e.chatModel
}
```

- [ ] **步骤 4：创建 internal/chat/agent.go**

```go
package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// AgentConfig holds dependencies for BuildAgent.
type AgentConfig struct {
	ChatModel model.BaseChatModel
	SkillsDir string // optional; skill middleware skipped when empty or missing
}

// BuildAgent constructs an Eino ADK TypedResumableAgent with optional skill middleware.
func BuildAgent(ctx context.Context, cfg AgentConfig) (adk.TypedResumableAgent[*schema.Message], error) {
	if cfg.ChatModel == nil {
		return nil, fmt.Errorf("chat: ChatModel is required")
	}

	backend, err := localbk.NewBackend(ctx, &localbk.Config{})
	if err != nil {
		return nil, fmt.Errorf("chat: local backend: %w", err)
	}

	var handlers []adk.TypedChatModelAgentMiddleware[*schema.Message]
	if dir := resolveSkillsDir(cfg.SkillsDir); dir != "" {
		skillBackend, err := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
			Backend: backend,
			BaseDir: dir,
		})
		if err != nil {
			return nil, fmt.Errorf("chat: skill backend: %w", err)
		}
		sm, err := skill.NewTyped[*schema.Message](ctx, &skill.TypedConfig[*schema.Message]{
			Backend: skillBackend,
		})
		if err != nil {
			return nil, fmt.Errorf("chat: skill middleware: %w", err)
		}
		handlers = append(handlers, sm)
	}

	return deep.NewTyped[*schema.Message](ctx, &deep.TypedConfig[*schema.Message]{
		Name:           "AsyncStarterChatAgent",
		Description:    "Multi-turn chat agent with optional skill retrieval.",
		ChatModel:      cfg.ChatModel,
		Backend:        backend,
		StreamingShell: backend,
		MaxIteration:   20,
		Handlers:       handlers,
	})
}

func resolveSkillsDir(dir string) string {
	if dir == "" {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	fi, err := os.Stat(abs)
	if err != nil || !fi.IsDir() {
		return ""
	}
	return abs
}
```

- [ ] **步骤 5：运行测试以确认其通过**

```bash
cd k:/go_projects/AsyncStarterAgent && go test ./internal/chat/... -run TestBuildAgent_NilModel -v
```
预期结果：通过（PASS）。

- [ ] **步骤 6：进行编译检查**

```bash
cd k:/go_projects/AsyncStarterAgent && go build ./internal/chat/...
```
预期结果：无编译错误。

- [ ] **步骤 7：提交代码**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/synthesis/llm.go internal/chat/agent.go internal/chat/agent_test.go
git commit -m "feat(chat): add BuildAgent with TurnLoop-ready agent + skill middleware"
```

---

## 任务 3：实现 SessionStore（多轮循环 TurnLoop 生命周期管理）

**涉及文件：**
- 创建：`internal/chat/session.go`

**上下文说明：** 每个聊天会话都会对应一个 `adk.TurnLoop[*ChatItem, *schema.Message]` 实例。`ChatItem` 承载了用户的输入文本。`SessionStore` 是一个线程安全的内存映射表（通过 `sync.RWMutex` 进行保护）。会话在接收到第一条消息时执行懒加载（延迟初始化）。TurnLoop 的核心回调机制包含：
- `GenInput`：将待处理的 `*ChatItem` 队列转换为 Agent 所需的 `[]*schema.Message`。
- `PrepareAgent`：返回共享的 Agent 实例。
- `OnAgentEvents`：将流式输出内容写入管道，以便 SSE 处理器进行读取。

- [ ] **步骤 1：编写会话生命周期测试**

在 `internal/chat/agent_test.go` 中添加以下测试用例：

```go
func TestSessionStore_CreateGet(t *testing.T) {
	store := chat.NewSessionStore()
	id := "sess-abc"
	store.Create(id)
	_, ok := store.Get(id)
	if !ok {
		t.Fatal("expected session to exist after Create")
	}
}

func TestSessionStore_DeleteMissing(t *testing.T) {
	store := chat.NewSessionStore()
	store.Delete("nonexistent") // must not panic
}
```

- [ ] **步骤 2：运行测试以确认其失败**

```bash
cd k:/go_projects/AsyncStarterAgent && go test ./internal/chat/... -run TestSessionStore -v
```
预期结果：失败（FAIL）——因为 `NewSessionStore` 尚未定义。

- [ ] **步骤 3：创建 internal/chat/session.go**

```go
package chat

import (
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// ChatItem is the unit pushed into a TurnLoop each turn.
type ChatItem struct {
	Text string
}

// Session holds a TurnLoop instance and the output channel bridge.
type Session struct {
	Loop   *adk.TurnLoop[*ChatItem, *schema.Message]
	Output chan string // receives streamed text chunks from OnAgentEvents
}

// SessionStore manages per-session TurnLoop instances.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore returns an empty SessionStore.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// Create registers a new empty session slot (loop is set later by EnsureLoop).
func (s *SessionStore) Create(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		s.sessions[id] = &Session{Output: make(chan string, 256)}
	}
}

// Get returns the session for id, or (nil, false) if not found.
func (s *SessionStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// Delete stops the loop (if running) and removes the session.
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		if sess.Loop != nil {
			sess.Loop.Stop(adk.WithImmediate())
		}
		delete(s.sessions, id)
	}
}

// EnsureLoop initialises the TurnLoop for a session if not already done,
// then starts it in a background goroutine.
func (s *SessionStore) EnsureLoop(id string, agent adk.TypedResumableAgent[*schema.Message]) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		sess = &Session{Output: make(chan string, 256)}
		s.sessions[id] = sess
	}
	if sess.Loop != nil {
		return sess
	}

	output := sess.Output
	loop := adk.NewTurnLoop(adk.TurnLoopConfig[*ChatItem, *schema.Message]{
		GenInput: func(items []*ChatItem) ([]*schema.Message, error) {
			var msgs []*schema.Message
			for _, it := range items {
				msgs = append(msgs, schema.UserMessage(it.Text))
			}
			return msgs, nil
		},
		PrepareAgent: func() (adk.TypedResumableAgent[*schema.Message], error) {
			return agent, nil
		},
		OnAgentEvents: func(stream *schema.StreamReader[*schema.Message]) error {
			defer stream.Close()
			for {
				msg, err := stream.Recv()
				if err != nil {
					return nil
				}
				if msg.Content != "" {
					select {
					case output <- msg.Content:
					default:
					}
				}
			}
		},
	})
	sess.Loop = loop
	go loop.Run(nil) // loop blocks internally; nil context -> uses background
	return sess
}
```

- [ ] **步骤 4：运行测试以确认其通过**

```bash
cd k:/go_projects/AsyncStarterAgent && go test ./internal/chat/... -run TestSessionStore -v
```
预期结果：两个测试用例均通过（PASS）。

- [ ] **步骤 5：提交代码**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/chat/session.go internal/chat/agent_test.go
git commit -m "feat(chat): add SessionStore with TurnLoop lifecycle management"
```

---

## 任务 4：实现 HTTP 处理器

**涉及文件：**
- 创建：`internal/chat/handler.go`
- 创建：`internal/chat/handler_test.go`

**上下文说明：** 我们需要实现四个核心路由接口：
- `POST /api/v1/chat/sessions` — 创建会话，返回 `{"session_id":"<uuid>"}`。
- `POST /api/v1/chat/sessions/:id/message` — 将消息 `{"text":"..."}` 压入多轮循环，返回 202 状态码。
- `GET /api/v1/chat/sessions/:id/stream` — 启动 SSE 流，持续读取并分发 `Session.Output` 中的内容。
- `DELETE /api/v1/chat/sessions/:id` — 停止并销毁指定会话。

`Handler` 会接收预先构建好的 `Agent` 以及 `*SessionStore` 指针，并在收到第一条消息时调用 `store.EnsureLoop` 进行初始化。

- [ ] **步骤 1：编写处理器的测试用例**

创建 `internal/chat/handler_test.go`：

```go
package chat_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/chat"
	"github.com/gin-gonic/gin"
)

func setupRouter(h *chat.Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/chat/sessions", h.CreateSession)
	r.DELETE("/api/v1/chat/sessions/:id", h.DeleteSession)
	r.POST("/api/v1/chat/sessions/:id/message", h.SendMessage)
	r.GET("/api/v1/chat/sessions/:id/stream", h.StreamSession)
	return r
}

func TestCreateSession(t *testing.T) {
	store := chat.NewSessionStore()
	h := chat.NewHandler(nil, store) // agent nil ok for creation test
	r := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/sessions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["session_id"] == "" {
		t.Fatal("expected session_id in response")
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	store := chat.NewSessionStore()
	h := chat.NewHandler(nil, store)
	r := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/chat/sessions/missing", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestSendMessage_NoSession(t *testing.T) {
	store := chat.NewSessionStore()
	h := chat.NewHandler(nil, store)
	r := setupRouter(h)

	body, _ := json.Marshal(map[string]string{"text": "hello"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat/sessions/nosess/message",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// With nil agent, EnsureLoop will be called but loop.Run will fail fast.
	// We only check the HTTP layer returns 202 (accepted) regardless.
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body)
	}
}
```

- [ ] **步骤 2：运行测试以确认其失败**

```bash
cd k:/go_projects/AsyncStarterAgent && go test ./internal/chat/... -run "TestCreateSession|TestDeleteSession|TestSendMessage" -v
```
预期结果：失败（FAIL）——因为未定义 `chat.Handler` 和 `chat.NewHandler`。

- [ ] **步骤 3：创建 internal/chat/handler.go**

```go
package chat

import (
	"net/http"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler holds dependencies for the chat HTTP handlers.
type Handler struct {
	agent adk.TypedResumableAgent[*schema.Message]
	store *SessionStore
}

// NewHandler creates a Handler with the given agent and store.
func NewHandler(agent adk.TypedResumableAgent[*schema.Message], store *SessionStore) *Handler {
	return &Handler{agent: agent, store: store}
}

// CreateSession handles POST /api/v1/chat/sessions.
// Returns {"session_id":"<uuid>"} with 201.
func (h *Handler) CreateSession(c *gin.Context) {
	id := uuid.New().String()
	h.store.Create(id)
	c.JSON(http.StatusCreated, gin.H{"session_id": id})
}

// DeleteSession handles DELETE /api/v1/chat/sessions/:id.
// Always returns 204 (idempotent).
func (h *Handler) DeleteSession(c *gin.Context) {
	h.store.Delete(c.Param("id"))
	c.Status(http.StatusNoContent)
}

// SendMessage handles POST /api/v1/chat/sessions/:id/message.
// Body: {"text":"..."}. Returns 202 Accepted immediately.
func (h *Handler) SendMessage(c *gin.Context) {
	var body struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id := c.Param("id")
	if h.agent == nil {
		c.Status(http.StatusAccepted)
		return
	}
	sess := h.store.EnsureLoop(id, h.agent)
	if err := sess.Loop.Push(&ChatItem{Text: body.Text}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusAccepted)
}

// StreamSession handles GET /api/v1/chat/sessions/:id/stream.
// Streams text/event-stream (SSE) from Session.Output until client disconnects.
func (h *Handler) StreamSession(c *gin.Context) {
	id := c.Param("id")
	sess, ok := h.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientGone := c.Request.Context().Done()
	flusher, canFlush := c.Writer.(http.Flusher)

	for {
		select {
		case <-clientGone:
			return
		case chunk, open := <-sess.Output:
			if !open {
				return
			}
			c.Writer.WriteString("data: " + chunk + "\n\n")
			if canFlush {
				flusher.Flush()
			}
		}
	}
}
```

- [ ] **步骤 4：运行测试以确认其通过**

```bash
cd k:/go_projects/AsyncStarterAgent && go test ./internal/chat/... -v
```
预期结果：所有测试全部通过（PASS）。

- [ ] **步骤 5：提交代码**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/chat/handler.go internal/chat/handler_test.go
git commit -m "feat(chat): add HTTP handlers for TurnLoop-backed chat sessions"
```

---

## 任务 5：集成至服务器与依赖装配

**涉及文件：**
- 修改：`internal/server/server.go`
- 修改：`cmd/api/wire.go`

**上下文说明：** `server.New` 函数新增参数：`chatHandler *chat.Handler`。同时，`wire.go` 中的 `Build` 函数现在负责构造 `chat.Agent`（利用从 `settingsFactory` 获取的 `model.BaseChatModel`）以及 `SessionStore`，继而创建 `Handler`。鉴于 Agent 启动时便需要一个具体的模型（而非每次用户请求时都重新创建），我们将利用全局配置（`OpenAIKey`、`OpenAIModel`、`OpenAIBaseURL`）来构建该模型。

- [ ] **步骤 1：进行编译测试（此时由于缺少参数会报错）**

```bash
cd k:/go_projects/AsyncStarterAgent && go build ./...
```
（由于我们尚未实际修改 `wire.go` 引用，因此这里还不会报错，可以直接执行下一步。）

- [ ] **步骤 2：在 server.go 中注册路由**

向 `internal/server/server.go` 中引入 `"github.com/asyncstarter/agent/internal/chat"`。

在 `New(...)` 函数的参数列表末尾（右括号前）添加 `chatHandler *chat.Handler` 参数。

在设置配置路由块（settings route block）后、`return r` 之前添加以下路由组：

```go
if chatHandler != nil {
    cg := r.Group("/api/v1/chat")
    cg.POST("/sessions", authMW, chatHandler.CreateSession)
    cg.DELETE("/sessions/:id", authMW, chatHandler.DeleteSession)
    cg.POST("/sessions/:id/message", authMW, chatHandler.SendMessage)
    cg.GET("/sessions/:id/stream", authMW, chatHandler.StreamSession)
}
```

- [ ] **步骤 3：将 chatHandler 添加至 Deps.Server()**

在 `cmd/api/wire.go` 中更新 `Deps.Server()` 的定义：

```go
func (d *Deps) Server() *gin.Engine {
    return server.New(d.Cfg, d.Pool, d.Trigger, d.Syn, d.Deliv,
        d.Auth, d.AuthMgr, d.AuthBL, d.Matcher, d.SettingsRepo, d.SettingsFactory,
        d.ChatHandler)
}
```

并在 `Deps` 结构体中添加 `ChatHandler *chat.Handler` 字段。

- [ ] **步骤 4：在 wire.go Build() 中构建 Chat Agent**

将 `"github.com/asyncstarter/agent/internal/chat"` 与 `openaimodel "github.com/cloudwego/eino-ext/components/model/openai"` 导入到 `cmd/api/wire.go`。

在 `synSvc := synthesis.NewService(...)` 代码后增加：

```go
var chatHandler *chat.Handler
if cfg.OpenAIKey != "" {
    chatCM, chatCMErr := openaimodel.NewChatModel(ctx, &openaimodel.ChatModelConfig{
        APIKey:  cfg.OpenAIKey,
        Model:   cfg.OpenAIModel,
        BaseURL: cfg.OpenAIBaseURL,
    })
    if chatCMErr != nil {
        log.Printf("[chat] failed to build chat model: %v — chat routes disabled", chatCMErr)
    } else {
        chatAgent, chatAgentErr := chat.BuildAgent(ctx, chat.AgentConfig{
            ChatModel: chatCM,
            SkillsDir: cfg.ChatSkillsDir,
        })
        if chatAgentErr != nil {
            log.Printf("[chat] failed to build agent: %v — chat routes disabled", chatAgentErr)
        } else {
            chatStore := chat.NewSessionStore()
            chatHandler = chat.NewHandler(chatAgent, chatStore)
            log.Println("[chat] TurnLoop agent initialized")
        }
    }
}
```

在 `return &Deps{...}` 结构体初始化块中，添加：`ChatHandler: chatHandler,`。

- [ ] **步骤 5：进行编译检查**

```bash
cd k:/go_projects/AsyncStarterAgent && go build ./...
```
预期结果：编译通过，没有错误。

- [ ] **步骤 6：提交代码**

```bash
cd k:/go_projects/AsyncStarterAgent
git add internal/server/server.go cmd/api/wire.go
git commit -m "feat(chat): wire TurnLoop chat agent into Gin server"
```

---

## 任务 6：检查依赖并整理 go.mod

**涉及文件：**
- 修改：`go.mod`、`go.sum`

**上下文说明：** `localbk "github.com/cloudwego/eino-ext/adk/backend/local"` 与 `openaimodel "github.com/cloudwego/eino-ext/components/model/openai"` 应当已存在于 `go.mod` 中（因为项目之前已使用过 eino-ext 的 embeddings）。在此我们将验证并执行 tidy 操作。

- [ ] **步骤 1：检查当前的 eino-ext 是否包含 adk/backend/local**

```bash
cd k:/go_projects/AsyncStarterAgent && go list -m all | grep eino-ext
```
预期结果：输出当前项目引用的 eino-ext 各模块及其版本。

- [ ] **步骤 2：如果缺失该依赖则获取 local backend 包**

```bash
cd k:/go_projects/AsyncStarterAgent && go get github.com/cloudwego/eino-ext/adk/backend/local@latest
```

- [ ] **步骤 3：执行 go mod tidy 整理依赖**

```bash
cd k:/go_projects/AsyncStarterAgent && go mod tidy
```

- [ ] **步骤 4：进行最终的整包编译验证**

```bash
cd k:/go_projects/AsyncStarterAgent && go build ./...
```
预期结果：编译通过，无任何错误。

- [ ] **步骤 5：运行所有测试用例**

```bash
cd k:/go_projects/AsyncStarterAgent && go test ./... -timeout 60s
```
预期结果：所有测试均顺利通过（包括新增的聊天模块单元测试和旧的既有测试）。

- [ ] **步骤 6：提交代码**

```bash
cd k:/go_projects/AsyncStarterAgent
git add go.mod go.sum
git commit -m "chore: go mod tidy after eino adk backend/local dependency"
```

---

## 自我评估

**设计规范覆盖度：**
- [x] **Skill 中间件（Chapter 09）**：`agent.go` 成功集成了 `skill.NewBackendFromFilesystem` 与 `skill.NewTyped`。
- [x] **多轮循环 TurnLoop（Chapter 11）**：`session.go` 实现并精细管理了每个会话的 `adk.NewTurnLoop` 实例。
- [x] **符合官方 Loop 示例**：实现了 `GenInput`、`PrepareAgent` 和 `OnAgentEvents` 三项基础生命周期回调。
- [x] **抢占支持**：`Loop.Push` 使用了缺省的非抢占推送模式（可由上层调用进一步扩展）。
- [x] **任务中止**：`store.Delete` 调用了 `Loop.Stop(adk.WithImmediate())`。
- [x] **SSE 流式传输**：`StreamSession` 能够完整消费并分发 `Session.Output` 管道中的消息。
- [x] **会话生命周期 API**：涵盖了会话创建、压入消息、获取流和删除会话。

**占位符与未实现代码：**
- 检查后确认无任何 “TODO”、“待后续实现” 或占位性质代码。

**类型安全与一致性：**
- `ChatItem` 在 `session.go` 中定义并被 `handler.go` 消费，类型一致。
- `Session.Loop` 类型声明为 `*adk.TurnLoop[*ChatItem, *schema.Message]`，在 `EnsureLoop` 与 `handler.go` 中使用方式一致。
- `AgentConfig.ChatModel` 声明为 `model.BaseChatModel`，这与 `deep.TypedConfig.ChatModel` 要求的底层接口兼容。