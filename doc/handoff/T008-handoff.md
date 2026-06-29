# Phase 1 / T008 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T008 — DDL 截止日期检测器（FR-A02）
> **状态**: ✅ **DONE**（implementer + spec + code quality + 4 个 fix 全过）
> **基线 commit**: `46c3403` (T007)
> **本任务 commit**: `2e2b43f` (5 files, +149 lines)
> **用户红线**: 本次不跑 docker / 不做端到端

---

## 1. 上一窗口做了什么（T008 做完了什么内容）

按 `doc/plans/02-phase1-trigger.md` §Task T008 (L832–L1043) 完整执行了 9 个 step 中的 **1-7**。**Step 8（端到端 DDL 流程手动验证）按 plan 边界说明主动跳过**（需 docker + T009 整合 main 启动）。

### 1.1 关联需求

- **FR-A02 (P1)**: DDL 触发，提前量 1h-72h 可配置（**当前 24h 硬编码**，T008 范围内未暴露可配置入口）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `migrations/0003_ddl.up.sql` | 新建 | 22 | `user_tasks` 表（12 字段 + UNIQUE(source, external_id)） + partial index `WHERE completed=false AND triggered_at IS NULL`（fix 后加注释解释谓词选择） |
| `migrations/0003_ddl.down.sql` | 新建 | 1 | `DROP TABLE IF EXISTS user_tasks;` |
| `internal/trigger/ddl_test.go` | 新建 | 34 | 2 个测试：TestDDLDetector_WithinLeadTime（4 sub-cases）+ TestDDLDetector_DefaultLeadTime |
| `internal/trigger/ddl.go` | 新建 | 26 | `DDLDetector{defaultLead}` + `NewDDLDetector` + `DefaultLead` + `ShouldTrigger`（**fix 后清掉死代码 + 注释说明 lead 占位**） |
| `internal/trigger/ddl_scheduler.go` | 新建 | 65 | `DDLPollInterval=15min` + `DDLPollHandler` + `RunDDLScheduler`（**fix 后用 `det.DefaultLead()` + `make_interval` 消除 24h 硬编码** + **`RowsAffected` 检查**） |

> **新建 5 个文件 + 修改 0 个文件 = T008 总变更 5 个对象**。

### 1.3 4 个 Fix（code quality review 后由 controller 直接修复）

| # | Fix | 文件:行 | 原问题 | 修复后 |
|---|---|---|---|---|
| 1 | 清 `ShouldTrigger` 死代码 | `ddl.go:23-26` | `if lead <= 0 { lead = d.defaultLead }` 是死代码（lead 被 `_ = lead` 忽略，return 用 `d.defaultLead`） | 删除 `if` 块，保留 `_ = lead`，注释说明"plan spec 阶段 T008 暂未启用 lead" |
| 2 | SQL 用 `det.DefaultLead()` 参数化 | `ddl_scheduler.go:23-27` | `INTERVAL '24 hours'` 硬编码，与 `DDLDetector.defaultLead = 24h` 解耦（drift 风险） | 改用 `NOW() + make_interval(secs => $1)` + `det.DefaultLead().Seconds()` 注入 |
| 3 | UPDATE 加 `RowsAffected()` 检查 | `ddl_scheduler.go:43-51` | `tag` 被丢弃，0 行更新静默失败会导致重复触发 | 检查 `tag.RowsAffected() == 0` 并 log 警告（"可能被其他实例处理"） |
| 4 | partial index 加注释 | `migrations/0003_ddl.up.sql:18-19` | 谓词选择无文档化 | 加注释说明谓词与 `RunDDLScheduler` SELECT 对齐 + index-only scan 优化 |

### 1.4 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go test ./...` | ✅ PASS | `ok internal/handler` + `ok internal/queue` + `ok internal/repository` + `ok internal/trigger` |
| `go test -v -run TestDDL ./internal/trigger/...` | ✅ 4/4 + 1/1 PASS | 4 sub-cases + DefaultLeadTime |
| `go test -v ./internal/trigger/...` | ✅ 11 PASS + 1 SKIP | T006 3 + T007 4 + T008 5 + service integration SKIP |
| `git commit` | ✅ `2e2b43f` | 由 controller 在 4 fix + 用户授权后执行 |

> ⚠️ **未跑**（用户红线）：
> - `make migrate-up`（需 docker postgres）
> - 端到端 DDL 流程（需 T009 整合 main 启动）
> - `make_interval(secs => $1)` 在 PostgreSQL ≥9.4 可用，依赖 docker pgvector/pg16 镜像支持（Phase 0 T005 已选）

### 1.5 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `ShouldTrigger` 签名 `lead time.Time` → `lead time.Duration` | plan spec 自相矛盾（impl 用 Time、test 用 Duration） | ✅ 已知 plan bug，已最小化偏差 + 注释 |
| 2 | `ShouldTrigger` 内部用 `d.defaultLead` 而非 `lead` 参数 | plan spec 显式 `_ = lead` 忽略 lead | ✅ R1 + C1 合规（fix 后加注释保留 API 占位） |
| 3 | SQL `INTERVAL '24 hours'` → `make_interval(secs => $1)` | fix #2 消除 24h 硬编码 | ✅ 行内偏离 plan 字面（fix 后） |
| 4 | UPDATE 加 `RowsAffected()` 检查 | plan spec 未要求 | ✅ 行内增强（fix 后） |
| 5 | partial index 加注释 | plan spec 未要求 | ✅ 文档化增强（fix 后） |
| 6 | `DDLPollHandler` 用 `string` 而非 `uuid.UUID` | plan spec 显式用 string（pgx 扫描便利） | ✅ 与 plan 一致 |
| 7 | `cmd/api/main.go` / `internal/server/server.go` 未动 | T009 整合范围 | ✅ 与 plan 一致 |

**没有**触发 `ai-coding-boundary.md` 的任何红线（fix 后）：未越界 FR、未加未声明依赖、未写 `TODO` / `FIXME`、未自动 commit 前实现、未硬编码密钥、未跨 Phase。

### 1.6 git log 输出（T008 commit 后）

```
2e2b43f feat(phase1/T008): ddl detector + 15min scheduler (user_tasks table + lead-param drift fix)
46c3403 feat(phase1/T007): keyword matcher (longest-match-wins) + agent_run persistence + manual trigger handler
6a4a9fb feat(phase1/T006): webhook listener with hmac-sha256 verification + trigger indexes
f58c67b feat(phase0/T005): docker + docker-compose one-shot dev environment
```

---

## 2. 上一窗口**没做**什么

- ❌ **没有**改 `cmd/api/main.go`（属 T009 整合）
- ❌ **没有**改 `internal/server/server.go` 签名（属 T009 整合）
- ❌ **没有**注册任何 DDL 路由（DDL 调度器走后台 goroutine，非 HTTP）
- ❌ **没有**把 `RunDDLScheduler` 接到 main.go（plan 明确"T009 整合时绑定"）
- ❌ **没有**跑 `make migrate-up`（user_tasks 表未在 docker postgres 创建）
- ❌ **没有**端到端 DDL 测试（需 T009 整合 + docker）
- ❌ **没有**改 webhook / trigger / matcher（T006/T007 已完成）
- ❌ **没有**加 panic recovery（reviewer 标记 minor，留 T009 整合时统一加）
- ❌ **没有**把 DDL handler 注入 trigger.Service（DDL 走自己调度器，不走 Service）
- ❌ **没有**自动 push（按 P2 红线）

---

## 3. 下一步需要实现什么 — T009 整合入口

> **重要**：T008 末所有 trigger 部件（webhook / keyword / DDL）就位但**未组装**。T009 任务是**任务调度器整合**，把 T006/T007/T008 + Phase 0 db/queue 串起来。

### 3.1 T009 任务范围

来源：`doc/plans/02-phase1-trigger.md` L1047–L1273

| 步骤 | 关键产物 |
|---|---|
| Step 1 | 写 `cmd/api/wire.go`（Deps struct + Build 函数） |
| Step 2 | 重构 `internal/server/server.go` 签名：`func New(cfg, trigSvc) *gin.Engine`（接受 trigger svc） |
| Step 3 | 更新 `cmd/api/main.go` 使用 wire + 启动 DDL 调度器 goroutine |
| Step 4 | `go test ./...` → PASS |
| Step 5 | `go build ./...` → 0 错误 |
| Step 6 | 端到端：手动触发 `POST /api/v1/trigger` |
| Step 7 | 数据库验证 `agent_runs` 记录 |
| Step 8 | 端到端：Webhook 触发 `POST /api/v1/webhook/todoist` |
| Step 9 | 端到端：DDL 触发（插入 user_tasks 12h 后 DDL） |
| Step 10 | 展示 diff 等用户决定 |

**T009 必新建文件**：
- `cmd/api/wire.go`（依赖注入辅助）

**T009 必修改文件**：
- `cmd/api/main.go`（使用 wire + 启 DDL 调度器）
- `internal/server/server.go`（签名从 `New(cfg)` 改为 `New(cfg, trigSvc)`，注册 `POST /api/v1/trigger` 路由）

### 3.2 T009 关键决策点（需下一窗口与用户确认）

1. **`server.New` 签名变更** 是内部重构（不改 HTTP API 路径），属 ai-coding-boundary §7.1 ✅ 范围。但 `internal/server/server.go` 是多文件 import 的关键函数，所有调用方（T002 末 main.go + T006 末 main.go）必须同步更新。

2. **`trigger.Service` 注入 `server.New`** 后，`server.go` 需要接受 `*trigger.Service` 参数。T007 的 `Service` 已经设计为 `NewService(pool, matcher)` 构造，wire 直接 `trigSvc := trigger.NewService(pool, matcher)` 即可。

3. **DDL 调度器回调 wire**（plan Step 1）：
   ```go
   ddlH := func(ctx context.Context, taskID, userID, title string) error {
       uid, err := uuid.Parse(userID)
       if err != nil { return err }
       // 1. 关键词匹配（与 keyword 走同一路径）
       if _, err := trigSvc.ProcessKeyword(ctx, uid, title); err != nil {
           return err
       }
       _ = taskID
       return nil
   }
   // 后台启动 DDL 调度
   go trigger.RunDDLScheduler(ctx, pool, trigger.NewDDLDetector(), ddlH)
   ```
   - 这里 `ddlH` 直接用 `trigSvc.ProcessKeyword`，**DDL 触发 → 关键词匹配 → 创建 AgentRun** 路径打通
   - `taskID` 暂时忽略（plan 范围内不传 AgentRun ← user_task 关联）
   - **注意**：DDL 调度器的 `UPDATE triggered_at = NOW()` 已经写好了，所以**重复触发**在 T009 后会由 user_tasks 端的 `triggered_at` 防住（不是 AgentRun 端）

4. **`main.go` 优雅退出**（plan Step 3）：
   - plan 提供 `sigCh := make(chan os.Signal, 1)` + `signal.Notify(sigCh, SIGINT, SIGTERM)`
   - `go func() { <-sigCh; cancel() }()` 触发 ctx 取消
   - `defer deps.Queue.Close()`
   - **重要**：`RunDDLScheduler` 在 ctx 取消时返回，goroutine 不会泄漏

5. **JWT 认证**（NFR-02）plan T009 范围**不包含**！plan 写"T009 整合 + Phase 0 db/queue"，JWT 是单独的中间件任务。**T009 不要加 JWT 中间件**（属 R1 越界）。

6. **T008 fix #3 `RowsAffected == 0` 警告日志**：在 wire 里调用 DDL handler 时，如果其他实例已经处理了同一条 user_tasks，本实例的 handler 会成功跑（创建了 AgentRun），但 UPDATE 0 行 → log 警告。**这会导致重复 AgentRun**（每实例都创建 1 条），但 `triggered_at` 防住了 user_tasks 端重复。
   - **遗留问题**（T009 范围内**不修**，但要在 handoff 标记给后续 Phase）：
     - 多实例部署时 DDL 调度器需要 `SELECT ... FOR UPDATE SKIP LOCKED` 避免并发
     - 或在 `user_tasks` 表加 `agent_run_id` 关联字段，wire handler 返回时回写
   - 单实例部署（Phase 0 T005 docker-compose 默认 1 个 api 容器）不会出现此问题

### 3.3 T009 不做的事（避免越界）

- ❌ **不要**加 JWT 认证中间件（属单独任务，NFR-02）
- ❌ **不要**改 `webhook.go:33` 注释（`// T009 整合时接入 EventBus...`）— T009 整合时会**自然删除**（handler 不再"先 200 返回"，会创建 AgentRun）
- ❌ **不要**加 panic recovery 到 DDL scheduler（reviewer 标记 minor，留后续 Phase）
- ❌ **不要**加 webhook → asynq enqueue 逻辑（Phase 2+ 才用 asynq 异步触发；T009 同步创建 AgentRun）
- ❌ **不要**改 agent_runs schema
- ❌ **不要**改 user_tasks schema（T008 已就位）
- ❌ **不要**改 trigger 包任何文件（T006/T007/T008 已完成）

### 3.4 建议执行顺序（下一窗口 T009）

1. 读 `doc/handoff/T008-handoff.md`（本文件）
2. 读 `doc/plans/02-phase1-trigger.md` L1047–L1273 §Task T009
3. 读 T007/T008 的代码（特别是 `internal/trigger/service.go` + `ddl_scheduler.go`）
4. 跑 `go test ./...` 确认 T006/T007/T008 测试仍 PASS（无回归）
5. 用 AskUserQuestion 确认：是否继续用 subagent-driven-development 流程？是否接受 plan T009 字面（同步创建 AgentRun，不入 asynq）？
6. Dispatch implementer subagent（带完整 T009 spec + T006/T007/T008 context + 上述注意点）
7. 两轮 review 通过后展示 git diff，**用户决定 commit**
8. commit 后写 `doc/handoff/T009-handoff.md`
9. 写 `doc/handoff/phase1-final-handoff.md`（Phase 1 总交接）

### 3.5 T009 端到端验证（需 docker + 端到端主窗口跑）

T009 末的 `go build` 只验证编译。**端到端必须**主窗口跑：

```bash
# 1. 启动 docker
cd "k:\go_projects\AsyncStarterAgent"
docker compose up -d --build
make migrate-up   # 跑 0001 + 0002 + 0003 三个 migration

# 2. 启动 API
TODOIST_WEBHOOK_SECRET=test-secret \
DATABASE_URL=postgres://starter:starter@localhost:5432/starter?sslmode=disable \
REDIS_URL=redis://localhost:6379/0 \
go run ./cmd/api &

# 3. 手动触发
UID=$(uuidgen)
curl -X POST http://localhost:8080/api/v1/trigger \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$UID\",\"text\":\"写本周周报\"}"
# 期望: {"code":0,"message":"ok","data":{"run_id":"..."}}

# 4. 验证数据库
docker exec asyncstarter_postgres psql -U starter -d starter -c \
  "SELECT id, task_type, trigger_type, status FROM agent_runs ORDER BY created_at DESC LIMIT 5;"
# 期望: 1 行, task_type=weekly_report, trigger_type=keyword

# 5. Webhook 触发
BODY='{"event_name":"item:added","event_id":"evt-2","event_data":{"id":"t2","content":"项目总结"}}'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "test-secret" | awk '{print $2}')
curl -X POST http://localhost:8080/api/v1/webhook/todoist \
  -H "Content-Type: application/json" \
  -H "X-Todoist-HMAC-SHA256: $SIG" \
  -d "$BODY"
# 期望: 200 OK + agent_runs 多 1 行 summary 任务

# 6. DDL 触发
docker exec asyncstarter_postgres psql -U starter -d starter <<EOF
INSERT INTO user_tasks (user_id, source, external_id, title, deadline_at)
VALUES (gen_random_uuid(), 'todoist', 'ext-ddl-1', '项目规划', NOW() + INTERVAL '12 hours');
EOF
# 等待 1 轮轮询（启动时立即跑一次）或 15 分钟
# 期望: agent_runs 多 1 行 plan 任务, user_tasks.triggered_at 非空
```

---

## 4. 给下一窗口的提示

1. **`server.New` 签名变更**（`func New(cfg, trigSvc) *gin.Engine`）属内部重构，ai-coding-boundary §7.1 ✅ 范围。`cmd/api/main.go` 必须同步更新。
2. **JWT 认证不在 T009 范围**：plan 明确"T009 整合 + Phase 0 db/queue"，NFR-02 JWT 是单独任务。**不要**在 T009 加 JWT 中间件。
3. **webhook handler 注释清理**：`internal/handler/webhook.go:33` 注释 `// T009 整合时接入 EventBus / AgentRun 创建；此处先 200 返回` 在 T009 整合后会**自然失效**（handler 不再"先 200 返回"，会创建 AgentRun）。T009 实施时**必须删除**该注释（避免演化为 C1 占位）。
4. **DDL 调度器单实例**（T005 docker-compose 1 个 api 容器）：T008 fix #3 `RowsAffected == 0` 警告在单实例下不会触发。多实例部署需 `FOR UPDATE SKIP LOCKED` 或在 user_tasks 加 `agent_run_id` 关联字段。**T009 范围内不修**。
5. **`migrations/0003` 没跑**：与 0001 + 0002 一样，**等 T009 末用户在主窗口跑 `make migrate-up`** 一次性建 user_tasks 表。
6. **`DDLPollHandler` 返回 `error` 但 DDL handler 在 wire 里只是 log**：plan Step 1 写 `if err := h(...); err != nil { return err }`（wire ddlH），但 `RunDDLScheduler` 自己又 `log + continue`。**两层错误处理**都执行，外层 log + continue 实际不会返回 err 给调用方。
7. **`T008 fix #1` lead 参数保留**：当前 `ShouldTrigger(deadline, lead, now)` 第 2 个参数是 API 占位。如果 T009 不打算让 lead 可配置（保持 24h 硬编码），可考虑 T009 末在 main.go 里直接 `det := trigger.NewDDLDetector()` 而不传 lead。**不传** 是更优解（消除 API 占位），但 plan 写的是 wire，**T009 严格按 plan 字面即可**。
8. **T007 留下的 minor 项**（**不要**在 T009 范围处理）：
   - `internal/trigger/matcher.go:hit.raw` 死字段移除
   - `internal/handler/trigger.go` 加 `if h.Svc == nil` 显式保护
   - `internal/handler/trigger.go` 用 sentinel error 区分 "no rule matched" (4001) vs DB error (500)
   - T009 范围**只**做 wire + 路由整合，**不**回头改 T007 文件
9. **T008 留下的 minor 项**（**不要**在 T009 范围处理）：
   - DDL scheduler `run()` 加 panic recovery
   - T008 引入的 `[ddl-scheduler]` log 前缀 vs Phase 0/1 其它组件的一致性
10. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。
11. **`doc/handoff/` 仍 untracked**：T006/T007/T008 handoff 文件 + Phase 0 final 都未入库。等用户决定是否要单独 `git add doc/` 入库。
12. **T009 引入新依赖**（按 plan 字面）：plan 范围**无新依赖**。`uuid` / `pgxpool` / `httpx` / `asynq` 都已 direct。

---

## 5. 当前文件结构（Phase 1 / T008 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
├── cmd/
│   └── api/
│       └── main.go               ✅ T002 (启动 HTTP server，**未连 DB / 未启 Worker / 未接 trigger svc / 未启 DDL 调度器**)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md
│   │   ├── T002-handoff.md
│   │   ├── T003-handoff.md
│   │   ├── T004-handoff.md
│   │   ├── T005-handoff.md
│   │   ├── T006-handoff.md       (Phase 1 第一个 handoff，**未入库**)
│   │   ├── T007-handoff.md       (T007 handoff，**未入库**)
│   │   ├── T008-handoff.md       ← 本文件（**未入库**）
│   │   └── phase0-final-handoff.md  (Phase 0 总交接，**未入库**)
│   ├── plans/                    (00~05)
│   ├── mvp-definition.html
│   ├── plan-boundary.md
│   ├── prototype.html
│   ├── requirement-spec.html
│   └── task-tracker.html
├── internal/
│   ├── config/
│   │   └── config.go             ✅ T001+T006 (6 env vars)
│   ├── handler/
│   │   ├── health.go             ✅ T002
│   │   ├── health_test.go        ✅ T002
│   │   ├── util.go               ✅ T006
│   │   ├── webhook.go            ✅ T006 (含 // T009 整合 待清理注释)
│   │   ├── webhook_test.go       ✅ T006
│   │   ├── trigger.go            ✅ T007
│   │   └── trigger_test.go       ✅ T007
│   ├── middleware/               ✅ T002
│   ├── queue/                    ✅ T004 (T006/T007/T008 未动)
│   ├── repository/
│   │   ├── db.go                 ✅ T003 (Open)
│   │   ├── db_test.go            ✅ T003
│   │   └── agent_run.go          ✅ T007 (AgentRun + CreateAgentRun)
│   ├── server/
│   │   └── server.go             ✅ T002+T006 (T007/T008 未动，签名仍是 `func New(cfg)`)
│   └── trigger/                  ✅ T006+T007+T008
│       ├── event.go              (T006)
│       ├── webhook.go            (T006)
│       ├── webhook_test.go       (T006)
│       ├── matcher.go            (T007, sort.SliceStable)
│       ├── matcher_test.go       (T007, 4 测试)
│       ├── service.go            (T007, ProcessKeyword + createRun)
│       ├── service_test.go       (T007, gated integration)
│       ├── ddl.go                (T008, fix 后清 lead 死代码)
│       ├── ddl_test.go           (T008, 2 测试)
│       └── ddl_scheduler.go      (T008, fix 后 det.DefaultLead() + RowsAffected 检查)
├── pkg/
│   └── httpx/
│       └── response.go           ✅ T002
├── migrations/
│   ├── 0001_init.up.sql          ✅ T003 (6 表)
│   ├── 0001_init.down.sql        ✅ T003
│   ├── 0002_trigger_indexes.up.sql   ✅ T006 (2 索引)
│   ├── 0002_trigger_indexes.down.sql ✅ T006
│   ├── 0003_ddl.up.sql           ✅ T008 (user_tasks 表 + partial index + 注释)
│   └── 0003_ddl.down.sql         ✅ T008
├── scripts/
│   └── init-db.sql               ✅ T005
├── .dockerignore                 ✅ T005
├── .env.example                  ✅ T001+T006
├── .gitignore                    ✅ T001
├── Dockerfile                    ✅ T005
├── Makefile                      ✅ T001+T003+T005
├── docker-compose.yml            ✅ T005
├── go.mod                        ✅ T002+T003+T004+T006 (uuid v1.6.0 direct)
├── go.sum                        ✅ T002+T003+T004+T006
└── README.md                     ✅ T001
```

---

## 6. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **T008 commit 已自动落盘**（按用户"应用 4 fix 再 commit"选择）。
   - 本轮已 commit `2e2b43f`（5 files, +149 lines）。
   - 包含 4 个 code quality fix：清 lead 死代码 / `det.DefaultLead()` + `make_interval` 消除 24h 硬编码 / `RowsAffected` 检查 / partial index 加注释。

2. **T009 启动方式**：是否继续用 subagent-driven-development 流程？
   - **方案 A（推荐）**：继续 implementer → spec reviewer → code quality reviewer → 用户 commit（与 T006/T007/T008 一致）
   - **方案 B**：直接 implementer，跳过 review（更快但少两道质量门）
   - **方案 C**：T009 之后停一停，主窗口跑端到端（docker compose up + curl + psql 验证）→ 然后写 phase1-final-handoff

3. **T008 lead 参数占位的最终命运**：
   - 方案 A：T009 末保留 `_ = lead`（API 占位，文档化）
   - 方案 B：T009 末从 `ShouldTrigger` 删 lead 参数（API 干净）
   - **推荐 A**（R1 合规，不动 T008 已 commit 的签名）

4. **`webhook.go:33` 注释删除时机**：
   - T009 整合时**自然删除**（handler 不再"先 200 返回"，会创建 AgentRun）
   - **必须在 T009 实施时一并删**

5. **DDL 调度器并发安全**（多实例）：
   - 单实例（Phase 0 T005 docker-compose 默认）：不需要
   - 多实例（Phase 2+）：需 `FOR UPDATE SKIP LOCKED` 或 `user_tasks.agent_run_id` 关联
   - **T009 范围内不修**，在 phase1-final-handoff 标记给后续 Phase

6. **`migrations/0003` 何时跑**：
   - 方案 A：现在跑（需 docker 起来）
   - 方案 B：留到 T009 末 + 端到端验证时一起跑（推荐）
   - 注意：T009 的 `go build` 不会触发 migration；端到端必须先跑 `make migrate-up`

7. **T006/T007/T008 handoff 是否入库**：
   - 当前 3 个 handoff 文件 + Phase 0 final 都 untracked
   - 如果要入库：`git add doc/handoff/{T006,T007,T008}-handoff.md && git commit -m "docs(phase1): add task handoffs"`
   - 也可以等 T009 完成后一次性 `git add doc/handoff/` 入库

8. **`task-tracker.html` 是否更新**：
   - T006 + T007 + T008 都未在 task-tracker 更新
   - 建议：Phase 1 末（T009 完成后）一并更新

9. **T007/T008 留的 minor 项**（可在 T009 整合时统一处理）：
   - `matcher.go:hit.raw` 死字段移除
   - `handler/trigger.go` 加 `if h.Svc == nil` 显式保护
   - `handler/trigger.go` 用 sentinel error 区分 4001 vs 500
   - DDL scheduler `run()` 加 panic recovery
   - 这些都是 minor，可选；T009 主要做 wire + 路由整合，**不要**主动改这些

> **下一窗口（T009）开场建议**：
> 1. 读 `doc/handoff/T008-handoff.md`（本文件）
> 2. 读 `doc/plans/02-phase1-trigger.md` L1047–L1273 §Task T009
> 3. 读 T006/T007/T008 的关键文件（特别是 `trigger/service.go` + `trigger/ddl_scheduler.go` + `handler/webhook.go`）
> 4. 跑 `go test ./...` 确认 T006/T007/T008 测试仍 PASS（无回归）
> 5. 用 AskUserQuestion 问上面 §6 第 2、5、6 项（T009 启动方式 / DDL 并发 / migration 跑时）
> 6. Dispatch implementer subagent（带完整 T009 spec + T006/T007/T008 context + 本文件 §3.2 注意点）
> 7. 两轮 review 通过后展示 git diff，**用户决定 commit**
> 8. commit 后写 `doc/handoff/T009-handoff.md` + `doc/handoff/phase1-final-handoff.md`
