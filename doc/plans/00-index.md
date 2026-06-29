# AsyncStarterAgent MVP — 实施计划总索引

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **AI 行为约束**: 本计划的每一步执行必须遵守 [ai-coding-boundary.md](../ai-coding-boundary.md)。任何规范外想法必须先询问用户。

**Goal**: 13 周内交付 MVP 可运行的 "完成前 30%" 异步行动起跑器 Agent，覆盖 13 项 CORE 需求，跑通 5 个用户故事。

**Architecture**: Go + Gin 后端 + Eino DAG 编排 + PostgreSQL+pgvector 数据层 + Tauri v2 桌面客户端。四阶段流水线 (Ingestion → Context Harvesting → Synthesis → Delivery) 通过 SSE 流式推送。

**Tech Stack**: Go 1.22+ / Gin / Eino / PostgreSQL 16 + pgvector / Redis 7 / Tauri v2 / React + Lobe UI / OpenAI or Claude API

---

## 0. 阅读顺序

1. **必读**：[requirement-spec.html](../requirement-spec.html) — 20 项 FR + 5 项 NFR
2. **必读**：[mvp-definition.html](../mvp-definition.html) — MVP IN/OUT + 用户故事
3. **必读**：[plan-boundary.md](../plan-boundary.md) — 范围边界矩阵
4. **必读**：[ai-coding-boundary.md](../ai-coding-boundary.md) — AI 编码行为规范
5. **执行**：下方 5 个 Phase 计划文件

---

## 1. 任务编号 ↔ Phase ↔ 档位 矩阵

| 任务ID | 任务名称 | Phase | 周次 | 优先级 | 档位 | 关联 FR | 状态 |
|---|---|---|---|---|---|---|---|
| T001 | Go项目初始化与目录结构 | Phase 0 | W1 | P0 | CORE | 基础设施 | 待开始 |
| T002 | Gin框架搭建与中间件配置 | Phase 0 | W1 | P0 | CORE | NFR-02 | 待开始 |
| T003 | 数据库Schema设计与迁移 | Phase 0 | W1-W2 | P0 | CORE | 数据模型 | 待开始 |
| T004 | Redis任务队列集成 | Phase 0 | W2 | P1 | PLUS | NFR-04 | 待开始 |
| T005 | Docker开发环境配置 | Phase 0 | W2 | P1 | PLUS | NFR-03 | 待开始 |
| T006 | Webhook监听服务 | Phase 1 | W3 | P0 | CORE | FR-A03 | 待开始 |
| T007 | 关键词匹配规则引擎 | Phase 1 | W3 | P0 | CORE | FR-A01 | 待开始 |
| T008 | DDL截止日期检测器 | Phase 1 | W3-W4 | P1 | PLUS | FR-A02 | 待开始 |
| T009 | 任务调度器整合 | Phase 1 | W4 | P1 | PLUS | 整合 | 待开始 |
| T010 | GitHub数据源适配器 | Phase 2 | W4-W5 | P0 | CORE | FR-B01 | 待开始 |
| T011 | 日历数据源适配器 | Phase 2 | W5 | P1 | PLUS | FR-B02 | 待开始 |
| T012 | IM沟通记录适配器 | Phase 2 | W5-W6 | P1 | PLUS | FR-B03 | 待开始 |
| T013 | 笔记数据源适配器 | Phase 2 | W6 | P1 | PLUS | FR-B04 | 待开始 |
| T014 | ETL管线与增量同步 | Phase 2 | W6-W7 | P0 | CORE | FR-B05, FR-B06 | 待开始 |
| T015 | RAG向量检索模块 | Phase 3 | W7 | P0 | CORE | FR-C01 | 待开始 |
| T016 | Eino Agent编排实现 | Phase 3 | W7-W8 | P0 | CORE | FR-C03 | 待开始 |
| T017 | 模板引擎开发 | Phase 3 | W8 | P1 | CORE | FR-C02 | 待开始 |
| T018 | LLM调用与润色 | Phase 3 | W8-W9 | P0 | CORE | FR-C03 | 待开始 |
| T019 | [待补充]标记系统 | Phase 3 | W9 | P1 | CORE | FR-C04 | 待开始 |
| T020 | SSE流式输出接口 | Phase 3 | W9 | P0 | CORE | FR-C05 | 待开始 |
| T026 | Eino Workflow编排集成 | Phase 3 | W9-W10 | P0 | CORE | Eino集成 | 待开始 |
| T021 | 方案一Tauri前端原型 | Phase 4 | W10-W12 | P0 | CORE | 客户端 | 待开始 |
| T022 | 方案二Flutter前端原型 | Phase 4 | W10-W12 | P1 | PLUS | 客户端 | 待开始 |
| T023 | Notion API文档交付 | Phase 4 | W12 | P0 | CORE | FR-D01 | 待开始 |
| T024 | Obsidian Vault交付 | Phase 4 | W12-W13 | P1 | V1.5 | FR-D02 | 待开始 |
| T025 | 飞书文档交付 | Phase 4 | W13 | P2 | V1.5 | FR-D03 | 待开始 |

**档位说明**：
- **CORE** (13 项)：MVP 必须交付，缺一不可
- **PLUS** (5 项)：MVP 资源允许时交付
- **V1.5** (2 项)：MVP 发布后迭代
- **待评估** (T022)：方案二选其一，MVP 默认只做 T021

---

## 2. 关键路径

按 task-tracker.html 标注：

```
T001 → T002 → T006 → T007 → T009 → T014 → T015 → T016 → T018 → T020 → T021
```

任一关键任务延期 1 周，后续全部顺延。

---

## 3. 里程碑

| 里程碑 | 周次末 | 退出标准 |
|---|---|---|
| M0 | W2 末 | `/health` 200, 可登录 1 个 OAuth Provider, Schema migration 可执行 |
| M1 | W4 末 | 触发事件成功创建 AgentRun 记录 |
| M2 | W7 末 | 4 数据源可拉取, 增量同步与噪音过滤通过测试 |
| M3 | W9 末 | 草稿流式输出, 完成度 ≥ 30%, SSE 首字节 < 3s |
| M4 | W13 末 | US-01~05 端到端跑通, 13 项 CORE 全部通过验收 |

---

## 4. 文档结构

| 文件 | 内容 | 任务数 |
|---|---|---|
| [00-index.md](00-index.md) | 本文件（总索引） | — |
| [01-phase0-foundation.md](01-phase0-foundation.md) | Phase 0: 基础设施 | 5 |
| [02-phase1-trigger.md](02-phase1-trigger.md) | Phase 1: 触发引擎 | 4 |
| [03-phase2-context.md](03-phase2-context.md) | Phase 2: 上下文搜集 | 5 |
| [04-phase3-synthesis.md](04-phase3-synthesis.md) | Phase 3: 草稿生成 | 6 |
| [05-phase4-delivery.md](05-phase4-delivery.md) | Phase 4: 前端与交付 | 5 |

---

## 5. 执行方式

按 ai-coding-boundary §6 标准动作：

```
对每个任务：
  1. 打开对应 Phase 文档
  2. 按 TDD 步骤执行（写测试 → 跑失败 → 写实现 → 跑通过）
  3. 每个原子改动完成后展示 diff，等用户决定是否 commit
  4. 完成 Phase 后更新 task-tracker.html 状态
  5. 触发边界情况时立即停止 + AskUserQuestion
```

---

## 6. 验收指标（来自 plan-boundary §9）

| 指标 | 目标 | 测量时间点 |
|---|---|---|
| 草稿完成度 | ≥ 30% | Phase 3 末 (W9) |
| 用户修改率 | < 50% | Phase 4 末 (W13) |
| 端到端延迟 | < 30s | Phase 3 末 (W9) |
| 数据源覆盖 | ≥ 3 个 | Phase 2 末 (W7) |
| 用户满意度 | ≥ 4.0/5 | Phase 4 末 (W13) |
| SSE 首字节 | < 3s | Phase 3 末 (W9) |
| Notion 写入成功率 | > 99% | Phase 4 末 (W13) |

---

## 7. 不做清单 (Explicit Non-Goals)

来自 plan-boundary §2.5，本计划不实现：

- ❌ 移动端原生 App（T022 在资源允许时可选）
- ❌ 自研 LLM / 自部署 Embedding
- ❌ 多用户协作
- ❌ 用户数据训练模型
- ❌ 跨境数据同步
- ❌ 自定义工作流引擎
- ❌ 离线模式

---

## 8. 风险预案

| 风险 | 触发 | 行动 |
|---|---|---|
| Eino 学习成本高 | T016 延期 | 退化为自实现 DAG，损失可观测性 |
| pgvector 性能不足 | T015 召回率 < 80% | 改用 Qdrant 或 Milvus（需用户同意新增依赖） |
| 第三方 API 限流 | T010/T011/T012 集成测试失败 | 加重试 + 缓存（已在 T014 设计中） |
| Tauri 与 SSE 集成问题 | T021 实施受阻 | 改用 T022 Flutter 方案 |
| 工期紧张 | Phase 0 延期 ≥ 3 天 | 砍 MVP-PLUS 5 项，专注 CORE 13 项 |

---

**下一步**：从 [01-phase0-foundation.md](01-phase0-foundation.md) 开始执行 Phase 0。
