# Phase 2 / T010 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（implementer + spec reviewer + code quality reviewer + 1 fix subagent）
> **当前任务**: T010 — GitHub 数据源适配器（FR-B01 / FR-B06）
> **状态**: ✅ **DONE**（implementer + spec + code quality + fix 全过，commit `62c88d9`）
> **基线 commit**: `be96d66` (T009)
> **本任务 commit**: `62c88d9` (5 files, +307)
> **用户红线**: 不跑 docker / 不做端到端

---

## 1. 本窗口做了什么（T010 做完了什么内容）

按 `doc/plans/03-phase2-context.md` §Task T010 完整执行了 Step 1-5 + 质量修复。

### 1.1 关联需求

- **FR-B01 (P0)**: GitHub 拉取（commit / PR / review）
- **FR-B06 (P0)**: 增量同步（last_sync_at 时间戳）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `internal/harvesting/context.go` | **新建** | 28 | `ContextItem` + `ContextSnapshot` + `FilterMeta` 类型定义（含 JSON tags） |
| `internal/harvesting/source/github.go` | **新建** | 128 | `GitHubConfig` + `GitHubAdapter` + `NewGitHubAdapter` + `FetchCommits` + `FetchPullRequests` + `Name` + `Fetch` |
| `internal/harvesting/source/github_test.go` | **新建** | 139 | `newMockGitHubClient` + 3 个 mock HTTP 测试 + 编译时接口检查 |
| `go.mod` | **修改** | +3 | 新增 `go-github/v57` + `ghinstallation/v2` + `oauth2` |
| `go.sum` | **修改** | +7 | 对应校验和 |

> **新建 3 个文件 + 修改 2 个文件 = T010 总变更 5 个对象**。

### 1.3 Code Quality Review 后的修复

| # | Fix | 文件:行 | 原问题 | 修复后 |
|---|---|---|---|---|
| C-1 | 缺少 `Name()` + `Fetch()` 方法 | `github.go` | `GitHubAdapter` 未实现 `sourceAdapter` 接口，无法被 Pipeline 消费 | 添加 `Name()` 返回 `"github"` + `Fetch()` 合并 commits+PRs |
| C-2 | PR 未做增量过滤 | `github.go:FetchPullRequests` | `FetchPullRequests` 忽略 `cfg.Since`，拉取全量 PR | 添加客户端 `Since` 过滤，丢弃 `OccurredAt < Since` 的 PR |
| C-3 | `ContextSnapshot` 缺 JSON tag | `context.go` | `AgentRunID`/`UserID`/`SyncedAt` 等字段无 json tag | 补全所有字段 json tags |
| C-4 | `var _` 无意义 | `github_test.go` | `var _ = func() []harvesting.ContextItem { return nil }` 不校验任何接口 | 替换为编译时接口检查 `var _ interface{Name(); Fetch(...)} = (*GitHubAdapter)(nil)` |
| C-5 | 缺少 Fetch 统一方法测试 | `github_test.go` | 无 `TestGitHubAdapter_Fetch_Merged` | 新增测试验证 Fetch 合并 1 commit + 1 PR = 2 items |

### 1.4 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/harvesting/source/...` | ✅ 3 PASS | FetchCommits_Mock + FetchPRs_Mock + Fetch_Merged |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 测试仍通过） |
| `git commit` | ✅ `62c88d9` | 5 files, +307 |

### 1.5 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `c.Commit.Author.Date.Time` 替代 `*c.Commit.Author.Date` | go-github `Timestamp` 嵌入 `time.Time`，不能直接解引用 | ✅ 修正 plan spec 笔误 |
| 2 | `ghinstallation/v2` 依赖已添加但代码中未使用 | plan Step 1 要求添加依赖，当前只支持 PAT 认证；GitHub App 安装认证留给后续需求 | ✅ 依赖已入库，代码路径待扩展 |
| 3 | 新增 `Name()` + `Fetch()` 方法 | plan T010 未明确要求，但 T014 pipeline 需要 `sourceAdapter` 接口 | ✅ 合理架构准备，未越界 FR |
| 4 | 分页未处理（仅取首页 PerPage 50/30） | plan T010 未要求分页，属于优化项 | ✅ T010 范围可接受 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖、未写 `TODO` / `FIXME`、未自动 commit 前实现、未硬编码密钥、未跨 Phase。

### 1.6 git log 输出（T010 commit 后）

```
62c88d9 feat(phase2/T010): github data source adapter + context types (FR-B01/FR-B06)
be96d66 feat(phase1/T009): wire trigger pipeline (webhook/keyword/ddl -> agent_run) + graceful shutdown
2e2b43f feat(phase1/T008): ddl detector + 15min scheduler (user_tasks table + lead-param drift fix)
46c3403 feat(phase1/T007): keyword matcher (longest-match-wins) + agent_run persistence + manual trigger handler
6a4a9fb feat(phase1/T006): webhook listener with hmac-sha256 verification + trigger indexes
```

---

## 2. 本窗口**没做**什么

- ❌ **没有**跑 `make migrate-up`（需 docker postgres）
- ❌ **没有**实现 T011（日历适配器）
- ❌ **没有**实现 T012（飞书适配器）
- ❌ **没有**实现 T013（Obsidian 适配器）
- ❌ **没有**实现 T014（ETL 管线 + 增量同步 + 过滤器）
- ❌ **没有**处理分页（仅取首页，留后续优化）
- ❌ **没有**加 `GitHubConfig.Validate()` 方法
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**入库 `doc/handoff/` 目录

---

## 3. 下一步需要实现什么 — T011: 日历数据源适配器

### 3.1 T011 任务范围

来源：`doc/plans/03-phase2-context.md` §Task T011

| 任务 | 关键产物 |
|---|---|
| T011 | `internal/harvesting/source/calendar.go` + `google_calendar.go` + `calendar_test.go` |

**关联**: FR-B02 (P1), MVP 阶段只做 Google Calendar

**Step 清单**:
1. 写 Calendar 接口（抽象 Google/Outlook）— `CalendarProvider` interface + `CalendarAdapter`
2. 写 Google Calendar 实现 — `GoogleCalendarProvider`（使用 `google.golang.org/api`）
3. 写 Calendar 测试（接口 mock）— `mockCalendar` + `TestCalendarAdapter_Fetch`
4. 跑测试通过
5. 展示 diff 等用户决定

### 3.2 T011 关键决策点（需下一窗口与用户确认）

1. **`google.golang.org/api` 版本**：plan 指定 `v0.157.0`，M8 规则建议 `@latest`。需确认。
2. **Google Calendar OAuth 流程**：plan 代码用 `option.WithCredentialsFile`，但实际生产需要 OAuth2 授权流程。MVP 阶段是否简化为 service account？
3. **`CalendarAdapter` 是否也需要 `Name()` + `Fetch()` 方法**：T010 已为 GitHubAdapter 添加了这两个方法以匹配 `sourceAdapter` 接口。CalendarAdapter 也应实现同样接口。

### 3.3 T012-T014 后续任务概览

| 任务 | 关键产物 | 关联需求 |
|---|---|---|
| T012 | `feishu.go` + `feishu_test.go` | FR-B03 (P1) |
| T013 | `obsidian.go` + `obsidian_test.go` | FR-B04 (P1) |
| T014 | `filter/rules.go` + `filter/llm.go` + `sync.go` + `pipeline.go` + `migrations/0004_context.up.sql` | FR-B05 (P0) + FR-B06 (P0) |

### 3.4 建议执行顺序（下一窗口 T011）

1. 读 `doc/handoff/T010-handoff.md`（本文件）
2. 读 `doc/plans/03-phase2-context.md` §Task T011 完整 spec
3. 跑 `go test ./...` 确认 T010 测试仍 PASS（无回归）
4. 用 AskUserQuestion 确认 Google API 版本 + OAuth 方式
5. Dispatch implementer subagent（带完整 T011 spec + T010 context + 上述注意点）
6. 两轮 review 通过后展示 git diff，**用户决定 commit**
7. commit 后写 `doc/handoff/T011-handoff.md`

---

## 4. 给下一窗口的提示

1. **`internal/harvesting/context.go` 已稳定**：`ContextItem` + `ContextSnapshot` + `FilterMeta` 类型已定义。后续 T011-T014 的适配器都引用这些类型。**不要**改这些类型的字段名或包路径。

2. **`sourceAdapter` 接口模式**：T010 的 `GitHubAdapter` 已实现 `Name()` + `Fetch(ctx, userID, since)` 方法。T011-T013 的适配器也应实现同样接口，以便 T014 Pipeline 统一消费。接口定义在 T014 的 `pipeline.go` 中，但各适配器应提前准备。

3. **`ContextItem.UserID` 字段**：T010 的 `ContextItem` 目前**没有** `UserID` 字段。T014 plan 的 `sync.go` 中 `UpsertContextItem` 用到了 `item.UserID`。T014 实现时需要给 `ContextItem` 添加 `UserID` 字段。**T010-T013 范围不修**。

4. **`ghinstallation/v2` 依赖已入库但未使用**：当前 `NewGitHubAdapter` 只支持 PAT 认证。如果后续需要 GitHub App 安装认证，需在 `github.go` 中添加 `NewGitHubAppAdapter` 构造函数。

5. **分页未处理**：`FetchCommits` 和 `FetchPullRequests` 只取首页。如果数据量超过一页，将静默丢失数据。后续优化时需处理 `resp.NextPage`。

6. **`go-github/v57` 的 `Timestamp` 类型**：`gh.Timestamp` 嵌入 `time.Time`，访问时间值用 `.Time` 而非解引用。后续适配器如果使用 go-github，注意此点。

7. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。

8. **用户已确认的选型决策**：
   - LLM: OpenAI（用于 FR-B05 噪音过滤）
   - Embedding: text-embedding-3-small（用于 T013 向量存储）
   - 执行方式: Subagent-Driven
   - 端到端: 暂不启动 docker

9. **`doc/handoff/` 状态**：T006-T009 handoff + Phase 0/1 final + T010 handoff 仍未入库（待用户决定是否要单独 `git add doc/` 入库）。

10. **Phase 2 引入的新依赖（T010 已添加）**：
    - `github.com/google/go-github/v57 v57.0.0`
    - `github.com/bradleyfalzon/ghinstallation/v2 v2.19.0`
    - `golang.org/x/oauth2 v0.36.0`

---

## 5. 当前文件结构（Phase 2 / T010 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009 (未动)
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
│   ├── handler/                   ✅ T002+T006+T009 (未动)
│   ├── harvesting/                ✅ T010 (新建)
│   │   ├── context.go             ✅ T010 (ContextItem + ContextSnapshot + FilterMeta)
│   │   └── source/                ✅ T010 (新建)
│   │       ├── github.go          ✅ T010 (GitHubAdapter + Name + Fetch)
│   │       └── github_test.go     ✅ T010 (3 tests + compile-time check)
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
│   └── T010-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T010 (新增 3 个依赖)
└── go.sum                         ✅ T010 (更新校验和)
```

---

## 6. Phase 2 退出标准验证（T010 末进度）

| # | 标准 | T010 末状态 |
|---|---|---|
| 1 | 4 数据源适配器单元测试通过 | ⚠️ 1/4 完成（GitHub ✅，Calendar/Feishu/Obsidian 待实现） |
| 2 | 规则 + LLM 噪音过滤测试通过 | ❌ T014 范围 |
| 3 | 增量同步（只拉新数据）测试通过 | ❌ T014 范围 |
| 4 | 集成测试：手动触发 → 4 数据源拉取 → 过滤 → 入库 | ❌ T014 范围 |

---

## 7. 待用户决定（必须问，不可以自动做）

1. **T010 commit 已落盘**（commit `62c88d9`，5 files, +307）。

2. **`doc/handoff/` 是否入库**：当前 10 个 handoff 文件 + 1 个 phase0-final + 1 个 phase1-final（待写）+ 1 个 T010 handoff = 13 个文件 untracked。

3. **T011 Google Calendar OAuth 方式**：
   - 方案 A：Service Account（简化，无需用户授权流程）
   - 方案 B：OAuth2 用户授权（完整，但需回调 URL + token 存储）
   - 方案 C：先写接口 + mock，Google Calendar 实现留后续

4. **`google.golang.org/api` 版本**：plan 指定 `v0.157.0`，M8 建议用 `@latest`。

5. **T010-T013 留的 minor 项**（可在 T014 统一处理）：
   - 分页未处理
   - `GitHubConfig.Validate()` 缺失
   - `NewGitHubAdapter` 用 `context.Background()` 创建 oauth2 client
   - 测试断言粒度可更细
