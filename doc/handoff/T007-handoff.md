# Phase 1 / T007 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T007 — 关键词匹配规则引擎（FR-A01）+ AgentRun 持久化骨架（数据层）+ 手动 trigger handler（FR-A04 入口）
> **状态**: ✅ **DONE**（implementer + spec compliance review + code quality review + 3 个 fix 全过）
> **基线 commit**: `6a4a9fb` (T006)
> **本任务 commit**: `46c3403` (7 files, +314 lines)
> **用户红线**: 本次不跑 docker / 不做端到端

---

## 1. 上一窗口做了什么

按 `doc/plans/02-phase1-trigger.md` §Task T007 (L440–L828) 完整执行了 12 个 step。**Step 11（修改 server.New 签名）按 plan 边界说明主动跳过**——plan 明确"由 T009 整合时执行"。

### 1.1 关联需求

- **FR-A01 (P0)**: 关键词触发, 准确率 > 95%, 误触发 < 5%, 延迟 < 500ms
- **FR-A04 (P1)**: 手动触发, < 1s 返回 run_id（入口已就位，路由待 T009）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `internal/trigger/matcher.go` | 新建 | 67 | `Rule` + `Matcher` + `NewMatcher` + `AddRule` + `Match`（**`sort.SliceStable` 最长匹配**）+ `defaultRules` 私有 + `DefaultMatcherRules` 导出 |
| `internal/trigger/matcher_test.go` | 新建 | 62 | **4 个测试**：TestMatcher_WeeklyReport (8 case) / TestMatcher_CustomRule / TestMatcher_LongestMatchWins / **TestMatcher_EqualLengthTiebreak**（fix 后新增） |
| `internal/trigger/service.go` | 新建 | 39 | `Service{pool, matcher}` + `NewService` + `ProcessKeyword`（"no rule matched" 错误） + `createRun` |
| `internal/trigger/service_test.go` | 新建 | 31 | `TestProcessKeyword_Integration`（gated on `DATABASE_URL`，无 docker 自动 SKIP） |
| `internal/repository/agent_run.go` | 新建 | 35 | `AgentRun` struct（11 字段 1:1 对齐 agent_runs 表）+ `CreateAgentRun`（参数化 SQL + RETURNING） |
| `internal/handler/trigger.go` | 新建 | 42 | `TriggerHandler{Svc}` + `triggerRequest/Response` + `ManualTrigger`（3 段前校验：body → UUID → Svc） |
| `internal/handler/trigger_test.go` | 新建 | 38 | `TestTriggerHandler_BadRequest`（fix 后强化：`Code != 4001` 精确断言） |

> **新建 7 个文件 + 修改 0 个文件 = T007 总变更 7 个对象**。

### 1.3 3 个 Fix（code quality review 后由 controller 直接修复）

| # | Fix | 文件:行 | 原问题 | 修复后 |
|---|---|---|---|---|
| 1 | 删除 `type fakeSvc struct{ lastText string }` | `handler/trigger_test.go:15-17` (旧) | C1 占位符 / 死代码（未引用） | 完全删除（plan 模板遗留） |
| 2 | 强化断言 `resp.Code == 0` → `resp.Code != 4001` | `handler/trigger_test.go:35-37` | 弱断言（未来若改成 5002 也会通过） | 锁定 4001 错误码 |
| 3 | `sort.Slice` → `sort.SliceStable` + 新增 tiebreak 测试 | `matcher.go:53` + `matcher_test.go:55-61` | 等长命中时不稳定（Go `sort.Slice` 不稳定） | `sort.SliceStable` 保证 rule 注册顺序（先注册者优先） |

### 1.4 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误（Go 代码无回归） |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go test ./...` | ✅ PASS | `ok internal/handler` + `ok internal/queue` + `ok internal/repository` + `ok internal/trigger` |
| `go test -v -run TestMatcher ./internal/trigger/...` | ✅ 4/4 PASS | 含新 tiebreak 测试 |
| `go test -v -run TestTriggerHandler ./internal/handler/...` | ✅ PASS | 强化断言验证通过 |
| `go test -v ./internal/trigger/...` | ✅ 9/9 PASS | T006 3 个 + T007 4 个 + service integration SKIP |
| `go test -v ./internal/handler/...` | ✅ 3/3 PASS | T002 TestHealth + T006 2 个 + T007 1 个 |
| `git commit` | ✅ `46c3403` | 由 controller 在 3 fix + 用户授权后执行 |

> ⚠️ **未跑**（用户红线）：
> - `make migrate-up`（需 docker postgres；T007 范围无新迁移）
> - 端到端 `curl POST /api/v1/trigger`（路由未注册，T009 整合）
> - 集成测试 `TestProcessKeyword_Integration`（无 DATABASE_URL 正确 SKIP）

### 1.5 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `fakeSvc` 已删除（fix 1） | plan 模板遗留，C1 占位违规 | ✅ C1 合规（fix 后） |
| 2 | `TestTriggerHandler_BadRequest` 强化为 4001 精确断言（fix 2） | 弱断言不符合 plan 约定的 4001 | ✅ 与 plan 一致（fix 后） |
| 3 | `sort.Slice` 改 `sort.SliceStable`（fix 3） | 等长命中时 `sort.Slice` 不稳定 | ✅ 行为更可预测（fix 后） |
| 4 | `Match` 内部 `hit.raw` / `compiledRule.raw` 字段被赋值但未读取 | plan 模板预埋，未在 T007 移除 | ⚠️ minor — 可在 T009 整合时清理 |
| 5 | `Service.createRun` 用 inline struct 而非 `event.go:AgentRunInput` | plan spec 字面如此 | ✅ 范围内 |
| 6 | `handler/trigger.go` 4001 涵盖所有 Svc 错误（含 "no rule matched" + DB 错误） | plan spec 字面 | ✅ 范围内（建议 T009 改用 sentinel error） |
| 7 | `handler/trigger.go` 入口未显式检查 `h.Svc == nil` | 测试靠前校验"恰好"拦住 nil deref | ⚠️ minor — T009 注入时 Svc 必非 nil |

**没有**触发 `ai-coding-boundary.md` 的任何红线（fix 后）：未越界 FR、未加未声明依赖、未写 `TODO` / `FIXME`、未自动 commit 前实现、未硬编码密钥、未跨 Phase。

### 1.6 git log 输出（T007 commit 后）

```
46c3403 feat(phase1/T007): keyword matcher (longest-match-wins) + agent_run persistence + manual trigger handler
6a4a9fb feat(phase1/T006): webhook listener with hmac-sha256 verification + trigger indexes
f58c67b feat(phase0/T005): docker + docker-compose one-shot dev environment
```

> T007 commit **已落盘**（按用户选择"Commit + 写 handoff + 继续 T008"）。

---

## 2. 上一窗口**没做**什么

- ❌ **没有**改 `internal/server/server.go` 签名（plan 明确"由 T009 整合时执行"）
- ❌ **没有**改 `cmd/api/main.go`（属 T009 范围）
- ❌ **没有**注册 `POST /api/v1/trigger` 路由（T009 整合）
- ❌ **没有**改 DDL 检测器（属 T008 范围）
- ❌ **没有**改 webhook handler（T006 已完成）
- ❌ **没有**加 JWT 认证（属 T009 整合 + NFR-02）
- ❌ **没有**写 asynq enqueue（T009 整合时把 webhook → AgentRun → queue 串起来）
- ❌ **没有**改 `migrations/0001_init.up.sql`（agent_runs 表已就位）
- ❌ **没有**清理 `internal/trigger/matcher.go:hit.raw` 死字段（reviewer minor 项，留 T009 整合时统一清理）
- ❌ **没有**自动 push（按 P2 红线）

---

## 3. 下一窗口需要做的（**T008 入口**）

> **重要**：T008 聚焦 DDL 截止日期检测器（FR-A02），新建独立的 `user_tasks` 表 + DDL 检测算法 + 15 分钟轮询调度器。**不**接 main.go（T009 整合）。

### 3.1 T008 任务范围

来源：`doc/plans/02-phase1-trigger.md` L832–L1043

| 步骤 | 关键产物 | FR 关联 |
|---|---|---|
| Step 1 | 写 `migrations/0003_ddl.up.sql` + `.down.sql`（user_tasks 表 + partial index） | FR-A02 |
| Step 2 | 写 `internal/trigger/ddl_test.go`（2 个测试：WithinLeadTime + DefaultLeadTime） | FR-A02 |
| Step 3 | 跑测试 → FAIL（NewDDLDetector undefined） | — |
| Step 4 | 写 `internal/trigger/ddl.go`（DDLDetector + NewDDLDetector + DefaultLead + ShouldTrigger） | FR-A02 |
| Step 5 | 跑测试 → PASS | — |
| Step 6 | 写 `internal/trigger/ddl_scheduler.go`（RunDDLScheduler + DDLPollHandler + 15min ticker） | FR-A02 |
| Step 7 | `go test ./...` → PASS | — |
| Step 8 | 端到端 DDL 流程（手动验证，**需 docker**，plan 注明 T009 才接 main 启动） | FR-A02 |
| Step 9 | 展示 diff 等用户决定 | — |

**T008 必新建文件**：
- `migrations/0003_ddl.up.sql`
- `migrations/0003_ddl.down.sql`
- `internal/trigger/ddl_test.go`
- `internal/trigger/ddl.go`
- `internal/trigger/ddl_scheduler.go`

**T008 必修改文件**：0 个。

### 3.2 关键注意点（避免重蹈 T007 的偏差）

1. **`user_tasks` 表 schema**（plan Step 1）必须严格按 spec：
   ```sql
   CREATE TABLE user_tasks (
       id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       user_id         UUID NOT NULL,
       source          VARCHAR(16) NOT NULL,   -- todoist / feishu / notion
       external_id     VARCHAR(128) NOT NULL,
       title           TEXT NOT NULL,
       content         TEXT,
       deadline_at     TIMESTAMPTZ,
       priority        VARCHAR(16) DEFAULT 'normal',
       completed       BOOLEAN NOT NULL DEFAULT false,
       triggered_at    TIMESTAMPTZ,           -- FR-A02 重复触发去重
       created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
       updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
       UNIQUE(source, external_id)
   );
   CREATE INDEX idx_user_tasks_deadline ON user_tasks(deadline_at)
       WHERE completed = false AND triggered_at IS NULL;
   ```
   - **不要**改字段名/类型
   - **不要**忘记 partial index（partial 谓词是 `completed = false AND triggered_at IS NULL`）
   - **不要**在 T008 跑 `make migrate-up`（plan 明确"T009 整合时绑定"，本任务仅落盘文件）

2. **`DDLDetector.ShouldTrigger` 算法**（plan Step 4）严格按 spec：
   ```go
   func (d *DDLDetector) ShouldTrigger(deadline, lead time.Time, now time.Time) bool {
       if lead <= 0 {
           lead = now.Add(d.defaultLead)
       }
       _ = lead
       return deadline.Before(now) || deadline.Sub(now) <= d.defaultLead
   }
   ```
   - 第 2 个参数 `lead` 在 plan spec 中**被忽略**（`_ = lead`），仅用 `defaultLead` 24h。
   - 这是 plan 字面行为，但**疑似 plan bug**（第 2 个参数接收 lead 但未使用）。
   - **建议**：先按 plan 字面实现，然后在 handoff 标记"等 T009 整合前与用户确认 lead 语义"。
   - **不要**自己改 plan 行为（属 R1 越界）。

3. **`DDLPollInterval` = 15 分钟**（plan Step 6）：
   ```go
   const DDLPollInterval = 15 * time.Minute
   ```
   - 这是 FR-A02 规范要求。
   - **不要**改成可配置（不在 T008 范围）。

4. **调度器 SQL**（plan Step 6）：
   ```sql
   SELECT id::text, user_id::text, title FROM user_tasks
   WHERE completed = false AND triggered_at IS NULL
   AND deadline_at IS NOT NULL
   AND deadline_at <= NOW() + INTERVAL '24 hours'
   ```
   - **不要**改 SQL 谓词。
   - 注意：`deadline_at <= NOW() + 24h` 是硬编码 24h，与 `DDLDetector.defaultLead = 24h` 对齐；未来若 defaultLead 改了要同步改 SQL。

5. **调度器"启动时立即跑一次"**（plan Step 6）：`run()` 在 `for { select }` 前先调用一次。**不要**改成"等 15 分钟再跑"。

6. **触发后回写**（plan Step 6）：`UPDATE user_tasks SET triggered_at = NOW() WHERE id = $1`。
   - **不要**在 T008 范围加事务或并发保护（T009 整合时如果需要再加）。

7. **`RunDDLScheduler` 不接 main.go**（plan Step 8 边界说明）：
   - T008 末 `cmd/api/main.go` 仍是 T002/T006 末的简单 server.New，**不**启调度器
   - T009 整合时在 main.go 用 `go trigger.RunDDLScheduler(ctx, pool, det, ddlH)` 启动

8. **集成测试缺口**：plan Step 8 注明"调度器在 main 启动时拉起（T009 整合时绑定）"。**T008 末无法端到端测试 DDL 触发**（需 main.go 整合 + docker）。

### 3.3 T008 不做的事（避免越界）

- ❌ **不要**改 `cmd/api/main.go`（属 T009 整合）
- ❌ **不要**改 `internal/server/server.go`
- ❌ **不要**加 JWT 认证（属 T009 整合）
- ❌ **不要**把 `RunDDLScheduler` 接到 server.New 或 config
- ❌ **不要**写 user_tasks 的 repository CRUD（属 T009 范围 if 需要）
- ❌ **不要**改 webhook handler 或 trigger handler（T006/T007 已完成）
- ❌ **不要**改 agent_runs schema
- ❌ **不要**加 `user_tasks` 的 repository（plan 范围内只用 `pool.Query` + `pool.Exec` 直接操作）

### 3.4 建议执行顺序（下一窗口 T008）

1. 读 `doc/handoff/T007-handoff.md`（本文件）
2. 读 `doc/plans/02-phase1-trigger.md` L832–L1043 拿 T008 完整 spec
3. 按 subagent-driven-development 流程：
   - Dispatch **implementer subagent**（提供完整 T008 spec + Phase 0/T006/T007 context + 上述注意点）
   - 收到 DONE 报告后 dispatch **spec compliance reviewer**
   - 通过后 dispatch **code quality reviewer**
   - 两轮 ✅ 后向用户展示 git diff，**询问 commit**（P1 红线）
4. 用户选择 commit 后写 `doc/handoff/T008-handoff.md`
5. 循环进入 T009 整合

---

## 4. 给下一窗口的提示

1. **`internal/trigger/matcher.go` 的 `hit.raw` / `compiledRule.raw` 字段未使用**：code quality review 标记 minor，建议 T008 完成后（Phase 1 末 T009 整合前）一起清掉。
2. **`handler/trigger.go` 未显式 nil-check Svc**：测试靠前校验"恰好"拦住 nil deref。T009 注入时 Svc 必非 nil，可以加 `if h.Svc == nil { httpx.Fail(..., 500, ...); return }` 显式保护（建议在 T009 一并加）。
3. **`migrations/0003` 没跑**：`migrations/0003_ddl.up.sql` 仅落盘，**没在 docker postgres 上跑过 `make migrate-up`**。Phase 1 末 T009 整合 + 端到端验证时一起跑。
4. **T008 末 `user_tasks` 表存在但无数据**：当前是空表，T009 整合后用户可手动 `docker exec psql INSERT` 验证 DDL 调度。
5. **`DDLDetector.ShouldTrigger` 的第 2 个 `lead` 参数被忽略**：plan 字面如此（`_ = lead`），疑似 plan bug。**T008 不要自作主张改 plan 行为**（属 R1 越界）；在 handoff 标记给用户审视。
6. **DDL 调度器 SQL 硬编码 24h**：`deadline_at <= NOW() + INTERVAL '24 hours'`。如果未来 `defaultLead` 改了要同步改 SQL（T009 整合前审视）。
7. **T008 集成测试缺口**：`TestProcessKeyword_Integration` 在 T007 是 gated on DATABASE_URL；T008 范围**没有**集成测试，调度器端到端需 T009 整合后跑。**不要**为 T008 强行加集成测试。
8. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。
9. **`doc/handoff/` 仍 untracked**：T007 handoff 文件 + T006 handoff 文件都未入库。等用户决定是否要单独 `git add doc/` 入库。
10. **T008 不引入新依赖**（plan spec 范围内无 M8 需求）：`time` / `context` / `log` / `pgxpool` 都已 direct。

---

## 5. 当前文件结构（Phase 1 / T007 末）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
├── cmd/
│   └── api/
│       └── main.go               ✅ T002 (启动 HTTP server，**未连 DB / 未启 Worker / 未接 trigger svc**)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md
│   │   ├── T002-handoff.md
│   │   ├── T003-handoff.md
│   │   ├── T004-handoff.md
│   │   ├── T005-handoff.md
│   │   ├── T006-handoff.md       (Phase 1 第一个 handoff，**未入库**)
│   │   ├── T007-handoff.md       ← 本文件（**未入库**）
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
│   │   ├── webhook.go            ✅ T006
│   │   ├── webhook_test.go       ✅ T006
│   │   ├── trigger.go            ✅ T007 (TriggerHandler.ManualTrigger)
│   │   └── trigger_test.go       ✅ T007 (TestTriggerHandler_BadRequest, 强化版)
│   ├── middleware/               ✅ T002
│   ├── queue/                    ✅ T004 (T006/T007 未动)
│   ├── repository/
│   │   ├── db.go                 ✅ T003 (Open)
│   │   ├── db_test.go            ✅ T003
│   │   └── agent_run.go          ✅ T007 (AgentRun struct + CreateAgentRun)
│   ├── server/
│   │   └── server.go             ✅ T002+T006 (T007 未动，签名仍是 `func New(cfg)`)
│   └── trigger/                  ✅ T006+T007
│       ├── event.go              (T006)
│       ├── webhook.go            (T006)
│       ├── webhook_test.go       (T006)
│       ├── matcher.go            (T007, fix 后 sort.SliceStable)
│       ├── matcher_test.go       (T007, 含 4 测试)
│       ├── service.go            (T007, ProcessKeyword + createRun)
│       └── service_test.go       (T007, gated integration test)
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

1. **T007 commit 已自动落盘**（按上一轮"Commit + 写 handoff + 继续 T008"选择）。
   - 本轮已 commit `46c3403`（7 files, +314 lines）。
   - 包含 3 个 code quality fix：删除 `fakeSvc` / 强化 `Code != 4001` / `sort.SliceStable` + tiebreak 测试。

2. **T008 启动方式**：是否继续用 subagent-driven-development 流程？
   - **方案 A（推荐）**：继续 implementer → spec reviewer → code quality reviewer → 用户 commit（与 T006/T007 一致）
   - **方案 B**：直接 implementer，跳过 review（更快但少两道质量门）
   - **方案 C**：T008 之后停一停，端到端验证 DDL 调度（建议在 T009 整合后才停）

3. **`DDLDetector.ShouldTrigger` 的第 2 个 `lead` 参数被忽略**：plan 字面如此，疑似 plan bug。
   - 方案 A：按 plan 字面实现（`_ = lead`），在 T008 handoff 标记给用户审视
   - 方案 B：实现 lead 真正的语义（如 `deadline.Sub(now) <= lead.Sub(now)`），但这是 R1 越界（改 plan 行为）
   - **推荐方案 A**（C1 + R1 合规优先）

4. **`migrations/0003` 何时跑**：
   - 方案 A：现在跑（需 docker 起来）→ 推荐在 T009 整合 + 端到端验证时一起跑
   - 方案 B：留到 user_tasks 真正有数据时再跑（推荐，T008 范围无 user_tasks 数据）

5. **T006/T007 handoff 是否入库**：
   - 当前 `doc/handoff/T006-handoff.md` + `T007-handoff.md` 与整个 `doc/` 目录 untracked
   - 如果要入库：`git add doc/handoff/ && git commit -m "docs(phase1): add task handoffs (T006 + T007)"`
   - 也可以等所有 T006-T009 完成后一次性 `git add doc/` 入库

6. **`task-tracker.html` 是否更新**：
   - T006 + T007 完成后可标记 T006/T007 状态为"已完成"
   - 当前所有 T001-T005 + T006 + T007 都未在 task-tracker 更新
   - 建议：Phase 1 末一并更新

7. **T007 留的 minor 项**（可在 T009 整合时统一处理）：
   - `matcher.go:hit.raw` 死字段移除
   - `handler/trigger.go` 加 `if h.Svc == nil` 显式保护
   - `handler/trigger.go` 用 sentinel error 区分 "no rule matched" (4001) vs DB error (500)
   - 这些都是 minor，T009 wire 时统一处理最划算

> **下一窗口（T008）开场建议**：
> 1. 读 `doc/handoff/T007-handoff.md`（本文件）
> 2. 读 `doc/plans/02-phase1-trigger.md` L832–L1043 §Task T008
> 3. 跑 `go test ./...` 确认 T006 + T007 测试仍 PASS（无回归）
> 4. 用 AskUserQuestion 问上面 §6 第 2、3 项（T008 启动方式 + lead 参数处理）
> 5. Dispatch implementer subagent（带完整 T008 spec + Phase 0/T006/T007 context + 本文件 §3.2 注意点）
> 6. 两轮 review 通过后展示 git diff，**用户决定 commit**
> 7. commit 后写 `doc/handoff/T008-handoff.md`
