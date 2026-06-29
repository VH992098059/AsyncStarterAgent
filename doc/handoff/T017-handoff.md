# Phase 3 / T017 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T017 — 模板引擎开发（FR-C02 P1, FR-C03 P0）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014/T015/T016 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T017 做完了什么内容）

按 `doc/plans/04-phase3-synthesis.md` §Task T017 执行。

### 1.1 关联需求

- **FR-C02 (P1)**: 模板套用（周报/总结/规划/纪要/通用）
- **FR-C03 (P0)**: LLM 润色生成，通顺度 > 4/5

### 1.2 新建文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/synthesis/template.go` | **新建** | Template + TemplateData + LoadTemplate + Render + funcMap + SelectByTaskType |
| `internal/synthesis/template_test.go` | **新建** | 2 个单元测试 |
| `templates/weekly_report.md.tmpl` | **新建** | 周报模板 |
| `templates/summary.md.tmpl` | **新建** | 总结模板 |
| `templates/plan.md.tmpl` | **新建** | 规划模板 |
| `templates/meeting_minutes.md.tmpl` | **新建** | 会议纪要模板 |

> **新建 6 个文件 = T017 总变更 6 个对象**。未修改任何已有文件，未新增依赖。

### 1.3 架构设计说明

T017 实现了 Go `text/template` 包装层，支持 4 种任务类型的模板选择和渲染：

| 层 | 类型 | 职责 |
|---|---|---|
| 数据层 | `TemplateData` | 模板渲染数据（Items 使用 `harvesting.ContextItem`） |
| 引擎层 | `Template` + `LoadTemplate` + `Render` | 模板加载与渲染 |
| 函数层 | `funcMap` | 自定义模板函数（upper / join / date） |
| 选择层 | `SelectByTaskType` | 根据 task_type 选择模板文件路径 |

**关键设计决策**：
1. **`TemplateData.Items` 使用 `[]harvesting.ContextItem`**：计划中定义了自定义 `ContextItem`，但项目已有 `harvesting.ContextItem`（含 Title/Source/Content/URL 等字段），按 M7（保持类型/命名一致）复用已有类型，避免重复定义
2. **`SelectByTaskType` 返回相对路径**：如 `templates/weekly_report.md.tmpl`，与计划一致。部署时需确保工作目录正确
3. **`date` 函数使用 `time.Now()`**：与计划一致。`TemplateData.Now` 字段已定义但 `date` 函数未使用，后续可改为闭包注入以支持确定性测试
4. **4 种任务类型 + default**：`weekly_report` / `summary` / `plan` / `meeting_minutes`，default 回退到 `summary`

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `TemplateData.Items` 使用 `[]harvesting.ContextItem` 而非计划中的自定义 `ContextItem` | M7 合规改进，复用已有类型，T016 交接文档第 5 点已提示 | ✅ M7 合规 |
| 2 | 测试中 `contains` 辅助函数手写而非用 `strings.Contains` | 与计划一致，计划中就是这样实现的 | ✅ 符合计划 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规）、未改已有文件（C2）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestTemplate_LoadAndRender` | synthesis | 从临时文件加载模板 + 渲染 + 验证输出包含预期字符串 |
| 2 | `TestTemplate_SelectByTaskType` | synthesis | 5 个 case（4 种类型 + unknown→summary） |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/synthesis/...` | ✅ 2 PASS | 全部通过 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + Phase 2 + T015/T016 测试仍通过） |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现 LLM 调用与润色 — T018 范围
- ❌ **没有**实现标记系统 — T019 范围
- ❌ **没有**实现 SSE 流式输出 — T020 范围
- ❌ **没有**实现 Workflow 集成 — T026 范围
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**新增任何依赖

---

## 3. 下一步需要实现什么 — Phase 3: T018

### 3.1 T018: LLM 调用与润色

来源：`doc/plans/04-phase3-synthesis.md` §Task T018

- 创建 `internal/synthesis/llm.go`：LLM 客户端接口 + PromptBuilder
- 创建 `internal/synthesis/llm_test.go`：mock LLM 客户端测试
- 创建 `internal/synthesis/openai.go`：真实 OpenAI 客户端（streaming）

**⚠️ 重要提醒**：计划中 T018 的 `OpenAIClient` 是手写 HTTP 客户端，但用户已确认**全程使用 Eino**。T018 应使用 Eino `model.BaseChatModel` 接口（T014 已在 `harvesting/filter/llm.go` 中使用），而非手写 OpenAI HTTP。需在 T018 启动前与用户确认。

### 3.2 T017 完成后 Phase 3 进度

| 任务 | 状态 |
|---|---|
| T015: RAG 向量检索 | ✅ 完成 |
| T016: Eino Agent 编排 | ✅ 完成 |
| T017: 模板引擎 | ✅ 完成 |
| T018: LLM 调用与润色 | ⬜ 待实现 |
| T019: 标记系统 | ⬜ 待实现 |
| T020: SSE 流式输出 | ⬜ 待实现 |
| T026: Eino Workflow 集成 | ⬜ 待实现 |

---

## 4. 给下一窗口的提示

1. **`TemplateData` 是 T017 的核心数据结构**：包含 `TaskType`、`Title`、`Items`（`[]harvesting.ContextItem`）、`Now`、`UserID`、`CustomVars`。T026 集成时 DAG 的 template 节点会使用此结构。

2. **`SelectByTaskType` 映射**：
   - `weekly_report` → `templates/weekly_report.md.tmpl`
   - `summary` → `templates/summary.md.tmpl`
   - `plan` → `templates/plan.md.tmpl`
   - `meeting_minutes` → `templates/meeting_minutes.md.tmpl`
   - default → `templates/summary.md.tmpl`

3. **`LoadTemplate` 用法**：`LoadTemplate(name, path)` 从文件加载模板，name 用于模板命名，path 是文件路径。返回 `*Template`，调用 `Render(data)` 渲染。

4. **模板函数**：`upper`（转大写）、`join`（字符串连接）、`date`（格式化当前时间）。4 个模板文件中 `date` 被使用，`upper` 和 `join` 预留供 LLM 润色后的后处理使用。

5. **T018 需注意**：计划中 T018 的 `OpenAIClient` 是手写 HTTP 客户端，但用户已确认全程使用 Eino。T018 应使用 Eino `model.BaseChatModel` 接口（T014 已在 `harvesting/filter/llm.go` 中使用），而非手写 OpenAI HTTP。需在 T018 启动前与用户确认。

6. **`date` 函数已知限制**：`date` 模板函数使用 `time.Now()` 而非 `TemplateData.Now`，导致输出不可复现。后续可改为闭包注入 `Now` 以支持确定性测试，但当前与计划一致。

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

9. **Phase 2 引入的依赖 + T015 新增**（T010-T017 累计）：
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

---

## 5. 当前文件结构（Phase 3 / T017 末）

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
│   ├── synthesis/                 ✅ T015+T016+T017
│   │   ├── rag.go                 ✅ T015
│   │   ├── rag_test.go            ✅ T015
│   │   ├── pgvector.go            ✅ T015
│   │   ├── eino_embedder.go       ✅ T015
│   │   ├── template.go            ✅ T017（Template + TemplateData + LoadTemplate + Render + funcMap + SelectByTaskType）
│   │   ├── template_test.go       ✅ T017（2 tests）
│   │   └── agent/                 ✅ T016
│   │       ├── dag.go             ✅ T016
│   │       └── dag_test.go        ✅ T016
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── templates/                     ✅ T017 新建
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
│   └── T017-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T017 未修改
└── go.sum                         ✅ T017 未修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015+T016+T017 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T018 启动前需确认**：
   - LLM 客户端是否使用 Eino `model.BaseChatModel` 接口（用户已确认全程使用 Eino，但计划中 T018 写的是手写 OpenAI HTTP 客户端 + `LLMClient` 接口，需确认是否替换为 Eino 原生）
   - `PromptBuilder` 中的 system prompt 内容是否需要调整

4. **T010-T014 留的 minor 项**是否在 Phase 3 统一处理。

5. **`date` 函数改进**：是否需要将 `date` 函数改为使用 `TemplateData.Now`（当前使用 `time.Now()`，输出不可复现）。
