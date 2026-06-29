# Phase 1 / T009 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（implementer + spec reviewer + code quality reviewer + 1 fix subagent）
> **当前任务**: T009 — 任务调度器整合（FR-A01 / FR-A02 / FR-A03 整合）
> **状态**: ✅ **DONE**（implementer + spec + code quality + 1 fix 全过，commit `be96d66`）
> **基线 commit**: `2e2b43f` (T008)
> **本任务 commit**: `be96d66` (5 files, +285 / -10)
> **用户红线**: 本次不跑 docker / 不做端到端

---

## 1. 上一窗口做了什么（T009 做完了什么内容）

按 `doc/plans/02-phase1-trigger.md` §Task T009 (L1047–L1273) 完整执行了 10 个 step 中的 **1-5 + 10**。**Step 6-9（端到端 3 流程）按 plan 边界说明 + 用户决策主动跳过**（需 docker + DATABASE_URL）。

### 1.1 关联需求

- **FR-A01 (P0)**: Todoist webhook 触发
- **FR-A02 (P1)**: 关键词触发（4 类：周报/总结/规划/纪要）+ DDL 提前触发（1h-72h 可配置，当前 24h 硬编码）
- **FR-A03 (P2)**: 手动触发 `/api/v1/trigger`

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `cmd/api/wire.go` | **新建** | 75 | `Deps{Cfg, Trigger, Queue}` + `Build(ctx, cfg)` + DDL 调度器后台 goroutine + `Deps.Server()` |
| `cmd/api/main.go` | **修改** | 64 | ctx cancel + signal 优雅退出 + `http.Server` 显式 + `srv.Shutdown` 模式（I-1 fix 后） |
| `internal/server/server.go` | **重构** | 34 | `func New(cfg, trigSvc *trigger.Service) *gin.Engine` + 注册 `/api/v1/trigger` 路由 |
| `internal/handler/webhook.go` | **修改** | 76 | 加 `Svc *trigger.Service` 字段 + 接入 `trigSvc.ProcessKeyword` + 删 L33 注释 + M-1 nil-Svc log 警告 |
| `internal/handler/webhook_test.go` | **扩展** | 165 | 保留 ValidSignature / InvalidSignature + 新增 Matched / NoMatch（DB SKIP）+ decodeData helper + 删重复 NoSvc（M-2） |

> **新建 1 个文件 + 修改 4 个文件 = T009 总变更 5 个对象**。

### 1.3 I-1 + M-1 + M-2 修复（code quality review 后由 fix subagent 直接修复）

| # | Fix | 文件:行 | 原问题 | 修复后 |
|---|---|---|---|---|
| I-1 | 优雅退出实际不退出 | `main.go:32-63` | `deps.Server().Run(addr)` 是 `gin.Engine.Run`，对 ctx / signal 无感知 → SIGINT 后进程挂起 → `defer deps.Queue.Close()` 死代码 | 显式 `http.Server` + `ListenAndServe` goroutine + `select{sig, serveErr}` + `srv.Shutdown(shCtx)` + `cancel()`；`http.ErrServerClosed` 走 `return` |
| M-1 | nil Svc 静默吞工作 | `webhook.go:53-59` | `Svc=nil` 路径返回 `200 + matched=true`，外部 webhook 端误以为匹配成功 | nil 分支加 `log.Printf("[webhook] Svc=nil, ...")` 警告 + 返回 `matched: false` |
| M-2 | 重复测试用例 | `webhook_test.go:67-93` | `TestWebhook_Todoist_NoSvc` 与 `TestWebhook_Todoist_ValidSignature` 输入/断言完全等价 | 删 `NoSvc`，更新 `ValidSignature` 注释和断言为 `matched: false`（反映 M-1 修复后行为） |

### 1.4 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go test -count=1 ./...` | ✅ PASS | handler + queue + repository + trigger 全部包 |
| `go test -v ./internal/handler/...` | ✅ 4 PASS + 2 SKIP | TestHealth + TestTriggerHandler_BadRequest + TestWebhook_Todoist_ValidSignature/InvalidSignature + Matched/NoMatch（DB SKIP） |
| `go test -v ./internal/trigger/...` | ✅ 11 PASS + 1 SKIP | T006 3 + T007 4 + T008 5 + service integration SKIP（无回归） |
| `git commit` | ✅ `be96d66` | 由 controller 在 I-1+M-1+M-2 修复 + 用户授权后执行 |

> ⚠️ **未跑**（用户红线）：
> - `make migrate-up`（需 docker postgres）
> - 端到端 3 流程（需 docker + T009 整合验证）
> - JWT 认证中间件（属 NFR-02 单独任务，**T009 范围外**）

### 1.5 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `Deps.Server()` 返回 `*gin.Engine` 而非 `*server.Server` | plan 写 `*server.Server` 是 spec 笔误（T002 server.New 已实现返回 `*gin.Engine`） | ✅ 修正 plan spec 笔误 |
| 2 | `I-1` 优雅退出改造（plan spec 字面有缺陷） | plan 写 `signal.Notify` + `deps.Server().Run`，SIGINT 后进程挂起；按 plan 字面实现"defer Close"是死代码 | ✅ I-1 review 后由用户决策修复 |
| 3 | `M-1` nil Svc log 警告 | plan 未明确 Svc==nil 行为 | ✅ review 后由用户决策修复 |
| 4 | `M-2` 删 `NoSvc` 测试 | 与 `ValidSignature` 等价 | ✅ review 后由用户决策修复 |
| 5 | webhook handler `"no rule matched"` 字符串比较 | trigger.Service 用 `fmt.Errorf`（T007），无 typed sentinel | ✅ T008 handoff §4.8 已知 minor，不在 T009 范围 |
| 6 | webhook `user_id` 用 `uuid.Nil` 占位 | plan 未定义 webhook→user 映射 | ✅ Phase 2+ 引入 user identity provider 后替换 |
| 7 | DDL handler `taskID` 用 `_ = taskID` 忽略 | plan 范围内不传 AgentRun ← user_task 关联 | ✅ T008 handoff §3.2 #3 已说明 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖、未写 `TODO` / `FIXME`、未自动 commit 前实现、未硬编码密钥、未跨 Phase。

### 1.6 git log 输出（T009 commit 后）

```
be96d66 feat(phase1/T009): wire trigger pipeline (webhook/keyword/ddl -> agent_run) + graceful shutdown
2e2b43f feat(phase1/T008): ddl detector + 15min scheduler (user_tasks table + lead-param drift fix)
46c3403 feat(phase1/T007): keyword matcher (longest-match-wins) + agent_run persistence + manual trigger handler
6a4a9fb feat(phase1/T006): webhook listener with hmac-sha256 verification + trigger indexes
f58c67b feat(phase0/T005): docker + docker-compose one-shot dev environment
```

---

## 2. 上一窗口**没做**什么

- ❌ **没有**跑 `make migrate-up`（user_tasks 表未在 docker postgres 创建）
- ❌ **没有**端到端 3 流程验证（需 docker + 端到端主窗口跑）
- ❌ **没有**加 JWT 认证中间件（属 NFR-02 单独任务）
- ❌ **没有**实现"重复 event_id webhook 去重"（FR-A01 验收标准之一，Phase 2+ 范围）
- ❌ **没有**加 panic recovery 到 DDL scheduler（reviewer minor，留后续 Phase）
- ❌ **没有**加 webhook → asynq enqueue 逻辑（Phase 2+ 范围）
- ❌ **没有**改 trigger 包任何文件（T006/T007/T008 已完成）
- ❌ **没有**改 queue / repository / middleware / config / migrations 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**处理 M-3 ~ M-8 minor（累计到 phase1-final 一起修）

---

## 3. 下一步需要实现什么 — Phase 2 上下文搜集入口

> **重要**：T009 末所有 trigger 部件（webhook / keyword / DDL）+ Phase 0 db/queue + wire 整合完成。Phase 2 任务是**上下文搜集**（T010-T014）。

### 3.1 Phase 2 任务范围

来源：`doc/plans/03-phase2-context.md`（待下一窗口读完整 spec）

按 T008-handoff 推断：

| 任务 | 关键产物 |
|---|---|
| T010 | 数据源适配器（GitHub / RSS / Notion 拉取） |
| T011 | LLM 客户端（prompt 模板 + token 控制） |
| T012 | 上下文组装（datasources → LLM → context） |
| T013 | ChromaDB / pgvector 向量存储（**注意**：Phase 0 T005 已选 pgvector/pg16 镜像） |
| T014 | 检索增强生成（RAG）管道 |

### 3.2 Phase 2 关键决策点（需下一窗口与用户确认）

1. **LLM 选型**：plan 字面未指定（OpenAI / Anthropic / 自托管 Ollama / vLLM）。NFR-01 提到"延迟 < 5s"，需要决定**先接入哪一家**。
2. **Embedding 模型**：同上，plan 未指定。需决定 `text-embedding-3-small` (OpenAI) 还是开源 `bge-small-en`。
3. **向量存储索引类型**：pgvector 支持 IVFFLAT 和 HNSW（pgvector ≥ 0.5）。需要在 Phase 0 已有 pg16 镜像上确认扩展是否已 `CREATE EXTENSION vector`。
4. **webhook → user_id 映射**：T009 用了 `uuid.Nil` 占位。Phase 2 需要引入 user identity provider（如 NextAuth / Clerk / 自建 JWT）。NFR-02 JWT 是该任务的延伸。
5. **重复 event_id 去重**：FR-A01 验收标准之一，Phase 2+ 实施。T009 范围未做。建议在 Phase 2 早期任务（T010?）中实现，因为 webhook 处理会接触事件存储。
6. **DDL 调度器多实例并发**：T008 fix #3 `RowsAffected == 0` 警告在多实例部署时会触发。Phase 2+ 引入横向扩容时需要 `FOR UPDATE SKIP LOCKED` 或 `user_tasks.agent_run_id` 关联字段。**T009 范围不修**。

### 3.3 Phase 2 不做的事（避免越界）

- ❌ **不要**实现 Phase 3（合成）+ Phase 4（交付）的功能
- ❌ **不要**改 trigger 包任何文件（T006-T009 已完成）
- ❌ **不要**改 server.New 签名（已稳定为 `(cfg, trigSvc)`）
- ❌ **不要**改 wire.go 的 DDL 调度器回调（plan 范围内不传 AgentRun ← user_task 关联；T009 handoff §1.5 #7）
- ❌ **不要**加 Phase 1 已就位的功能（事件总线、用户管理、JWT 认证不在 Phase 1）

### 3.4 建议执行顺序（下一窗口 Phase 2 T010）

1. 读 `doc/handoff/phase1-final-handoff.md`（即将写，见 §6）
2. 读 `doc/handoff/T009-handoff.md`（本文件）
3. 读 `doc/plans/03-phase2-context.md` 完整 spec
4. 跑 `go test ./...` 确认 T006-T009 测试仍 PASS（无回归）
5. 跑 `docker compose up -d && make migrate-up` 创建 `user_tasks` 表（Phase 0 + T008 migration 落地）
6. 跑端到端 3 流程验证 Phase 1 退出标准（plan §Phase 1 退出标准验证）
7. 用 AskUserQuestion 确认 Phase 2 启动方式 + LLM/Embedding 选型
8. Dispatch implementer subagent（带完整 Phase 2 spec + T006-T009 context + 上述注意点）
9. 两轮 review 通过后展示 git diff，**用户决定 commit**
10. commit 后写 `doc/handoff/T010-handoff.md` 等

### 3.5 Phase 1 端到端验证（需 docker + 端到端主窗口跑）

T009 末的 `go test / go build` 只验证编译。**端到端必须**主窗口跑：

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

# 3. 手动触发（FR-A03）
UID=$(uuidgen)
curl -X POST http://localhost:8080/api/v1/trigger \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$UID\",\"text\":\"写本周周报\"}"
# 期望: {"code":0,"message":"ok","data":{"run_id":"..."}}

# 4. 验证数据库
docker exec asyncstarter_postgres psql -U starter -d starter -c \
  "SELECT id, task_type, trigger_type, status FROM agent_runs ORDER BY created_at DESC LIMIT 5;"
# 期望: 1 行, task_type=weekly_report, trigger_type=keyword

# 5. Webhook 触发（FR-A01）
BODY='{"event_name":"item:added","event_id":"evt-2","event_data":{"id":"t2","content":"项目总结"}}'
SIG=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "test-secret" | awk '{print $2}')
curl -X POST http://localhost:8080/api/v1/webhook/todoist \
  -H "Content-Type: application/json" \
  -H "X-Todoist-HMAC-SHA256: $SIG" \
  -d "$BODY"
# 期望: 200 OK + matched=true + agent_runs 多 1 行 summary 任务

# 6. DDL 触发（FR-A02）
docker exec asyncstarter_postgres psql -U starter -d starter <<EOF
INSERT INTO user_tasks (user_id, source, external_id, title, deadline_at)
VALUES (gen_random_uuid(), 'todoist', 'ext-ddl-1', '项目规划', NOW() + INTERVAL '12 hours');
EOF
# 等待 1 轮轮询（启动时立即跑一次）或 15 分钟
# 期望: agent_runs 多 1 行 plan 任务, user_tasks.triggered_at 非空
```

---

## 4. 给下一窗口的提示

1. **`server.New` 签名稳定**（`func New(cfg, trigSvc *trigger.Service) *gin.Engine`）— 任何 Phase 2 任务**不要**改这个签名。`trigSvc` 注入为后续业务 handler 留口子（如 `/api/v1/run/:id` GET）。

2. **JWT 认证不在 Phase 1 范围**：NFR-02 是单独任务，可能在 Phase 2 早期。**不要**在 Phase 2 第一批任务里加 JWT 中间件，除非 plan §Phase 2 spec 明确要求。

3. **webhook handler `user_id` 占位**：`internal/handler/webhook.go:63` 用 `uuid.Nil`，Phase 2+ 引入 user identity provider 后替换。**当前 `agent_runs` 表的 `user_id` 字段接收 `uuid.Nil`**。

4. **DDL 调度器单实例**（T005 docker-compose 1 个 api 容器）：T008 fix #3 `RowsAffected == 0` 警告在单实例下不会触发。多实例部署（Phase 2+ 横向扩容）需 `FOR UPDATE SKIP LOCKED` 或在 user_tasks 加 `agent_run_id` 关联字段。**Phase 2 范围不修**。

5. **`migrations/0003` 没跑**：与 0001 + 0002 一样，**等 Phase 2 窗口在主窗口跑 `make migrate-up`** 一次性建 user_tasks 表。

6. **DDLPollHandler 返回 `error` 但 DDL handler 在 wire 里只是 log**：`RunDDLScheduler` 自己又 `log + continue`，**两层错误处理**都执行，外层 log + continue 实际不会返回 err 给调用方。Phase 2+ 引入 metrics/tracing 时需重构成统一错误处理。

7. **T008 fix #1 lead 参数保留**：当前 `ShouldTrigger(deadline, lead, now)` 第 2 个参数是 API 占位。如果 Phase 2 打算让 lead 可配置（保持 24h 硬编码），可考虑在 Phase 2 末从 `ShouldTrigger` 删 lead 参数。**Phase 2 范围不修**。

8. **T006/T007/T008/T009 留的 minor 项**（**不要**在 Phase 2 范围处理）：
   - `internal/trigger/matcher.go:hit.raw` 死字段移除
   - `internal/handler/trigger.go` 加 `if h.Svc == nil` 显式保护
   - `internal/handler/trigger.go` 用 sentinel error 区分 4001 vs 500
   - DDL scheduler `run()` 加 panic recovery
   - `log` 前缀风格统一 `[ddl-scheduler]` vs `[ddl]`
   - 测试样板代码提取 helper（M-3）
   - `decodeData` helper 改名 `decodeDataMap`（M-4）
   - `log.Fatalf` 绕过 defers（M-6）

9. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。

10. **`doc/handoff/` 状态**：T006/T007/T008 handoff + Phase 0 final + T009 handoff + phase1-final handoff 仍未入库（待用户决定是否要单独 `git add doc/` 入库）。

11. **Phase 2 引入新依赖**（待 plan §Phase 2 spec 决定）：可能包括 `github.com/jackc/pgx/v5/pgvector`、LLM 客户端（`github.com/sashabaranov/go-openai` 或 `github.com/anthropics/anthropic-sdk-go`）、embedding 模型（按选型）。**M8 红线**：`go get <pkg>@latest`（不锁版本）；major bump 需 `decision-log.md` 记录。

---

## 5. 当前文件结构（Phase 1 / T009 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
├── cmd/
│   └── api/
│       ├── main.go               ✅ T002 + T009 (Build + signal + http.Server.Shutdown)
│       └── wire.go               ✅ T009 (Deps + Build + DDL scheduler goroutine)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md       (untracked)
│   │   ├── T002-handoff.md       (untracked)
│   │   ├── T003-handoff.md       (untracked)
│   │   ├── T004-handoff.md       (untracked)
│   │   ├── T005-handoff.md       (untracked)
│   │   ├── T006-handoff.md       (untracked)
│   │   ├── T007-handoff.md       (untracked)
│   │   ├── T008-handoff.md       (untracked)
│   │   ├── T009-handoff.md       ← 本文件 (untracked)
│   │   ├── phase0-final-handoff.md  (untracked)
│   │   └── phase1-final-handoff.md  (待写, untracked)
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
│   │   ├── webhook.go            ✅ T006+T009 (Svc + ProcessKeyword + 删 L33 注释)
│   │   ├── webhook_test.go       ✅ T006+T009 (4 tests + decodeData helper)
│   │   ├── trigger.go            ✅ T007
│   │   └── trigger_test.go       ✅ T007
│   ├── middleware/               ✅ T002
│   ├── queue/                    ✅ T004 (T009 未动)
│   ├── repository/
│   │   ├── db.go                 ✅ T003 (Open)
│   │   ├── db_test.go            ✅ T003
│   │   └── agent_run.go          ✅ T007 (AgentRun + CreateAgentRun)
│   ├── server/
│   │   └── server.go             ✅ T002+T006+T009 (New(cfg, trigSvc) + /api/v1/trigger)
│   └── trigger/                  ✅ T006+T007+T008 (T009 未动)
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

## 6. Phase 1 退出标准验证

完成 T006-T009 后，逐项验证 M1 退出标准：

| # | 标准 | T009 末状态 |
|---|---|---|
| 1 | `go test ./...` → 全部 PASS | ✅ T006 3 + T007 4 + T008 5 + T009 4 = 16 PASS + 1 SKIP（service integration）+ 2 SKIP（webhook integration） |
| 2 | Todoist webhook → 200 + 数据库新增 AgentRun | ⚠️ T009 代码已实现（`webhook.go:64` → `trigSvc.ProcessKeyword` → `createRun`），**端到端需主窗口 docker 验证** |
| 3 | 关键词 "周报/总结/规划/纪要" → 数据库新增 AgentRun | ⚠️ T009 代码已实现（`main.go:25 Build` → `trigger.NewService` → `cmd/api/main.go:25`），**端到端需主窗口 docker 验证** |
| 4 | 重复 event_id webhook → 数据库只新增 1 行（去重） | ❌ **未实现**（FR-A01 验收标准之一，Phase 2+ 范围） |
| 5 | DDL 12h 后到期任务 → 启动后 1 轮内触发 | ⚠️ T009 代码已实现（`wire.go:65 go trigger.RunDDLScheduler`），**端到端需主窗口 docker 验证** |
| 6 | 更新 [task-tracker.html](../../task-tracker.html) 中 T006-T009 状态 | ❌ **未更新**（可选，Phase 1 末可一并更新） |

> **Phase 1 形式上完成**（代码 + 测试 + wire + 优雅退出），**端到端验证 4/6 需主窗口 docker 跑**。

---

## 7. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **T009 commit 已自动落盘**（按用户"应用 4 fix 再 commit"选择）。
   - 本轮已 commit `be96d66`（5 files, +285 / -10）。
   - 包含 I-1 + M-1 + M-2 修复。

2. **`doc/handoff/` 是否入库**：
   - 当前 9 个 handoff 文件 + 1 个 phase0-final + 1 个 phase1-final（待写）= 11 个文件 untracked
   - 如果要入库：`git add doc/handoff/` + `git commit -m "docs(phase1): add task handoffs for T001-T009 + phase0/1 final"`
   - 也可以等 Phase 2 完成后一次性入库

3. **`task-tracker.html` 是否更新**：
   - T006 + T007 + T008 + T009 都未在 task-tracker 更新
   - 建议：Phase 1 末一并更新

4. **Phase 1 端到端验证时机**：
   - 方案 A：现在跑（需 docker 起来）
   - 方案 B：留到 Phase 2 启动前跑
   - 方案 C：Phase 1 直接跳过端到端，依赖 Phase 2 任务的 docker 验证

5. **重复 event_id webhook 去重**（FR-A01 验收标准）：
   - Phase 2 早期任务（建议 T010 之前）
   - 需要在 `webhook_events` 表或 `agent_runs.event_id` 唯一约束

6. **T007/T008/T009 留的 minor 项**（可在 Phase 2 早期任务统一处理）：
   - `matcher.go:hit.raw` 死字段移除
   - `handler/trigger.go` 加 `if h.Svc == nil` 显式保护
   - `handler/trigger.go` 用 sentinel error 区分 4001 vs 500
   - DDL scheduler `run()` 加 panic recovery
   - `log` 前缀风格统一 `[ddl-scheduler]` vs `[ddl]`
   - 测试样板代码提取 helper（M-3）
   - `decodeData` helper 改名 `decodeDataMap`（M-4）
   - `log.Fatalf` 绕过 defers（M-6）

7. **Phase 2 启动方式**：
   - **方案 A（推荐）**：继续 implementer → spec reviewer → code quality reviewer → 用户 commit
   - **方案 B**：直接 implementer，跳过 review
   - **方案 C**：Phase 2 整体规划后再分任务

> **下一窗口（Phase 2 第一任务）开场建议**：
> 1. 读 `doc/handoff/phase1-final-handoff.md`（即将写）
> 2. 读 `doc/handoff/T009-handoff.md`（本文件）
> 3. 读 `doc/plans/03-phase2-context.md` 完整 spec
> 4. 跑 `docker compose up -d && make migrate-up` 落地 3 个 migration
> 5. 跑 Phase 1 端到端 3 流程验证（plan §Phase 1 退出标准验证）
> 6. 用 AskUserQuestion 问上面 §7 第 4、5、7 项
> 7. Dispatch implementer subagent（带完整 Phase 2 spec + Phase 1 context + 上述注意点）
> 8. 两轮 review 通过后展示 git diff，**用户决定 commit**
> 9. commit 后写 `doc/handoff/T010-handoff.md` 等
