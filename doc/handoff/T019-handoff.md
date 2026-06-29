# Phase 3 / T019 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T019 — 标记系统（FR-C04 P1）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014-T018 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T019 做完了什么内容）

按 `doc/plans/04-phase3-synthesis.md` §Task T019 执行。

### 1.1 关联需求

- **FR-C04 (P1)**: 可编辑草稿标记系统

### 1.2 新建文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/synthesis/marks.go` | **新建** | Mark 类型 + ExtractMarks + ReplaceMark + Completeness |
| `internal/synthesis/marks_test.go` | **新建** | 7 个单元测试 |

> **新建 2 个文件 = T019 总变更 2 个对象**。未修改任何已有文件，未新增依赖。

### 1.3 架构设计说明

T019 实现了 `[待补充:xxx]` 标记的提取、替换与完整度计算：

| 函数 | 职责 |
|---|---|
| `ExtractMarks(md)` | 从 markdown 中提取所有 `[待补充:xxx]` 占位符 |
| `ReplaceMark(md, markID, items, value)` | 按 markID 替换单个占位符 |
| `Completeness(md)` | 根据占位符数量计算 0.0-1.0 完成度 |

**关键设计决策**：
1. **`synthesis.Mark` 独立定义**：T016 在 `agent` 包中已定义 `agent.Mark`（用于 DAG State）。T019 在 `synthesis` 包中定义 `synthesis.Mark`（用于标记提取/替换），两个结构体字段相同但位于不同层。T026 集成时需统一。
2. **`ReplaceMark` 基于 Position 精确替换**：计划示例代码使用 `ReplaceAllString` 全局按 hint 替换，会导致 hint 重复时全部替换、且 `$` 符号被正则解析。T019 改为按记录的 `Position` 做单次切片替换，并验证 placeholder 匹配，正确处理重复 hint 和特殊字符。
3. **`Completeness` 公式与计划一致**：`1.0 - marks*20.0/total`，空文本返回 0，markPenalty 超过总长度时返回 0。
4. **`Mark.Resolved` 字段预留**：当前未被 T019 写入，T026 集成时由上层决定何时设为 `true`。

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `ReplaceMark` 未使用 `ReplaceAllString`，改为基于 `Position` 的单次切片替换 | 代码质量审查发现原设计有 hint 重复替换和 `$` 符号解析 bug | ✅ 合规改进 |
| 2 | `Completeness` 测试期望值从计划示例的 `0.5` 调整为 `0.1` | 按实际公式计算，原示例值与公式矛盾 | ✅ 合理修正 |
| 3 | 测试数量从计划的 3 个增加到 7 个 | 添加了 DuplicateHints、SpecialValue、AfterExtractNoMarks 等边界测试 | ✅ 合规增强 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规）、未改已有文件（C2）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestExtractMarks` | synthesis | 提取多个 mark，验证 hint 和 ID |
| 2 | `TestReplaceMark` | synthesis | 正常替换单个 mark |
| 3 | `TestReplaceMark_NotFound` | synthesis | markID 不存在时返回错误 |
| 4 | `TestReplaceMark_DuplicateHints` | synthesis | hint 重复时只替换指定 mark |
| 5 | `TestReplaceMark_SpecialValue` | synthesis | value 含 `$` 符号时作为纯文本处理 |
| 6 | `TestReplaceMark_AfterExtractNoMarks` | synthesis | 替换后再提取，确认 mark 已消失 |
| 7 | `TestCompleteness` | synthesis | 无 mark、少量 mark、大量 mark 三种场景 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/synthesis/...` | ✅ 7 PASS | 全部通过 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + Phase 2 + T015/T016/T017/T018 测试仍通过） |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现 SSE 流式输出 — T020 范围
- ❌ **没有**实现 Workflow 集成 — T026 范围
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**新增任何依赖

---

## 3. 下一步需要实现什么 — Phase 3: T020

### 3.1 T020: SSE 流式输出接口

来源：`doc/plans/04-phase3-synthesis.md` §Task T020

- 创建 `internal/synthesis/sse.go`：SSEWriter helper
- 创建 `internal/synthesis/sse_test.go`：SSEWriter 测试
- 创建 `internal/handler/draft.go`：HTTP handler for draft generation
- 创建 `internal/handler/draft_test.go`：handler 测试
- 修改 `internal/server/server.go`：注册路由

### 3.2 T019 完成后 Phase 3 进度

| 任务 | 状态 |
|---|---|
| T015: RAG 向量检索 | ✅ 完成 |
| T016: Eino Agent 编排 | ✅ 完成 |
| T017: 模板引擎 | ✅ 完成 |
| T018: LLM 调用与润色 | ✅ 完成 |
| T019: 标记系统 | ✅ 完成 |
| T020: SSE 流式输出 | ⬜ 待实现 |
| T026: Eino Workflow 集成 | ⬜ 待实现 |

---

## 4. 给下一窗口的提示

1. **`synthesis.Mark` 是 T019 的核心类型**：包含 `ID`/`Hint`/`Position`/`Resolved`。T026 集成时需注意与 `agent.Mark`（在 `internal/synthesis/agent/dag.go` 中定义）的统一问题。

2. **`ExtractMarks` 用法**：`ExtractMarks(md)` 返回按出现顺序编号的 `[]Mark`（ID 为 `mark-0`、`mark-1`…）。`Position` 是 placeholder 在原文中的起始字节位置。

3. **`ReplaceMark` 用法**：`ReplaceMark(md, markID, items, value)` 按 `markID` 在 `items` 中查找对应的 mark，按记录的 `Position` 做**单次**切片替换。替换值作为纯文本处理，不解析 `$` 等正则符号。

4. **`Completeness` 用法**：`Completeness(md)` 返回 0.0-1.0。公式为 `1 - marks*20/len(md)`，mark 越多完成度越低。对短文本敏感，T026 集成时可根据业务需求调整公式。

5. **T020 需注意**：
   - 计划中 `service.go` 使用了 `gin.H`，需在 import 加 `"github.com/gin-gonic/gin"`
   - SSE writer 需要实现 `http.Flusher` 接口
   - 这是 T020 第一次修改 `handler` 和 `server`，需要确认路由注册方式与现有项目一致

6. **T026 需注意**：
   - 需要统一 `synthesis.Mark` 和 `agent.Mark`
   - 需要将 T015（RAG）、T017（Template）、T018（LLM）、T019（Marks）组合成 Eino DAG
   - `Completeness` 计算结果需要写回 `State.Completeness`

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

9. **Phase 2 引入的依赖 + T015 新增**（T010-T019 累计）：
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

---

## 5. 当前文件结构（Phase 3 / T019 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009 (未动)
├── internal/
│   ├── config/                    ✅ T001+T006 (未动)
│   ├── handler/                   ✅ T002+T006+T009 (T020 将修改)
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
│   ├── server/                    ✅ T002+T009 (T020 将修改)
│   ├── synthesis/                 ✅ T015+T016+T017+T018+T019
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
│   └── T019-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T019 未修改
└── go.sum                         ✅ T019 未修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015-T019 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T020 启动前需确认**：
   - SSE 接口的路径设计是否与现有 handler/server 风格一致
   - draft handler 是否需要认证/鉴权（当前项目已有哪些中间件）

4. **T026 启动前需确认**：
   - `synthesis.Mark` 与 `agent.Mark` 是否统一为同一类型？若统一，定义在哪里？
   - `Completeness` 公式是否需要根据业务调整（当前对短文本敏感）？

5. **T010-T014 留的 minor 项**是否在 Phase 3 统一处理。
