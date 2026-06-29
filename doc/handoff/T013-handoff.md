# Phase 2 / T013 交接文档 — 给下一窗口

> **生成时间**: 2026-06-19 (Asia/Taipei)
> **生成方式**: subagent-driven-development（implementer + spec reviewer + code quality reviewer + fix subagent）
> **当前任务**: T013 — 笔记数据源适配器（FR-B04）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，commit `74e17a3`）
> **基线 commit**: `a4c1cc4` (T012)
> **本任务 commit**: `74e17a3` (2 files, +379)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T013 做完了什么内容）

按 `doc/plans/03-phase2-context.md` §Task T013 执行，采用 Subagent-Driven 流程（implementer → spec review → code quality review → fix → 验证）。

### 1.1 关联需求

- **FR-B04 (P1)**: 笔记读取（Obsidian/Notion，MVP 阶段只做 Obsidian 本地）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 行数 | 说明 |
|---|---|---|---|
| `internal/harvesting/source/obsidian.go` | **新建** | 112 | `ObsidianConfig` + `ObsidianAdapter`（Name + Fetch）+ `NewObsidianAdapter`（默认 MaxDepth=3/MaxFiles=200）+ `errObsidianMaxFiles` 哨兵错误 + `filepath.WalkDir` 遍历 + 深度剪枝 + .md 大小写过滤 + mtime 过滤 + MaxFiles 限流 + ctx 取消检查 + `filepath.ToSlash` URL 规范化 |
| `internal/harvesting/source/obsidian_test.go` | **新建** | 228 | 10 个测试 + 编译期接口断言 + 3 个辅助函数（writeNote / findItemByTitle / timeApprox） |

> **新建 2 个文件 = T013 总变更 2 个对象**。未修改 go.mod / go.sum / context.go（无新依赖，仅用 stdlib `io/fs`）。

### 1.3 架构设计说明

T013 采用与 GitHubAdapter 一致的「无 Provider 分层」模式（用户确认）：

| 层 | 类型 | 职责 |
|---|---|---|
| 配置层 | `ObsidianConfig` | VaultPath + MaxDepth + MaxFiles |
| 适配器层 | `ObsidianAdapter` | 实现 `sourceAdapter` 接口（Name + Fetch），直接用 `filepath.WalkDir` 遍历本地 vault |
| 构造器 | `NewObsidianAdapter` | 应用默认值（MaxDepth=3, MaxFiles=200） |

**与 T011/T012 的差异**：Calendar/Feishu 有 Provider 接口分层（远程 API 需要 mock）；Obsidian 是本地 FS，用 `t.TempDir()` 真实测试，不需要 Provider 抽象（与 GitHubAdapter 一致）。

### 1.4 用户确认的设计决策

| # | 决策点 | 用户选择 | 理由 |
|---|---|---|---|
| 1 | ObsidianAdapter 是否实现 Name() + Fetch() | ✅ 是 | 匹配 sourceAdapter 接口，T014 Pipeline 统一消费 |
| 2 | 是否需要 ObsidianProvider 接口分层 | ❌ 否 | 本地 FS，t.TempDir() 可真实测试，与 GitHubAdapter 一致 |
| 3 | FetchNotes 签名如何适配 | 只实现 Fetch(ctx, userID, since) | 不保留 plan 的 FetchNotes(ctx, from)，文件遍历逻辑放 Fetch 内 |

### 1.5 测试覆盖

| # | 测试名 | 覆盖场景 |
|---|---|---|
| 1 | `TestObsidianAdapter_Name` | Name() 返回 "obsidian" |
| 2 | `TestObsidianAdapter_Fetch_DefaultsApplied` | 零值配置应用默认 MaxDepth=3/MaxFiles=200 |
| 3 | `TestObsidianAdapter_Fetch` | 基本路径：2 .md + 1 .txt，验证 Source/Type/Title/Content/URL/OccurredAt |
| 4 | `TestObsidianAdapter_Fetch_FiltersByMtime` | 旧文件（before since）排除，新文件（after since）收录 |
| 5 | `TestObsidianAdapter_Fetch_EmptyVault` | 空 vault 返回 0 items，无错误 |
| 6 | `TestObsidianAdapter_Fetch_RespectsMaxFiles` | 5 文件 MaxFiles=3，返回恰好 3 |
| 7 | `TestObsidianAdapter_Fetch_RespectsMaxDepth` | 深度超限文件跳过，浅文件收录（含深度语义文档） |
| 8 | `TestObsidianAdapter_Fetch_OnlyMarkdown` | .md/.MD 收录，.txt/.markdown 排除（大小写不敏感） |
| 9 | `TestObsidianAdapter_Fetch_RespectsContextCancellation` | 已取消 ctx 返回错误 |
| 10 | `TestObsidianAdapter_Fetch_NonexistentVault` | 不存在路径优雅降级（0 items，无错误） |
| 11 | 编译期断言 | `ObsidianAdapter` 满足 `Name() + Fetch(ctx, userID, since)` 签名 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/harvesting/source/... -run TestObsidianAdapter -v` | ✅ 10 PASS | Obsidian 测试全过 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + T010-T012 测试仍通过） |
| `git commit` | ✅ `74e17a3` | 2 files, +379 |

### 1.7 Subagent-Driven 流程记录

| 阶段 | 子代理 | 结果 |
|---|---|---|
| 实现 | implementer subagent | DONE：TDD 写测试 → 确认失败 → 写实现 → 8/8 通过 |
| 规格审查 | spec reviewer subagent | ✅ 合规：16 项检查全过，无 missing/extra |
| 代码质量审查 | code quality reviewer subagent | ✅ Approved with suggestions：2 Important + 3 valuable Minor |
| 修复 | fix subagent | DONE：Walk→WalkDir + ToSlash + 2 新测试 + URL 断言加强，10/10 通过 |

### 1.8 代码质量审查修复项

| # | 严重度 | 问题 | 修复 |
|---|---|---|---|
| 1 | 🟡 Important | `filepath.Walk` 过时（Go 1.16+ 推荐 WalkDir） | 改用 `filepath.WalkDir` + `fs.DirEntry` + `d.Info()` |
| 2 | 🟡 Important | Windows `file://` URL 含反斜杠 | 加 `filepath.ToSlash(path)` |
| 3 | 🟢 Minor | 缺上下文取消测试 | 新增 `TestObsidianAdapter_Fetch_RespectsContextCancellation` |
| 4 | 🟢 Minor | 缺不存在 vault 路径测试 | 新增 `TestObsidianAdapter_Fetch_NonexistentVault` |
| 5 | 🟢 Minor | URL 断言过弱（仅检查非空） | 改为 `strings.HasPrefix(it.URL, "file://")` |

### 1.9 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | 无 ObsidianProvider 接口分层 | 用户确认（本地 FS 不需要 mock） | ✅ 用户决策 |
| 2 | 无 FetchNotes 方法，只实现 Fetch | 用户确认（匹配 sourceAdapter 接口） | ✅ 用户决策 |
| 3 | `filepath.WalkDir` 替代 plan 的 `filepath.Walk` | 代码质量审查建议，Go 1.16+ 推荐 | ✅ 质量改进 |
| 4 | `file://` URL 加 `filepath.ToSlash` | 代码质量审查建议，修复 Windows 路径 | ✅ 质量改进 |
| 5 | 10 个测试替代 plan 的 1 个 | 完整覆盖边界场景 | ✅ 质量改进 |
| 6 | `OnlyMarkdown` 测试用 alpha.md/beta.MD 替代 file.md/file.MD | Windows FS 大小写不敏感，同名文件冲突 | ✅ 平台适配 |
| 7 | 深度语义文档化（depth=分隔符数，root=0） | plan 注释与公式差 1，按公式实现并文档化 | ✅ 文档改进 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加新依赖（仅 stdlib）、未写 TODO/FIXME、未自动 commit（用户确认后提交）、未硬编码密钥、未改 context.go、未跨 Phase。

### 1.10 git log 输出（T013 commit 后）

```
74e17a3 feat(phase2/T013): obsidian note data source adapter (FR-B04)
a4c1cc4 feat(phase2/T012): feishu IM data source adapter + lark provider (FR-B03)
3ba1b62 feat(phase2/T011): calendar data source adapter + google calendar provider (FR-B02)
62c88d9 feat(phase2/T010): github data source adapter + context types (FR-B01/FR-B06)
```

---

## 2. 本窗口**没做**什么

- ❌ **没有**跑 `make migrate-up`（需 docker postgres）
- ❌ **没有**实现 T014（ETL 管线 + 增量同步 + 过滤器）
- ❌ **没有**改 trigger / handler / server / config / context.go 任何文件
- ❌ **没有**给 `ContextItem` 加 `UserID` 字段（T014 范围）
- ❌ **没有**定义 `sourceAdapter` 接口（T014 的 pipeline.go 范围）
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**入库 `doc/handoff/` 目录（用户选择只提交 T013 代码文件）

---

## 3. 下一步需要实现什么 — T014: ETL 管线与增量同步

### 3.1 T014 任务范围

来源：`doc/plans/03-phase2-context.md` §Task T014

| 任务 | 关键产物 | 关联需求 |
|---|---|---|
| T014 | `filter/rules.go` + `filter/rules_test.go` + `filter/llm.go` + `filter/llm_test.go` + `sync.go` + `sync_test.go` + `pipeline.go` + `pipeline_test.go` + `migrations/0004_context.up.sql` + `migrations/0004_context.down.sql` | FR-B05 (P0) + FR-B06 (P0) |

**T014 是 Phase 2 的最后一个任务，完成后达到 M2 退出标准。**

### 3.2 T014 Step 清单（15 步）

1. 写 `context_items` 表迁移（`migrations/0004_context.up.sql` + `.down.sql`）
2. 写规则过滤器 `filter/rules.go`（5 类默认规则 + 空内容过滤）
3. 写规则过滤器测试 `filter/rules_test.go`
4. 跑测试确认失败 → 通过
5. （Step 2 已实现，跳过）
6. 写 LLM 过滤器 `filter/llm.go`（NoiseClassifier 接口 + LLMFilter + PromptTemplate + ParseResponse）
7. 写 LLM 过滤器测试 `filter/llm_test.go`（mock classifier + 错误时保留所有）
8. 跑测试通过
9. 写 sync 增量同步 `sync.go`（SyncStore + GetLastSync + UpdateLastSync + UpsertContextItem）
10. 写 sync 测试 `sync_test.go`（需 DATABASE_URL，跳过集成）
11. 写 pipeline 编排 `pipeline.go`（Pipeline + sourceAdapter 接口 + NewPipeline + Run + findAdapter + NotFoundError）
12. 写 pipeline 测试 `pipeline_test.go`（mock adapter + mock classifier，测 filter flow）
13. 跑全部测试 `go test ./...`
14. 编译验证 `go build ./...`
15. 展示 diff 等用户决定

### 3.3 T014 关键决策点（需下一窗口与用户确认）

1. **`ContextItem` 添加 `UserID` 字段**：plan §T014 Step 9 明确要求修改 `context.go` 给 `ContextItem` 加 `UserID string`。各适配器（GitHub/Calendar/Feishu/Obsidian）需在创建 ContextItem 时填入 UserID。**这是跨文件重构，§7.1 允许，但建议先问用户确认**。

2. **`sourceAdapter` 接口定义位置**：plan 把 `sourceAdapter` 接口定义在 `pipeline.go` 中。T010-T013 的 4 个适配器都已实现 `Name() + Fetch(ctx, userID, since)` 方法，但接口本身未定义（各适配器用编译期断言验证签名）。T014 在 pipeline.go 定义接口后，4 个适配器自动满足。

3. **LLM 过滤器实现深度**：plan 的 `llm.go` 只定义了 `NoiseClassifier` 接口 + `LLMFilter` + `PromptTemplate` + `ParseResponse`，没有真实 OpenAI 调用实现。用户已确认 LLM 用 OpenAI（用于 FR-B05 噪音过滤）。**需确认**：T014 是否实现真实 OpenAI 调用，还是只留接口 + mock？

4. **sync 集成测试**：plan 的 `sync_test.go` 在无 `DATABASE_URL` 时 `t.Skip`。用户红线「不跑 docker」，所以 sync 集成测试会跳过。**需确认**：是否接受 sync 只有编译通过 + 跳过测试？

5. **`sync_timestamps` 表**：plan 的 `sync.go` 用到 `sync_timestamps` 表，但 `migrations/0004_context.up.sql` 只创建了 `context_items` 表，**没有** `sync_timestamps` 表。**需确认**：T014 是否需要在迁移中加 `sync_timestamps` 表？

6. **新增依赖**：T014 的 `pipeline.go` 用到 `github.com/jackc/pgx/v5/pgxpool`（已在 go.mod 中）。LLM 过滤器若实现真实 OpenAI 调用，需新增 `github.com/sashabaranov/go-openai` 或类似依赖。**需先问用户**（C7 红线）。

### 3.4 建议执行顺序（下一窗口 T014）

1. 读 `doc/handoff/T013-handoff.md`（本文件）
2. 读 `doc/plans/03-phase2-context.md` §Task T014 完整 spec
3. 跑 `go test ./...` 确认 T013 测试仍 PASS（无回归）
4. 用 AskUserQuestion 确认上述 6 个决策点
5. 按 Step 1-15 实现（建议用 Subagent-Driven，每几个 Step 一个 implementer subagent）
6. 跑测试 + build 验证
7. 展示 git diff，**用户决定 commit**
8. commit 后写 `doc/handoff/T014-handoff.md` + Phase 2 final handoff

---

## 4. 给下一窗口的提示

1. **`internal/harvesting/context.go` 仍稳定但 T014 需改**：`ContextItem` 目前**没有** `UserID` 字段。T014 plan §Step 9 要求添加。添加后 4 个适配器（GitHub/Calendar/Feishu/Obsidian）都需更新 ContextItem 构造处填入 UserID。ObsidianAdapter 的 Fetch 接收 `userID` 参数但当前未存储（有注释说明是 T014 scope）。

2. **`sourceAdapter` 接口模式已就绪**：4 个适配器（GitHubAdapter/CalendarAdapter/FeishuAdapter/ObsidianAdapter）均已实现 `Name() string` + `Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)`。T014 的 `pipeline.go` 定义接口后自动满足。接口签名：
   ```go
   type sourceAdapter interface {
       Name() string
       Fetch(ctx context.Context, userID string, since time.Time) ([]ContextItem, error)
   }
   ```

3. **ObsidianAdapter 深度语义**：`depth = strings.Count(rel, string(filepath.Separator))`，root=depth 0。文件 depth <= MaxDepth 收录，目录 depth > MaxDepth 剪枝（SkipDir）。已在 `obsidian.go` doc comment 和 `obsidian_test.go` 测试注释中文档化。

4. **ObsidianAdapter MaxFiles 实现**：用哨兵错误 `errObsidianMaxFiles` 提前终止 `filepath.WalkDir`，返回前用 `errors.Is` 过滤，不暴露给调用方。检查顺序：depth → .md → mtime → MaxFiles → ReadFile（先过滤再限流再读文件，避免浪费 I/O）。

5. **ObsidianAdapter URL 格式**：`"file://" + filepath.ToSlash(path)`。Windows 上产生 `file://C:/Users/...`（两个斜杠，非完全 RFC 8089 合规，但 MVP 可接受，URL 仅用于展示）。

6. **ObsidianAdapter 无 Metadata 字段**：与 GitHub/Feishu 不同，ObsidianAdapter 的 ContextItem 未设置 `Metadata`。如后续需按路径过滤/去重，可加 `Metadata: map[string]string{"path": rel}`。不阻塞 T014。

7. **ai-coding-boundary P1 红线继续生效**：implementer subagent **不得** `git commit`；由 controller 在两轮 review 通过后展示 diff，**用户在主窗口决定**是否 commit。

8. **用户已确认的选型决策**（累计）：
   - LLM: OpenAI（用于 FR-B05 噪音过滤）
   - Embedding: text-embedding-3-small（用于 Phase 3 向量存储）
   - 执行方式: Subagent-Driven
   - 端到端: 暂不启动 docker
   - Google API: `@latest`（当前 v0.285.0）
   - Google Calendar OAuth: Service Account
   - 飞书 SDK: `@latest`（当前 v3.9.6）
   - T013 Obsidian: 无 Provider 分层 + 只实现 Fetch + WalkDir + ToSlash

9. **`doc/handoff/` 状态**：T001-T013 handoff + Phase 0/1 final 仍未入库（用户选择只提交 T013 代码文件，未提交 doc/）。

10. **Phase 2 引入的依赖（T010-T013 累计）**：
    - `github.com/google/go-github/v57`（T010）
    - `golang.org/x/oauth2`（T010）
    - `google.golang.org/api`（T011）
    - `github.com/larksuite/oapi-sdk-go/v3 v3.9.6`（T012）
    - T013 **无新依赖**（仅用 stdlib `io/fs`）

11. **T010-T013 留的 minor 项**（可在 T014 统一处理）：
    - GitHub 分页未处理
    - `GitHubConfig.Validate()` 缺失
    - `NewGitHubAdapter` 用 `context.Background()` 创建 oauth2 client
    - Google Calendar `Source` 字段在 Provider 和 Adapter 中双重赋值（Adapter 覆盖 Provider，Provider 中的赋值冗余但不影响功能）
    - ObsidianAdapter 无 Metadata 字段（可选改进）

---

## 5. 当前文件结构（Phase 2 / T013 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009 (未动)
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
│   ├── handler/                   ✅ T002+T006+T009 (未动)
│   ├── harvesting/                ✅ T010+T011+T012+T013
│   │   ├── context.go             ✅ T010 (ContextItem + ContextSnapshot + FilterMeta，T014 将加 UserID)
│   │   └── source/                ✅ T010+T011+T012+T013
│   │       ├── github.go          ✅ T010 (GitHubAdapter + Name + Fetch)
│   │       ├── github_test.go     ✅ T010 (3 tests + compile-time check)
│   │       ├── calendar.go        ✅ T011 (CalendarProvider + CalendarAdapter + Name + Fetch)
│   │       ├── google_calendar.go ✅ T011 (GoogleCalendarProvider + Service Account + 分页)
│   │       ├── calendar_test.go   ✅ T011 (5 tests + compile-time check)
│   │       ├── feishu.go          ✅ T012 (FeishuProvider + FeishuAdapter + LarkProvider + 迭代器分页)
│   │       ├── feishu_test.go     ✅ T012 (8 tests + compile-time check)
│   │       ├── obsidian.go        ✅ T013 (ObsidianAdapter + Name + Fetch + WalkDir + MaxDepth/MaxFiles)
│   │       └── obsidian_test.go   ✅ T013 (10 tests + compile-time check)
│   ├── middleware/                ✅ T002 (未动)
│   ├── queue/                     ✅ T004 (未动)
│   ├── repository/                ✅ T003+T007 (未动)
│   ├── server/                    ✅ T002+T009 (未动)
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── migrations/                    ✅ T003+T006+T008 (未动，T014 将加 0004)
├── doc/handoff/
│   ├── T001-T012-handoff.md       (untracked)
│   ├── phase0-final-handoff.md    (untracked)
│   ├── phase1-final-handoff.md    (untracked)
│   └── T013-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T012 (未动，T013 无新依赖)
└── go.sum                         ✅ T012 (未动)
```

---

## 6. Phase 2 退出标准验证（T013 末进度）

| # | 标准 | T013 末状态 |
|---|---|---|
| 1 | 4 数据源适配器单元测试通过 | ✅ **4/4 完成**（GitHub ✅，Calendar ✅，Feishu ✅，Obsidian ✅） |
| 2 | 规则 + LLM 噪音过滤测试通过 | ❌ T014 范围 |
| 3 | 增量同步（只拉新数据）测试通过 | ❌ T014 范围 |
| 4 | 集成测试：手动触发 → 4 数据源拉取 → 过滤 → 入库 | ❌ T014 范围 |

**Phase 2 进度：T010 ✅ + T011 ✅ + T012 ✅ + T013 ✅ + T014 ❌ = 4/5 任务完成。T014 完成后达到 M2 退出标准。**

---

## 7. 待用户决定（必须问，不可以自动做）

1. **T013 commit 已落盘**（commit `74e17a3`，2 files, +379）。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件 + T013 handoff = 多个文件 untracked。用户本次选择只提交 T013 代码文件。

3. **T014 `ContextItem` 添加 `UserID` 字段**：plan §Step 9 要求。需确认是否执行此跨文件重构（影响 4 个适配器）。

4. **T014 LLM 过滤器实现深度**：只留接口 + mock，还是实现真实 OpenAI 调用（需新增依赖，C7 红线需先问）？

5. **T014 sync 集成测试**：无 DATABASE_URL 时 t.Skip，是否接受？

6. **T014 `sync_timestamps` 表**：plan 的 sync.go 用到此表但迁移未创建，是否需要在 0004 迁移中添加？

7. **T010-T013 留的 minor 项**（可在 T014 统一处理）：
   - GitHub 分页未处理
   - `GitHubConfig.Validate()` 缺失
   - `NewGitHubAdapter` 用 `context.Background()` 创建 oauth2 client
   - Google Calendar `Source` 字段双重赋值（冗余但不影响功能）
   - ObsidianAdapter 无 Metadata 字段（可选改进）
