# Phase 3 / T020 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T020 — SSE 流式输出接口（FR-C05 P0, NFR-04）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014-T019 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T020 做完了什么内容）

按 `doc/plans/04-phase3-synthesis.md` §Task T020 执行。

### 1.1 关联需求

- **FR-C05 (P0)**: SSE 流式输出
- **NFR-04**: 低延迟

### 1.2 新建/修改文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/synthesis/sse.go` | **新建** | SSEWriter — SSE 协议编解码 |
| `internal/synthesis/sse_test.go` | **新建** | 3 个测试（Write + NotSupported + Headers） |
| `internal/synthesis/service.go` | **新建** | Service + StreamDraft — 草稿流式推送业务逻辑 |
| `internal/handler/draft.go` | **新建** | DraftStreamHandler — HTTP 层调度 |
| `internal/handler/draft_test.go` | **新建** | 1 个测试（SSE 不支持路径） |
| `internal/server/server.go` | **修改** | 新增 `synthSvc` 参数 + 注册 `/api/v1/drafts/:id/stream` 路由 |
| `cmd/api/wire.go` | **修改** | `Server()` 传入 `nil`（T026 将注入真实 Service） |

> **新建 5 个文件 + 修改 2 个文件 = T020 总变更 7 个对象**。未新增依赖。

### 1.3 架构设计说明

T020 实现了 SSE 流式输出接口，将 synthesis 层与 HTTP 层解耦：

| 层 | 类型 | 职责 |
|---|---|---|
| 协议层 | `SSEWriter` | SSE 事件格式编解码 + HTTP 响应头设置 + flush |
| 业务层 | `Service.StreamDraft` | 从 DB 查询 draft → 30 字符分块流式推送 → 发送 marks + completeness |
| 调度层 | `DraftStreamHandler.Stream` | HTTP 参数提取 → 创建 SSEWriter → 调用 Service → 错误处理 |

**关键设计决策**：
1. **`synthesis` 包不依赖 `gin`**：计划中 `service.go` 使用 `gin.H`，但实现改用 `map[string]interface{}`，避免 synthesis 包引入 web 框架依赖
2. **`DraftStreamHandler.Svc` nil 检查**：`wire.go` 当前传入 `nil`（T026 才注入真实 Service），handler 对 `nil` 返回 503 而非 panic
3. **SSE 事件类型**：`delta`（文本块）、`complete`（marks + completeness）、`error`（错误消息）
4. **`StreamDraft` 只查需要的列**：SQL 改为 `SELECT markdown_content FROM drafts`，移除无用的 `id` 扫描

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `service.go` 使用 `map[string]interface{}` 而非 `gin.H` | 避免 synthesis 包引入 gin 依赖，保持包边界清晰 | ✅ 合规改进 |
| 2 | `StreamDraft` SQL 只查 `markdown_content` 而非 `id, markdown_content` | 代码质量审查发现 `id` 被扫描但未使用 | ✅ 合规改进 |
| 3 | `DraftStreamHandler.Stream` 添加了 `Svc == nil` 检查 | Spec 审查发现 wire.go 传 nil 会导致 panic | ✅ 合规改进 |
| 4 | `sse_test.go` 使用 `flushRecorder` 而非直接用 `httptest.NewRecorder` | Go 新版本 `httptest.ResponseRecorder` 实现了 `http.Flusher`，需自定义类型避免继承 | ✅ 合规改进 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestSSEWriter_Write` | synthesis | delta + complete 事件格式验证 |
| 2 | `TestSSEWriter_NotSupported` | synthesis | 非 Flusher writer 返回错误 |
| 3 | `TestSSEWriter_Headers` | synthesis | SSE 响应头和状态码验证 |
| 4 | `TestDraftStreamHandler_SSEUnsupported` | handler | 非 Flusher writer 时 SSE 创建失败 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + Phase 2 + T015-T019 测试仍通过） |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现 Workflow 集成 — T026 范围
- ❌ **没有**在 `wire.go` 中注入真实 `synthesis.Service`（T026 范围）
- ❌ **没有**补充 handler 层完整测试（nil-Svc 路径、正常流路径等）
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**新增任何依赖

---

## 3. 下一步需要实现什么 — Phase 3: T026

### 3.1 T026: Eino Workflow 集成

来源：`doc/plans/04-phase3-synthesis.md` §Task T026

- 将 T015（RAG）、T017（Template）、T018（LLM）、T019（Marks）组合成 Eino DAG
- 使用 T016 的 `BuildWorkflow` API 构建 5 阶段 DAG
- 在 `wire.go` 中注入真实 `synthesis.Service`
- 统一 `synthesis.Mark` 和 `agent.Mark`

### 3.2 T020 完成后 Phase 3 进度

| 任务 | 状态 |
|---|---|
| T015: RAG 向量检索 | ✅ 完成 |
| T016: Eino Agent 编排 | ✅ 完成 |
| T017: 模板引擎 | ✅ 完成 |
| T018: LLM 调用与润色 | ✅ 完成 |
| T019: 标记系统 | ✅ 完成 |
| T020: SSE 流式输出 | ✅ 完成 |
| T026: Eino Workflow 集成 | ⬜ 待实现 |

---

## 4. 给下一窗口的提示

1. **`SSEWriter` 是 T020 的核心类型**：`NewSSEWriter(w)` 创建，`Write(eventType, data)` 发送事件。事件类型：`delta`（文本块）、`complete`（marks + completeness）、`error`（错误消息）。

2. **`Service` 是 T020 的业务入口**：持有 `pool`/`llm`/`rag`，`StreamDraft(ctx, runID, w)` 从 DB 查询 draft 并流式推送。T026 集成时需要将 DAG 运行结果写入 `drafts` 表，或改为实时流式生成。

3. **`wire.go` 当前传 `nil`**：`server.New(d.Cfg, d.Trigger, nil)` 中 `synthSvc` 为 nil。T026 需要在 `Deps` 中添加 `Synth *synthesis.Service` 字段，在 `Build()` 中构造，并在 `Server()` 中传入。

4. **`/api/v1/drafts/:id/stream` 路由已注册**：GET 方法，`:id` 为 agent_run_id。

5. **T026 需注意**：
   - 需要统一 `synthesis.Mark` 和 `agent.Mark`（字段相同但不同包）
   - 需要将 T015（RAG）、T017（Template）、T018（LLM）、T019（Marks）组合成 Eino DAG
   - `Completeness` 计算结果需要写回 `State.Completeness`
   - `Service.StreamDraft` 当前从 DB 读 draft，T026 可能需要改为实时 DAG 生成
   - `drafts` 表需要确认是否已创建（migration）

6. **ai-coding-boundary P1 红线继续生效**：不得 `git commit`；由用户在主窗口决定是否 commit。

7. **用户已确认的选型决策**（累计）：
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

8. **Phase 2 引入的依赖 + T015 新增**（T010-T020 累计）：
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

---

## 5. 当前文件结构（Phase 3 / T020 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009+T020
│   └── wire.go                    ✅ T020 修改（Server() 传 nil）
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
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
│   ├── server/                    ✅ T002+T009+T020
│   │   └── server.go              ✅ T020 修改（新增 synthSvc 参数 + draft 路由）
│   ├── synthesis/                 ✅ T015+T016+T017+T018+T019+T020
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
│   │   ├── service.go             ✅ T020（Service + StreamDraft）
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
│   ├── T018-handoff.md            (untracked)
│   ├── T019-handoff.md            (untracked)
│   └── T020-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T020 未修改
└── go.sum                         ✅ T020 未修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015-T020 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T026 启动前需确认**：
   - `synthesis.Mark` 与 `agent.Mark` 是否统一为同一类型？若统一，定义在哪里？
   - `Service.StreamDraft` 是否改为实时 DAG 生成（而非从 DB 读 draft）？
   - `drafts` 表是否需要新建 migration？
   - `wire.go` 中 `synthesis.Service` 的构造方式（pool/llm/rag 如何注入）？

4. **T010-T014 留的 minor 项**是否在 Phase 3 统一处理。
