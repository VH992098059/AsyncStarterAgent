# Phase 3 / T018 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T018 — LLM 调用与润色（FR-C03 P0）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014-T017 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T018 做完了什么内容）

按 `doc/plans/04-phase3-synthesis.md` §Task T018 执行，结合用户已确认的 Eino 选型决策。

### 1.1 关联需求

- **FR-C03 (P0)**: LLM 润色生成，通顺度 > 4/5

### 1.2 新建文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/synthesis/llm.go` | **新建** | Message + ChatRequest + ChatChunk + LLMClient 接口 + PromptBuilder + EinoLLM + readStream + sendChunk |
| `internal/synthesis/llm_test.go` | **新建** | 5 个单元测试（mockLLM + mockChatModel） |

> **新建 2 个文件 = T018 总变更 2 个对象**。未修改任何已有文件，未新增依赖。

### 1.3 架构设计说明

T018 实现了 LLM 调用与润色模块，按用户决策全程使用 Eino 框架：

| 层 | 类型 | 职责 |
|---|---|---|
| 数据层 | `Message` / `ChatRequest` / `ChatChunk` | LLM 请求/响应数据结构 |
| 接口层 | `LLMClient` | 流式聊天接口 `Chat(ctx, req) (<-chan ChatChunk, error)` |
| 构造层 | `PromptBuilder` | 根据任务类型、草稿、上下文素材构建 LLM prompt |
| 实现层 | `EinoLLM` | 包装 Eino `model.BaseChatModel`，实现 `LLMClient` |
| 流式层 | `readStream` + `sendChunk` | 从 Eino StreamReader 读取流式响应，发送到 ChatChunk channel |

**关键设计决策**：
1. **使用 Eino `model.BaseChatModel` 替代手写 OpenAI HTTP 客户端**：按用户决策"全程使用 Eino workflow（不手写 HTTP 客户端）"，不创建 `openai.go`，而是用 `EinoLLM` 包装 Eino 的 `model.BaseChatModel` 接口
2. **`PromptBuilder.Build` 使用 `[]harvesting.ContextItem`**：与 T017 一致，按 M7 复用已有类型
3. **`ChatRequest` 简化**：去掉了计划中的 `Model`（在 EinoLLM 构造时确定）和 `Stream`（Chat 方法始终流式）字段
4. **`sendChunk` 防止 goroutine 泄漏**：使用 `select { case out <- chunk: case <-ctx.Done(): }` 模式，确保消费者停止读取时 goroutine 不会阻塞
5. **`schema.AssistantMessage` 签名**：实际 API 为 `AssistantMessage(content string, toolCalls []ToolCall)`，已修正为 `AssistantMessage(m.Content, nil)`

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | 未创建 `openai.go`（手写 OpenAI HTTP 客户端），改为 `EinoLLM` 包装 `model.BaseChatModel` | 用户决策"全程使用 Eino workflow（不手写 HTTP 客户端）" | ✅ 用户决策 |
| 2 | `ChatRequest` 去掉了 `Model` 和 `Stream` 字段 | EinoLLM 的 model 在构造时确定，Chat 始终流式 | ✅ 合理简化 |
| 3 | `PromptBuilder.Build` 使用 `[]harvesting.ContextItem` 而非计划中的自定义 `ContextItem` | M7 合规改进，与 T017 一致 | ✅ M7 合规 |
| 4 | 新增 `sendChunk` 方法（计划中无） | 代码质量审查发现 goroutine 泄漏风险，修复后新增 | ✅ 合规改进 |
| 5 | 新增 `TestEinoLLM_Chat` 和 `TestEinoLLM_Chat_StreamError` 测试 | 代码质量审查发现 EinoLLM 核心路径零测试覆盖 | ✅ 合规改进 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规，mockLLM/mockChatModel 是接口实现）、未改已有文件（C2）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestLLM_Chat_Stream` | synthesis | mockLLM 流式输出拼接验证 |
| 2 | `TestPromptBuilder` | synthesis | 消息数量、角色、周报特殊提示验证 |
| 3 | `TestLLM_Chat_Error` | synthesis | mockLLM 错误返回验证 |
| 4 | `TestEinoLLM_Chat` | synthesis | EinoLLM 完整流式路径（Eino StreamReader → ChatChunk） |
| 5 | `TestEinoLLM_Chat_StreamError` | synthesis | EinoLLM Stream 调用失败 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/synthesis/...` | ✅ 5 PASS | 全部通过 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + Phase 2 + T015/T016/T017 测试仍通过） |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现标记系统 — T019 范围
- ❌ **没有**实现 SSE 流式输出 — T020 范围
- ❌ **没有**实现 Workflow 集成 — T026 范围
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**新增任何依赖

---

## 3. 下一步需要实现什么 — Phase 3: T019

### 3.1 T019: 标记系统

来源：`doc/plans/04-phase3-synthesis.md` §Task T019

- 创建 `internal/synthesis/marks.go`：Mark 解析（`[待补充:xxx]`）+ 替换 + Completeness 计算
- 创建 `internal/synthesis/marks_test.go`：ExtractMarks + ReplaceMark + Completeness 测试

### 3.2 T018 完成后 Phase 3 进度

| 任务 | 状态 |
|---|---|
| T015: RAG 向量检索 | ✅ 完成 |
| T016: Eino Agent 编排 | ✅ 完成 |
| T017: 模板引擎 | ✅ 完成 |
| T018: LLM 调用与润色 | ✅ 完成 |
| T019: 标记系统 | ⬜ 待实现 |
| T020: SSE 流式输出 | ⬜ 待实现 |
| T026: Eino Workflow 集成 | ⬜ 待实现 |

---

## 4. 给下一窗口的提示

1. **`LLMClient` 是 T018 的核心接口**：`Chat(ctx, req) (<-chan ChatChunk, error)`。T026 集成时 DAG 的 llm 节点会使用此接口。

2. **`EinoLLM` 是 `LLMClient` 的 Eino 实现**：通过 `NewEinoLLM(chatModel model.BaseChatModel)` 创建，chatModel 可以是 Eino 的 OpenAI 适配器（`openai.NewChatModel`）或任何实现 `model.BaseChatModel` 的模型。

3. **`PromptBuilder.Build` 签名**：`Build(taskType, draft string, items []harvesting.ContextItem) []Message`。返回 system + user 两条消息。`weekly_report` 类型会追加周报专用提示。

4. **`sendChunk` 防泄漏模式**：所有向 `out` channel 发送 `ChatChunk` 的操作都通过 `sendChunk` 方法，使用 `select { case out <- chunk: case <-ctx.Done(): }` 防止 goroutine 泄漏。

5. **T019 需注意**：计划中的 `Mark` 结构体（ID/Hint/Position/Resolved）已在 T016 `agent/dag.go` 中定义。T019 的 `ExtractMarks`/`ReplaceMark`/`Completeness` 函数在 `synthesis` 包中实现，与 `agent.Mark` 是不同类型。T026 集成时需要统一。

6. **T020 需注意**：计划中 `service.go` 使用了 `gin.H`，需在 import 加 `"github.com/gin-gonic/gin"`。SSE writer 需要实现 `http.Flusher` 接口。

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

9. **Phase 2 引入的依赖 + T015 新增**（T010-T018 累计）：
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

---

## 5. 当前文件结构（Phase 3 / T018 末）

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
│   ├── synthesis/                 ✅ T015+T016+T017+T018
│   │   ├── rag.go                 ✅ T015
│   │   ├── rag_test.go            ✅ T015
│   │   ├── pgvector.go            ✅ T015
│   │   ├── eino_embedder.go       ✅ T015
│   │   ├── template.go            ✅ T017
│   │   ├── template_test.go       ✅ T017
│   │   ├── llm.go                 ✅ T018（LLMClient + PromptBuilder + EinoLLM）
│   │   ├── llm_test.go            ✅ T018（5 tests）
│   │   └── agent/                 ✅ T016
│   │       ├── dag.go             ✅ T016
│   │       └── dag_test.go        ✅ T016
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
│   └── T018-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T018 未修改
└── go.sum                         ✅ T018 未修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015+T016+T017+T018 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T019 启动前需确认**：
   - `synthesis.Mark` 与 `agent.Mark` 类型统一问题：T016 在 `agent/dag.go` 中定义了 `Mark`（ID/Hint/Position/Resolved），T019 计划在 `synthesis` 包中也定义 `Mark` 相关函数。需确认是复用 `agent.Mark` 还是在 `synthesis` 包中定义独立类型。

4. **T010-T014 留的 minor 项**是否在 Phase 3 统一处理。

5. **`ChatRequest.Temperature` 零值问题**：当前 `Temperature > 0` 条件使得无法设置 Temperature=0（确定性输出）。是否需要改为指针类型 `*float32` 以区分"未设置"与"设为 0"。
