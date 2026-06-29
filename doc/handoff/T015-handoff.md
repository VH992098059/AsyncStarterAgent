# Phase 3 / T015 交接文档 — 给下一窗口

> **生成时间**: 2026-06-20 (Asia/Taipei)
> **当前任务**: T015 — RAG 向量检索模块（FR-C01）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T015 做完了什么内容）

按 `doc/plans/04-phase3-synthesis.md` §Task T015 执行，结合用户确认的 3 个决策点。

### 1.1 关联需求

- **FR-C01 (P0)**: RAG 语义检索，召回率 > 80%

### 1.2 新建/修改文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `migrations/0005_embeddings.up.sql` | **新建** | 添加 pgvector 扩展 + context_items.embedding vector(1536) 列 + HNSW 索引 |
| `migrations/0005_embeddings.down.sql` | **新建** | 回滚 embedding 列和索引 |
| `internal/synthesis/rag.go` | **新建** | `EmbeddingProvider` + `VectorStore` 接口 + `RAG` + `Retrieve` + `Index` + `toFloat32Slice` |
| `internal/synthesis/rag_test.go` | **新建** | 6 个单元测试 + 编译期断言（testEmbedder 满足 embedding.Embedder） |
| `internal/synthesis/pgvector.go` | **新建** | `PGVectorStore` + `Search`（cosine distance）+ `Upsert` |
| `internal/synthesis/eino_embedder.go` | **新建** | `EinoEmbedderConfig` + `NewEinoEmbedder`（Eino OpenAI Embedder 真实实现） |
| `go.mod` | **修改** | 新增 `github.com/pgvector/pgvector-go v0.4.0` + `github.com/cloudwego/eino-ext/components/embedding/openai` 及传递依赖 |
| `go.sum` | **修改** | 对应更新 |

> **新建 6 个文件 + 修改 2 个文件 = T015 总变更 8 个对象**。

### 1.3 架构设计说明

T015 采用分层架构，通过接口抽象实现 RAG 检索：

| 层 | 类型 | 职责 |
|---|---|---|
| 接口层 | `VectorStore` + `EmbeddingProvider` | 向量存储和 embedding 的抽象接口 |
| 核心层 | `RAG` | 检索增强生成核心（Retrieve + Index） |
| 实现层 | `PGVectorStore` + `EinoEmbedder` | pgvector 真实存储 + Eino OpenAI 真实 embedding |
| 测试层 | `testEmbedder` + `testStore` | 确定性算法的测试替身（C8 合规：非 mock 数据） |

**关键设计决策**：
1. `EmbeddingProvider` 包装 Eino 的 `embedding.Embedder` 接口，提供 `Embed(ctx, text string) ([]float32, error)` 便利方法，内部处理 `[][]float64` → `[]float32` 转换
2. `VectorStore` 接口独立于具体存储实现，`PGVectorStore` 使用 pgvector 的 cosine distance (`<=>`) 运算符
3. `EinoEmbedder` 使用 `eino-ext/components/embedding/openai` 真实调用 OpenAI API，默认模型 `text-embedding-3-small`，维度 1536

### 1.4 用户确认的设计决策

| # | 决策点 | 用户选择 | 理由 |
|---|---|---|---|
| 1 | LLM 调用方式 | 全程使用 Eino workflow | 统一框架，不手写 HTTP 客户端 |
| 2 | pgvector 依赖 | 使用 Eino 框架 + pgvector | Eino embedding + pgvector 存储 |
| 3 | 执行方式 | 先做基础模块再集成 | T015-T019 先做，T020+T026 后集成 |

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestRAG_Retrieve` | synthesis | RAG 检索 + 分数递减排序验证 |
| 2 | `TestRAG_Index` | synthesis | RAG 索引写入 |
| 3 | `TestRAG_EmptyRetrieve` | synthesis | 空库检索返回 0 结果 |
| 4 | `TestEmbeddingProvider_Embed` | synthesis | EmbeddingProvider 单文本 embedding |
| 5 | `TestEmbeddingProvider_Dimension` | synthesis | 维度返回值 |
| 6 | `TestToFloat32Slice` | synthesis | float64→float32 转换 |
| 7 | 编译期断言 | synthesis | `testEmbedder` 满足 `embedding.Embedder` 接口 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./internal/synthesis/...` | ✅ 6 PASS | 全部通过 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1 + Phase 2 测试仍通过） |

### 1.7 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `EmbeddingProvider` 包装 Eino `Embedder` 接口，而非直接暴露 `EmbedStrings` | 用户决策全程使用 Eino，需要适配 `[]float64`→`[]float32` | ✅ 用户决策 |
| 2 | 测试使用 `testEmbedder`（确定性哈希算法）替代 `mockEmbed`（硬编码） | C8 合规：禁止 mock 数据，允许接口抽象 + 依赖注入 | ✅ 合规改进 |
| 3 | `PGVectorStore.Search` 使用 `1 - (embedding <=> $1)` 计算 cosine similarity | pgvector cosine distance 是距离，1-distance 即为相似度 | ✅ 实现必需 |
| 4 | `PGVectorStore.Upsert` 使用 `UPDATE ... WHERE external_id = $3` 而非 `INSERT ... ON CONFLICT` | context_items 表已有数据（由 harvesting pipeline 写入），只需更新 embedding 列 | ✅ 实现必需 |
| 5 | `EinoEmbedder` 使用 `eino-ext/components/embedding/openai` 而非手写 HTTP 客户端 | 用户决策全程使用 Eino | ✅ 用户决策 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、新依赖经用户确认（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未硬编码密钥（C6）、未写 mock 数据（C8 合规）。

---

## 2. 本窗口**没做**什么

- ❌ **没有**跑 `make migrate-up`（需 docker postgres + pgvector 扩展）
- ❌ **没有**实现 T016-T020 + T026（Phase 3 剩余任务）
- ❌ **没有**改 trigger / handler / server / config 任何文件
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**实现 PGVectorStore 集成测试（需 docker + pgvector）

---

## 3. 下一步需要实现什么 — Phase 3: T016

### 3.1 T016: Eino Agent 编排实现

来源：`doc/plans/04-phase3-synthesis.md` §Task T016

- 创建 `internal/synthesis/agent/dag.go`：DAG 骨架 + State + Node 接口
- 创建 `internal/synthesis/agent/dag_test.go`：DAG 测试
- Eino 依赖已在 T014 添加（`github.com/cloudwego/eino v0.9.9`）

### 3.2 T015 完成后 Phase 3 进度

| 任务 | 状态 |
|---|---|
| T015: RAG 向量检索 | ✅ 完成 |
| T016: Eino Agent 编排 | ⬜ 待实现 |
| T017: 模板引擎 | ⬜ 待实现 |
| T018: LLM 调用与润色 | ⬜ 待实现 |
| T019: [待补充] 标记系统 | ⬜ 待实现 |
| T020: SSE 流式输出 | ⬜ 待实现 |
| T026: Eino Workflow 集成 | ⬜ 待实现 |

---

## 4. 给下一窗口的提示

1. **`EmbeddingProvider` 包装 Eino `Embedder`**：`Embed()` 方法返回 `[]float32`，内部调用 `EmbedStrings()` 返回 `[][]float64` 再转换。调用方不需要关心 float64/float32 差异。

2. **`VectorStore` 接口**：`Search` 返回按相似度降序排列的 `[]ScoredItem`，`Upsert` 按 `external_id` 更新 embedding 列。

3. **`PGVectorStore` 依赖 pgvector 扩展**：需要在 PostgreSQL 中先 `CREATE EXTENSION vector`，迁移 0005 已包含此语句。集成测试需 docker + pgvector 镜像。

4. **`EinoEmbedder` 需要 `OPENAI_API_KEY`**：集成测试需环境变量，单元测试使用 `testEmbedder` 不需要。

5. **`testEmbedder` 使用确定性哈希算法**：基于文本字符的归一化哈希，非硬编码 mock 数据。dim=32 时有足够区分度。

6. **Eino 框架依赖**（累计）：
   - `github.com/cloudwego/eino v0.9.9`（T014）
   - `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`（T014）
   - `github.com/cloudwego/eino-ext/components/embedding/openai v0.0.0-20260616080858-ab17b7308bf8`（T015）
   - `github.com/pgvector/pgvector-go v0.4.0`（T015）

7. **T016 需注意**：按用户决策，全程使用 Eino workflow。DAG 节点应使用 Eino 的 `compose.Workflow` / `compose.Lambda` 而非自定义 DAG 骨架。计划中的自定义 `DAG` 结构体可能需要替换为 Eino 原生编排。

8. **ai-coding-boundary P1 红线继续生效**：不得 `git commit`；由用户在主窗口决定是否 commit。

9. **用户已确认的选型决策**（累计）：
   - LLM: Eino 框架 + OpenAI（用于 FR-B05 噪音过滤 + FR-C01 embedding + FR-C03 草稿生成）
   - Embedding: text-embedding-3-small（1536 维）
   - 向量存储: pgvector（cosine distance）
   - 执行方式: 先做基础模块再集成
   - 全程使用 Eino workflow（不手写 HTTP 客户端）
   - Google API: `@latest`（当前 v0.285.0）
   - Google Calendar OAuth: Service Account
   - 飞书 SDK: `@latest`（当前 v3.9.6）
   - T013 Obsidian: 无 Provider 分层 + 只实现 Fetch + WalkDir + ToSlash
   - T014 LLM: Eino 框架 + 真实 OpenAI 调用 + 无 mock 数据
   - T014 Sync: 接受 Skip + 添加 sync_timestamps 表

10. **Phase 2 引入的依赖 + T015 新增**（T010-T015 累计）：
    - `github.com/google/go-github/v57`（T010）
    - `golang.org/x/oauth2`（T010）
    - `google.golang.org/api`（T011）
    - `github.com/larksuite/oapi-sdk-go/v3 v3.9.6`（T012）
    - `github.com/cloudwego/eino v0.9.9`（T014）
    - `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`（T014）
    - `github.com/cloudwego/eino-ext/components/embedding/openai`（T015）
    - `github.com/pgvector/pgvector-go v0.4.0`（T015）
    - T013 **无新依赖**

---

## 5. 当前文件结构（Phase 3 / T015 末）

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
│   ├── synthesis/                 ✅ T015 新建
│   │   ├── rag.go                 ✅ T015（EmbeddingProvider + VectorStore + RAG）
│   │   ├── rag_test.go            ✅ T015（6 tests + compile-time check）
│   │   ├── pgvector.go            ✅ T015（PGVectorStore 真实实现）
│   │   └── eino_embedder.go       ✅ T015（Eino OpenAI Embedder 实现）
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── migrations/                    ✅ T003+T006+T008+T014+T015
│   ├── 0001_init.up.sql           ✅ T003
│   ├── 0002_trigger_indexes.up.sql ✅ T006
│   ├── 0003_ddl.up.sql            ✅ T008
│   ├── 0004_context.up.sql        ✅ T014
│   ├── 0004_context.down.sql      ✅ T014
│   ├── 0005_embeddings.up.sql     ✅ T015 新建（pgvector + embedding 列 + HNSW 索引）
│   └── 0005_embeddings.down.sql   ✅ T015 新建
├── doc/handoff/
│   ├── T001-T013-handoff.md       (untracked)
│   ├── phase0-final-handoff.md    (untracked)
│   ├── phase1-final-handoff.md    (untracked)
│   ├── T014-handoff.md            (untracked)
│   └── T015-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T015 修改（+pgvector-go +eino-ext/embedding/openai）
└── go.sum                         ✅ T015 修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T016 启动前需确认**：
   - DAG 编排是否使用 Eino `compose.Workflow`（用户已确认全程使用 Eino，但计划中 T016 写的是自定义 DAG 骨架，需确认是否替换为 Eino 原生编排）
   - `State` 结构体是否与计划中一致（包含 `AgentRunID`、`UserID`、`TaskType`、`Context`、`Retrieved`、`Template`、`Draft`、`Completeness`、`Marks`、`Errors`）

4. **T010-T014 留的 minor 项**是否在 Phase 3 统一处理。
