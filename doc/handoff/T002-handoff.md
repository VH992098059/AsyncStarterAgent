# Phase 0 / T002 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T002 — Gin HTTP server + 中间件 + 统一响应 + 健康检查
> **状态**: ✅ 已完成 / 待用户确认 commit
> **基线 commit**: `aae8a2a` (T001)

---

## 1. 上一窗口做了什么

按 `doc/plans/01-phase0-foundation.md` §Task T002 执行了 12 个 step 中的 1~12（验证 12 由用户决定 commit）。严格 TDD：test → red → impl → green → 手动验证。

### 1.1 新建/修改文件清单

| 文件 | 操作 | 大小 | 说明 |
|---|---|---|---|
| `pkg/httpx/response.go` | 新建 | ~700 B | 统一响应结构 `Response{Code, Message, Data}` + `OK/Fail` 辅助 |
| `internal/handler/health.go` | 新建 | ~400 B | `Health` handler + `HealthResponse{Status, Env}` |
| `internal/handler/health_test.go` | 新建 | ~900 B | TDD 测试（见 §1.4 偏差说明） |
| `internal/middleware/recovery.go` | 新建 | ~450 B | `panic recover` 中间件，log + 500 |
| `internal/middleware/logger.go` | 新建 | ~400 B | 结构化 HTTP 访问日志 |
| `internal/server/server.go` | 新建 | ~600 B | Gin engine 装配（Release mode + Recovery + Logger + env 中间件 + /health 路由） |
| `cmd/api/main.go` | 修改 | 22 行 | 启动 HTTP server（替换 T001 的"打印即退出"） |
| `go.mod` / `go.sum` | 修改 | +34 / +N | 加入 `github.com/gin-gonic/gin v1.12.0` + 27 个 indirect 依赖 |
| `bin/api.exe` | 重新构建 | ~7.5 MB | 集成 gin 后体积增大（已 gitignore） |

### 1.2 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go get github.com/gin-gonic/gin@latest` | ✅ exit 0 | 安装 `gin v1.12.0`（**实际版本**） |
| `go test ./internal/handler/...` (Step 4 red) | ✅ FAIL | `no non-test Go files`（handler.Health 还没写） |
| `go test ./...` (Step 9 green) | ✅ PASS | `ok github.com/asyncstarter/agent/internal/handler` |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go build -o bin/api.exe ./cmd/api` | ✅ exit 0 | 编译通过 |
| `go mod tidy` | ✅ exit 0 | go.sum 整理完毕 |
| `Invoke-WebRequest http://localhost:8080/health` | ✅ `{"code":0,"message":"ok","data":{"status":"ok","env":"development"}}` | 期望输出完全一致 |
| `go list -m github.com/gin-gonic/gin` | ✅ `v1.12.0` | M8 强制 @latest，装到 v1.12.0 |

### 1.3 实际 gin 版本

```
$ go list -m github.com/gin-gonic/gin
github.com/gin-gonic/gin v1.12.0
```

> ⚠️ **与计划偏差**: 计划锁定 `v1.10.0`，但 M8 规则强制 `@latest`，实际安装 `v1.12.0`（2026-06 最新稳定版）。已在 T001-handoff §6 告知用户并经用户确认按 M8 执行。
> v1.12.0 与 v1.10.0 API 完全兼容（`gin.New / gin.SetMode / gin.HandlerFunc / c.JSON / c.AbortWithStatus / r.GET / r.Use` 均无 breaking change）。

### 1.4 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `health_test.go` 把 `resp["status"]` 改为 `data["status"]`（外层多一层 "data" map） | 计划 Step 3 测试代码与 Step 5 实现**自相矛盾**：实现用 `httpx.OK` 会把 health 包到 `Response{code,message,data}` 包装层，测试若按 `resp["status"]` 取必失败。`Step 10` 的期望 curl 输出 `{"code":0,"message":"ok","data":{"status":"ok","env":"development"}}` 也证实了包装层存在。修正后既符合期望 JSON 输出，又对齐 M7「跨文件命名一致」 | ✅ 修正必要（实现是真理） |
| 2 | gin 实际安装 `v1.12.0` 而非计划 `v1.10.0` | M8 强制 `@latest` | ✅ 用户已确认 |
| 3 | `recovery.go` panic 日志格式用 `[panic] %v %s %s` 而 `logger.go` 用 `[http] %d %s %s %v` | 计划原文如此，保留两个不同的前缀便于日志过滤 | ✅ 与计划一致 |
| 4 | `cmd/api/main.go` 用 `log.Fatalf` 处理 config 错误（计划原文） | T001 用 `fmt.Fprintf + os.Exit(1)`，T002 改用 `log.Fatalf`（自动打印时间戳） | ⚠️ 微小风格差异（计划原文如此） |
| 5 | 验证 curl 用 `Invoke-WebRequest -UseBasicParsing` 替代 PowerShell `curl` 别名 | PowerShell 的 `curl` 是 `Invoke-WebRequest` 的别名，会触发 `安全警告` 交互提示（默认 N）。改用 `-UseBasicParsing` 跳过脚本解析警告 | ⚠️ 纯环境适配（不改变验证结论） |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖、未做超出 Phase 0 范围的事、未自动 commit。

---

## 2. 上一窗口**没做**什么

明确列出**未完成**事项，避免下一窗口误以为已就绪：

- ❌ **没有**创建数据库 schema / migration（属于 T003）
- ❌ **没有** Docker / docker-compose（属于 T005）
- ❌ **没有** Redis 队列（属于 T004+）
- ❌ **没有** JWT 中间件（属于 T009，NFR-02）
- ❌ **没有** Swagger / OpenAPI（不属于 Phase 0）
- ❌ **没有** Prometheus metrics（不属于 Phase 0）
- ❌ **没有** CORS / 限流（不属于 Phase 0）
- ❌ **没有** `git add` / `git commit`（按 P1 红线等用户决定）
- ❌ **没有** `internal/handler/health_test.go` 中测试 `env` 字段（因为单元测试没装 server.New 的 env 中间件，`c.GetString("env")` 会返回 `""`；env 字段在集成测试 / curl 验证层覆盖）

---

## 3. 下一窗口需要做的（T003）

### 3.1 T003 目标（来源：plan/01-phase0-foundation.md §Task T003）

> 实现数据模型 (AgentRun/Draft/DataSource/Delivery/Template) + pgx 驱动 + golang-migrate 迁移文件，端到端 `make migrate` 成功。

### 3.2 关键文件（T003 计划原文）

**新建**：
- `internal/store/postgres.go`（pgx 连接池 + 健康检查）
- `migrations/0001_init.up.sql` / `0001_init.down.sql`（5 张表 schema）
- `internal/model/*.go`（5 个领域模型 struct）

**修改**：
- `go.mod`（加 `github.com/jackc/pgx/v5` + `github.com/golang-migrate/migrate/v4`）
- `Makefile`（`migrate-up` / `migrate-down` 目标）

### 3.3 建议执行顺序

T003 计划原文 Step 9-10 要求 docker postgres 跑起来跑迁移——但 T005（docker-compose）尚未执行。建议：
- **方案 A（推荐）**: T005 → T003 → T004
  - 先把 postgres docker 跑起来，再写 migration 即可直接验证
- **方案 B**: 跳过 docker 验证，先只写代码 + `go vet` + 编译（要求用户后续补 docker 验证）

### 3.4 依赖选择

- `github.com/jackc/pgx/v5@latest`（pgx 5.x 是当前主流；不要锁 v4）
- `github.com/golang-migrate/migrate/v4@latest`（注意 v4 的 sub-package 路径）
- 测试库：`github.com/stretchr/testify@latest`（可选，T002 起开始 TDD 后建议引入）

详细计划在 `doc/plans/01-phase0-foundation.md` L515+。

### 3.5 验证命令（执行 T003 后跑这些）

```powershell
cd "k:\go_projects\AsyncStarterAgent"

# 1. 编译
go build ./...

# 2. 跑测试
go test ./...

# 3. 跑迁移（需要 docker postgres）
make migrate-up
psql $env:DATABASE_URL -c "\dt"   # 期望看到 5 张表
make migrate-down
```

---

## 4. 给下一窗口的提示

1. **TDD 不要跳**: 跟 T002 一样的 red → green 节奏。`internal/store/postgres.go` 最好有 `TestNew_InvalidDSN` 之类的失败用例先行。
2. **不要顺手加额外功能**（C1/C2 + R1/R2 红线）。T003 只做 schema + migrate 工具，**不要**加：
   - JWT auth（NFR-02 属于 T009）
   - 业务 CRUD handler（属于 Phase 1）
   - Redis 队列（属于 T004）
3. **migration 文件名遵循 `0001_init.{up,down}.sql` 命名**（golang-migrate 标准），不要用 `001_` 或 `1_`。
4. **不要锁旧版本**: `pgx/v5` 和 `migrate/v4` 都用 `@latest`（M8 规则）。
5. **DSN 已存在于 `config.Config.DSN`**，T003 只需读取 `cfg.DSN` 传给 `pgxpool.New`，不要重新解析 env。
6. **`Makefile` 的 `migrate-up` 命令**: 用 `migrate -path migrations -database "$$DATABASE_URL" up` 这种 shell 形式（PowerShell 也兼容），不要用 `&&` 链式。
7. **测试表前缀**: 不要污染默认 `public` schema，建议每个测试用自己的 schema（`SET search_path TO test_xxx`）。
8. **每次完成一个原子改动后展示 diff + 等用户决定 commit**。不要自动 `git commit`。
9. **完成后**请按本文件 §3 的结构生成下一份交接文档 `doc/handoff/T003-handoff.md`。

---

## 5. 当前文件结构（Phase 0 / T002 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
│   └── api.exe                   (~7.5 MB, T002 验证用)
├── cmd/
│   └── api/
│       └── main.go               ✅ T002 (启动 HTTP server)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md
│   │   └── T002-handoff.md       ← 本文件
│   ├── plans/                    (00~05)
│   ├── mvp-definition.html
│   ├── plan-boundary.md
│   ├── prototype.html
│   ├── requirement-spec.html
│   └── task-tracker.html
├── internal/
│   ├── config/
│   │   └── config.go             ✅ T001 (5 env vars)
│   ├── handler/
│   │   ├── health.go             ✅ T002
│   │   └── health_test.go        ✅ T002
│   ├── middleware/
│   │   ├── logger.go             ✅ T002
│   │   └── recovery.go           ✅ T002
│   └── server/
│       └── server.go             ✅ T002
├── pkg/
│   └── httpx/
│       └── response.go           ✅ T002
├── migrations/                   (空，T003 创建)
├── scripts/                      (空，T005 创建 init-db.sql)
├── test/
│   └── integration/              (空，Phase 末才用)
├── .env.example                  ✅ T001
├── .gitignore                    ✅ T001
├── go.mod                        ✅ T002 (gin v1.12.0 + 27 indirect)
├── go.sum                        ✅ T002
├── Makefile                      ✅ T001
└── README.md                     ✅ T001
```

---

## 6. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **是否 `git add` 当前 7 个新文件 + 2 个修改文件并提交？**
   - 建议命令（**用户在主窗口执行**）：
     ```bash
     cd "k:\go_projects\AsyncStarterAgent"
     git add pkg/httpx/response.go \
             internal/handler/health.go \
             internal/handler/health_test.go \
             internal/middleware/recovery.go \
             internal/middleware/logger.go \
             internal/server/server.go \
             cmd/api/main.go \
             go.mod \
             go.sum
     git commit -m "feat(phase0/T002): gin http server + middleware + /health endpoint

     - Add gin v1.12.0 dependency
     - Implement unified Response{code,message,data} in pkg/httpx
     - Add /health handler with status+env response
     - Add Recovery + Logger middleware
     - Wire server.New(cfg) in main, replacing print-and-exit
     - Manual verify: curl /health returns expected JSON"
     ```
   - 如果用户说 NO，则保持当前未提交状态。

2. **是否同意 T003 按"方案 A"（T005 → T003 → T004）执行**？还是按"方案 B"先写代码、跳过 docker 验证？

3. **T003 引入的依赖**是否仍按 `@latest` 原则（pgx/v5 + migrate/v4）？

> **下一窗口开场建议**：
> 1. 读 `doc/handoff/T002-handoff.md`（本文件）
> 2. 读 `doc/handoff/T001-handoff.md`（上下文）
> 3. 读 `doc/plans/01-phase0-foundation.md` §T003（L515+）
> 4. 用 `AskUserQuestion` 问上面 §6 三个决定
> 5. 获答复后按 TDD 执行 T003
