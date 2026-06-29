# Phase 2 / T012 交接文档 — 给下一窗口

> **生成时间**: 2026-06-19 (Asia/Taipei)
> **生成方式**: subagent-driven-development（手动 implementer + 测试验证）
> **当前任务**: T012 — IM 沟通记录适配器（FR-B03）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，commit `a4c1cc4`）
> **基线 commit**: `3ba1b62` (T011)
> **本任务 commit**: `a4c1cc4` (4 files, +342)
> **用户红线**: 不跑 docker / 不做端到端

---

## 1. 本窗口做了什么（T012 做完了什么内容）

按 `doc/plans/03-phase2-context.md` §Task T012 完整执行了 Step 1-5 + 质量改进。

### 1.1 关联需求

- **FR-B03 (P1)**: IM 拉取（飞书/Slack，MVP 阶段只做飞书）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `internal/harvesting/source/feishu.go` | **新建** | 142 | `FeishuProvider` 接口 + `FeishuAdapter`（Name + Fetch）+ `LarkProvider`（迭代器分页 + AppID/AppSecret 校验）+ `NewFeishuAdapter` 便利构造器 + `derefStr` 辅助函数 |
| `internal/harvesting/source/feishu_test.go` | **新建** | 135 | `mockFeishu` + `mockFeishuFunc` + 8 个测试 + 编译期断言 |
| `go.mod` | **修改** | +1 | 新增 `github.com/larksuite/oapi-sdk-go/v3 v3.9.6`（直接依赖） |
| `go.sum` | **修改** | +33 | 对应校验和 |

> **新建 2 个文件 + 修改 2 个文件 = T012 总变更 4 个对象**。

### 1.3 架构设计说明

T012 采用了与 T011 CalendarAdapter 一致的分层模式：

| 层 | 类型 | 职责 |
|---|---|---|
| 接口层 | `FeishuProvider` | 抽象消息拉取，支持 mock 测试 |
| 适配器层 | `FeishuAdapter` | 实现 `sourceAdapter` 接口（Name + Fetch），遍历 ChatIDs，统一设置 Source/Type |
| 实现层 | `LarkProvider` | 使用飞书 SDK 的 `ListByIterator` 自动分页，毫秒时间戳解析，nil 指针安全 |
| 构造器 | `NewFeishuAdapter` | 便利函数，一步创建 Provider + Adapter |

### 1.4 测试覆盖

| # | 测试名 | 覆盖场景 |
|---|---|---|
| 1 | `TestFeishuAdapter_Fetch` | 基本路径：单 chat，正常返回 |
| 2 | `TestFeishuAdapter_Name` | Name() 返回正确 Source |
| 3 | `TestFeishuAdapter_Fetch_DefaultType` | Type 为空时默认填充 "message" |
| 4 | `TestFeishuAdapter_Fetch_EmptyResult` | Provider 返回空列表 |
| 5 | `TestFeishuAdapter_Fetch_ProviderError` | Provider 返回错误，错误被包装 |
| 6 | `TestFeishuAdapter_Fetch_MultipleChats` | 多 ChatID 遍历，每个 chat 调用一次 |
| 7 | `TestFeishuAdapter_Fetch_NoChatIDs` | ChatIDs 为 nil 时返回空列表 |
| 8 | `TestNewLarkProvider_MissingAppID` | AppID 为空返回明确错误 |
| 9 | `TestNewLarkProvider_MissingAppSecret` | AppSecret 为空返回明确错误 |
| 10 | 编译期断言 | `FeishuAdapter` 满足 `sourceAdapter` 接口 |

### 1.5 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/harvesting/source/...` | ✅ 17 PASS | 5 Calendar + 8 Feishu + 3 GitHub + 1 merged |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 测试仍通过） |
| `git commit` | ✅ `a4c1cc4` | 4 files, +342 |

### 1.6 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | SDK v3.9.6 替代 plan 指定的 v3.4.4 | M8 规则建议 `@latest`，用户确认选择 `@latest` | ✅ 用户决策 |
| 2 | 新增 `FeishuProvider` 接口 + `FeishuAdapter` 分层 | 与 CalendarAdapter 模式一致，支持 mock 测试 | ✅ 架构一致性 |
| 3 | 新增 `Name()` + `Fetch()` 方法 | plan T012 未明确要求，但 T014 pipeline 需要 `sourceAdapter` 接口，用户确认 | ✅ 用户决策 |
| 4 | 使用迭代器分页替代单次 `List` | 避免消息丢失，与 T011 分页改进一致 | ✅ 质量改进 |
| 5 | 新增 AppID/AppSecret 校验 | 与 T011 CredentialsFile 校验一致，属于基本防御性编程 | ✅ 质量改进 |
| 6 | 新增 8 个测试替代 plan 的 2 个 | 用户确认完整测试深度 | ✅ 用户决策 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖（用户已确认）、未写 `TODO` / `FIXME`、未自动 commit 前实现、未硬编码密钥、未跨 Phase。

### 1.7 git log 输出（T012 commit 后）

```
a4c1cc4 feat(phase2/T012): feishu IM data source adapter + lark provider (FR-B03)
3ba1b62 feat(phase2/T011): calendar data source adapter + google calendar provider (FR-B02)
62c88d9 feat(phase2/T010): github data source adapter + context types (FR-B01/FR-B06)
be96d66 feat(phase1/T009): wire trigger pipeline (webhook/keyword/ddl -> agent_run) + graceful shutdown
2e2b43f feat(phase1/T008): ddl detector + 15min scheduler (user_tasks table + lead-param drift fix)
```

---

## 2. 本窗口**没做**什么

- ❌ **没有**跑 `make migrate-up`（需 docker postgres）
- ❌ **没有**实现 T013（Obsidian 适配器）
- ❌ **没有**实现 T014（ETL 管线 + 增量同步 + 过滤器）
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**入库 `doc/handoff/` 目录

---

## 3. 下一步需要实现什么 — T013: 笔记数据源适配器

### 3.1 T013 任务范围

来源：`doc/plans/03-phase2-context.md` §Task T013

| 任务 | 关键产物 |
|---|---|
| T013 | `internal/harvesting/source/obsidian.go` + `obsidian_test.go` |

**关联**: FR-B04 (P1), MVP 阶段只做 Obsidian 本地

**Step 清单**:
1. 写 Obsidian 本地读取 — `ObsidianConfig` + `ObsidianAdapter` + `FetchNotes`
2. 写 Obsidian 测试（用 `t.TempDir()` 模拟 vault）— 编译期断言 + 基本路径 + 过滤
3. 跑测试通过
4. 展示 diff 等用户决定

### 3.2 T013 关键决策点（需下一窗口与用户确认）

1. **ObsidianAdapter 是否也需要 `Name()` + `Fetch()` 方法**：T010-T012 已为 GitHubAdapter/CalendarAdapter/FeishuAdapter 添加了这两个方法以匹配 `sourceAdapter` 接口。ObsidianAdapter 也应实现同样接口。
2. **ObsidianAdapter 是否需要 `ObsidianProvider` 接口分层**：T011/T012 采用了 Provider 接口 + Adapter 分层模式。Obsidian 是本地文件系统读取，不需要远程 API mock，但为了一致性可以考虑分层。
3. **`FetchNotes` 方法签名**：plan 中 `FetchNotes(_ context.Context, from time.Time)` 只有 `from` 没有 `to`，与 `sourceAdapter.Fetch(ctx, userID, since)` 签名不同。需要决定如何适配。

### 3.3 T014 后续任务概览

| 任务 | 关键产物 | 关联需求 |
|---|---|---|
| T014 | `filter/rules.go` + `filter/llm.go` + `sync.go` + `pipeline.go` + `migrations/0004_context.up.sql` | FR-B05 (P0) + FR-B06 (P0) |

### 3.4 建议执行顺序（下一窗口 T013）

1. 读 `doc/handoff/T012-handoff.md`（本文件）
2. 读 `doc/plans/03-phase2-context.md` §Task T013 完整 spec
3. 跑 `go test ./...` 确认 T012 测试仍 PASS（无回归）
4. 用 AskUserQuestion 确认 Name/Fetch 接口对齐 + Provider 分层 + FetchNotes 签名适配
5. 实现 ObsidianAdapter + 测试
6. 跑测试 + build 验证
7. 展示 git diff，**用户决定 commit**
8. commit 后写 `doc/handoff/T013-handoff.md`

---

## 4. 给下一窗口的提示

1. **`internal/harvesting/context.go` 已稳定**：`ContextItem` + `ContextSnapshot` + `FilterMeta` 类型已定义。后续 T013-T014 的适配器都引用这些类型。**不要**改这些类型的字段名或包路径。

2. **`sourceAdapter` 接口模式**：T010 的 `GitHubAdapter`、T011 的 `CalendarAdapter`、T012 的 `FeishuAdapter` 均已实现 `Name()` + `Fetch(ctx, userID, since)` 方法。T013 的 `ObsidianAdapter` 也应实现同样接口，以便 T014 Pipeline 统一消费。接口定义在 T014 的 `pipeline.go` 中，但各适配器应提前准备。

3. **`ContextItem.UserID` 字段**：T010-T012 的 `ContextItem` 目前**没有** `UserID` 字段。T014 plan 的 `sync.go` 中 `UpsertContextItem` 用到了 `item.UserID`。T014 实现时需要给 `ContextItem` 添加 `UserID` 字段。**T010-T013 范围不修**。

4. **FeishuAdapter 使用迭代器分页**：T012 的 `LarkProvider.ListMessages` 使用 `ListByIterator` 自动处理分页，不需要后续优化。

5. **FeishuAdapter 时间处理**：飞书 API 的 `StartTime`/`EndTime` 使用秒级时间戳，`CreateTime` 是毫秒级时间戳字符串。`LarkProvider.convertMessage` 使用 `time.UnixMilli` 正确转换。

6. **FeishuAdapter nil 指针安全**：所有 SDK 返回的指针字段（`MessageId`、`MsgType`、`Body.Content`、`Sender.Id`、`CreateTime`）都通过 `derefStr` 或显式 nil 检查安全处理。

7. **`derefStr` 函数**：定义在 `feishu.go` 中，如果其他适配器也需要，可以提取到公共位置。当前仅在 feishu.go 中使用。

8. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。

9. **用户已确认的选型决策**：
   - LLM: OpenAI（用于 FR-B05 噪音过滤）
   - Embedding: text-embedding-3-small（用于 T013 向量存储）
   - 执行方式: Subagent-Driven
   - 端到端: 暂不启动 docker
   - Google API: `@latest`（当前 v0.285.0）
   - Google Calendar OAuth: Service Account
   - 飞书 SDK: `@latest`（当前 v3.9.6）

10. **`doc/handoff/` 状态**：T006-T012 handoff + Phase 0/1 final 仍未入库（待用户决定是否要单独 `git add doc/` 入库）。

11. **Phase 2 引入的新依赖（T012 已添加）**：
    - `github.com/larksuite/oapi-sdk-go/v3 v3.9.6`

---

## 5. 当前文件结构（Phase 2 / T012 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009 (未动)
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
│   ├── handler/                   ✅ T002+T006+T009 (未动)
│   ├── harvesting/                ✅ T010+T011+T012
│   │   ├── context.go             ✅ T010 (ContextItem + ContextSnapshot + FilterMeta)
│   │   └── source/                ✅ T010+T011+T012
│   │       ├── github.go          ✅ T010 (GitHubAdapter + Name + Fetch)
│   │       ├── github_test.go     ✅ T010 (3 tests + compile-time check)
│   │       ├── calendar.go        ✅ T011 (CalendarProvider + CalendarAdapter + Name + Fetch)
│   │       ├── google_calendar.go ✅ T011 (GoogleCalendarProvider + Service Account + 分页)
│   │       ├── calendar_test.go   ✅ T011 (5 tests + compile-time check)
│   │       ├── feishu.go          ✅ T012 (FeishuProvider + FeishuAdapter + LarkProvider + 迭代器分页)
│   │       └── feishu_test.go     ✅ T012 (8 tests + compile-time check)
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
│   ├── T011-handoff.md            (untracked)
│   └── T012-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T012 (新增 larksuite/oapi-sdk-go/v3)
└── go.sum                         ✅ T012 (更新校验和)
```

---

## 6. Phase 2 退出标准验证（T012 末进度）

| # | 标准 | T012 末状态 |
|---|---|---|
| 1 | 4 数据源适配器单元测试通过 | ⚠️ 3/4 完成（GitHub ✅，Calendar ✅，Feishu ✅，Obsidian 待实现） |
| 2 | 规则 + LLM 噪音过滤测试通过 | ❌ T014 范围 |
| 3 | 增量同步（只拉新数据）测试通过 | ❌ T014 范围 |
| 4 | 集成测试：手动触发 → 4 数据源拉取 → 过滤 → 入库 | ❌ T014 范围 |

---

## 7. 待用户决定（必须问，不可以自动做）

1. **T012 commit 已落盘**（commit `a4c1cc4`，4 files, +342）。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件 + T012 handoff = 多个文件 untracked。

3. **T013 ObsidianAdapter 是否实现 `Name()` + `Fetch()` 方法**：按 sourceAdapter 接口模式，应该实现。

4. **T013 ObsidianAdapter 是否需要 Provider 接口分层**：Obsidian 是本地文件系统，不需要远程 API mock，但为了一致性可以考虑。

5. **T013 `FetchNotes` 签名适配**：plan 中 `FetchNotes(_ context.Context, from time.Time)` 与 `sourceAdapter.Fetch(ctx, userID, since)` 签名不同，需要决定如何适配。

6. **T010-T013 留的 minor 项**（可在 T014 统一处理）：
   - GitHub 分页未处理
   - `GitHubConfig.Validate()` 缺失
   - `NewGitHubAdapter` 用 `context.Background()` 创建 oauth2 client
   - Google Calendar `Source` 字段在 Provider 和 Adapter 中双重赋值（Adapter 覆盖 Provider，Provider 中的赋值冗余但不影响功能）
