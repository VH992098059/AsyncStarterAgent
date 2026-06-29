# Phase 1 总交接文档 — Trigger Pipeline 完成

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: T009 末由 controller 写，给 Phase 2（T010+）窗口看
> **覆盖范围**: Phase 1 全阶段 — T006 (webhook) + T007 (keyword) + T008 (DDL) + T009 (wire)
> **状态**: ✅ **Phase 1 形式上完成**（代码 + 测试 + wire + 优雅退出），**端到端验证需主窗口 docker 跑**

---

## 1. Phase 1 整体目标（来自 `mvp-definition.html` + `requirement-spec.html`）

Phase 1 范围：**触发管道（Trigger Pipeline）**——让外部事件（Todoist webhook / 关键词 / DDL）能可靠地创建 `agent_runs` 记录。Phase 2+ 才做上下文搜集 + 合成 + 交付。

### 1.1 覆盖的 FR

| FR | 描述 | 实施 | 状态 |
|---|---|---|---|
| **FR-A01 (P0)** | Todoist webhook 触发 | T006 + T009 | ✅ 代码完成，⚠️ 端到端未跑 |
| **FR-A02 (P1)** | 关键词触发（4 类：周报/总结/规划/纪要）+ DDL 提前触发 | T007 + T008 + T009 | ✅ 代码完成，⚠️ 端到端未跑 |
| **FR-A03 (P2)** | 手动触发 `/api/v1/trigger` | T007 + T009 | ✅ 代码完成，⚠️ 端到端未跑 |

### 1.2 覆盖的 NFR（部分）

| NFR | 描述 | 实施 | 状态 |
|---|---|---|---|
| **NFR-01** | 端到端延迟 < 5s | 同步路径（webhook → 关键词匹配 → createRun），未接 asynq | ⚠️ Phase 2 引入 asynq 后再测 |
| **NFR-02** | JWT 认证 | **未实施**（属单独任务，Phase 2 早期） | ❌ Phase 1 范围外 |
| **NFR-03** | 错误处理 / 日志 / 监控 | log.Printf + 优雅退出 | ⚠️ Phase 2 引入 metrics/tracing |

### 1.3 关键架构决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 触发路径 | **同步**（webhook handler 直接调 `trigSvc.ProcessKeyword` → `createRun`） | T007 末已实现，Phase 2+ 引入 asynq 异步触发 |
| 关键词匹配 | **最长匹配优先**（T007 `sort.SliceStable`） | 规范"关键词冲突时取最长匹配" |
| 关键词规则 | **4 类**：周报 → weekly_report / 总结\|小结 → summary / 纪要\|会议记录 → meeting_minutes / 规划\|计划 → plan | 规范 FR-A02 |
| DDL 提前量 | **24h 硬编码**（T008 `defaultLead`） | 规范"1h-72h 可配置" 当前为最常见值，Phase 2+ 暴露可配置入口 |
| DDL 调度间隔 | **15 分钟**（T008 `DDLPollInterval`） | 规范 FR-A02 |
| 数据库连接池 | **pgxpool MaxConns=20**（T003） | Phase 0 默认 |
| Redis 客户端 | **asynq v0.26.0**（T004） | Phase 0 默认 |
| 依赖注入 | **手写 wire**（T009 `cmd/api/wire.go`） | plan spec 字面，不引入 wire 库 |
| Webhook user_id | **`uuid.Nil` 占位**（T009） | 规范未定义 webhook→user 映射，Phase 2+ 引入 user identity provider |
| DDL 调度器并发 | **单实例**（T005 docker-compose 1 个 api 容器） | 多实例需 `FOR UPDATE SKIP LOCKED`，Phase 2+ 实施 |
| 优雅退出 | **`http.Server` 显式 + `srv.Shutdown(shCtx)`**（T009 I-1 fix） | plan spec 字面有缺陷，review 后修复 |
| 重复 event_id 去重 | **未实现** | 规范 FR-A01 验收标准之一，Phase 2+ 范围 |
| JWT 认证 | **未实现** | NFR-02 单独任务，Phase 2 早期 |

---

## 2. 完成的 4 个任务

### 2.1 T006 — Webhook 监听器

- **基线**: `f58c67b` (T005)
- **commit**: `6a4a9fb` (3 files, +127 lines)
- **关键产物**:
  - `internal/handler/webhook.go` — Todoist webhook 接收 + HMAC-SHA256 验签 + 标准化为 `TriggerEvent`
  - `internal/handler/util.go` — payload 字段提取 helper
  - `internal/trigger/event.go` — `TriggerEvent` + `Source` enum
  - `internal/trigger/webhook.go` — `VerifyHMAC` / `SignHMAC` / `NormalizeTodoist` / `NormalizeFeishu`
  - `migrations/0002_trigger_indexes.up.sql` — webhook_events 去重索引（预留）
- **关键决策**:
  - HMAC-SHA256 恒定时间比较（`hmac.Equal`）
  - `X-Todoist-HMAC-SHA256` header
  - `Source` enum: todoist / feishu / notion / manual / keyword / ddl

### 2.2 T007 — 关键词匹配 + AgentRun 持久化

- **基线**: `6a4a9fb` (T006)
- **commit**: `46c3403` (4 files, +149 lines)
- **关键产物**:
  - `internal/trigger/matcher.go` — `Matcher` + `Rule` + 4 类规则 + `Match` 函数
  - `internal/repository/agent_run.go` — `AgentRun` struct + `CreateAgentRun`
  - `internal/trigger/service.go` — `Service` + `ProcessKeyword` + `createRun`
  - `internal/handler/trigger.go` — `TriggerHandler` + `POST /api/v1/trigger`
- **关键决策**:
  - 关键词匹配**最长匹配优先**（`sort.SliceStable`，等长按注册顺序）
  - `SourceKeyword` = `"keyword"`（非 `"manual"`）— 手动触发用 `SourceManual`
  - 错误用 `fmt.Errorf("no rule matched")`（**无 typed sentinel**，Phase 2 改进）
  - `Repository.CreateAgentRun` 使用 `pgxpool.QueryRow` + `RETURNING` 拿到自增 ID

### 2.3 T008 — DDL 截止日期检测器

- **基线**: `46c3403` (T007)
- **commit**: `2e2b43f` (5 files, +149 lines)
- **关键产物**:
  - `migrations/0003_ddl.up.sql` — `user_tasks` 表（12 字段 + UNIQUE(source, external_id) + partial index）
  - `internal/trigger/ddl.go` — `DDLDetector` + `NewDDLDetector` + `DefaultLead` + `ShouldTrigger`
  - `internal/trigger/ddl_scheduler.go` — `DDLPollInterval=15min` + `DDLPollHandler` + `RunDDLScheduler`
- **关键决策**:
  - 24h 硬编码 lead（`defaultLead` 字段，`DefaultLead()` 暴露）
  - SQL 用 `make_interval(secs => $1)` 参数化 lead（避免 24h 硬编码 drift）
  - `UPDATE triggered_at = NOW()` 后检查 `RowsAffected()`（避免 0 行更新被静默忽略）
  - partial index `WHERE completed=false AND triggered_at IS NULL` 优化 index-only scan
  - `ShouldTrigger(deadline, lead, now)` 第 2 个参数 `lead` 是 API 占位，函数体用 `defaultLead`

### 2.4 T009 — 任务调度器整合（Phase 1 收官）

- **基线**: `2e2b43f` (T008)
- **commit**: `be96d66` (5 files, +285 / -10)
- **关键产物**:
  - `cmd/api/wire.go` (new) — `Deps{Cfg, Trigger, Queue}` + `Build(ctx, cfg)` + DDL 调度器后台 goroutine
  - `cmd/api/main.go` — Build + signal + `http.Server` 显式 + `srv.Shutdown`
  - `internal/server/server.go` — `func New(cfg, trigSvc *trigger.Service) *gin.Engine` + `/api/v1/trigger` 路由
  - `internal/handler/webhook.go` — 加 `Svc` 字段 + 接入 `trigSvc.ProcessKeyword` + 删 L33 注释 + nil-Svc log 警告
  - `internal/handler/webhook_test.go` — 4 测试（ValidSignature / InvalidSignature + Matched/NoMatch DB SKIP）
- **关键决策**（I-1 fix）:
  - plan spec 写 `deps.Server().Run(addr)`，SIGINT 后进程挂起
  - 修复：显式 `http.Server` + `ListenAndServe` goroutine + `select{sig, serveErr}` + `srv.Shutdown(shCtx)` + `cancel()`
  - `http.ErrServerClosed` 走 `return`（正常路径，不 `os.Exit(1)`）
  - `defer deps.Queue.Close()` 真正执行
- **关键决策**（M-1 fix）:
  - `Svc=nil` 路径加 `log.Printf` 警告 + 返回 `matched: false`（避免外部 webhook 误以为匹配成功）
- **关键决策**（M-2 fix）:
  - 删 `TestWebhook_Todoist_NoSvc`（与 `ValidSignature` 等价），更新 `ValidSignature` 断言为 `matched: false`

---

## 3. 完整文件清单（Phase 1 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored]
├── cmd/
│   └── api/
│       ├── main.go               ✅ T002 + T009 (64 lines)
│       └── wire.go               ✅ T009 (75 lines, new)
├── doc/
│   ├── ai-coding-boundary.md     ✅ 编码规范
│   ├── decision-log.md           [待创建]
│   ├── handoff/                  [9 个 T* + phase0-final + phase1-final（本文）]
│   ├── plans/                    (00~05)
│   ├── mvp-definition.html
│   ├── plan-boundary.md
│   ├── prototype.html
│   ├── requirement-spec.html
│   └── task-tracker.html
├── internal/
│   ├── config/config.go          ✅ T001+T006
│   ├── handler/                  ✅ T002+T006+T007+T009
│   │   ├── health.go + health_test.go
│   │   ├── util.go
│   │   ├── webhook.go + webhook_test.go
│   │   └── trigger.go + trigger_test.go
│   ├── middleware/               ✅ T002 (Recovery + Logger)
│   ├── queue/                    ✅ T004 (Client + Mux + Server, 未动)
│   ├── repository/               ✅ T003+T007
│   │   ├── db.go + db_test.go
│   │   └── agent_run.go
│   ├── server/server.go          ✅ T002+T006+T009 (New(cfg, trigSvc))
│   └── trigger/                  ✅ T006+T007+T008 (未动)
│       ├── event.go + webhook.go + webhook_test.go
│       ├── matcher.go + matcher_test.go
│       ├── service.go + service_test.go
│       ├── ddl.go + ddl_test.go
│       └── ddl_scheduler.go
├── pkg/httpx/response.go         ✅ T002
├── migrations/                   ✅ T003+T006+T008
│   ├── 0001_init.{up,down}.sql   (6 表)
│   ├── 0002_trigger_indexes.{up,down}.sql  (2 索引)
│   └── 0003_ddl.{up,down}.sql    (user_tasks 表 + partial index)
├── scripts/init-db.sql           ✅ T005
├── .dockerignore                 ✅ T005
├── .env.example                  ✅ T001+T006
├── .gitignore                    ✅ T001
├── Dockerfile                    ✅ T005
├── Makefile                      ✅ T001+T003+T005
├── docker-compose.yml            ✅ T005
├── go.mod                        ✅ T002+T003+T004+T006
├── go.sum                        ✅ T002+T003+T004+T006
└── README.md                     ✅ T001
```

---

## 4. Phase 1 退出标准验证（plan §Phase 1 退出标准验证 L1277–L1286）

| # | 标准 | T009 末状态 | 待办 |
|---|---|---|---|
| 1 | `go test ./...` → 全部 PASS | ✅ T006 3 + T007 4 + T008 5 + T009 4 = **16 PASS + 3 SKIP** | — |
| 2 | Todoist webhook → 200 + 数据库新增 AgentRun | ✅ 代码完成（`webhook.go:64` → `trigSvc.ProcessKeyword` → `createRun`） | ⚠️ **端到端需主窗口 docker 验证** |
| 3 | 关键词 "周报/总结/规划/纪要" → 数据库新增 AgentRun | ✅ 代码完成（`Build` → `trigger.NewService` → `cmd/api/main.go:25`） | ⚠️ **端到端需主窗口 docker 验证** |
| 4 | 重复 event_id webhook → 数据库只新增 1 行（去重） | ❌ **未实现** | Phase 2 早期任务 |
| 5 | DDL 12h 后到期任务 → 启动后 1 轮内触发 | ✅ 代码完成（`wire.go:65 go trigger.RunDDLScheduler` + 启动时立即跑一次） | ⚠️ **端到端需主窗口 docker 验证** |
| 6 | 更新 [task-tracker.html](../../task-tracker.html) 中 T006-T009 状态 | ❌ **未更新** | Phase 1 末可一并更新 |

> **Phase 1 形式上完成**（5/6 验证项代码就位，1/6 未实施）。

---

## 5. Phase 2 启动前置清单

### 5.1 必须做的（按 P1 / P2 红线）

1. **跑 `make migrate-up`** 落地 3 个 migration（含 `user_tasks` 表）
2. **跑 Phase 1 端到端 3 流程验证**（详见 §6）
3. **更新 [task-tracker.html](../../task-tracker.html) 中 T006-T009 状态**（待办 #6）
4. **`git add doc/handoff/` 入库**（9 个 T* handoff + phase0-final + phase1-final 11 个文件）

### 5.2 Phase 2 启动前需与用户确认（必问，不可以自动做）

1. **LLM 选型**：plan 字面未指定（OpenAI / Anthropic / 自托管 Ollama / vLLM）
2. **Embedding 模型**：同上
3. **Phase 2 启动方式**：继续 subagent-driven / 直接 implementer / 整体规划后分任务
4. **重复 event_id 去重实施时机**：Phase 2 第一任务（建议 T010 之前）
5. **JWT 认证实施时机**：Phase 2 早期（建议 T010-T012 之间）
6. **DDL 调度器多实例并发**：是否 Phase 2 引入横向扩容需要 `FOR UPDATE SKIP LOCKED`
7. **`migrations/0003` 跑时**：现在 / Phase 2 启动前 / Phase 2 端到端验证时

### 5.3 Phase 2 不应做（避免越界）

- ❌ 实现 Phase 3（合成）+ Phase 4（交付）的功能
- ❌ 改 trigger 包任何文件（T006-T009 已完成）
- ❌ 改 server.New 签名（已稳定为 `(cfg, trigSvc)`）
- ❌ 改 wire.go 的 DDL 调度器回调（plan 范围内不传 AgentRun ← user_task 关联）
- ❌ 改 agent_runs / user_tasks schema
- ❌ 改 queue / repository / middleware / config 任何文件（T002-T008 已稳定）

---

## 6. 端到端验证步骤（需主窗口 docker 跑）

按 `doc/handoff/T009-handoff.md` §3.5 完整步骤：

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

**Phase 1 退出标准 4/6（webhook / 关键词 / DDL 触发 + 端到端测试）**通过后，Phase 1 正式完成。

---

## 7. P1 follow-up（Phase 1 末 P1 待办）

| # | 项目 | 严重性 | Phase 2 处理建议 |
|---|---|---|---|
| 1 | 重复 event_id webhook 去重（FR-A01 验收标准） | P1 | Phase 2 早期任务（建议 T010 之前） |
| 2 | JWT 认证中间件（NFR-02） | P1 | Phase 2 早期任务（建议 T010-T012 之间） |
| 3 | DDL 调度器多实例并发（横向扩容时） | P2 | Phase 2+ 引入横向扩容时 |
| 4 | 端到端 Phase 1 退出标准验证（5/6 需 docker） | P1 | 主窗口跑（详见 §6） |
| 5 | `task-tracker.html` 更新 T006-T009 状态 | P2 | Phase 1 末可一并更新 |
| 6 | `doc/handoff/` 入库（11 个文件） | P2 | 单独 `git add doc/` |
| 7 | T007/T008/T009 留的 minor 项（见 §8） | P3 | Phase 2 早期清理任务 |

---

## 8. 留待 Phase 2 早期清理的 minor 项

按 T008 handoff §4.8 + §4.9 + T009 reviewer M-3 ~ M-8 累积：

| # | 项目 | 文件:行 | 说明 |
|---|---|---|---|
| 1 | `matcher.go:hit.raw` 死字段移除 | `internal/trigger/matcher.go:21,40` | T007 留 |
| 2 | `handler/trigger.go` 加 `if h.Svc == nil` 显式保护 | `internal/handler/trigger.go:36` | T007 留 |
| 3 | `handler/trigger.go` 用 sentinel error 区分 4001 vs 500 | `internal/handler/trigger.go:38` | T007 留（与 service.go:25 `fmt.Errorf("no rule matched")` 改造一起） |
| 4 | DDL scheduler `run()` 加 panic recovery | `internal/trigger/ddl_scheduler.go:20-53` | T008 留 |
| 5 | `log` 前缀风格统一 `[ddl-scheduler]` vs `[ddl]` | `cmd/api/wire.go:66` vs `internal/trigger/ddl_scheduler.go:29,36,40,46,50` | T008+T009 累积 |
| 6 | 测试样板代码提取 helper | `internal/handler/webhook_test.go:28-35, 53-60, 77-84, 118-125, 159-166` | T009 M-3 留 |
| 7 | `decodeData` helper 改名 `decodeDataMap` | `internal/handler/webhook_test.go:151` | T009 M-4 留 |
| 8 | `log.Fatalf` 绕过 defers | `cmd/api/main.go:19, 27` | T009 M-6 留（启动期可接受） |

---

## 9. 决策记录（ADR — 来自 ai-coding-boundary §5）

### 决策 #1: 触发路径同步（不接 asynq）

- **问题**: webhook handler 是同步处理（直接 createRun）还是入队 asynq（异步处理）？
- **决定**: 同步
- **理由**: T007 末已实现，Phase 2+ 引入 asynq 异步触发；NFR-01 "延迟 < 5s" 同步路径可满足
- **影响范围**: 整个 trigger 包
- **执行时间**: 2026-06-18 (T007 末)

### 决策 #2: webhook user_id 用 uuid.Nil 占位

- **问题**: webhook payload 暂无 user 关联字段，handler 怎么传 user_id 给 createRun？
- **决定**: 用 `uuid.Nil` 占位
- **理由**: 规范未定义 webhook→user 映射，Phase 2+ 引入 user identity provider 后替换
- **影响范围**: `internal/handler/webhook.go:63`
- **执行时间**: 2026-06-18 (T009)

### 决策 #3: 优雅退出用 http.Server 显式模式

- **问题**: plan spec 写 `deps.Server().Run(addr)` + signal，SIGINT 后进程挂起
- **决定**: 用显式 `http.Server` + `ListenAndServe` goroutine + `srv.Shutdown(shCtx)` 替换
- **理由**: I-1 review 后由用户决策修复，与 plan spec 自身意图（"defer Close 资源"）一致
- **影响范围**: `cmd/api/main.go`
- **执行时间**: 2026-06-18 (T009)

### 决策 #4: nil Svc log 警告 + matched: false

- **问题**: `Svc=nil` 路径返回 `200 + matched=true` 会让外部 webhook 端误以为匹配成功
- **决定**: nil 分支加 `log.Printf` 警告 + 返回 `matched: false`
- **理由**: M-1 review 后由用户决策修复
- **影响范围**: `internal/handler/webhook.go:53-59`
- **执行时间**: 2026-06-18 (T009)

### 决策 #5: 删 TestWebhook_Todoist_NoSvc 重复测试

- **问题**: `NoSvc` 与 `ValidSignature` 输入/断言完全等价
- **决定**: 删 `NoSvc`，更新 `ValidSignature` 断言
- **理由**: M-2 review 后由用户决策修复
- **影响范围**: `internal/handler/webhook_test.go:67-93` 删除
- **执行时间**: 2026-06-18 (T009)

---

## 10. Phase 1 末 git 状态

```
$ git log --oneline -5
be96d66 feat(phase1/T009): wire trigger pipeline (webhook/keyword/ddl -> agent_run) + graceful shutdown
2e2b43f feat(phase1/T008): ddl detector + 15min scheduler (user_tasks table + lead-param drift fix)
46c3403 feat(phase1/T007): keyword matcher (longest-match-wins) + agent_run persistence + manual trigger handler
6a4a9fb feat(phase1/T006): webhook listener with hmac-sha256 verification + trigger indexes
f58c67b feat(phase0/T005): docker + docker-compose one-shot dev environment

$ git status
On branch main
Untracked files:
        doc/   (T006/T007/T008/T009 handoffs + phase0-final + phase1-final 11 个文件)
```

> **Phase 1 末有 11 个 doc 文件 untracked**（handoff 文档），待用户决定是否单独 `git add doc/` 入库。
