# Phase 1 / T006 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T006 — Webhook 监听服务（HMAC-SHA256 验签 + Todoist handler + 触发器索引）
> **状态**: ✅ **DONE**（implementer + spec compliance review + code quality review 三轮全通过）
> **基线 commit**: `f58c67b` (T005)
> **本任务 commit**: `6a4a9fb` (12 files, +263/-11)
> **用户红线**: 本次不跑 docker / 不做端到端 curl 验证（仅 `go build` / `go test` / `go vet`）

---

## 1. 上一窗口做了什么

按 `doc/plans/02-phase1-trigger.md` §Task T006 (L43–L437) 完整执行了 14 个 step 中的 **1–13**。**Step 14（端到端 curl 验证）主动跳过**（按用户红线：docker 端到端验证在主窗口跑）。

### 1.1 关联需求

- **FR-A03 (P0)**: Webhook 接入，HMAC-SHA256 验签
- **FR-A04 (P1)**: 手动 / Webhook 入口（entry point，event_id 幂等留 T009）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `internal/trigger/event.go` | 新建 | 46 | `Source` 枚举（6 个常量）+ `TriggerEvent` + `NewWebhookEvent` + `AgentRunInput` |
| `internal/trigger/webhook.go` | 新建 | 42 | `SignHMAC` + `VerifyHMAC`（`hmac.Equal` 恒定时间）+ `NormalizeTodoist`（flatten event_data）+ `NormalizeFeishu`（占位透传） |
| `internal/trigger/webhook_test.go` | 新建 | 41 | 3 个测试：VerifyHMAC_Valid / VerifyHMAC_Invalid / NormalizeTodoistTaskCreated |
| `internal/handler/webhook.go` | 新建 | 39 | `WebhookHandler{Secret}` + `Todoist(c)` 方法（read body → 验签 → bind JSON → normalize → 200） |
| `internal/handler/webhook_test.go` | 新建 | 54 | 2 个 httptest：ValidSignature（200） / InvalidSignature（403） |
| `internal/handler/util.go` | 新建 | 14 | `jsonUnmarshal` + `getStr` 辅助（per plan spec） |
| `migrations/0002_trigger_indexes.up.sql` | 新建 | 3 | `idx_agent_runs_trigger_type` + `idx_agent_runs_user_trigger` |
| `migrations/0002_trigger_indexes.down.sql` | 新建 | 2 | 反向 DROP（顺序：复合索引 → 单列索引） |
| `internal/config/config.go` | 修改 | +9/-7 | 追加 `TodoistWebhookSecret` 字段 + `TODOIST_WEBHOOK_SECRET` env 加载 |
| `internal/server/server.go` | 修改 | +4/-0 | 追加 `POST /api/v1/webhook/todoist` 路由 |
| `.env.example` | 修改 | +1/-0 | 追加 `TODOIST_WEBHOOK_SECRET=` |
| `go.mod` | 修改 | +1/-1 | `github.com/google/uuid v1.6.0` 由 indirect 提升为 direct |

> **新建 8 个文件 + 修改 4 个文件 = T006 总变更 12 个对象**。

### 1.3 验证结果（仅 Go 静态 + 单元测试）

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误（Go 代码无回归） |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go test ./...` | ✅ PASS | `ok internal/handler` + `ok internal/queue` + `ok internal/repository` + `ok internal/trigger` |
| `go test -v ./internal/trigger/...` | ✅ 3/3 PASS | TestVerifyHMAC_Valid (0.00s) / TestVerifyHMAC_Invalid (0.00s) / TestNormalizeTodoistTaskCreated (0.00s) |
| `go test -v ./internal/handler/...` | ✅ 3/3 PASS | TestHealth (既有) / TestWebhook_Todoist_ValidSignature (0.00s) / TestWebhook_Todoist_InvalidSignature (0.00s) |
| `go mod tidy` | ✅ OK | uuid v1.6.0 indirect → direct，go.sum 无变化（C7 合规） |
| `git commit` | ✅ `6a4a9fb` | 由 controller 在两轮 review 通过后执行（符合 P1 + 用户选择"Commit + 写 handoff + 继续 T007"） |

> ⚠️ **未跑**（用户红线）：
> - `make migrate-up`（需 docker postgres；plan Step 7 仅要求创建文件，运行时执行由用户在主窗口跑）
> - `docker compose up -d` + `curl http://localhost:8080/api/v1/webhook/todoist`
> - 真实 Todoist 沙箱对接

### 1.4 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `Config` struct 字段对齐 go fmt 后列宽比 plan 截图略宽 | `gofmt` 标准行为 | ✅ 纯格式 |
| 2 | `bindJSON` 包装 `jsonUnmarshal` 是 1:1 别名 | plan spec Step 8 明确要求 | ✅ 与 plan 一致（code quality review 标记为 "可延后优化"） |
| 3 | `NormalizeFeishu` 是透传占位 | plan spec 明确"按飞书 v2 协议 真实实现待定" | ✅ 范围内 |
| 4 | `AgentRunInput` 在 T006 范围未被使用 | plan spec 预埋给 T007 service | ✅ 范围内 |
| 5 | `webhook.go:33` 含 `// T009 整合时接入 EventBus / AgentRun 创建` 注释 | plan spec 原文；非 `C1` 占位 | ✅ 范围内（code quality review 建议 T009 后清理） |
| 6 | 缺失/缺少 sig header 与错误签名的 4003 vs 4004 区分 | plan spec 写同一错误码 | ✅ 范围内（code quality review 标记 "可延后"） |
| 7 | 缺 `TestVerifyHMAC_NonHexSignature` 等边缘测试 | plan spec 未要求 | ⚠️ 范围外（建议 T007 后补） |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖（仅 uuid 提升，非新增）、未写 `TODO` / `FIXME` / `TBD`、未自动 commit 前的实现、未硬编码密钥（`Secret` 走 env）、未跨 Phase。

### 1.5 git log 输出（T006 commit 后）

```
6a4a9fb feat(phase1/T006): webhook listener with hmac-sha256 verification + trigger indexes
f58c67b feat(phase0/T005): docker + docker-compose one-shot dev environment
bfbc4f0 feat(phase0/T004): redis task queue (asynq) with client + server + mux
```

> T006 commit **已落盘**（按用户选择"Commit + 写 handoff + 继续 T007"），不留在工作区。

---

## 2. 上一窗口**没做**什么

- ❌ **没有**跑端到端 `curl http://localhost:8080/api/v1/webhook/todoist`（需 docker + API 起服务 + HMAC 计算）—— 用户红线
- ❌ **没有**跑 `make migrate-up`（需 docker postgres）—— 迁移文件已就位，等用户在主窗口跑
- ❌ **没有**改 `agent_runs` schema（FR-A04 幂等性 `event_id` 持久化方案未定）
- ❌ **没有**创建 `AgentRun` 持久化代码（`internal/repository/agent_run.go` 留 T007 范围）
- ❌ **没有**改 `cmd/api/main.go`（仍走 Phase 0 末的简单 server.New；T009 整合时改）
- ❌ **没有**写 DDL 调度器（属 T008 范围）
- ❌ **没有**写 JWT 中间件（属 T009 范围 — NFR-02）
- ❌ **没有**写 `queue` 集成入队逻辑（webhook handler 收 200 但不入 asynq 队列；T009 整合）
- ❌ **没有**加 `agent_runs.triggered_event_id` 唯一索引（FR-A04 幂等需 T009 设计后再加）
- ❌ **没有**自动 push（按 P2 红线）

---

## 3. 下一窗口需要做的（**T007 入口**）

> **重要**：T006 范围内 webhook handler 是"收 200 不入队"。T009 整合时会把 webhook → AgentRun → asynq enqueue 串起来。T007 聚焦**关键词匹配规则引擎** + AgentRun 持久化骨架。

### 3.1 T007 任务范围

来源：`doc/plans/02-phase1-trigger.md` L440–L828

| 步骤 | 关键产物 | FR 关联 |
|---|---|---|
| Step 1 | 写 matcher_test.go（3 个测试） | FR-A01 |
| Step 2 | 跑测试 → FAIL（NewMatcher undefined） | — |
| Step 3 | 写 matcher.go（Rule + NewMatcher + AddRule + Match + defaultRules + DefaultMatcherRules） | FR-A01 |
| Step 4 | 跑测试 → PASS | — |
| Step 5 | 写 trigger service.go（ProcessKeyword + createRun） | FR-A01 + FR-A04 |
| Step 6 | 写 repository/agent_run.go（AgentRun struct + CreateAgentRun） | 数据层 |
| Step 7 | 写 trigger service_test.go（集成测试，需 DATABASE_URL） | — |
| Step 8 | 写 handler/trigger.go（TriggerHandler.ManualTrigger） | FR-A04 |
| Step 9 | 写 handler/trigger_test.go（httptest，bad request 路径） | — |
| Step 10 | `go test ./...` → PASS | — |
| Step 11 | `go build ./...` → 0 错误（route 暂不接 server.New，T009 整合） | — |
| Step 12 | 展示 diff 等用户决定 | — |

**T007 必新建文件**：
- `internal/trigger/matcher.go`
- `internal/trigger/matcher_test.go`
- `internal/trigger/service.go`
- `internal/trigger/service_test.go`
- `internal/handler/trigger.go`
- `internal/handler/trigger_test.go`
- `internal/repository/agent_run.go`

**T007 必修改文件**：
- `internal/trigger/matcher.go`（plan Step 7 注：把 `defaultRules` 改名为 `DefaultMatcherRules`）
- `internal/server/server.go`（仅 go build 通过，路由注册留 T009）

### 3.2 关键注意点（避免重蹈 T006 的偏差）

1. **`DefaultMatcherRules` 导出**：plan Step 7 明确要求把 `defaultRules` 改为 `DefaultMatcherRules`（首字母大写），给 service_test.go 用。
2. **`Match` 的"最长匹配优先"**：plan L567 `sort.Slice` 按 `loc[1]-loc[0]` 长度降序。已有 1 个测试 `TestMatcher_LongestMatchWins` 验证 "项目总结" 走 `summary`（更具体），不要写错。
3. **`ProcessKeyword` 返回 `(uuid.UUID, error)`**：当 `matcher.Match` 返回 `!ok` 时返回 `uuid.Nil, fmt.Errorf("no rule matched")`。
4. **`createRun` 调用 `repository.CreateAgentRun`**：T007 自己实现这个函数（不要等 T006/T009）。T006 没有这个文件 — 是 T007 新增。
5. **`AgentRun` struct 字段顺序**严格对齐 `migrations/0001_init.up.sql` agent_runs 表：
   `ID, UserID, TaskType, Status, CurrentStage, TriggerType, TriggerSource, ErrorMessage, CreatedAt, UpdatedAt, CompletedAt`
6. **集成测试 `TestProcessKeyword_Integration`**：需 `DATABASE_URL` env（Phase 0 T005 docker 起来后才会设）；没设就 `t.Skip`，不要 panic。
7. **handler/trigger.go 的 `nil Svc` 容忍**：T007 末的 `server.New` 仍未集成，`TriggerHandler.Svc` 在测试里可能为 `nil`（handler 自己有 nil 检查）；T009 才真正注入。
8. **不要**在 T007 范围加 `server.New` 的 trigger svc 参数（plan 明确"Phase 1 末 T009 整合"）。

### 3.3 T007 不做的事（避免越界）

- ❌ **不要**改 webhook handler（T006 已完成，T009 整合时才一起改）
- ❌ **不要**写 DDL 检测器（属 T008）
- ❌ **不要**改 `cmd/api/main.go`（属 T009 整合）
- ❌ **不要**加 JWT 认证（属 T009 整合 + NFR-02）
- ❌ **不要**写 asynq enqueue（属 T009 整合）
- ❌ **不要**写 `internal/queue` producer 代码（T009 才用）
- ❌ **不要**改 schema（agent_runs 表已 Phase 0 就位）

### 3.4 建议执行顺序（下一窗口 T007）

1. 读 `doc/handoff/T006-handoff.md`（本文件）
2. 读 `doc/plans/02-phase1-trigger.md` L440–L828 拿 T007 完整 spec
3. 按 subagent-driven-development 流程：
   - Dispatch **implementer subagent**（提供完整 T007 spec + Phase 0/T006 context + 上述注意点）
   - 收到 DONE 报告后 dispatch **spec compliance reviewer**
   - 通过后 dispatch **code quality reviewer**
   - 两轮 ✅ 后向用户展示 git diff，**询问 commit**（P1 红线）
4. 用户选择 commit 后写 `doc/handoff/T007-handoff.md`
5. 循环进入 T008 / T009

---

## 4. 给下一窗口的提示

1. **T006 集成进 `cmd/api/main.go` 的"待办"**：`internal/handler/webhook.go:33` 注释 `// T009 整合时接入 EventBus / AgentRun 创建；此处先 200 返回` 是 plan 内嵌的衔接注释，**不是** `C1` 占位代码。T009 完成后**必须**删除此注释。
2. **`bindJSON` 是否要合并为 `jsonUnmarshal`**：plan spec 要求保留 `bindJSON` 别名（便于后续 mock 注入）。code quality review 标记"可延后优化"，T007 不动。
3. **`migrations/0002` 没跑**：`migrations/0002_trigger_indexes.up.sql` 仅落盘，**没在 docker postgres 上跑过 `make migrate-up`**。Phase 0 末端到端验证时一起跑（用户主窗口）。
4. **`uuid` 已为 direct 依赖**：T007 service.go / repository/agent_run.go / handler/trigger.go 都可以直接 import `github.com/google/uuid`，go.mod 无需再改。
5. **webhook handler 暂不接 `pool` / `queue.Client`**：T006 的 `WebhookHandler` 只有 `Secret` 字段，没有 `*pgxpool.Pool` 或 `*queue.Client`；T009 整合时扩展。
6. **FR-A04 幂等性未实现**：当前重复 `event_id` 会被多次 `200 OK`（每次都创建一条 agent_runs 行 — 等 T009 整合后）。`migrations/0002` 没加 `agent_runs.triggered_event_id` 唯一索引。
7. **`NormalizeFeishu` 暂为透传占位**：plan spec 明确"按飞书 v2 协议 真实实现待定"。T006 范围只搭骨架，飞书 webhook 路由也不在 T006 范围。
8. **`AgentRunInput` 是 T006 预埋**：T006 没在 T006 范围使用；T007 `service.go:createRun` 会用到这个类型。
9. **T007 集成测试（service_test.go）需 docker postgres**：`TestProcessKeyword_Integration` 内部 `os.Getenv("DATABASE_URL") == ""` 时 `t.Skip`。在没 docker 的开发机也能 build + 跑其他单元测试。
10. **T007 末不要**改 `server.New` 签名（保持 `func New(cfg) *gin.Engine`）；T009 整合时统一改成 `func New(cfg, trigSvc) *gin.Engine`。
11. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。
12. **`doc/handoff/` 仍 untracked**：git status 看到 `doc/` 整个目录未入库（从 Phase 0 起）。T006 完成后 `doc/handoff/T006-handoff.md` 也是 untracked。如果用户要入库，需要单独 `git add doc/`。

---

## 5. 当前文件结构（Phase 1 / T006 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
├── cmd/
│   └── api/
│       └── main.go               ✅ T002 (启动 HTTP server，**未连 DB / 未启 Worker / 未接 webhook svc**)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md
│   │   ├── T002-handoff.md
│   │   ├── T003-handoff.md
│   │   ├── T004-handoff.md
│   │   ├── T005-handoff.md
│   │   ├── T006-handoff.md       ← 本文件（**未入库**）
│   │   └── phase0-final-handoff.md  (Phase 0 总交接，**未入库**)
│   ├── plans/                    (00~05)
│   ├── mvp-definition.html
│   ├── plan-boundary.md
│   ├── prototype.html
│   ├── requirement-spec.html
│   └── task-tracker.html
├── internal/
│   ├── config/
│   │   └── config.go             ✅ T001+T006 (6 env vars: APP_ENV/APP_PORT/DATABASE_URL/REDIS_URL/JWT_SECRET/**TODOIST_WEBHOOK_SECRET**)
│   ├── handler/
│   │   ├── health.go             ✅ T002
│   │   ├── health_test.go        ✅ T002
│   │   ├── util.go               ✅ T006 (14 行：jsonUnmarshal + getStr)
│   │   ├── webhook.go            ✅ T006 (39 行：WebhookHandler.Todoist)
│   │   └── webhook_test.go       ✅ T006 (54 行：2 个 httptest)
│   ├── middleware/               ✅ T002
│   │   ├── logger.go
│   │   └── recovery.go
│   ├── queue/                    ✅ T004 (T006 未动)
│   ├── repository/               ✅ T003 (T006 未动；**T007 要加 agent_run.go**)
│   ├── server/
│   │   └── server.go             ✅ T002+T006 (+POST /api/v1/webhook/todoist)
│   └── trigger/                  ✅ T006 (新目录)
│       ├── event.go              (46 行)
│       ├── webhook.go            (42 行)
│       └── webhook_test.go       (41 行)
├── pkg/
│   └── httpx/
│       └── response.go           ✅ T002
├── migrations/
│   ├── 0001_init.up.sql          ✅ T003 (6 表)
│   ├── 0001_init.down.sql        ✅ T003
│   ├── 0002_trigger_indexes.up.sql   ✅ T006 (2 索引，**未跑**)
│   └── 0002_trigger_indexes.down.sql ✅ T006
├── scripts/
│   └── init-db.sql               ✅ T005
├── .dockerignore                 ✅ T005
├── .env.example                  ✅ T001+T006 (+TODOIST_WEBHOOK_SECRET)
├── .gitignore                    ✅ T001
├── Dockerfile                    ✅ T005
├── Makefile                      ✅ T001+T003+T005
├── docker-compose.yml            ✅ T005
├── go.mod                        ✅ T002+T003+T004+T006 (uuid v1.6.0 已 direct)
├── go.sum                        ✅ T002+T003+T004+T006
└── README.md                     ✅ T001
```

---

## 6. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **T006 commit 已自动落盘**（按上一轮"Commit + 写 handoff + 继续 T007"选择）。
   - 本轮已 commit `6a4a9fb`（12 files, +263/-11）。
   - 如果想撤销：`git reset --soft HEAD~1`（不推荐；改完要重新做 review）。

2. **T007 启动方式**：是否继续用 subagent-driven-development 流程？
   - **方案 A（推荐）**：继续 implementer → spec reviewer → code quality reviewer → 用户 commit（与 T006 一致）
   - **方案 B**：下一窗口直接 implementer，跳过 spec/code quality review（更快但少两道质量门）
   - **方案 C**：T007 之后停一停，让用户做端到端验证（建议在 T009 末才停）

3. **`migrations/0002` 何时跑**：
   - 方案 A：现在跑（需 docker 起来）→ 推荐在 Phase 0 末端到端验证时跑（用户在主窗口）
   - 方案 B：留到 T007 集成测试需要时再跑（推荐，T007 service_test.go 有 `os.Getenv("DATABASE_URL")` 跳过来兼容）

4. **`webhook.go:33` 注释**：`// T009 整合时接入 EventBus / AgentRun 创建；此处先 200 返回` 是否保留？
   - 当前状态：plan spec 原文保留（合规）
   - 建议：T009 完成后**必须**删除（避免演化为 C1 占位）
   - 本次**不删**

5. **T006 handoff 是否入库**：
   - 当前 `doc/handoff/T006-handoff.md` 与整个 `doc/` 目录 untracked
   - 如果要入库：`git add doc/handoff/T006-handoff.md && git commit -m "docs(phase1/T006): add task handoff"`
   - 也可以等所有 T006-T009 完成后一次性 `git add doc/` 入库

6. **`task-tracker.html` 是否更新**：
   - T006 完成后可标记 T006 状态为"已完成"
   - 当前所有 T001-T005 + T006 都未在 task-tracker 更新（Phase 0 末就 deferred）
   - 建议：Phase 1 末一并更新

> **下一窗口（T007）开场建议**：
> 1. 读 `doc/handoff/T006-handoff.md`（本文件）
> 2. 读 `doc/plans/02-phase1-trigger.md` L440–L828 §Task T007
> 3. 跑 `go test ./...` 确认 T006 测试仍 PASS（无回归）
> 4. 用 AskUserQuestion 问上面 §6 第 2 项（T007 启动方式）
> 5. Dispatch implementer subagent（带完整 T007 spec + Phase 0/T006 context + 本文件 §3.2 注意点）
> 6. 两轮 review 通过后展示 git diff，**用户决定 commit**
> 7. commit 后写 `doc/handoff/T007-handoff.md`
