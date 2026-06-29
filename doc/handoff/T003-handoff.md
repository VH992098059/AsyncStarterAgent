# Phase 0 / T003 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T003 — PostgreSQL 数据模型 + pgx 驱动 + golang-migrate 迁移
> **状态**: ✅ 已完成 / 待用户确认 commit
> **基线 commit**: `780a7f9` (T002)

---

## 1. 上一窗口做了什么

按 `doc/plans/01-phase0-foundation.md` §Task T003 执行了 12 个 step 中的 1~8、12~13。**Step 9-11 主动跳过**（用户决定不启动 docker，详见 §2）。

严格 TDD：test → red（确认 FAIL）→ impl → green（确认 PASS）→ vet/build/tidy 验证。

### 1.1 新建/修改文件清单

| 文件 | 操作 | 大小 | 说明 |
|---|---|---|---|
| `migrations/0001_init.up.sql` | 新建 | ~80 行 | 6 张表 schema（agent_runs / drafts / data_sources / deliveries / templates / sync_timestamps）+ `pgcrypto` + `vector` 扩展 + 9 个索引 + 1 个 UNIQUE 约束 |
| `migrations/0001_init.down.sql` | 新建 | 6 行 | 逆序 DROP 6 张表 |
| `internal/repository/db_test.go` | 新建 | ~20 行 | TDD test：`TestDBConnect_RequiresDSN`（空 DSN 必须返回 error） |
| `internal/repository/db.go` | 新建 | ~30 行 | `Open(ctx, dsn) (*pgxpool.Pool, error)` —— DSN 必填校验、ParseConfig、MaxConns=20、NewWithConfig、Ping 失败主动 Close |
| `Makefile` | 修改 | +6/-2 | 加 `MIGRATE` + `DATABASE_URL` 变量；`migrate-up` / `migrate-down` 改为 `go run -tags 'postgres' $(MIGRATE) -database "$(DATABASE_URL)" -path ./migrations up/down 1` |
| `go.mod` | 修改 | +10/-1 | 加 `github.com/jackc/pgx/v5 v5.10.0` 为 direct，`github.com/jackc/puddle/v2 v2.2.2` 等为 indirect |
| `go.sum` | 修改 | +15/-2 | 同步锁定 pgx + puddle 等依赖哈希 |

> **注意**: `golang-migrate/migrate/v4` **没有**进 `go.mod` / `go.sum`，因为我们只在 Makefile 里用 `go run ...@latest` 跑它，并未在 Go 源码里 import。这与 plan 一致（migrate CLI 是 tool 依赖，不是 library 依赖）。

### 1.2 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go get github.com/jackc/pgx/v5@latest` | ✅ exit 0 | 安装 **v5.10.0** |
| `go get github.com/golang-migrate/migrate/v4@latest` | ✅ exit 0 | 安装 **v4.19.1**（不进 go.mod，详见 1.1） |
| `go get .../database/postgres` / `.../source/file` | ✅ exit 0 | migrate sub-package（migrate 加载用，不进 go.mod） |
| `go test ./internal/repository/...` (Step 5 red) | ✅ FAIL | `no non-test Go files`（Open 还没写） |
| `go test ./...` (Step 12 green) | ✅ PASS | `ok internal/handler` + `ok internal/repository` |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go build ./...` | ✅ exit 0 | 编译通过 |
| `go mod tidy` | ✅ exit 0 | go.sum 整理（puddle/v2 因 pgxpool transitive 引入） |
| `make -n migrate-up` / `make -n migrate-down` | ✅ OK | dry-run 输出正确展开 `$(MIGRATE)` + `$(DATABASE_URL)` |
| `git diff --stat` | ✅ 3 modified + 3 untracked | 见 §1.4 |

### 1.3 实际安装版本

```
$ go list -m github.com/jackc/pgx/v5
github.com/jackc/pgx/v5 v5.10.0

$ go list -m github.com/golang-migrate/migrate/v4
go: module github.com/golang-migrate/migrate/v4: not a known dependency
```

> ⚠️ **M8 偏差**: 计划锁定 pgx `v5.5.5` / migrate `v4.17.1`，按 M8 强制 `@latest` 规则实际装到：
> - pgx/v5 **v5.10.0**（与 v5.5.5 完全兼容：`pgxpool.Pool` / `pgxpool.NewWithConfig` / `pgxpool.ParseConfig` / `pool.Ping` / `pool.Close` API 无变化）
> - migrate/v4 **v4.19.1**（仅 CLI 行为，Go API 也不进我们的项目）

### 1.4 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | migrate/v4 不在 go.mod / go.sum | 我们没有在 `.go` 源码里 import 它（仅 Makefile `go run` 调用）—— Go 工具链把它识别为「可丢弃 tool 依赖」并从 go.mod 中清理 | ✅ 正确（与 plan 字面行为一致） |
| 2 | pgx 装到 v5.10.0（非 plan 锁的 v5.5.5） | M8 强制 `@latest` | ✅ 用户已确认 |
| 3 | migrate/v4 装到 v4.19.1（非 plan 锁的 v4.17.1） | M8 强制 `@latest` | ✅ 用户已确认 |
| 4 | migration SQL 中 `vector` 扩展 IF NOT EXISTS | 这是 plan 原文写法。`CREATE EXTENSION IF NOT EXISTS` 本身就是 PostgreSQL 幂等语义，**不是**占位符（C1 红线） | ✅ 与 plan 一致 |
| 5 | `migrations/0001_init.up.sql` 中 `marks JSONB NOT NULL DEFAULT '[]'::jsonb` | plan 原文如此；用 `'[]'::jsonb` cast 显式说明类型避免歧义 | ✅ 与 plan 一致 |
| 6 | `go.mod` 中 `pgx/v5` 进 direct block（不是 indirect） | 因为 `internal/repository/db.go` 直接 import `github.com/jackc/pgx/v5/pgxpool` | ✅ 工具链自动归类 |
| 7 | `bin/api.exe` 文件夹未重建 | T003 没动 main.go / server.go 任何代码，main 入口不变；不需要重新构建 | ⚠️ 微小省力（编译路径仍通过 `go build ./...`） |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖、未做超出 T003 范围的事、未自动 commit。

---

## 2. 上一窗口**没做**什么

明确列出**未完成**事项，避免下一窗口误以为已就绪：

- ❌ **没有**启动 docker postgres（用户决定）
- ❌ **没有** `make migrate-up` 实际跑迁移（依赖 docker postgres）
- ❌ **没有** `psql \dt` 验证 6 张表（依赖 docker postgres）
- ❌ **没有** Redis 客户端（属于 T004）
- ❌ **没有** business CRUD handler（属于 Phase 1+）
- ❌ **没有** JWT 中间件（属于 T009，NFR-02）
- ❌ **没有** Swagger / OpenAPI（不属于 Phase 0）
- ❌ **没有** Prometheus metrics（不属于 Phase 0）
- ❌ **没有** CORS / 限流（不属于 Phase 0）
- ❌ **没有** `git add` / `git commit`（按 P1 红线等用户决定）
- ❌ **没有** `internal/handler/health_test.go` 之外的 handler 测试（仅 repository 一个新测试）
- ❌ **没有** `docker-compose.yml`（属于 T005）
- ❌ **没有** `main.go` 集成 `repository.Open(cfg.DSN)`（T007 范围；现在 main 仍只起 HTTP server，**未连接数据库**）

> ⚠️ **当前 `main.go` 没有调用 `repository.Open`** —— 这是 T007 的工作（server startup 集成）。T003 只提供 `repository` 包，不改 `main.go`。

---

## 3. 下一窗口需要做的（T004）

### 3.1 T004 目标（来源：plan/01-phase0-foundation.md §Task T003 L753+）

> 实现 Redis 队列（asynq）基础设施：Producer 端 `Enqueue`、Worker 端 `Server.Run`、任务类型定义、健康检查。

### 3.2 用户环境已具备

- ✅ 本地 **Redis** 已安装（用户已确认）
- ✅ `cfg.RedisURL` 字段已存在（T001 写入 `config.Config.RedisURL`）
- ✅ `internal/config/config.go` 未改动（T003 也不动它）

### 3.3 关键文件（T004 计划原文）

**新建**：
- `internal/queue/queue.go`（asynq Client 封装 + Server 启动）
- `internal/queue/queue_test.go`（TDD）
- `internal/queue/tasks.go`（任务类型常量 + payload struct）

**修改**：
- `go.mod`（加 `github.com/hibiken/asynq@latest`）
- `Makefile`（加 `redis-up` / `redis-down` 目标可选，T004 也可纯用本地 redis）
- `cmd/api/main.go`（T004 末集成 Worker 启动？**待 plan 复核**；按 T002 节奏应该是 T007 才接 main）

### 3.4 建议执行顺序

1. T004 范围：纯 `internal/queue` 包 + 单测；可以**真跑** Redis（本地已装），不像 T003 那样跳过 docker 验证。
2. TDD 节奏：先写 `TestEnqueue_NilClient` / `TestServer_Start_Stop` 这类边界用例 red → green。
3. T004 末不一定要改 `main.go`（worker 通常是单独的进程入口，例如 `cmd/worker/main.go`）—— **需 T004 阶段读 plan L753+ 确认**。

### 3.5 验证命令（执行 T004 后跑这些）

```powershell
cd "k:\go_projects\AsyncStarterAgent"

# 1. 编译
go build ./...

# 2. 跑测试
go test ./...

# 3. 真跑 redis 集成（用户在主窗口决定是否启动 redis-server）
redis-server --version  # 确认本地装了
# （T004 阶段需要在测试 setup 里连真实 redis，DSN 用 cfg.RedisURL）
```

### 3.6 依赖选择

- `github.com/hibiken/asynq@latest`（asynq 是 Go 生态 Redis 队列主流）
- 不要锁版本（M8 规则）

详细计划在 `doc/plans/01-phase0-foundation.md` L753+。

---

## 4. 给下一窗口的提示

1. **TDD 不要跳**: 跟 T002/T003 一样的 red → green 节奏。`internal/queue/queue.go` 最好先有 `TestEnqueue_NilClient` / `TestNew_InvalidRedisURL` 之类的失败用例先行。
2. **不要顺手加额外功能**（C1/C2 + R1/R2 红线）。T004 只做 queue 基础设施，**不要**加：
   - JWT auth（NFR-02 属于 T009）
   - 业务 handler（属于 Phase 1）
   - 数据库表（已经 T003 全部建好）
3. **DSN/RedisURL 已存在于 `config.Config`**：T004 只需读取 `cfg.RedisURL` 传给 asynq 客户端，不要重新解析 env。
4. **T004 vs T005 顺序**：用户已确认 Phase 0 顺序 T003 → T004 → T005，T005 写 `docker-compose.yml` 时**不要**回填 postgres 验证到 T003（已说明"不启动 docker"）。
5. **T004 可以真跑 Redis**：本地 redis 已装，T004 的集成测试 / smoke 验证可以直接用 `cfg.RedisURL` 连。
6. **`cmd/api/main.go` 集成 Worker？** —— T004 范围需要明确。建议：T004 只做 `internal/queue` 包 + 单测；T007 才把 Worker / DB / Server 三个 bootstrap 串到 main。
7. **asynq 任务 payload 用 JSON**：参考 asynq 官方文档，`asynq.NewTask(typeName, payload)` + `client.Enqueue(task)`。
8. **每次完成一个原子改动后展示 diff + 等用户决定 commit**。不要自动 `git commit`。
9. **完成后**请按本文件 §3 的结构生成下一份交接文档 `doc/handoff/T004-handoff.md`。

---

## 5. 当前文件结构（Phase 0 / T003 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
│   └── api.exe                   (~7.5 MB, T002 验证用)
├── cmd/
│   └── api/
│       └── main.go               ✅ T002 (启动 HTTP server，**未连 DB**)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md
│   │   ├── T002-handoff.md
│   │   └── T003-handoff.md       ← 本文件
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
│   ├── repository/               ✅ T003 (新)
│   │   ├── db.go
│   │   └── db_test.go
│   └── server/
│       └── server.go             ✅ T002
├── pkg/
│   └── httpx/
│       └── response.go           ✅ T002
├── migrations/                   ✅ T003 (新)
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── scripts/                      (空，T005 创建 init-db.sql)
├── test/
│   └── integration/              (空，Phase 末才用)
├── .env.example                  ✅ T001
├── .gitignore                    ✅ T001
├── go.mod                        ✅ T002+T003 (gin v1.12.0 + pgx v5.10.0)
├── go.sum                        ✅ T002+T003
├── Makefile                      ✅ T001+T003
└── README.md                     ✅ T001
```

---

## 6. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **是否 `git add` 当前 7 个新文件 + 2 个修改文件并提交？**
   - 建议命令（**用户在主窗口执行**）：
     ```bash
     cd "k:\go_projects\AsyncStarterAgent"
     git add migrations/0001_init.up.sql \
             migrations/0001_init.down.sql \
             internal/repository/db.go \
             internal/repository/db_test.go \
             Makefile \
             go.mod \
             go.sum
     git commit -m "feat(phase0/T003): postgresql schema + pgx pool + golang-migrate

     - Add migrations/0001_init.{up,down}.sql (6 tables: agent_runs, drafts,
       data_sources, deliveries, templates, sync_timestamps)
     - Enable pgcrypto + vector extensions (Phase 3 will use vector)
     - Add internal/repository.Open(ctx, dsn) with DSN validation, MaxConns=20,
       and ping-with-rollback semantics
     - Add TDD test TestDBConnect_RequiresDSN
     - Update Makefile: migrate-up / migrate-down now use
       go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest
     - Versions: pgx v5.10.0, migrate v4.19.1 (both @latest per M8 rule)
     - Did NOT start docker postgres (per user decision); migration files
       not yet executed against a real DB"
     ```
   - 如果用户说 NO，则保持当前未提交状态。

2. **是否同意 T003 跳过 docker 验证（Step 9-10 跳到 T005 docker-compose 写完后）？**
   - 是 → 继续 T004（Redis 队列，本地 redis 已装，可真跑）
   - 否 → 用户先在主窗口 `docker compose up -d postgres`（T005 还未写 compose，可临时用 plan 之外的 docker run），然后 `make migrate-up` 验证，再开 T004

3. **T003 引入的依赖**是否仍按 `@latest` 原则（pgx/v5 v5.10.0 + migrate/v4 v4.19.1）？—— 已按 M8 强制执行，复核确认。

> **下一窗口（T004）开场建议**：
> 1. 读 `doc/handoff/T003-handoff.md`（本文件）
> 2. 读 `doc/handoff/T002-handoff.md`（上下文）
> 3. 读 `doc/handoff/T001-handoff.md`（项目基线）
> 4. 读 `doc/plans/01-phase0-foundation.md` §T004（L753+）
> 5. 用 `AskUserQuestion` 问上面 §6 三个决定
> 6. 获答复后按 TDD 执行 T004
