# Phase 3 / T026 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T026 — Eino Workflow 编排集成（FR-C03 P0, T026）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014-T020 + T026 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T026 做完了什么内容）

按 `doc/plans/04-phase3-synthesis.md` §Task T026 执行。

### 1.1 关联需求

- **FR-C03 (P0)**: LLM 润色生成
- **T026 (P0)**: Eino Workflow 5 阶段 DAG 集成

### 1.2 新建/修改文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/synthesis/agent/workflow.go` | **新建** | Workflow + 4 阶段 DAG 编排（retrieve → template → llm → mark） |
| `internal/synthesis/agent/workflow_test.go` | **新建** | 7 个测试（编译 + nil 参数 + 生成 + LLM 错误 + 转换 + 长文本截断） |
| `internal/synthesis/service.go` | **修改** | 新增 DraftGenerator 接口 + DraftResult 类型 + gen 字段 + GenerateDraft 方法 + marshalMarks |
| `internal/synthesis/agent/dag.go` | **修改** | 统一 Mark 类型：移除 agent.Mark，State.Marks 改用 synthesis.Mark |
| `internal/config/config.go` | **修改** | 新增 OpenAIKey + OpenAIModel 配置字段 |
| `cmd/api/wire.go` | **修改** | 注入真实 synthesis.Service（RAG + LLM + Workflow） |

> **新建 2 个文件 + 修改 4 个文件 = T026 总变更 6 个对象**。未新增依赖。

### 1.3 架构设计说明

T026 实成了 Eino Workflow 编排集成，将 T015-T020 各模块串联为可运行的 DAG：

| 层 | 类型 | 职责 |
|---|---|---|
| 接口层 | `synthesis.DraftGenerator` | 定义草稿生成接口，打破 synthesis ↔ agent 循环依赖 |
| 数据层 | `synthesis.DraftResult` | DAG 运行结果结构体（Draft + Marks + Completeness + Template） |
| 编排层 | `agent.Workflow` | 4 阶段 DAG 编排，使用 T016 的 BuildWorkflow API |
| 服务层 | `synthesis.Service` | 持有 DraftGenerator，提供 GenerateDraft + StreamDraft |
| 注入层 | `cmd/api/wire.go` | 构造 RAG → LLM → Workflow → Service 完整依赖链 |

**关键设计决策**：

1. **DraftGenerator 接口打破循环依赖**：`synthesis.Service` 需要持有 `agent.Workflow`，但 `agent` 包导入 `synthesis` 包。通过在 `synthesis` 包定义 `DraftGenerator` 接口，`agent.Workflow` 隐式实现该接口，避免 `synthesis` 导入 `agent`。

2. **Mark 类型统一**：移除 `agent.Mark`（与 `synthesis.Mark` 字段完全相同），`agent.State.Marks` 改用 `[]synthesis.Mark`。消除类型不一致。

3. **WorkflowConfig 可选参数**：`NewWorkflow` 接受可选 `WorkflowConfig{TemplateDir}` 参数，支持测试时指定模板目录，生产环境使用相对路径。

4. **GenerateDraft 保存到 DB**：`Service.GenerateDraft` 运行 DAG 后，将结果写入 `drafts` 表（INSERT ON CONFLICT DO UPDATE），确保 `StreamDraft` 能读取到已生成的草稿。

5. **wire.go 条件初始化**：仅在 `OPENAI_API_KEY` 非空时构建 Synthesis Service，否则 `Syn` 为 nil，DraftStreamHandler 返回 503。

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | 使用 `DraftGenerator` 接口而非直接依赖 `agent.Workflow` | 计划中 `synthesis.NewService(pool, llm, rag, wf)` 会导致 synthesis → agent 循环依赖 | ✅ 合规改进 |
| 2 | `NewWorkflow` 使用 `BuildWorkflow` API 而非直接 `compose.NewWorkflow` | T016 已提供 `BuildWorkflow` 封装，避免重复 Eino 工作流构建逻辑 | ✅ 合规改进 |
| 3 | `WorkflowConfig{TemplateDir}` 为可选参数 | 测试需要指定模板目录，生产环境使用相对路径 | ✅ 合规改进 |
| 4 | `GenerateDraft` 将结果保存到 `drafts` 表 | DAG 生成后需持久化，否则 `StreamDraft` 无法读取 | ✅ 合规改进 |
| 5 | `agent.Mark` 移除，统一使用 `synthesis.Mark` | 两个类型字段完全相同，消除冗余和不一致 | ✅ 合规改进 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规——测试替身使用确定性算法，非硬编码返回值）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestNewWorkflow_Compilation` | agent | Workflow 编译成功 |
| 2 | `TestNewWorkflow_NilRAG` | agent | nil RAG 返回错误 |
| 3 | `TestNewWorkflow_NilLLM` | agent | nil LLM 返回错误 |
| 4 | `TestWorkflow_GenerateDraft` | agent | 完整 4 阶段 DAG 执行（retrieve → template → llm → mark） |
| 5 | `TestWorkflow_LLMError` | agent | LLM 错误传播 |
| 6 | `TestScoredItemToContextItem` | agent | ScoredItem → ContextItem 转换 |
| 7 | `TestScoredItemToContextItem_LongContent` | agent | 长文本标题截断（>80 字符） |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + Phase 2 + T015-T020 + T026 测试仍通过） |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现实时 SSE 流式生成 — `StreamDraft` 仍从 DB 读取已保存的草稿
- ❌ **没有**在 Trigger 流程中调用 `GenerateDraft` — 需在触发器创建 AgentRun 后调用
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**新增任何依赖
- ❌ **没有**做端到端验证（按用户红线）

---

## 3. 下一步需要实现什么 — Phase 4: 交付

### 3.1 Phase 4 概览

来源：`doc/plans/05-phase4-delivery.md`

Phase 3（草稿生成）已全部完成。下一步进入 Phase 4（交付与前端）：

- T021: 前端项目搭建
- T022: 草稿列表与详情页
- T023: 草稿编辑与标记交互
- T024: 交付集成（Notion/Obsidian/飞书）
- T025: 端到端测试与优化

### 3.2 T026 完成后 Phase 3 进度

| 任务 | 状态 |
|---|---|
| T015: RAG 向量检索 | ✅ 完成 |
| T016: Eino Agent 编排 | ✅ 完成 |
| T017: 模板引擎 | ✅ 完成 |
| T018: LLM 调用与润色 | ✅ 完成 |
| T019: 标记系统 | ✅ 完成 |
| T020: SSE 流式输出 | ✅ 完成 |
| T026: Eino Workflow 集成 | ✅ 完成 |

**Phase 3 全部完成！**

---

## 4. 给下一窗口的提示

1. **`DraftGenerator` 是 T026 的核心接口**：定义在 `synthesis` 包，`agent.Workflow` 隐式实现。`Service.GenerateDraft(ctx, runID, userID, taskType)` 运行 DAG 并保存结果到 `drafts` 表。

2. **`Workflow` 是 4 阶段 DAG**：retrieve → template → llm → mark。通过 `NewWorkflow(ctx, rag, llm, WorkflowConfig{TemplateDir: dir})` 创建。`WorkflowConfig` 是可选参数。

3. **Mark 类型已统一**：`agent.State.Marks` 类型为 `[]synthesis.Mark`，不再有 `agent.Mark`。所有 Mark 操作统一使用 `synthesis.ExtractMarks` / `synthesis.ReplaceMark` / `synthesis.Completeness`。

4. **`wire.go` 已注入真实 Service**：`Deps.Syn` 为 `*synthesis.Service`。如果 `OPENAI_API_KEY` 未设置，`Syn` 为 nil，`DraftStreamHandler.Stream` 返回 503。

5. **`config.go` 新增字段**：`OpenAIKey`（环境变量 `OPENAI_API_KEY`）和 `OpenAIModel`（环境变量 `OPENAI_MODEL`，默认 `gpt-4o-mini`）。

6. **Phase 4 需注意**：
   - `Service.GenerateDraft` 需要在 Trigger 流程中被调用（创建 AgentRun 后）
   - `StreamDraft` 目前从 DB 读取已保存草稿；实时流式生成需改造 `llmNode` 支持 SSE 推送
   - `drafts` 表已在 `0001_init.up.sql` 中定义，无需新建 migration
   - 前端 SSE 消费需处理 `delta` / `complete` / `error` 三种事件类型

7. **ai-coding-boundary P1 红线继续生效**：不得 `git commit`；由用户在主窗口决定是否 commit。

8. **用户已确认的选型决策**（累计）：
   - LLM: Eino 框架 + OpenAI（用于 FR-B05 噪音过滤 + FR-C01 embedding + FR-C03 草稿生成）
   - Embedding: text-embedding-3-small（1536 维）
   - 向量存储: pgvector（cosine distance）
   - 执行方式: 先做基础模块再集成
   - **全程使用 Eino workflow**（不手写 HTTP 客户端）
   - Google API: `@latest`（当前 v0.285.0）
   - Google Calendar OAuth: Service Account
   - 飞书 SDK: `@latest`（当前 v3.9.6）
   - T013 Obsidian: 无 Provider 分层 + 只实现 Fetch + WalkDir + ToSlash
   - T014 LLM: Eino 框架 + 真实 OpenAI 调用 + 无 mock 数据
   - T014 Sync: 接受 Skip + 添加 sync_timestamps 表
   - T016 DAG: 使用 Eino compose.Workflow 替代自定义 DAG 骨架
   - T017 TemplateData.Items: 使用 `harvesting.ContextItem`（M7 合规）
   - T018 LLM: 使用 EinoLLM 包装 `model.BaseChatModel`（不手写 OpenAI HTTP）
   - T020 SSE: synthesis 包不依赖 gin（使用 `map[string]interface{}`）
   - T026 Workflow: 使用 DraftGenerator 接口打破循环依赖

9. **Phase 2 引入的依赖 + T015 新增**（T010-T026 累计）：
   - `github.com/google/go-github/v57`（T010）
   - `golang.org/x/oauth2`（T010）
   - `google.golang.org/api`（T011）
   - `github.com/larksuite/oapi-sdk-go/v3 v3.9.6`（T012）
   - `github.com/cloudwego/eino v0.9.9`（T014）
   - `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`（T014）
   - `github.com/cloudwego/eino-ext/components/embedding/openai`（T015）
   - `github.com/pgvector/pgvector-go v0.4.0`（T015）
   - T013 **无新依赖**
   - T016 **无新依赖**
   - T017 **无新依赖**
   - T018 **无新依赖**
   - T019 **无新依赖**
   - T020 **无新依赖**
   - T026 **无新依赖**

---

## 5. 当前文件结构（Phase 3 / T026 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009+T020+T026
│   └── wire.go                    ✅ T026 修改（注入真实 synthesis.Service）
├── internal/
│   ├── config/                    ✅ T001+T006+T026
│   │   └── config.go              ✅ T026 修改（新增 OpenAIKey + OpenAIModel）
│   ├── handler/                   ✅ T002+T006+T009+T020
│   │   ├── health.go              ✅ T002
│   │   ├── health_test.go         ✅ T002
│   │   ├── webhook.go             ✅ T006
│   │   ├── webhook_test.go        ✅ T006
│   │   ├── trigger.go             ✅ T009
│   │   ├── trigger_test.go        ✅ T009
│   │   ├── util.go                ✅ T002
│   │   ├── draft.go               ✅ T020（DraftStreamHandler）
│   │   └── draft_test.go          ✅ T020
│   ├── harvesting/                ✅ T010+T011+T012+T013+T014 (未动)
│   ├── middleware/                ✅ T002 (未动)
│   ├── queue/                     ✅ T004 (未动)
│   ├── repository/                ✅ T003+T007 (未动)
│   ├── server/                    ✅ T002+T009+T020 (未动)
│   │   └── server.go              ✅ T020 修改（新增 synthSvc 参数 + draft 路由）
│   ├── synthesis/                 ✅ T015+T016+T017+T018+T019+T020+T026
│   │   ├── rag.go                 ✅ T015
│   │   ├── rag_test.go            ✅ T015
│   │   ├── pgvector.go            ✅ T015
│   │   ├── eino_embedder.go       ✅ T015
│   │   ├── template.go            ✅ T017
│   │   ├── template_test.go       ✅ T017
│   │   ├── llm.go                 ✅ T018
│   │   ├── llm_test.go            ✅ T018
│   │   ├── marks.go               ✅ T019
│   │   ├── marks_test.go          ✅ T019
│   │   ├── sse.go                 ✅ T020（SSEWriter）
│   │   ├── sse_test.go            ✅ T020
│   │   ├── service.go             ✅ T026 修改（DraftGenerator + DraftResult + GenerateDraft + marshalMarks）
│   │   └── agent/                 ✅ T016+T026
│   │       ├── dag.go             ✅ T026 修改（统一 Mark 类型）
│   │       ├── dag_test.go        ✅ T016
│   │       ├── workflow.go        ✅ T026（Workflow + 4 阶段 DAG）
│   │       └── workflow_test.go   ✅ T026
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── templates/                     ✅ T017
│   ├── weekly_report.md.tmpl      ✅ T017
│   ├── summary.md.tmpl            ✅ T017
│   ├── plan.md.tmpl               ✅ T017
│   └── meeting_minutes.md.tmpl    ✅ T017
├── migrations/                    ✅ T003+T006+T008+T014+T015 (未动)
├── doc/handoff/
│   ├── T001-T013-handoff.md       (untracked)
│   ├── phase0-final-handoff.md    (untracked)
│   ├── phase1-final-handoff.md    (untracked)
│   ├── T014-handoff.md            (untracked)
│   ├── T015-handoff.md            (untracked)
│   ├── T016-handoff.md            (untracked)
│   ├── T017-handoff.md            (untracked)
│   ├── T018-handoff.md            (untracked)
│   ├── T019-handoff.md            (untracked)
│   ├── T020-handoff.md            (untracked)
│   └── T026-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T026 未修改
└── go.sum                         ✅ T026 未修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015-T020 + T026 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **Phase 4 启动前需确认**：
   - 前端技术栈选型（React / Vue / Svelte 等）
   - 是否需要实时 SSE 流式生成（改造 `StreamDraft` 为实时 DAG 推送）
   - Trigger 流程中是否集成 `GenerateDraft` 调用
   - 交付目标平台优先级（Notion / Obsidian / 飞书）

4. **T010-T014 留的 minor 项**是否在 Phase 4 统一处理。
