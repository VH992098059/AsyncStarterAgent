# Phase 0: 基础设施（W1-W2）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **AI 行为约束**: 严格遵守 [ai-coding-boundary.md](../../ai-coding-boundary.md)。任何偏离需先询问。

**Goal**: 搭建可编译运行的 Go 后端骨架 + 数据库 schema + Redis 队列 + Docker 环境，端到端 `/health` 200。

**关联需求**: NFR-02 (认证), NFR-03 (可用性), NFR-04 (可扩展性), 数据模型 (AgentRun/Draft/DataSource/Delivery/Template)

**退出标准（M0）**:
- [ ] `go build ./...` 无错误
- [ ] `docker compose up` 一键启动 PostgreSQL + Redis + API
- [ ] `curl http://localhost:8080/health` 返回 200
- [ ] `make migrate` 执行 schema 迁移成功
- [ ] `make test` 全部测试通过

---

## 目录结构 (Phase 0 建立)

```
AsyncStarterAgent/
├── cmd/
│   └── api/main.go              # API 服务入口
├── internal/
│   ├── config/config.go         # 配置加载
│   ├── handler/health.go        # 健康检查 handler
│   ├── middleware/              # 中间件
│   │   ├── logger.go
│   │   └── recovery.go
│   └── server/server.go         # Gin server 装配
├── pkg/
│   └── httpx/response.go        # 统一响应结构
├── migrations/
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── test/
│   └── integration/
│       └── health_test.go
├── .env.example
├── .gitignore
├── .golangci.yml
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── Makefile
└── README.md
```

---

## Task T001: Go 项目初始化与目录结构

**Files:**
- Create: `go.mod`
- Create: `cmd/api/main.go`
- Create: `internal/config/config.go`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `.env.example`
- Create: `README.md`

**关联**: 基础设施

- [ ] **Step 1: 验证 Go 环境**

Run: `go version`
Expected: 输出 `go version go1.22.x` 或更高。若低于 1.22，先安装 Go 1.22+（不自动安装，询问用户）。

- [ ] **Step 2: 初始化 Go module**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
go mod init github.com/asyncstarter/agent
```
Expected: 创建 `go.mod` 文件，包含 `module github.com/asyncstarter/agent` 和 `go 1.22`

- [ ] **Step 3: 创建目录结构**

Run:
```bash
mkdir -p cmd/api internal/config internal/handler internal/middleware internal/server pkg/httpx migrations test/integration scripts
```
Expected: 所有目录创建完成（PowerShell 中用 `New-Item -ItemType Directory -Force` 替代）

- [ ] **Step 4: 写最小 main.go**

Create file `cmd/api/main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/asyncstarter/agent/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("AsyncStarterAgent API starting on :%s (env=%s)\n", cfg.Port, cfg.Env)
}
```

- [ ] **Step 5: 写配置加载**

Create file `internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env  string
	Port string
	DSN  string
	RedisURL string
	JWTSecret string
}

func Load() (*Config, error) {
	env := getEnv("APP_ENV", "development")
	port := getEnv("APP_PORT", "8080")
	dsn := getEnv("DATABASE_URL", "")
	redis := getEnv("REDIS_URL", "redis://localhost:6379/0")
	jwt := getEnv("JWT_SECRET", "")

	if env == "production" && jwt == "" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}

	return &Config{
		Env: env,
		Port: port,
		DSN: dsn,
		RedisURL: redis,
		JWTSecret: jwt,
	}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 6: 写 .env.example**

Create file `.env.example`:
```env
APP_ENV=development
APP_PORT=8080
DATABASE_URL=postgres://starter:starter@localhost:5432/starter?sslmode=disable
REDIS_URL=redis://localhost:6379/0
JWT_SECRET=change-me-in-production
LOG_LEVEL=info
```

- [ ] **Step 7: 写 .gitignore**

Create file `.gitignore`:
```
# Binaries
/bin/
/dist/
*.exe
*.dll
*.so
*.dylib

# Test binary
*.test
*.out

# Go workspace
go.work
go.work.sum

# Env
.env
.env.local

# IDE
.idea/
.vscode/
*.swp

# Coverage
coverage.html
coverage.txt
```

- [ ] **Step 8: 写 Makefile**

Create file `Makefile`:
```makefile
.PHONY: help build run test test-integration lint migrate-up migrate-down docker-up docker-down clean

GO ?= go
APP := bin/api

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-20s %s\n", $$1, $$2}'

build: ## 编译二进制
	$(GO) build -o $(APP) ./cmd/api

run: ## 本地运行
	$(GO) run ./cmd/api

test: ## 单元测试
	$(GO) test -race -short ./...

test-integration: ## 集成测试（需 docker compose up）
	$(GO) test -race -tags=integration ./test/integration/...

lint: ## golangci-lint
	golangci-lint run ./...

migrate-up: ## 数据库迁移
	@echo "TODO: integrate golang-migrate in T003"

docker-up: ## 启动 docker compose
	docker compose up -d

docker-down: ## 停止 docker compose
	docker compose down

clean: ## 清理
	rm -rf bin/ coverage.html coverage.txt
```

- [ ] **Step 9: 写 README**

Create file `README.md`:
```markdown
# AsyncStarterAgent

"完成前 30%" 异步行动起跑器 Agent — 后端服务。

## 快速开始

\`\`\`bash
make docker-up    # 启动 PostgreSQL + Redis
make build        # 编译
make run          # 运行
\`\`\`

健康检查: \`curl http://localhost:8080/health\`

## 文档

- 需求: \`doc/requirement-spec.html\`
- MVP 定义: \`doc/mvp-definition.html\`
- 计划: \`doc/plans/00-index.md\`
- AI 编码规范: \`doc/ai-coding-boundary.md\`
```

- [ ] **Step 10: 编译验证**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build ./cmd/api`
Expected: 无错误。若有错误，按编译器提示修复。

- [ ] **Step 11: 运行验证**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
APP_PORT=8080 go run ./cmd/api
```
Expected: 输出 `AsyncStarterAgent API starting on :8080 (env=development)`，Ctrl+C 退出。

- [ ] **Step 12: 展示 diff 并等待用户决定是否 commit**

按 ai-coding-boundary P1：展示所有新增文件清单，等用户确认。

---

## Task T002: Gin 框架搭建与中间件配置

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/handler/health.go`
- Create: `internal/middleware/logger.go`
- Create: `internal/middleware/recovery.go`
- Create: `pkg/httpx/response.go`
- Modify: `cmd/api/main.go`
- Test: `internal/handler/health_test.go`

**关联**: NFR-02, NFR-04

- [ ] **Step 1: 添加 Gin 依赖**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
go get github.com/gin-gonic/gin@v1.10.0
```
Expected: `go.mod` 增加 `gin v1.10.0` 依赖。

- [ ] **Step 2: 写 httpx 统一响应**

Create file `pkg/httpx/response.go`:
```go
package httpx

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus int, code int, msg string) {
	c.AbortWithStatusJSON(httpStatus, Response{Code: code, Message: msg})
}
```

- [ ] **Step 3: 写健康检查 handler（先写测试）**

Create file `internal/handler/health_test.go`:
```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/handler"
	"github.com/gin-gonic/gin"
)

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", handler.Health)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", resp["status"])
	}
}
```

- [ ] **Step 4: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/handler/...`
Expected: FAIL `handler.Health undefined`

- [ ] **Step 5: 写健康检查实现**

Create file `internal/handler/health.go`:
```go
package handler

import (
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type HealthResponse struct {
	Status string `json:"status"`
	Env    string `json:"env"`
}

func Health(c *gin.Context) {
	httpx.OK(c, HealthResponse{Status: "ok", Env: c.GetString("env")})
}
```

- [ ] **Step 6: 写 recovery 中间件**

Create file `internal/middleware/recovery.go`:
```go
package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[panic] %v %s %s", err, c.Request.Method, c.Request.URL.Path)
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[http] %d %s %s %v", c.Writer.Status(), c.Request.Method, c.Request.URL.Path, time.Since(start))
	}
}
```

- [ ] **Step 7: 写 server 装配**

Create file `internal/server/server.go`:
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
	return r
}
```

- [ ] **Step 8: 修改 main.go 启动 server**

Modify `cmd/api/main.go`:
```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	addr := ":" + cfg.Port
	fmt.Printf("AsyncStarterAgent API starting on %s (env=%s)\n", addr, cfg.Env)
	if err := server.New(cfg).Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 9: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS, 输出 `ok  github.com/asyncstarter/agent/internal/handler`

- [ ] **Step 10: 手动验证 /health**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
APP_PORT=8080 go run ./cmd/api &
sleep 2
curl http://localhost:8080/health
```
Expected: `{"code":0,"message":"ok","data":{"status":"ok","env":"development"}}`

- [ ] **Step 11: 展示 diff 等用户决定是否 commit**

---

## Task T003: 数据库 Schema 设计与迁移

**Files:**
- Create: `migrations/0001_init.up.sql`
- Create: `migrations/0001_init.down.sql`
- Create: `internal/repository/db.go`
- Test: `internal/repository/db_test.go`
- Modify: `internal/config/config.go`
- Modify: `Makefile`

**关联**: requirement-spec.html §7 数据模型设计 (AgentRun/Draft/DataSource/Delivery/Template), FR-B06 增量同步

- [ ] **Step 1: 添加数据库驱动依赖**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
go get github.com/jackc/pgx/v5@v5.5.5
go get github.com/golang-migrate/migrate/v4@v4.17.1
```
Expected: `go.mod` 增加这两个依赖。

- [ ] **Step 2: 写迁移 up 脚本**

Create file `migrations/0001_init.up.sql`:
```sql
-- 启用 pgvector 扩展（Phase 3 使用，Phase 0 预先装好）
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

-- AgentRun: 单次端到端执行实例（模块 A → D 编排状态）
CREATE TABLE agent_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    task_type       VARCHAR(32) NOT NULL,  -- weekly_report / summary / plan / meeting_minutes
    status          VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending / running / completed / failed / cancelled
    current_stage   VARCHAR(16) NOT NULL DEFAULT 'ingestion', -- ingestion / harvesting / synthesis / delivery
    trigger_type    VARCHAR(16) NOT NULL,  -- keyword / ddl / webhook / manual
    trigger_source  TEXT,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);
CREATE INDEX idx_agent_runs_user_status ON agent_runs(user_id, status);
CREATE INDEX idx_agent_runs_created_at ON agent_runs(created_at DESC);

-- Draft: LLM 生成的草稿
CREATE TABLE drafts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_run_id    UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    title           VARCHAR(255) NOT NULL,
    content         JSONB,                  -- 富文本内容
    markdown_content TEXT,                  -- Markdown 源
    completeness    REAL NOT NULL DEFAULT 0.0, -- 0.0 - 1.0
    marks           JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{id, hint, position, resolved}]
    status          VARCHAR(16) NOT NULL DEFAULT 'draft', -- draft / reviewed / delivered
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_drafts_run_id ON drafts(agent_run_id);
CREATE INDEX idx_drafts_status ON drafts(status);

-- DataSource: 第三方数据源配置
CREATE TABLE data_sources (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    type            VARCHAR(32) NOT NULL,  -- github / google_calendar / outlook / feishu / slack / notion / obsidian
    name            VARCHAR(128) NOT NULL,
    config          TEXT,                   -- 加密的 JSON
    status          VARCHAR(16) NOT NULL DEFAULT 'disconnected', -- disconnected / connected / error
    last_sync_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, type)
);
CREATE INDEX idx_data_sources_user ON data_sources(user_id);

-- Delivery: 草稿交付记录
CREATE TABLE deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draft_id        UUID NOT NULL REFERENCES drafts(id) ON DELETE CASCADE,
    target_type     VARCHAR(16) NOT NULL,  -- notion / obsidian / feishu
    target_url      TEXT,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending / success / failed
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_deliveries_draft ON deliveries(draft_id);

-- Template: 文档模板
CREATE TABLE templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(128) NOT NULL,
    type            VARCHAR(32) NOT NULL,  -- weekly_report / summary / plan / meeting_minutes / general
    content_markdown TEXT NOT NULL,
    category        VARCHAR(64),
    is_default      BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_templates_type ON templates(type);

-- 同步时间戳表（FR-B06 增量同步）
CREATE TABLE sync_timestamps (
    data_source_id  UUID PRIMARY KEY REFERENCES data_sources(id) ON DELETE CASCADE,
    last_sync_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 3: 写迁移 down 脚本**

Create file `migrations/0001_init.down.sql`:
```sql
DROP TABLE IF EXISTS sync_timestamps;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS data_sources;
DROP TABLE IF EXISTS drafts;
DROP TABLE IF EXISTS agent_runs;
```

- [ ] **Step 4: 写 db 仓库测试**

Create file `internal/repository/db_test.go`:
```go
package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/asyncstarter/agent/internal/repository"
)

func TestDBConnect_RequiresDSN(t *testing.T) {
	os.Setenv("DATABASE_URL", "")
	_, err := repository.Open(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when DSN is empty")
	}
}
```

- [ ] **Step 5: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/repository/...`
Expected: FAIL `repository.Open undefined`

- [ ] **Step 6: 写 db 仓库实现**

Create file `internal/repository/db.go`:
```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
```

- [ ] **Step 7: 修改 config 支持数据库 URL**

Modify `internal/config/config.go` - 已在 T001 中定义 `DSN` 字段，无需修改。

- [ ] **Step 8: 更新 Makefile migrate 目标**

Modify `Makefile` - 替换 `migrate-up` 和 `migrate-down`:
```makefile
MIGRATE := github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1

migrate-up: ## 应用迁移
	$(GO) run -tags 'postgres' $(MIGRATE) -database "$(DATABASE_URL)" -path ./migrations up

migrate-down: ## 回滚迁移
	$(GO) run -tags 'postgres' $(MIGRATE) -database "$(DATABASE_URL)" -path ./migrations down 1
```
并在 help 段保持 `DATABASE_URL` 默认值：
```makefile
DATABASE_URL ?= postgres://starter:starter@localhost:5432/starter?sslmode=disable
```

- [ ] **Step 9: 启动 docker postgres（依赖 T005）**

> 边界说明：T005 完成后才能跑迁移。本步仅在 Docker 启动后执行。
> 若 T005 尚未完成，**跳过 Step 9-10，等 T005 完成再回来执行**。

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
docker compose up -d postgres
```

- [ ] **Step 10: 执行迁移**

Run: `cd "k:\go_projects\AsyncStarterAgent" && make migrate-up`
Expected: 6 张表创建成功。

- [ ] **Step 11: 验证表结构**

Run:
```bash
docker exec -it asyncstarter_postgres psql -U starter -d starter -c "\dt"
```
Expected: 输出 `agent_runs`, `drafts`, `data_sources`, `deliveries`, `templates`, `sync_timestamps`

- [ ] **Step 12: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 13: 展示 diff 等用户决定**

---

## Task T004: Redis 任务队列集成

**Files:**
- Create: `internal/queue/queue.go`
- Create: `internal/queue/queue_test.go`
- Create: `internal/queue/handler.go`
- Test: `internal/queue/queue_test.go` (同源)

**关联**: NFR-04 可扩展性 (AgentRun 任务队列分布式消费)

- [ ] **Step 1: 添加 Asynq 依赖**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
go get github.com/hibiken/asynq@v0.24.1
```
Expected: `go.mod` 增加 asynq v0.24.1

- [ ] **Step 2: 写队列测试**

Create file `internal/queue/queue_test.go`:
```go
package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/queue"
)

func TestClientAndServer_EmptyURL(t *testing.T) {
	_, err := queue.NewClient("")
	if err == nil {
		t.Fatal("expected error for empty redis url")
	}
}

func TestEnqueueAndHandle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires redis")
	}
	redisURL := "redis://localhost:6379/0"
	c, err := queue.NewClient(redisURL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer c.Close()

	handled := make(chan string, 1)
	mux := queue.NewMux()
	mux.HandleFunc("ping", func(ctx context.Context, payload string) error {
		handled <- payload
		return nil
	})

	srv, err := queue.NewServer(redisURL, mux, 1)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	go func() { _ = srv.Start() }()
	defer srv.Stop()

	if err := c.Enqueue(context.Background(), "ping", "hello", 1*time.Minute); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case got := <-handled:
		if got != "hello" {
			t.Fatalf("expected hello, got %s", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler not invoked within 3s")
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/queue/...`
Expected: FAIL `queue.NewClient undefined`

- [ ] **Step 4: 写 Client**

Create file `internal/queue/queue.go`:
```go
package queue

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

type Client struct {
	cli *asynq.Client
}

func NewClient(redisURL string) (*Client, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	cli := asynq.NewClient(asynq.RedisClientOpt{URL: redisURL})
	return &Client{cli: cli}, nil
}

func (c *Client) Close() { _ = c.cli.Close() }

func (c *Client) Enqueue(ctx context.Context, taskType string, payload string, ttl time.Duration) error {
	t := asynq.NewTask(taskType, []byte(payload), asynq.MaxRetry(3), asynq.Timeout(ttl))
	_, err := c.cli.EnqueueContext(ctx, t)
	return err
}
```

- [ ] **Step 5: 写 Handler/Server**

Create file `internal/queue/handler.go`:
```go
package queue

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

type HandlerFunc func(ctx context.Context, payload string) error

type Mux struct{ m *asynq.ServeMux }

func NewMux() *Mux { return &Mux{m: asynq.NewServeMux()} }

func (m *Mux) HandleFunc(taskType string, h HandlerFunc) {
	m.m.HandleFunc(taskType, func(c *asynq.Task) error {
		return h(context.Background(), string(c.Payload()))
	})
}

type Server struct{ s *asynq.Server }

func NewServer(redisURL string, mux *Mux, concurrency int) (*Server, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	s := asynq.NewServer(asynq.RedisClientOpt{URL: redisURL}, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{"default": 5},
	})
	return &Server{s: s}, nil
}

func (s *Server) Start() error  { return s.s.Start((*asynq.ServeMux)(nil).With) }
// 上面 Start 实现有误，修正如下（覆盖上面）：
```

> ⚠️ **修正**: 上面 Start 函数调用有误，应使用 mux.m 注入。重新覆盖。

Replace the entire `internal/queue/handler.go` with:
```go
package queue

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

type HandlerFunc func(ctx context.Context, payload string) error

type Mux struct{ m *asynq.ServeMux }

func NewMux() *Mux { return &Mux{m: asynq.NewServeMux()} }

func (m *Mux) HandleFunc(taskType string, h HandlerFunc) {
	m.m.HandleFunc(taskType, func(c *asynq.Task) error {
		return h(context.Background(), string(c.Payload()))
	})
}

type Server struct {
	srv  *asynq.Server
	mux  *asynq.ServeMux
}

func NewServer(redisURL string, mux *Mux, concurrency int) (*Server, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	srv := asynq.NewServer(asynq.RedisClientOpt{URL: redisURL}, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{"default": 5},
	})
	return &Server{srv: srv, mux: mux.m}, nil
}

func (s *Server) Start() error  { return s.srv.Start(s.mux) }
func (s *Server) Stop()         { s.srv.Stop() }
func (s *Server) Shutdown()     { s.srv.Shutdown() }
```

- [ ] **Step 6: 跑单元测试（不依赖 redis）**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test -short ./internal/queue/...`
Expected: PASS（TestClientAndServer_EmptyURL 通过，TestEnqueueAndHandle 跳过）

- [ ] **Step 7: 启动 redis（依赖 T005）**

> 边界说明：T005 完成后执行。

Run: `cd "k:\go_projects\AsyncStarterAgent" && docker compose up -d redis`

- [ ] **Step 8: 跑集成测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/queue/...`
Expected: PASS（TestEnqueueAndHandle 也通过）

- [ ] **Step 9: 展示 diff 等用户决定**

---

## Task T005: Docker 开发环境配置

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `.dockerignore`
- Create: `scripts/init-db.sql`
- Modify: `Makefile`

**关联**: NFR-03 可用性, T003/T004 依赖

- [ ] **Step 1: 写 Dockerfile（多阶段构建）**

Create file `Dockerfile`:
```dockerfile
# Stage 1: build
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

# Stage 2: runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/api"]
```

- [ ] **Step 2: 写 docker-compose.yml**

Create file `docker-compose.yml`:
```yaml
services:
  postgres:
    image: pgvector/pgvector:pg16
    container_name: asyncstarter_postgres
    environment:
      POSTGRES_USER: starter
      POSTGRES_PASSWORD: starter
      POSTGRES_DB: starter
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U starter"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: asyncstarter_redis
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  api:
    build: .
    container_name: asyncstarter_api
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      APP_ENV: development
      APP_PORT: "8080"
      DATABASE_URL: postgres://starter:starter@postgres:5432/starter?sslmode=disable
      REDIS_URL: redis://redis:6379/0
    ports:
      - "8080:8080"

volumes:
  pgdata:
```

- [ ] **Step 3: 写 .dockerignore**

Create file `.dockerignore`:
```
.git
.idea
.vscode
bin
coverage.*
*.log
.env
.env.local
test
docs
```

- [ ] **Step 4: 写 init-db.sql（启用扩展）**

Create file `scripts/init-db.sql`:
```sql
-- 数据库初始化：启用后续 Phase 需要的扩展
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;
```

- [ ] **Step 5: 更新 Makefile 添加 docker-build 目标**

在 `Makefile` 末尾添加：
```makefile
docker-build: ## 构建 Docker 镜像
	docker build -t asyncstarter/api:dev .

docker-logs: ## 查看 API 日志
	docker compose logs -f api
```

- [ ] **Step 6: 启动完整环境**

Run: `cd "k:\go_projects\AsyncStarterAgent" && docker compose up -d`
Expected: 3 个容器（postgres/redis/api）都 running，状态 healthy。

- [ ] **Step 7: 验证 /health**

Run: `curl http://localhost:8080/health`
Expected: `{"code":0,"message":"ok","data":{"status":"ok","env":"development"}}`

- [ ] **Step 8: 跑迁移**

Run: `cd "k:\go_projects\AsyncStarterAgent" && make migrate-up`
Expected: 6 张表创建成功。

- [ ] **Step 9: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 10: 端到端冒烟**

Run:
```bash
docker exec asyncstarter_postgres psql -U starter -d starter -c "SELECT count(*) FROM agent_runs;"
```
Expected: `0`（表已创建但为空）

- [ ] **Step 11: 展示 diff 等用户决定**

---

## Phase 0 退出标准验证

完成 T001-T005 后，逐项验证 M0 退出标准：

- [ ] `cd "k:\go_projects\AsyncStarterAgent" && go build ./...` → 0 错误
- [ ] `docker compose ps` → 3 容器 healthy
- [ ] `curl http://localhost:8080/health` → 200
- [ ] `make migrate-up` → 6 张表存在
- [ ] `make test` → 全部 PASS
- [ ] 更新 [task-tracker.html](../../task-tracker.html) 中 T001-T005 状态为"已完成"

---

**下一步**: 进入 [02-phase1-trigger.md](02-phase1-trigger.md) 执行触发引擎（T006-T009）。
