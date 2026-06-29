# Phase 0 / T001 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T001 — Go 项目初始化与目录结构
> **状态**: ✅ 已完成 / 待用户确认 commit

---

## 1. 上一窗口做了什么

按 `doc/plans/01-phase0-foundation.md` 的 T001 定义执行了 12 个 step 中的核心 step 2/3/4/5/6/7/8/9/10/11（验证 1 已通过 / 验证 12 由用户决定 commit）。

### 1.1 创建/修改的文件清单

| 文件 | 操作 | 大小 | 说明 |
|---|---|---|---|
| `go.mod` | 新建 | 48 B | `module github.com/asyncstarter/agent` + `go 1.25.5` |
| `cmd/api/main.go` | 新建 | 306 B | 最小入口，调用 `config.Load()` 打印启动信息 |
| `internal/config/config.go` | 新建 | 872 B | 集中管理配置，5 个 env var，生产环境强制要求 `JWT_SECRET` |
| `.env.example` | 新建 | 198 B | 6 个环境变量样例 |
| `.gitignore` | 新建 | 204 B | 排除 bin/dist/、.env、IDE、coverage |
| `Makefile` | 新建 | 1144 B | `build / run / test / lint / migrate-* / docker-* / clean` 目标 |
| `README.md` | 新建 | 712 B | 快速开始 + 文档导航 + Phase 0 任务清单 |
| 目录骨架 | 新建 | — | `cmd/api/ internal/{config,handler,middleware,server}/ pkg/httpx/ migrations/ test/integration/ scripts/` |
| `bin/api.exe` | 构建产物 | ~2.3 MB | `go build` 验证产出（已加入 .gitignore） |

### 1.2 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go version` | ✅ `go1.25.5 windows/amd64` | ≥ 1.22 满足计划要求 |
| `go mod init github.com/asyncstarter/agent` | ✅ exit 0 | 写出 `go.mod` |
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go build -o bin/api.exe ./cmd/api` | ✅ exit 0 | 二进制 2.3 MB |
| `APP_PORT=8080 ./bin/api.exe` | ✅ 输出 `AsyncStarterAgent API starting on :8080 (env=development)` 后 exit 0 | T001 的 main 打印消息即退出（T002 才起 HTTP server） |
| `go test ./...` | ⏭️ 无测试文件 | T001 不需要测试，T002 起开始 TDD |

### 1.3 与计划的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | Go 版本是 1.25.5（计划写 1.22+） | 系统已装更高版本 | ✅ 合规（≥1.22） |
| 2 | `go build` 默认产 `bin/api.exe` 而非 `bin/api` | Windows 平台自动加 `.exe` | ✅ 合规 |
| 3 | `Makefile` 的 `help` 用 PowerShell 兼容的 `echo` 而非 `awk` | Windows 原生不支持 `awk` | ✅ 合规（不破坏功能） |
| 4 | `internal/config/config.go` 加了 3 行中文 doc 注释 | 提高可读性 | ⚠️ 微小扩展（不违反 C1/C2） |
| 5 | `README.md` 末尾加了"状态"小节 | 方便追踪 Phase 0 进度 | ⚠️ 微小扩展（不违反规范） |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未加未声明依赖、未越界 FR、未做超出 Phase 0 范围的事。

---

## 2. 上一窗口**没做**什么

明确列出**未完成**事项，避免下一窗口误以为已就绪：

- ❌ **没有**安装任何第三方依赖（`go.sum` 为空，go.mod 只有 module + go directive）
- ❌ **没有**起 HTTP server（`main.go` 只是打印信息；T002 才接入 Gin）
- ❌ **没有**写测试文件（T001 计划无测试；T002 起 TDD）
- ❌ **没有**创建 Docker / docker-compose（属于 T005）
- ❌ **没有**创建 migration SQL（属于 T003）
- ❌ **没有** `git init` / `git commit`（按 ai-coding-boundary P1 禁止未经用户允许提交）

---

## 3. 下一窗口需要做的（T002）

### 3.1 T002 目标（来源：plan/01-phase0-foundation.md §Task T002）

> 实现 Gin 框架 + 中间件 + 统一响应结构 + 健康检查 handler，**端到端** `/health` 返回 200。

### 3.2 完整步骤清单

| Step | 内容 | 必做 |
|---|---|---|
| 1 | `go get github.com/gin-gonic/gin@v1.10.0` | ✅ |
| 2 | 创建 `pkg/httpx/response.go`（统一 `Response{code,message,data}` + `OK/Fail` 辅助） | ✅ |
| 3 | **TDD**: 先写 `internal/handler/health_test.go`（断言 `/health` 返回 200 且 `status=ok`） | ✅ |
| 4 | 跑测试确认失败（`undefined: handler.Health`） | ✅ |
| 5 | 写 `internal/handler/health.go`（实现 `Health` 函数） | ✅ |
| 6 | 写 `internal/middleware/recovery.go` + `logger.go` | ✅ |
| 7 | 写 `internal/server/server.go`（装配 Gin engine + 中间件 + 路由） | ✅ |
| 8 | 修改 `cmd/api/main.go`，用 `server.New(cfg).Run(addr)` 替代原来的打印 | ✅ |
| 9 | 跑测试 `go test ./...` 全过 | ✅ |
| 10 | **手动验证** `curl http://localhost:8080/health` → 200 + JSON | ✅ |
| 11 | 展示 diff 给用户，**等用户决定是否 commit** | ✅ |

### 3.3 T002 涉及文件

**新建**：
- `pkg/httpx/response.go`
- `internal/handler/health.go`
- `internal/handler/health_test.go`
- `internal/middleware/recovery.go`
- `internal/middleware/logger.go`
- `internal/server/server.go`

**修改**：
- `cmd/api/main.go`（接入 server 启动）
- `go.mod` / `go.sum`（加 gin 依赖）

### 3.4 关键参考

- **计划原文**: [doc/plans/01-phase0-foundation.md §Task T002](file:///k:/go_projects/AsyncStarterAgent/doc/plans/01-phase0-foundation.md) （L290–L512）
- **行为规范**: [doc/ai-coding-boundary.md](file:///k:/go_projects/AsyncStarterAgent/doc/ai-coding-boundary.md)
- **依赖**: 计划锁定 `gin v1.10.0`，不要换成其他版本（除非先问）

### 3.5 验证命令（执行 T002 后跑这些）

```powershell
cd "k:\go_projects\AsyncStarterAgent"

# 1. 编译
go build ./...

# 2. 跑测试
go test ./...

# 3. 启动 + 验证
$env:APP_PORT="8080"
Start-Process -FilePath ".\bin\api.exe" -PassThru
Start-Sleep -Seconds 2
curl http://localhost:8080/health
# 期望: {"code":0,"message":"ok","data":{"status":"ok","env":"development"}}
Get-Process api | Stop-Process -Force
```

---

## 4. 给下一窗口的提示

1. **TDD 不要跳**: Step 3 写测试 → Step 4 跑失败 → Step 5 写实现 → Step 9 跑通过。
2. **不要顺手加额外功能**（C1/C2 + R1/R2 红线）。比如别在 T002 加 Swagger、别加 Prometheus、别加 JWT 中间件（NFR-02 但 Phase 0 末才要求）。T009 末才整合。
3. **不熟悉的字段先 grep** `requirement-spec.html` 和 `mvp-definition.html`，看是否在 FR/NFR 范围。
4. **碰到规范空白**用 `AskUserQuestion` 暂停（按 ai-coding-boundary §3）。
5. **`go test -short ./...`** 默认会跳过集成测试（T004+ 才有），本阶段全过即可。
6. **每次完成一个原子改动后展示 diff + 等用户决定 commit**。不要自动 `git commit`。
7. **完成后**请按本文件 §3 的结构生成下一份交接文档 `doc/handoff/T002-handoff.md`。

---

## 5. 当前文件结构（Phase 0 / T001 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
│   └── api.exe                   (2.3 MB, T001 验证用)
├── cmd/
│   └── api/
│       └── main.go               ✅ T001
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/                  🆕 交接文档目录
│   │   └── T001-handoff.md       ← 本文件
│   ├── plans/                    (00~05)
│   ├── mvp-definition.html
│   ├── plan-boundary.md
│   ├── prototype.html
│   ├── requirement-spec.html
│   └── task-tracker.html
├── internal/
│   ├── config/
│   │   └── config.go             ✅ T001
│   ├── handler/                  (空，T002 创建 health.go)
│   ├── middleware/               (空，T002 创建 recovery/logger)
│   └── server/                   (空，T002 创建 server.go)
├── pkg/
│   └── httpx/                    (空，T002 创建 response.go)
├── migrations/                   (空，T003 创建)
├── scripts/                      (空，T005 创建 init-db.sql)
├── test/
│   └── integration/              (空，Phase 末才用)
├── .env.example                  ✅ T001
├── .gitignore                    ✅ T001
├── go.mod                        ✅ T001 (module + go 1.25.5)
├── Makefile                      ✅ T001
└── README.md                     ✅ T001
```

---

## 6. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **是否 `git init` 并把当前 6 个新文件 + go.mod 提交为 `chore: init go project`？**
   - 提交命令建议：`git init && git add . && git commit -m "chore(phase0/T001): init go project structure"`
   - 如果用户说 NO，则保持当前未版本化状态。

2. **M8 规则已强制使用 `@latest`**：T002 Step 1 应执行 `go get github.com/gin-gonic/gin@latest`（**而非**计划写的 `@v1.10.0`）。M8 vs 计划冲突时 M8 优先。是否同意按 M8 执行？

3. **是否同意 T002 末把 `cmd/api/main.go` 的"打印即退出"改成"启动 HTTP server"？** 这是计划的标准动作（ai-coding-boundary §7.1 ✅），但 T001 写下的 main.go 在 T002 会被修改，按 §3.3 应先确认。

> **下一窗口开场建议**：
> 1. 读 `doc/handoff/T001-handoff.md`（本文件）
> 2. 读 `doc/plans/01-phase0-foundation.md` §T002（L290–L512）
> 3. 用 `AskUserQuestion` 问上面 §6 三个决定
> 4. 获答复后按 TDD 执行 T002
