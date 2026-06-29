# Phase 2 / T014 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T014 — ETL 管线与增量同步（FR-B05 + FR-B06）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T014 做完了什么内容）

按 `doc/plans/03-phase2-context.md` §Task T014 执行，结合用户确认的 6 个决策点。

### 1.1 关联需求

- **FR-B05 (P0)**: 噪音过滤（规则 + LLM 双层）
- **FR-B06 (P0)**: 增量同步（last_sync_at 时间戳）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/harvesting/context.go` | **修改** | 添加 `NoiseClassifier` 接口 + `ContextItem.UserID` 字段 + `context` import |
| `internal/harvesting/source/github.go` | **修改** | Fetch 中填入 `UserID` |
| `internal/harvesting/source/calendar.go` | **修改** | Fetch 中填入 `UserID` |
| `internal/harvesting/source/feishu.go` | **修改** | Fetch 中填入 `UserID` |
| `internal/harvesting/source/obsidian.go` | **修改** | Fetch 中填入 `UserID` + 更新注释 |
| `internal/harvesting/filter/rules.go` | **新建** | `Rule` + `RuleFilter` + `DefaultRules`（5 类默认规则 + 空内容过滤） |
| `internal/harvesting/filter/rules_test.go` | **新建** | 5 个测试 |
| `internal/harvesting/filter/llm.go` | **新建** | `LLMFilter` + `EinoClassifier`（使用 Eino ChatModel 调用 OpenAI）+ `ParseResponse` + `DefaultPromptTemplate` |
| `internal/harvesting/filter/llm_test.go` | **新建** | 6 个单元测试 + 1 个集成测试（Skip 无 API Key）+ 编译期断言 |
| `internal/harvesting/sync.go` | **新建** | `SyncStore` + `GetLastSync`（默认 7 天前）+ `UpdateLastSync` + `UpsertContextItem` |
| `internal/harvesting/sync_test.go` | **新建** | 3 个集成测试（Skip 无 DATABASE_URL） |
| `internal/harvesting/pipeline.go` | **新建** | `Pipeline` + `sourceAdapter` 接口 + `RuleFilterer`/`LLMFilterer` 接口 + `NewPipeline` + `Run` + `findAdapter` + `NotFoundError` + `splitSource` |
| `internal/harvesting/pipeline_test.go` | **新建** | 5 个测试 + 编译期断言 |
| `migrations/0004_context.up.sql` | **新建** | `context_items` 表 + `sync_timestamps` 表 + 3 个索引 |
| `migrations/0004_context.down.sql` | **新建** | DROP 两张表 |
| `go.mod` | **修改** | 新增 `github.com/cloudwego/eino v0.9.9` + `github.com/cloudwego/eino-ext/components/model/openai v0.1.13` 及传递依赖 |
| `go.sum` | **修改** | 对应更新 |

> **新建 10 个文件 + 修改 7 个文件 = T014 总变更 17 个对象**。

### 1.3 架构设计说明

T014 采用分层架构，通过接口抽象打破 `harvesting` ↔ `filter` 的 import cycle：

| 层 | 类型 | 职责 |
|---|---|---|
| 类型层 | `harvesting.ContextItem` + `NoiseClassifier` | 共享数据类型 + 噪音分类接口（定义在 harvesting 包避免循环依赖） |
| 过滤层 | `filter.RuleFilter` + `filter.LLMFilter` + `filter.EinoClassifier` | 规则过滤 + LLM 过滤 + Eino OpenAI 实现 |
| 同步层 | `harvesting.SyncStore` | 增量同步状态管理 + ContextItem 持久化 |
| 编排层 | `harvesting.Pipeline` | ETL 管线编排（Fetch → RuleFilter → LLMFilter → Persist → UpdateSync） |

**关键设计决策**：`NoiseClassifier` 接口定义在 `harvesting` 包（而非 `filter` 包），因为 `pipeline.go` 需要引用过滤器接口，而 `filter` 包又依赖 `harvesting.ContextItem`，若 `NoiseClassifier` 在 `filter` 包中会导致 `harvesting` → `filter` → `harvesting` 的 import cycle。将接口提升到 `harvesting` 包，`pipeline.go` 通过 `RuleFilterer`/`LLMFilterer` 本地接口解耦，`filter` 包只依赖 `harvesting`（单向依赖）。

### 1.4 用户确认的设计决策

| # | 决策点 | 用户选择 | 理由 |
|---|---|---|---|
| 1 | ContextItem 添加 UserID | ✅ 添加 | FR-B06 增量同步 + 入库必需 |
| 2 | LLM 过滤器实现深度 | 使用 Eino 框架 + 真实 OpenAI 调用 | 用户明确要求用 Eino，不能有 mock 数据 |
| 3 | sync 集成测试 | 接受 Skip | 无 docker，无 DATABASE_URL |
| 4 | sync_timestamps 表 | ✅ 添加到 0004 迁移 | sync.go 依赖此表 |

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestRuleFilter_Apply` | filter | 5 类默认规则 + 空内容过滤 |
| 2 | `TestRuleFilter_Apply_AllKept` | filter | 全部保留场景 |
| 3 | `TestRuleFilter_Apply_EmptyInput` | filter | 空输入 |
| 4 | `TestRuleFilter_Apply_CustomRules` | filter | 自定义规则 |
| 5 | `TestRuleFilter_Apply_ContentMatch` | filter | Content 字段匹配 |
| 6 | `TestLLMFilter_Apply_WithPatternClassifier` | filter | LLM 过滤 + pattern classifier（接口抽象） |
| 7 | `TestLLMFilter_ErrorKeepsAll` | filter | LLM 错误时保守保留 |
| 8 | `TestLLMFilter_EmptyInput` | filter | 空输入 |
| 9 | `TestParseResponse_Noise` | filter | JSON 解析 is_noise=true |
| 10 | `TestParseResponse_NotNoise` | filter | JSON 解析 is_noise=false |
| 11 | `TestParseResponse_InvalidJSON` | filter | 无效 JSON 返回错误 |
| 12 | `TestEinoClassifier_Integration` | filter | 真实 OpenAI 调用（Skip 无 API Key） |
| 13 | `TestSyncStore_GetLastSync_Default` | harvesting | 集成测试（Skip 无 DB） |
| 14 | `TestSyncStore_UpdateLastSync` | harvesting | 集成测试（Skip 无 DB） |
| 15 | `TestSyncStore_UpsertContextItem` | harvesting | 集成测试（Skip 无 DB） |
| 16 | `TestPipeline_FilterFlow` | harvesting | 完整 filter flow（rule + llm） |
| 17 | `TestPipeline_FilterFlow_AllKept` | harvesting | 全部保留 |
| 18 | `TestPipeline_FilterFlow_EmptyInput` | harvesting | 空输入 |
| 19 | `TestSplitSource` | harvesting | dataSourceID 解析（5 种格式） |
| 20 | `TestNotFoundError` | harvesting | 错误消息格式 |
| 21 | 编译期断言 | filter + harvesting | `EinoClassifier`/`patternClassifier` 满足 `NoiseClassifier`；`testAdapter` 满足 `sourceAdapter`；`testRuleFilterer`/`testLLMFilterer` 满足接口 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/harvesting/filter/...` | ✅ 11 PASS + 1 SKIP | EinoClassifier 集成测试 Skip |
| `go test ./internal/harvesting/...` | ✅ 5 PASS + 3 SKIP | sync 集成测试 Skip |
| `go test ./internal/harvesting/source/...` | ✅ 27 PASS | 无回归 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + T010-T013 测试仍通过） |

### 1.7 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `NoiseClassifier` 定义在 `harvesting` 包而非 `filter` 包 | 打破 `harvesting` ↔ `filter` import cycle | ✅ 架构改进 |
| 2 | `pipeline.go` 不导入 `filter` 包，改用 `RuleFilterer`/`LLMFilterer` 接口 | 同上，避免循环依赖 | ✅ 架构改进 |
| 3 | `NewPipeline` 签名改为接受 `RuleFilterer` + `LLMFilterer` 而非 `NoiseClassifier` | 解耦 pipeline 与 filter 包 | ✅ 架构改进 |
| 4 | LLM 过滤器使用 Eino 框架（`github.com/cloudwego/eino` + `eino-ext/components/model/openai`）替代 plan 的简单接口 | 用户明确要求 | ✅ 用户决策 |
| 5 | `EinoClassifier` 实现真实 OpenAI 调用，非仅接口 + mock | 用户要求"不能有 mock 数据" | ✅ 用户决策 |
| 6 | 测试使用 `patternClassifier`（真实模式匹配逻辑）替代 `mockClassifier`（硬编码 ID） | C8 合规：禁止 mock 数据，允许接口抽象 + 依赖注入 | ✅ 合规改进 |
| 7 | `sync_timestamps` 表添加到 0004 迁移 | 用户确认 | ✅ 用户决策 |
| 8 | `ContextItem.UserID` 字段添加 | 用户确认 | ✅ 用户决策 |
| 9 | `UpsertContextItem` 中 `Metadata` 序列化为 JSONB | `map[string]string` 需转为 JSON 存入 JSONB 列 | ✅ 实现必需 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、新依赖经用户确认（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未硬编码密钥（C6）、未写 mock 数据（C8 合规——使用接口抽象 + 依赖注入）。

---

## 2. 本窗口**没做**什么

- ❌ **没有**跑 `make migrate-up`（需 docker postgres）
- ❌ **没有**实现 Phase 3（草稿生成 T015-T020）
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**入库 `doc/handoff/` 目录（用户选择只提交代码文件）

---

## 3. 下一步需要实现什么 — Phase 3: 草稿生成

### 3.1 Phase 3 任务范围

来源：`doc/plans/04-phase3-synthesis.md`

Phase 3 实现模块 C（草稿生成），包括：
- T015: LLM 草稿生成核心
- T016: Prompt 模板系统
- T017: 草稿结构化输出
- T018: 草稿评分与自优化
- T019: 草稿存储与版本管理
- T020: 草稿 API 端点
- T026: 集成测试

### 3.2 T014 完成后 Phase 2 退出标准验证

| # | 标准 | T014 末状态 |
|---|---|---|
| 1 | 4 数据源适配器单元测试通过 | ✅ **4/4 完成**（GitHub ✅，Calendar ✅，Feishu ✅，Obsidian ✅） |
| 2 | 规则 + LLM 噪音过滤测试通过 | ✅ **完成**（5 规则测试 + 6 LLM 测试 + 1 集成测试） |
| 3 | 增量同步（只拉新数据）测试通过 | ✅ **编译通过** + 集成测试 Skip（无 DB） |
| 4 | 集成测试：手动触发 → 4 数据源拉取 → 过滤 → 入库 | ⚠️ **Pipeline filter flow 测试通过**，完整 DB 集成测试需 docker |

**Phase 2 进度：T010 ✅ + T011 ✅ + T012 ✅ + T013 ✅ + T014 ✅ = 5/5 任务完成。M2 退出标准基本达到（完整 DB 集成测试需 docker 环境）。**

---

## 4. 给下一窗口的提示

1. **`NoiseClassifier` 接口在 `harvesting` 包中**：不在 `filter` 包。这是为了打破 import cycle。`filter.LLMFilter` 接受 `harvesting.NoiseClassifier`，`EinoClassifier` 也实现 `harvesting.NoiseClassifier`。

2. **`Pipeline` 不直接导入 `filter` 包**：通过 `RuleFilterer`/`LLMFilterer` 接口解耦。调用方需自行构造 filter 实例再传入 `NewPipeline`。

3. **Eino 框架依赖**：
   - `github.com/cloudwego/eino v0.9.9`（核心：schema, components/model）
   - `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`（OpenAI ChatModel 实现）
   - `EinoClassifier` 使用 `model.BaseChatModel.Generate()` 调用 LLM
   - 集成测试需 `OPENAI_API_KEY` 环境变量

4. **`EinoClassifier` 配置**：通过 `EinoClassifierConfig` 传入 Eino ChatModel 实例 + 可选自定义 prompt 模板。默认 prompt 模板为中文，要求 LLM 返回 `{"is_noise": bool, "reason": string}` JSON。

5. **`SyncStore` 依赖 pgxpool**：`GetLastSync` 首次返回 7 天前；`UpsertContextItem` 使用 `ON CONFLICT DO NOTHING`（基于 `user_id + source + external_id` 唯一约束）；`UpdateLastSync` 使用 `ON CONFLICT DO UPDATE`。

6. **`context_items` 表无 `embedding` 列**：plan 中有 `VECTOR(1536)` 列用于 Phase 3 RAG，但当前无 pgvector 扩展。如 Phase 3 需要向量存储，需先安装 pgvector 并添加迁移。

7. **`Metadata` 字段序列化**：`UpsertContextItem` 将 `map[string]string` 序列化为 JSONB 存入数据库。

8. **ai-coding-boundary P1 红线继续生效**：不得 `git commit`；由用户在主窗口决定是否 commit。

9. **用户已确认的选型决策**（累计）：
   - LLM: Eino 框架 + OpenAI（用于 FR-B05 噪音过滤）
   - Embedding: text-embedding-3-small（用于 Phase 3 向量存储）
   - 执行方式: Subagent-Driven
   - 端到端: 暂不启动 docker
   - Google API: `@latest`（当前 v0.285.0）
   - Google Calendar OAuth: Service Account
   - 飞书 SDK: `@latest`（当前 v3.9.6）
   - T013 Obsidian: 无 Provider 分层 + 只实现 Fetch + WalkDir + ToSlash
   - T014 LLM: Eino 框架 + 真实 OpenAI 调用 + 无 mock 数据
   - T014 Sync: 接受 Skip + 添加 sync_timestamps 表

10. **Phase 2 引入的依赖（T010-T014 累计）**：
    - `github.com/google/go-github/v57`（T010）
    - `golang.org/x/oauth2`（T010）
    - `google.golang.org/api`（T011）
    - `github.com/larksuite/oapi-sdk-go/v3 v3.9.6`（T012）
    - `github.com/cloudwego/eino v0.9.9`（T014）
    - `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`（T014）
    - T013 **无新依赖**

11. **T010-T014 留的 minor 项**（可在 Phase 3 统一处理）：
    - GitHub 分页未处理
    - `GitHubConfig.Validate()` 缺失
    - `NewGitHubAdapter` 用 `context.Background()` 创建 oauth2 client
    - Google Calendar `Source` 字段双重赋值（冗余但不影响功能）
    - ObsidianAdapter 无 Metadata 字段（可选改进）
    - Pipeline 完整 DB 集成测试需 docker 环境

---

## 5. 当前文件结构（Phase 2 / T014 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009 (未动)
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
│   ├── handler/                   ✅ T002+T006+T009 (未动)
│   ├── harvesting/                ✅ T010+T011+T012+T013+T014
│   │   ├── context.go             ✅ T014 修改（+NoiseClassifier +UserID）
│   │   ├── pipeline.go            ✅ T014 新建（Pipeline + sourceAdapter + RuleFilterer + LLMFilterer）
│   │   ├── pipeline_test.go       ✅ T014 新建（5 tests + compile-time checks）
│   │   ├── sync.go                ✅ T014 新建（SyncStore + GetLastSync + UpdateLastSync + UpsertContextItem）
│   │   ├── sync_test.go           ✅ T014 新建（3 integration tests, Skip）
│   │   ├── filter/                ✅ T014 新建
│   │   │   ├── rules.go           ✅ T014（Rule + RuleFilter + DefaultRules）
│   │   │   ├── rules_test.go      ✅ T014（5 tests）
│   │   │   ├── llm.go             ✅ T014（LLMFilter + EinoClassifier + ParseResponse）
│   │   │   └── llm_test.go        ✅ T014（6 unit + 1 integration + compile-time checks）
│   │   └── source/                ✅ T010+T011+T012+T013+T014
│   │       ├── github.go          ✅ T014 修改（+UserID 赋值）
│   │       ├── github_test.go     ✅ T010 (3 tests + compile-time check)
│   │       ├── calendar.go        ✅ T014 修改（+UserID 赋值）
│   │       ├── google_calendar.go ✅ T011
│   │       ├── calendar_test.go   ✅ T011 (5 tests + compile-time check)
│   │       ├── feishu.go          ✅ T014 修改（+UserID 赋值）
│   │       ├── feishu_test.go     ✅ T012 (8 tests + compile-time check)
│   │       ├── obsidian.go        ✅ T014 修改（+UserID 赋值 + 注释更新）
│   │       └── obsidian_test.go   ✅ T013 (10 tests + compile-time check)
│   ├── middleware/                ✅ T002 (未动)
│   ├── queue/                     ✅ T004 (未动)
│   ├── repository/                ✅ T003+T007 (未动)
│   ├── server/                    ✅ T002+T009 (未动)
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── migrations/                    ✅ T003+T006+T008+T014
│   ├── 0001_init.up.sql           ✅ T003
│   ├── 0002_trigger_indexes.up.sql ✅ T006
│   ├── 0003_ddl.up.sql            ✅ T008
│   ├── 0004_context.up.sql        ✅ T014 新建（context_items + sync_timestamps）
│   └── 0004_context.down.sql      ✅ T014 新建
├── doc/handoff/
│   ├── T001-T013-handoff.md       (untracked)
│   ├── phase0-final-handoff.md    (untracked)
│   ├── phase1-final-handoff.md    (untracked)
│   └── T014-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T014 修改（+eino + eino-ext/openai）
└── go.sum                         ✅ T014 修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T014 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **Phase 3 启动前需确认**：
   - Eino 框架是否也用于 Phase 3 草稿生成（FR-C01/C02）？
   - pgvector 扩展是否需要安装（Phase 3 RAG 需要）？
   - Phase 3 是否继续使用 Subagent-Driven 执行方式？

4. **T010-T014 留的 minor 项**是否在 Phase 3 统一处理。
