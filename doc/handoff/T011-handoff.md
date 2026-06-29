# Phase 2 / T011 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（implementer + spec reviewer + code quality reviewer + 手动 fix）
> **当前任务**: T011 — 日历数据源适配器（FR-B02）
> **状态**: ✅ **DONE**（implementer + spec review + code quality review + fix 全过，commit `3ba1b62`）
> **基线 commit**: `62c88d9` (T010)
> **本任务 commit**: `3ba1b62` (5 files, +314/-24)
> **用户红线**: 不跑 docker / 不做端到端

---

## 1. 本窗口做了什么（T011 做完了什么内容）

按 `doc/plans/03-phase2-context.md` §Task T011 完整执行了 Step 1-5 + Code Quality Review 后的修复。

### 1.1 关联需求

- **FR-B02 (P1)**: 日历拉取（Google/Outlook，MVP 阶段只做 Google Calendar）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `internal/harvesting/source/calendar.go` | **新建** | 38 | `CalendarProvider` 接口 + `CalendarAdapter`（含 `Name()` + `Fetch()`） |
| `internal/harvesting/source/google_calendar.go` | **新建** | 95 | `GoogleCalendarConfig` + `GoogleCalendarProvider`（Service Account 认证 + 分页 + 全天事件 fallback） |
| `internal/harvesting/source/calendar_test.go` | **新建** | 93 | `mockCalendar` + 5 个测试 + 编译时接口检查 |
| `go.mod` | **修改** | +33/-7 | 新增 `google.golang.org/api v0.285.0` + 间接依赖 |
| `go.sum` | **修改** | +79/-17 | 对应校验和 |

> **新建 3 个文件 + 修改 2 个文件 = T011 总变更 5 个对象**。

### 1.3 Code Quality Review 后的修复

| # | Fix | 文件:行 | 原问题 | 修复后 |
|---|---|---|---|---|
| C-1 | 删除 `since` 死代码字段 | `calendar.go` | `CalendarAdapter.since` 被赋值但从未读取，存在线程安全隐患 | 删除 `since` 字段，`Fetch` 直接将 `since` 参数传给 `ListEvents` |
| C-2 | Provider 错误未包装 | `calendar.go:Fetch` | `return nil, err` 直接透传，缺少上下文 | 改为 `fmt.Errorf("calendar adapter fetch: %w", err)` |
| C-3 | 无分页处理 | `google_calendar.go:ListEvents` | `MaxResults(100)` 硬编码，未处理 `NextPageToken`，超过 100 条静默丢失 | 添加分页循环，`MaxResults(250)` + `NextPageToken` 迭代 |
| C-4 | 时间解析错误被静默忽略 | `google_calendar.go:ListEvents` | `t, _ := time.Parse(...)` 丢弃错误，`OccurredAt` 为零值 | 解析失败回退 `time.Now()`；添加 `e.Start.Date` fallback（全天事件） |
| C-5 | `CredentialsFile` 无校验 | `google_calendar.go:NewGoogleCalendarProvider` | 空字符串导致不明确的文件系统错误 | 添加空值校验，返回明确错误 `"google calendar: credentials file path is required"` |
| C-6 | 测试覆盖不足 | `calendar_test.go` | 仅 2 个测试（基本路径），无错误路径/默认类型/空结果测试 | 新增 3 个测试：`DefaultType` + `EmptyResult` + `ProviderError` |

### 1.4 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/harvesting/source/...` | ✅ 8 PASS | 5 Calendar + 3 GitHub |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 测试仍通过） |
| `git commit` | ✅ `3ba1b62` | 5 files, +314/-24 |

### 1.5 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `google.golang.org/api@latest` (v0.285.0) 替代 plan 指定的 `v0.157.0` | M8 规则建议 `@latest`，用户确认选择 `@latest` | ✅ 用户决策 |
| 2 | Service Account 认证替代 plan 的 OAuth2 用户授权 | 用户确认 MVP 阶段简化为 Service Account | ✅ 用户决策 |
| 3 | 新增 `Name()` + `Fetch()` 方法 | plan T011 未明确要求，但 T014 pipeline 需要 `sourceAdapter` 接口 | ✅ 合理架构准备，用户确认 |
| 4 | 新增分页支持 | plan T011 未要求分页，但 Code Quality Review 发现数据丢失风险 | ✅ 质量改进，未越界 FR |
| 5 | 新增 `CredentialsFile` 校验 | plan 未要求，但属于基本防御性编程 | ✅ 质量改进 |
| 6 | `go` 版本从 `1.25.5` 升级到 `1.25.8` | `go get @latest` 自动更新 go directive | ✅ 依赖更新附带 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖（用户已确认）、未写 `TODO` / `FIXME`、未自动 commit 前实现、未硬编码密钥、未跨 Phase。

### 1.6 git log 输出（T011 commit 后）

```
3ba1b62 feat(phase2/T011): calendar data source adapter + google calendar provider (FR-B02)
62c88d9 feat(phase2/T010): github data source adapter + context types (FR-B01/FR-B06)
be96d66 feat(phase1/T009): wire trigger pipeline (webhook/keyword/ddl -> agent_run) + graceful shutdown
2e2b43f feat(phase1/T008): ddl detector + 15min scheduler (user_tasks table + lead-param drift fix)
46c3403 feat(phase1/T007): keyword matcher (longest-match-wins) + agent_run persistence + manual trigger handler
```

---

## 2. 本窗口**没做**什么

- ❌ **没有**跑 `make migrate-up`（需 docker postgres）
- ❌ **没有**实现 T012（飞书适配器）
- ❌ **没有**实现 T013（Obsidian 适配器）
- ❌ **没有**实现 T014（ETL 管线 + 增量同步 + 过滤器）
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**入库 `doc/handoff/` 目录

---

## 3. 下一步需要实现什么 — T012: IM 沟通记录适配器

### 3.1 T012 任务范围

来源：`doc/plans/03-phase2-context.md` §Task T012

| 任务 | 关键产物 |
|---|---|
| T012 | `internal/harvesting/source/feishu.go` + `feishu_test.go` |

**关联**: FR-B03 (P1), MVP 阶段只做飞书

**Step 清单**:
1. 添加飞书 SDK 依赖 — `go get github.com/larksuite/oapi-sdk-go@v3.4.4`
2. 写飞书消息接口 — `FeishuConfig` + `FeishuAdapter` + `FetchMessages`
3. 写飞书测试（mock 客户端）— 编译期断言 + `NewFeishuAdapter` 构造测试
4. 跑测试通过
5. 展示 diff 等用户决定

### 3.2 T012 关键决策点（需下一窗口与用户确认）

1. **飞书 SDK 版本**：plan 指定 `v3.4.4`，M8 规则建议 `@latest`。需确认。
2. **`FeishuAdapter` 是否也需要 `Name()` + `Fetch()` 方法**：T010/T011 已为 GitHubAdapter/CalendarAdapter 添加了这两个方法以匹配 `sourceAdapter` 接口。FeishuAdapter 也应实现同样接口。
3. **飞书 SDK mock 复杂度**：plan 中飞书测试只写了编译期断言 + 构造测试，因为飞书 SDK 客户端 mock 比较复杂。是否需要更深入的测试？

### 3.3 T013-T014 后续任务概览

| 任务 | 关键产物 | 关联需求 |
|---|---|---|
| T013 | `obsidian.go` + `obsidian_test.go` | FR-B04 (P1) |
| T014 | `filter/rules.go` + `filter/llm.go` + `sync.go` + `pipeline.go` + `migrations/0004_context.up.sql` | FR-B05 (P0) + FR-B06 (P0) |

### 3.4 建议执行顺序（下一窗口 T012）

1. 读 `doc/handoff/T011-handoff.md`（本文件）
2. 读 `doc/plans/03-phase2-context.md` §Task T012 完整 spec
3. 跑 `go test ./...` 确认 T011 测试仍 PASS（无回归）
4. 用 AskUserQuestion 确认飞书 SDK 版本 + Name/Fetch 接口对齐
5. Dispatch implementer subagent（带完整 T012 spec + T011 context + 上述注意点）
6. 两轮 review 通过后展示 git diff，**用户决定 commit**
7. commit 后写 `doc/handoff/T012-handoff.md`

---

## 4. 给下一窗口的提示

1. **`internal/harvesting/context.go` 已稳定**：`ContextItem` + `ContextSnapshot` + `FilterMeta` 类型已定义。后续 T012-T014 的适配器都引用这些类型。**不要**改这些类型的字段名或包路径。

2. **`sourceAdapter` 接口模式**：T010 的 `GitHubAdapter` 和 T011 的 `CalendarAdapter` 均已实现 `Name()` + `Fetch(ctx, userID, since)` 方法。T012-T013 的适配器也应实现同样接口，以便 T014 Pipeline 统一消费。接口定义在 T014 的 `pipeline.go` 中，但各适配器应提前准备。

3. **`ContextItem.UserID` 字段**：T010-T011 的 `ContextItem` 目前**没有** `UserID` 字段。T014 plan 的 `sync.go` 中 `UpsertContextItem` 用到了 `item.UserID`。T014 实现时需要给 `ContextItem` 添加 `UserID` 字段。**T010-T013 范围不修**。

4. **Google Calendar 分页已处理**：T011 的 `GoogleCalendarProvider.ListEvents` 已实现 `NextPageToken` 分页循环，`MaxResults(250)`。不需要后续优化。

5. **Google Calendar 时间解析**：已处理 `DateTime` + `Date`（全天事件）两种格式，解析失败回退 `time.Now()`。

6. **Google Calendar `CredentialsFile` 校验**：空值会返回明确错误。

7. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。

8. **用户已确认的选型决策**：
   - LLM: OpenAI（用于 FR-B05 噪音过滤）
   - Embedding: text-embedding-3-small（用于 T013 向量存储）
   - 执行方式: Subagent-Driven
   - 端到端: 暂不启动 docker
   - Google API: `@latest`（当前 v0.285.0）
   - Google Calendar OAuth: Service Account

9. **`doc/handoff/` 状态**：T006-T010 handoff + Phase 0/1 final + T011 handoff 仍未入库（待用户决定是否要单独 `git add doc/` 入库）。

10. **Phase 2 引入的新依赖（T011 已添加）**：
    - `google.golang.org/api v0.285.0`
    - `cloud.google.com/go/auth v0.20.0` (indirect)
    - `cloud.google.com/go/auth/oauth2adapt v0.2.8` (indirect)
    - `cloud.google.com/go/compute/metadata v0.9.0` (indirect)
    - `github.com/felixge/httpsnoop v1.0.4` (indirect)
    - `github.com/go-logr/logr v1.4.3` (indirect)
    - `github.com/go-logr/stdr v1.2.2` (indirect)
    - `github.com/google/s2a-go v0.1.9` (indirect)
    - `github.com/googleapis/enterprise-certificate-proxy v0.3.16` (indirect)
    - `github.com/googleapis/gax-go/v2 v2.22.0` (indirect)
    - `go.opentelemetry.io/*` 系列 (indirect)
    - `google.golang.org/genproto/googleapis/rpc` (indirect)
    - `google.golang.org/grpc v1.81.1` (indirect)

---

## 5. 当前文件结构（Phase 2 / T011 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009 (未动)
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
│   ├── handler/                   ✅ T002+T006+T009 (未动)
│   ├── harvesting/                ✅ T010+T011
│   │   ├── context.go             ✅ T010 (ContextItem + ContextSnapshot + FilterMeta)
│   │   └── source/                ✅ T010+T011
│   │       ├── github.go          ✅ T010 (GitHubAdapter + Name + Fetch)
│   │       ├── github_test.go     ✅ T010 (3 tests + compile-time check)
│   │       ├── calendar.go        ✅ T011 (CalendarProvider + CalendarAdapter + Name + Fetch)
│   │       ├── google_calendar.go ✅ T011 (GoogleCalendarProvider + Service Account + 分页)
│   │       └── calendar_test.go   ✅ T011 (5 tests + compile-time check)
│   ├── middleware/                ✅ T002 (未动)
│   ├── queue/                     ✅ T004 (未动)
│   ├── repository/                ✅ T003+T007 (未动)
│   ├── server/                    ✅ T002+T009 (未动)
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── migrations/                    ✅ T003+T006+T008 (未动)
├── doc/handoff/
│   ├── T001-T009-handoff.md       (untracked)
│   ├── phase0-final-handoff.md    (untracked)
│   ├── phase1-final-handoff.md    (待写, untracked)
│   ├── T010-handoff.md            (untracked)
│   └── T011-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T011 (新增 google.golang.org/api + 间接依赖)
└── go.sum                         ✅ T011 (更新校验和)
```

---

## 6. Phase 2 退出标准验证（T011 末进度）

| # | 标准 | T011 末状态 |
|---|---|---|
| 1 | 4 数据源适配器单元测试通过 | ⚠️ 2/4 完成（GitHub ✅，Calendar ✅，Feishu/Obsidian 待实现） |
| 2 | 规则 + LLM 噪音过滤测试通过 | ❌ T014 范围 |
| 3 | 增量同步（只拉新数据）测试通过 | ❌ T014 范围 |
| 4 | 集成测试：手动触发 → 4 数据源拉取 → 过滤 → 入库 | ❌ T014 范围 |

---

## 7. 待用户决定（必须问，不可以自动做）

1. **T011 commit 已落盘**（commit `3ba1b62`，5 files, +314/-24）。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件 + T011 handoff = 多个文件 untracked。

3. **T012 飞书 SDK 版本**：plan 指定 `v3.4.4`，M8 建议用 `@latest`。

4. **T012 FeishuAdapter 是否实现 `Name()` + `Fetch()` 方法**：按 sourceAdapter 接口模式，应该实现。

5. **T010-T013 留的 minor 项**（可在 T014 统一处理）：
   - GitHub 分页未处理
   - `GitHubConfig.Validate()` 缺失
   - `NewGitHubAdapter` 用 `context.Background()` 创建 oauth2 client
   - Google Calendar `Source` 字段在 Provider 和 Adapter 中双重赋值（Adapter 覆盖 Provider，Provider 中的赋值冗余但不影响功能）
