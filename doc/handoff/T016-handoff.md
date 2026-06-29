# Phase 3 / T016 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T016 — Eino Agent 编排实现（FR-C03）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014/T015 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T016 做完了什么内容）

按 `doc/plans/04-phase3-synthesis.md` §Task T016 执行，结合用户已确认的 Eino workflow 决策。

### 1.1 关联需求

- **FR-C03 (P0)**: LLM 润色生成，通顺度 > 4/5
- **T026**: Eino Workflow 5 阶段 DAG 集成

### 1.2 新建文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/synthesis/agent/dag.go` | **新建** | State + Mark + NodeDef + BuildWorkflow + RunDAG（Eino compose.Workflow DAG 骨架） |
| `internal/synthesis/agent/dag_test.go` | **新建** | 6 个单元测试 |

> **新建 2 个文件 = T016 总变更 2 个对象**。未修改任何已有文件，未新增依赖。

### 1.3 架构设计说明

T016 采用 Eino `compose.Workflow` 原生编排，替代计划中的自定义 DAG 骨架：

| 层 | 类型 | 职责 |
|---|---|---|
| 状态层 | `State` + `Mark` | DAG 节点间传递的共享状态 |
| 定义层 | `NodeDef` | 节点定义（Key + Fn） |
| 编排层 | `BuildWorkflow` | 使用 Eino compose.Workflow 构建并编译 DAG |
| 运行层 | `RunDAG` | 运行编译好的 DAG（便利方法） |

**关键设计决策**：
1. **使用 Eino `compose.Workflow` 替代自定义 `DAG` 结构体**：按用户决策全程使用 Eino，计划中的自定义 `DAG` + `Node` 接口替换为 Eino 原生 `compose.InvokableLambda` + `compose.Workflow`
2. **`NodeDef` 替代 `Node` 接口**：计划中的 `Node` 接口（`Name()` + `Run()`）替换为 `NodeDef` 结构体（`Key` + `Fn`），更贴合 Eino Lambda 的函数式风格
3. **`BuildWorkflow` 一次性构建**：所有节点和边在一次调用中完成构建并编译，避免多次 AddLambdaNode 导致的重复 key 问题
4. **串联模式**：`START → nodes[0] → nodes[1] → ... → nodes[n-1] → END`，线性串联，后续 T026 可扩展为分支 DAG

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | 使用 `compose.Workflow` + `NodeDef` 替代自定义 `DAG` + `Node` 接口 | 用户决策全程使用 Eino workflow（T015 交接文档第 7 点） | ✅ 用户决策 |
| 2 | `State` 使用 `[]harvesting.ContextItem` 而非计划中的自定义 `_Item` | 复用已有 `harvesting.ContextItem` 类型，保持类型一致（M7） | ✅ 合规改进 |
| 3 | 未实现 `RetrieveNode` 等具体节点 | 计划明确说明"T016 任务只建立 DAG 骨架与测试"，具体节点在 T017/T018/T019 | ✅ 符合计划 |
| 4 | 未添加 Eino 依赖（Step 1） | T014 已添加 `github.com/cloudwego/eino v0.9.9`，无需重复 | ✅ 已有依赖 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规）、未改已有文件（C2）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestBuildWorkflow_AllNodesRun` | agent | 3 节点顺序执行 + Completeness 累加验证 |
| 2 | `TestBuildWorkflow_StopsOnError` | agent | 节点报错时 DAG 中断 |
| 3 | `TestBuildWorkflow_EmptyNodes` | agent | 空节点列表报错 |
| 4 | `TestBuildWorkflow_StatePassThrough` | agent | State 字段在节点间正确传递 |
| 5 | `TestNewState` | agent | State 初始化 |
| 6 | `TestState_AppendError` | agent | 错误追加 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/synthesis/agent/...` | ✅ 6 PASS | 全部通过 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + Phase 2 + T015 测试仍通过） |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现具体节点（RetrieveNode / TemplateNode / LLMNode / MarkNode）— T017/T018/T019 范围
- ❌ **没有**实现 Workflow 集成（T026 范围）
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**新增任何依赖

---

## 3. 下一步需要实现什么 — Phase 3: T017

### 3.1 T017: 模板引擎开发

来源：`doc/plans/04-phase3-synthesis.md` §Task T017

- 创建 `internal/synthesis/template.go`：Go text/template 包装 + SelectByTaskType
- 创建 `internal/synthesis/template_test.go`：模板加载 + 渲染 + 类型选择测试
- 创建 4 个模板文件（`templates/*.md.tmpl`）

### 3.2 T016 完成后 Phase 3 进度

| 任务 | 状态 |
|---|---|
| T015: RAG 向量检索 | ✅ 完成 |
| T016: Eino Agent 编排 | ✅ 完成 |
| T017: 模板引擎 | ⬜ 待实现 |
| T018: LLM 调用与润色 | ⬜ 待实现 |
| T019: [待补充] 标记系统 | ⬜ 待实现 |
| T020: SSE 流式输出 | ⬜ 待实现 |
| T026: Eino Workflow 集成 | ⬜ 待实现 |

---

## 4. 给下一窗口的提示

1. **`BuildWorkflow` 是 T016 的核心 API**：接收 `[]NodeDef`，返回 `compose.Runnable[State, State]`。T026 集成时用此 API 构建 5 阶段 DAG。

2. **`State` 结构体**：包含 `AgentRunID`、`UserID`、`TaskType`、`Context`、`Retrieved`、`Template`、`Draft`、`Completeness`、`Marks`、`Errors` 字段。`Context` 和 `Retrieved` 使用 `[]harvesting.ContextItem` 类型（复用已有类型，M7 合规）。

3. **`Mark` 结构体**：与计划一致，包含 `ID`、`Hint`、`Position`、`Resolved`。T019 标记系统会使用此类型。

4. **Eino compose.Workflow 用法**：
   - `compose.NewWorkflow[State, State]()` 创建
   - `wf.AddLambdaNode(key, compose.InvokableLambda(fn))` 添加节点
   - `node.AddInput(compose.START)` 或 `node.AddInput(prevKey)` 连接
   - `wf.End().AddInput(lastKey)` 连接到 END
   - `wf.Compile(ctx)` 编译
   - `runner.Invoke(ctx, state)` 运行

5. **T017 需注意**：模板引擎需要定义 `ContextItem` 类型（或复用 `harvesting.ContextItem`）作为 `TemplateData.Items` 的类型。计划中的 `TemplateData` 使用了 `ContextItem`，需确认是否复用 `harvesting.ContextItem`。

6. **T018 需注意**：计划中 T018 的 `OpenAIClient` 是手写 HTTP 客户端，但用户已确认全程使用 Eino。T018 应使用 Eino `model.BaseChatModel` 接口（T014 已在 `harvesting/filter/llm.go` 中使用），而非手写 OpenAI HTTP。

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

9. **Phase 2 引入的依赖 + T015 新增**（T010-T016 累计）：
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

---

## 5. 当前文件结构（Phase 3 / T016 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009 (未动)
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
│   ├── handler/                   ✅ T002+T006+T009 (未动)
│   ├── harvesting/                ✅ T010+T011+T012+T013+T014 (未动)
│   │   ├── context.go             ✅ T014
│   │   ├── pipeline.go            ✅ T014
│   │   ├── pipeline_test.go       ✅ T014
│   │   ├── sync.go                ✅ T014
│   │   ├── sync_test.go           ✅ T014
│   │   ├── filter/                ✅ T014
│   │   └── source/                ✅ T010+T011+T012+T013+T014
│   ├── middleware/                ✅ T002 (未动)
│   ├── queue/                     ✅ T004 (未动)
│   ├── repository/                ✅ T003+T007 (未动)
│   ├── server/                    ✅ T002+T009 (未动)
│   ├── synthesis/                 ✅ T015+T016
│   │   ├── rag.go                 ✅ T015（EmbeddingProvider + VectorStore + RAG）
│   │   ├── rag_test.go            ✅ T015（6 tests + compile-time check）
│   │   ├── pgvector.go            ✅ T015（PGVectorStore 真实实现）
│   │   ├── eino_embedder.go       ✅ T015（Eino OpenAI Embedder 实现）
│   │   └── agent/                 ✅ T016 新建
│   │       ├── dag.go             ✅ T016（State + Mark + NodeDef + BuildWorkflow + RunDAG）
│   │       └── dag_test.go        ✅ T016（6 tests）
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── migrations/                    ✅ T003+T006+T008+T014+T015 (未动)
├── doc/handoff/
│   ├── T001-T013-handoff.md       (untracked)
│   ├── phase0-final-handoff.md    (untracked)
│   ├── phase1-final-handoff.md    (untracked)
│   ├── T014-handoff.md            (untracked)
│   ├── T015-handoff.md            (untracked)
│   └── T016-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T016 未修改
└── go.sum                         ✅ T016 未修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015+T016 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T017 启动前需确认**：
   - `TemplateData.Items` 类型是否复用 `harvesting.ContextItem`（计划中使用自定义 `ContextItem`，但项目已有 `harvesting.ContextItem`）
   - 模板文件目录 `templates/` 是否放在项目根目录

4. **T018 启动前需确认**：
   - LLM 客户端是否使用 Eino `model.BaseChatModel` 接口（用户已确认全程使用 Eino，但计划中 T018 写的是手写 OpenAI HTTP 客户端 + `LLMClient` 接口，需确认是否替换为 Eino 原生）

5. **T010-T014 留的 minor 项**是否在 Phase 3 统一处理。
