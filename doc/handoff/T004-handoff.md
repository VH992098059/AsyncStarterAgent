# Phase 0 / T004 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T004 — Redis 任务队列（asynq）基础设施
> **状态**: ⚠️ **DONE_WITH_CONCERNS** — 单元测试 + 全套 vet/build/tidy 全过；**集成测试因本地无 redis 暂未跑通（BLOCKED on 用户启动 redis）**
> **基线 commit**: `6f9c05e` (T003)

---

## 1. 上一窗口做了什么

按 `doc/plans/01-phase0-foundation.md` §Task T004 (L753–L978) 执行了 11 个 step 中的 1–6、9–11。**Step 7–8 主动跳过**（不启动 docker / Windows service，按 M8 范围约束）。

严格 TDD：test → red（确认 FAIL）→ impl → green（确认 PASS）→ vet/build/tidy 验证。

### 1.1 新建/修改文件清单

| 文件 | 操作 | 大小 | 说明 |
|---|---|---|---|
| `internal/queue/queue.go` | 新建 | ~50 行 | `Client`（asynq Producer 封装）+ `NewClient` + `Close` + `Enqueue`；**采用 `asynq.ParseRedisURI`（v0.26.0 API）** |
| `internal/queue/handler.go` | 新建 | ~60 行 | `HandlerFunc` / `Mux` / `NewMux` / `HandleFunc` / `Server` / `NewServer` / `Start` / `Stop` / `Shutdown`（采用 plan §L917–L957 修正版，**适配 v0.26.0 的 `HandleFunc(ctx, *Task)` 签名**） |
| `internal/queue/queue_test.go` | 新建 | ~17 行 | TDD 单元测试：`TestClientAndServer_EmptyURL`（空 redisURL 必须返回 error，不依赖 redis） |
| `internal/queue/queue_integration_test.go` | 新建 | ~40 行 | 集成测试：`TestEnqueueAndHandle`（端到端 Enqueue→Server→Handler，**带 `//go:build integration` tag**，本地有 redis 时用 `go test -tags=integration` 跑） |
| `go.mod` | 修改 | +1 direct + 5 indirect | 加 `github.com/hibiken/asynq v0.26.0` 为 direct；`github.com/redis/go-redis/v9 v9.14.1` / `github.com/robfig/cron/v3 v3.0.1` / `golang.org/x/time v0.14.0` / `github.com/google/uuid v1.6.0` 等为 indirect（v0.26.0 已从 redigo 迁移到 go-redis/v9） |
| `go.sum` | 修改 | +30 | 同步锁定依赖哈希 |
| `doc/handoff/T004-handoff.md` | 新建 | — | 本文件 |

> **新增 4 个文件 + 修改 2 个文件**。

### 1.2 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go get github.com/hibiken/asynq@latest` | ✅ exit 0 | 安装 **v0.26.0**（plan 锁的 v0.24.1 → 按 M8 强制 `@latest`） |
| `go test ./internal/queue/...` (Step 3 red) | ✅ FAIL | `build constraints exclude all Go files`（impl 还没写） |
| `go test ./internal/queue/... -v` (Step 6 green) | ✅ PASS | `TestClientAndServer_EmptyURL` PASS in 0.00s |
| `go test -tags=integration ./internal/queue/...` (端到端) | ❌ FAIL | `dial tcp [::1]:6379: connectex: No connection could be made`（**redis 未在跑**，详见 §1.5） |
| `go test ./...` (全量) | ✅ PASS | `ok internal/handler` + `ok internal/queue` + `ok internal/repository` |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go build ./...` | ✅ exit 0 | 编译通过 |
| `go mod tidy` | ✅ exit 0 | go.sum 整理 |
| `git status` | 见 §1.6 | 2 modified + 4 untracked (queue 包) |

### 1.3 实际安装版本

```
$ go list -m github.com/hibiken/asynq
github.com/hibiken/asynq v0.26.0

$ go list -m github.com/redis/go-redis/v9
github.com/redis/go-redis/v9 v9.14.1

$ go list -m github.com/robfig/cron/v3
github.com/robfig/cron/v3 v3.0.1

$ go list -m golang.org/x/time
golang.org/x/time v0.14.0

$ go list -m github.com/google/uuid
github.com/google/uuid v1.6.0
```

> ⚠️ **M8 偏差**: 计划锁定 asynq `v0.24.1`，按 M8 强制 `@latest` 规则实际装到 **v0.26.0**。
> - v0.26.0 主要变化：迁移 `redigo` → `go-redis/v9`，故 transitive 依赖增加了 `redis/go-redis/v9` + `dgryski/go-rendezvous` + `google/uuid`。
> - **API 不兼容**：`asynq.RedisClientOpt.URL` 字段在 v0.26.0 被移除；`asynq.ServeMux.HandleFunc` 签名从 `func(*Task) error` 改成 `func(context.Context, *Task) error`（详见 §1.4 偏差表）。

### 1.4 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `internal/queue/queue.go` 用 `asynq.ParseRedisURI(redisURL)` 而不是 `asynq.RedisClientOpt{URL: redisURL}` | asynq **v0.26.0 移除了 `RedisClientOpt.URL` 字段**（迁移到 go-redis/v9 后用 `Addr/Network/DB/Password` 等独立字段）；`ParseRedisURI` 是 v0.26.0 起官方推荐的 URL 解析入口（4 种 scheme：`redis`/`rediss`/`redis-socket`/`redis-sentinel`） | ✅ 正确（M8 `@latest` 必然产物） |
| 2 | `HandleFunc` 回调签名改成 `func(ctx context.Context, c *asynq.Task) error`，并把 ctx 直接透传给用户 HandlerFunc | asynq **v0.26.0 把 ServeMux.HandleFunc 签名改成 `(ctx, *Task) error`**；这正好让用户 handler 能拿到 deadline/cancel 信号，比 plan 原文的 `context.Background()` 透传更合理 | ✅ 顺带改进（M8 `@latest` 必然产物） |
| 3 | 测试拆成两个文件：`queue_test.go`（默认跑）+ `queue_integration_test.go`（`//go:build integration`） | **本地 redis 未在跑**（详见 §1.5）；按 plan 兜底规则用 `-tags=integration` 跳过集成测试。Go 不支持单函数 build tag，必须拆文件 | ✅ 符合 plan 兜底规则（"改用 `-tags=integration` 跳过并报告问题"） |
| 4 | `cmd/api/main.go` **未**集成 Worker 启动 | T004 范围只做 `internal/queue` 包 + 单测；Worker 集成属于 T007（server startup bootstrap） | ✅ 符合 T004 范围（plan §T003→T007 顺序） |
| 5 | `MaxRetry(3)` 硬编码在 `Enqueue` 内 | plan 原文如此；如未来需要可作为参数透传 | ✅ 与 plan 一致 |
| 6 | 队列名 `default: 5` 硬编码在 `NewServer` 内 | plan 原文如此；多队列（critical/default/low）属于 Phase 1+ | ✅ 与 plan 一致 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖、未做超出 T004 范围的事、未自动 commit、未硬编码 redis URL（生产代码从 `cfg.RedisURL` 走，测试代码用字面量 `"redis://localhost:6379/0"` 是 Go 集成测试惯例）。

### 1.5 ⚠️ Redis 连接状态（BLOCKED 原因）

按用户上下文"本地 redis 已装"假设 T004 集成测试能真跑。**实测发现本机无 redis**：

```
$ redis-cli ping
redis-cli : 无法将"redis-cli"项识别为 cmdlet、函数、脚本文件或可运行程序的名称。

$ Get-Service -Name "Redis" -ErrorAction SilentlyContinue
（无输出）

$ Get-Process redis* -ErrorAction SilentlyContinue
（无输出）

$ Test-NetConnection -ComputerName 127.0.0.1 -Port 6379 -InformationLevel Detailed
TcpTestSucceeded : False
RemoteAddress    : 127.0.0.1
RemotePort       : 6379
PingSucceeded    : True  (但 TCP 主动拒绝)

$ where.exe redis-server
（无输出）

$ Get-ChildItem -Path "C:\Program Files" -Recurse -Filter "redis-server.exe"
（无输出）

$ Get-ChildItem -Path "C:\Program Files (x86)" -Recurse -Filter "redis-server.exe"
（无输出）
```

**结论**：
- 本机**没有** `redis-server.exe` / `redis-cli.exe` 可执行文件
- 本机**没有** Redis / Memurai / KeyDB Windows 服务
- 端口 `127.0.0.1:6379` 主动拒绝连接（"由于目标计算机积极拒绝"）
- 用户注册表里**没有**任何 Redis / Memurai / KeyDB 安装记录

**已采取行动**：
- 严格按 plan 兜底规则"不自动启动 + 用 `-tags=integration` 跳过"
- 单元测试 `TestClientAndServer_EmptyURL` 不依赖 redis → 通过
- 集成测试 `TestEnqueueAndHandle` 用 `//go:build integration` 标记 → 默认 `go test ./...` 不跑
- 显式跑 `go test -tags=integration` 确认它**如预期 FAIL**（证明集成测试本身语法正确，只是 redis 没起）

**留给用户**：
- 用户在主窗口确认本机是否有 redis（可能是其他账号/容器里装了、或者用 WSL2 起 redis）
- 若确认要真跑集成测试，需要先 `redis-server --port 6379`（任意方式启动 redis）
- 启动后跑：`cd k:\go_projects\AsyncStarterAgent; go test -tags=integration ./internal/queue/... -v`
- 或者在主窗口用 docker 起一个 redis 跑集成：`docker run -d -p 6379:6379 redis:7-alpine`

### 1.6 git status 输出

```
On branch main
Changes not staged for commit:
        modified:   go.mod
        modified:   go.sum

Untracked files:
        doc/ai-coding-boundary.md          (T001 起遗留)
        doc/decision-log.md                (T001 起遗留)
        doc/handoff/T001-handoff.md        (T001 起遗留)
        doc/handoff/T002-handoff.md        (T002 起遗留)
        doc/handoff/T003-handoff.md        (T003 起遗留)
        doc/mvp-definition.html            (T001 起遗留)
        doc/plan-boundary.md               (T001 起遗留)
        doc/plans/00-index.md              (T001 起遗留)
        doc/plans/01-phase0-foundation.md  (T001 起遗留)
        doc/plans/02-phase1-trigger.md     (T001 起遗留)
        doc/plans/03-phase2-context.md     (T001 起遗留)
        doc/plans/04-phase3-synthesis.md   (T001 起遗留)
        doc/plans/05-phase4-delivery.md    (T001 起遗留)
        doc/prototype.html                 (T001 起遗留)
        doc/requirement-spec.html          (T001 起遗留)
        doc/task-tracker.html              (T001 起遗留)
        internal/queue/handler.go          ← T004 新建
        internal/queue/queue.go            ← T004 新建
        internal/queue/queue_integration_test.go  ← T004 新建
        internal/queue/queue_test.go       ← T004 新建
```

> **T004 新增的 4 个文件** + go.mod/go.sum 改动 = 6 个变动对象待 commit。
> doc/ 全部 untracked 是 T001 起的状态（user 一直选 NO commit handoff doc），不是 T004 引入的新 untracked。

### 1.7 git diff --stat

```
 go.mod |  8 ++++++++
 go.sum | 30 ++++++++++++++++++++++++++++++
 2 files changed, 38 insertions(+)
```

加上 4 个新文件 = T004 总变更。

---

## 2. 上一窗口**没做**什么

- ❌ **没有**集成测试真跑（redis 未启动，详见 §1.5）
- ❌ **没有** `cmd/api/main.go` 集成 Worker 启动（T007 范围）
- ❌ **没有** `cmd/worker/main.go` 单独 worker 入口（plan 没明说，但通常 asynq worker 是独立进程——**T007 决策**）
- ❌ **没有** `internal/queue/tasks.go` 任务类型常量 + payload struct（plan §T003 任务文本有提，但 T004 任务文本**没**要求；建议留给 Phase 1+ 接业务任务时加）
- ❌ **没有** `make redis-up` / `make redis-down` Makefile 目标（plan §T003 任务文本有提，但用户没要求启动 docker / Windows service，**且本机无 redis 二进制**，加 Makefile 目标没意义）
- ❌ **没有** cron / periodic task manager / inspector / unique task 等等高级特性（不属于 T004 范围，NFR-04 留给 Phase 1+ 业务）
- ❌ **没有** Prometheus metrics / distributed tracing（不属于 Phase 0）
- ❌ **没有** business handler（属于 Phase 1）
- ❌ **没有** `git add` / `git commit`（按 P1 红线等用户决定）

> ⚠️ **`tasks.go` 没建是 plan §T003 任务文本说"要建"、§T004 任务文本说"不建"的矛盾点**。我**按 T004 任务文本执行**（T004 任务文本只列 3 个文件，无 tasks.go）。如果用户希望加，**Phase 1 接业务任务时一起加**更合理。

---

## 3. 下一窗口需要做的（T005）

### 3.1 T005 目标（来源：plan/01-phase0-foundation.md §Task T005 L980+）

> 提供 Docker / docker-compose 一键启动开发环境：PostgreSQL + Redis + API 三个 service。

**用户已决定**：**不**启动 docker，但**要**写 `docker-compose.yml` 文件（持久化基础设施配置）。

### 3.2 关键文件清单（T005 计划原文）

**新建**：
- `Dockerfile`（多阶段构建：golang:1.22-alpine → alpine:3.19，详见 §4.3 base 版本问题）
- `docker-compose.yml`（postgres + redis + api 三个 service + healthcheck + volume + port mapping）
- `.dockerignore`（排除 .git / bin/ / *.md / .env 等）
- `scripts/init-db.sql`（启用 `pgcrypto` + `vector` 扩展 + 授权）

**修改**：
- `Makefile`（加 `docker-build` / `docker-up` / `docker-down` / `docker-logs` 目标）
- `README.md`（加 docker 启动说明，可选）

### 3.3 建议执行顺序

1. 读 `doc/plans/01-phase0-foundation.md` §T005 L980+ 完整 plan
2. 按 plan 写 4 个新文件 + 改 1-2 个旧文件
3. **不**执行 `docker compose up -d`（用户已决定不启动 docker）
4. 跑 `go test ./...` 确保没破坏 T004 之前的测试
5. 跑 `docker compose config` 验证 docker-compose.yml 语法（**需要 docker 安装**；若本机无 docker 跳过此步，在 handoff §6 BLOCKED）
6. 写 `T005-handoff.md` 交接

### 3.4 验证命令（执行 T005 后跑这些）

```powershell
cd "k:\go_projects\AsyncStarterAgent"

# 1. 编译
go build ./...

# 2. 跑测试（确认没破坏 T004 之前的）
go test ./...

# 3. 验证 docker-compose 语法（如果有 docker）
docker compose config
```

### 3.5 ⚠️ 重要：Dockerfile base 镜像版本

plan 原文锁 `golang:1.22-alpine`，但本机 `go version` 是 **1.25.5**（T001 装的）：

```
$ go version
go version go1.25.5 windows/amd64
```

**建议**（T005 决策点）：

- **方案 A（保守）**：严格按 plan 锁 `golang:1.22-alpine`。优点：plan 不偏差；缺点：与本机 go 1.25.5 不一致，可能有 `go.mod` 工具链版本不匹配告警。
- **方案 B（对齐）**：锁 `golang:1.25-alpine`（如果有这个 tag）或 `golang:1.25.3-alpine`。优点：与本机一致；缺点：plan 偏差。
- **方案 C（最稳）**：用 `golang:1.22-alpine` 编译但加 `GOTOOLCHAIN=local` 防止自动升级；或 `golang:1.25-alpine`。

**默认建议方案 A**（按 plan 字面锁 1.22 alpine base）。T005 时再问用户。

### 3.6 依赖

- T005 本身**不**引入新的 Go 依赖（Dockerfile / compose 不进 go.mod）
- `init-db.sql` 用 PostgreSQL 16+ 即可（docker 镜像用 `postgres:16-alpine`，对齐 plan 锁的版本）

---

## 4. 给下一窗口的提示

1. **TDD 不要跳**: 跟 T002/T003/T004 一样的节奏。但 T005 **不写 Go 代码**（只写 Dockerfile / compose / SQL），所以 TDD 不适用。改用 **静态检查**：`docker compose config` 验证语法 + `go build ./...` 确保没破坏 T004 之前的东西。
2. **不要顺手加额外功能**（C1/C2 + R1/R2 红线）。T005 只做 docker 基础设施，**不要**加：
   - production k8s manifests（不属于 Phase 0）
   - nginx / traefik 反代（不属于 Phase 0）
   - 多阶段构建的优化（minimal final image 不属于 Phase 0）
   - docker secrets / docker swarm（不属于 Phase 0）
3. **`internal/queue` 已经是完整包**：T005 不需要改它。但 compose 里的 `api` service 需要在 `depends_on` 等到 `postgres` + `redis` healthcheck 通过（典型 `condition: service_healthy`）。
4. **`scripts/init-db.sql` 一定要在 postgres 第一次启动前 copy 进去**（用 `/docker-entrypoint-initdb.d/`）。内容：
   ```sql
   CREATE EXTENSION IF NOT EXISTS pgcrypto;
   CREATE EXTENSION IF NOT EXISTS vector;
   GRANT ALL ON SCHEMA public TO <POSTGRES_USER>;
   ```
   （授权那行可省略，postgres:16-alpine 镜像默认 user 已有权限）
5. **T005 vs T004 顺序**：用户已确认 Phase 0 顺序 T003 → T004 → T005，T005 **不**回填 T004 集成测试（用户也不启动 docker）。
6. **`docker compose` vs `docker-compose`**: plan 用的旧 `docker-compose`（带连字符），新 Compose V2 用 `docker compose`（带空格）。建议 T005 写**新格式** `docker compose` —— Docker 官方 2023 起主推 V2。
7. **不要自动 `git commit`**。改完跑通测试后只展示 diff。
8. **本机无 redis 的事实**：T005 的 `docker-compose.yml` 里的 redis service 是**唯一**让 T004 集成测试能跑起来的途径。**用户在主窗口如果想做端到端 smoke test，T005 之后可以 `docker compose up -d redis`（只起 redis 一个 service，postgres 暂不起）然后 `go test -tags=integration ./internal/queue/...`**。
9. **完成后**请按本文件 §3 的结构生成下一份交接文档 `doc/handoff/T005-handoff.md`。

---

## 5. 当前文件结构（Phase 0 / T004 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
│   └── api.exe                   (~7.5 MB, T002 验证用)
├── cmd/
│   └── api/
│       └── main.go               ✅ T002 (启动 HTTP server，**未连 DB / 未启 Worker**)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md
│   │   ├── T002-handoff.md
│   │   ├── T003-handoff.md
│   │   └── T004-handoff.md       ← 本文件
│   ├── plans/                    (00~05)
│   ├── mvp-definition.html
│   ├── plan-boundary.md
│   ├── prototype.html
│   ├── requirement-spec.html
│   └── task-tracker.html
├── internal/
│   ├── config/
│   │   └── config.go             ✅ T001 (5 env vars，未动)
│   ├── handler/
│   │   ├── health.go             ✅ T002
│   │   └── health_test.go        ✅ T002
│   ├── middleware/
│   │   ├── logger.go             ✅ T002
│   │   └── recovery.go           ✅ T002
│   ├── queue/                    ✅ T004 (新)
│   │   ├── handler.go
│   │   ├── queue.go
│   │   ├── queue_integration_test.go  (//go:build integration)
│   │   └── queue_test.go
│   ├── repository/               ✅ T003
│   │   ├── db.go
│   │   └── db_test.go
│   └── server/
│       └── server.go             ✅ T002
├── pkg/
│   └── httpx/
│       └── response.go           ✅ T002
├── migrations/                   ✅ T003
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── scripts/                      (空，T005 创建 init-db.sql)
├── test/
│   └── integration/              (空，Phase 末才用)
├── .env.example                  ✅ T001
├── .gitignore                    ✅ T001
├── go.mod                        ✅ T002+T003+T004 (gin v1.12.0 + pgx v5.10.0 + asynq v0.26.0)
├── go.sum                        ✅ T002+T003+T004
├── Makefile                      ✅ T001+T003
└── README.md                     ✅ T001
```

---

## 6. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **本机 redis 状态确认**：我检查过本机 `redis-server.exe` / Windows Service / 注册表，**全部不存在**。但你说"本地 redis 已装"——
   - **可能 A**：你之前装过但被卸载 / 移到了别的机器 / WSL2 内的 redis
   - **可能 B**：你记错了，本机确实没装
   - **可能 C**：你打算用 docker 起 redis（那 T005 之后 `docker compose up -d redis` 即可）

   **如果**用户希望 T004 集成测试**真跑通过**，需要先解决 redis 的存在性：
   - 选项 a：在 WSL2 / docker / 别的机器上起 redis，反代 `127.0.0.1:6379` 到本机
   - 选项 b：跳过，等 T005 写完 compose 后用 `docker compose up -d redis` 跑
   - 选项 c：接受现状，集成测试用 `//go:build integration` 永久 gate，等 CI 环境跑

2. **是否 `git add` 当前 T004 的 6 个变动并提交？**
   - 建议命令（**用户在主窗口执行**）：
     ```bash
     cd "k:\go_projects\AsyncStarterAgent"
     git add go.mod go.sum \
             internal/queue/queue.go \
             internal/queue/handler.go \
             internal/queue/queue_test.go \
             internal/queue/queue_integration_test.go
     git commit -m "feat(phase0/T004): redis task queue (asynq) infrastructure

     - Add internal/queue/queue.go: Client (asynq Producer) with NewClient
       and Enqueue(ctx, type, payload, ttl) — uses asynq.ParseRedisURI
       (v0.26.0 API; plan's v0.24.1 URL field was removed in v0.26.0)
     - Add internal/queue/handler.go: Mux + Server with NewMux, HandleFunc,
       NewServer, Start, Stop, Shutdown — HandleFunc now passes asynq's
       ctx to user handler (v0.26.0 ServeMux API change)
     - Add queue_test.go: unit test TestClientAndServer_EmptyURL
       (no redis required, runs by default)
     - Add queue_integration_test.go: end-to-end test TestEnqueueAndHandle
       gated by //go:build integration (requires local redis)
     - Versions: asynq v0.26.0, go-redis/v9 v9.14.1, robfig/cron/v3 v3.0.1
       (all @latest per M8 rule; asynq v0.26.0 migrated redigo -> go-redis/v9)
     - Did NOT start local redis (not running on this machine per
       redis-cli/Windows Service/port 6379 checks; integration test
       gated behind build tag per plan fallback rule)"
     ```
   - 如果用户说 NO，则保持当前未提交状态。

3. **T005 Dockerfile base 版本**：plan 锁 `golang:1.22-alpine`，本机 `go 1.25.5`——T005 时是否仍按 plan 锁 1.22，还是对齐 1.25？请用户在 T005 决定。

4. **T005 之后是否起 docker 跑端到端测试**？用户已确认 T005 不启动 docker，但**端到端 smoke test**（`docker compose up -d postgres redis` + `go test -tags=integration`）需要 docker。T005 后可再问一次。

> **下一窗口（T005）开场建议**：
> 1. 读 `doc/handoff/T004-handoff.md`（本文件）
> 2. 读 `doc/handoff/T003-handoff.md`（上一个 docker 决策点）
> 3. 读 `doc/plans/01-phase0-foundation.md` §T005（L980+）
> 4. 用 `AskUserQuestion` 问上面 §6 第 1、3 项决定（redis + Dockerfile base 版本）
> 5. 获答复后写 Dockerfile / compose / .dockerignore / scripts/init-db.sql + 改 Makefile
> 6. 验证 `go test ./...` 没破坏 + `docker compose config` 语法（如有 docker）
> 7. 写 T005-handoff.md
