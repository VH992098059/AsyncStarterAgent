# "完成前30%" 异步行动起跑器 Agent — 计划边界规划

> **文档版本**: v0.1.0
> **创建日期**: 2026-06-18
> **状态**: 草稿（待评审）
> **输入文档**:
> - [requirement-spec.html](./requirement-spec.html) — 需求规格说明书（4 模块 / 20 项需求 / 5 项 NFR）
> - [mvp-definition.html](./mvp-definition.html) — MVP 范围定义（11 项 IN / 5 项 OUT / 5 个用户故事 / 5 个 Phase）

---

## 1. 规划目标

将分散在两份 HTML 文档中的需求与范围，**统一收敛为可执行的边界矩阵**，回答四个问题：

1. **做什么 / 不做什么**（范围边界）
2. **先做什么 / 后做什么**（优先级边界）
3. **谁做什么 / 在哪一层做**（模块边界 + 平台边界）
4. **什么时候做完**（时间边界）

本文件是后续 sprint 拆分、任务编排、验收测试的**单一事实来源（SSOT）**。

---

## 2. 范围边界（IN / OUT 矩阵）

将 `mvp-definition.html` 的 MVP IN/OUT 与 `requirement-spec.html` 的 P0/P1/P2 标签做交叉映射，得到三档范围：

| 档位 | 含义 | 对应文档标签 | 何时交付 |
| --- | --- | --- | --- |
| **MVP-CORE** | MVP 必须包含，端到端跑通 | MVP IN ∩ P0 | W1 - W13 |
| **MVP-PLUS** | MVP 阶段实现但不强制端到端 | MVP IN ∩ P1 | W1 - W13（与 CORE 并行） |
| **V1.5** | MVP 后第一轮迭代 | MVP IN ∩ P1（暂缓）+ P2 子集 | MVP 发布后 |
| **V2.0** | 规模化版本 | MVP OUT 中的战略功能 | V1.5 之后 |

### 2.1 MVP-CORE 清单（13 项）

| 需求编号 | 名称 | 阶段 | 依赖 |
| --- | --- | --- | --- |
| FR-A01 | 关键词触发 | Phase 1 | FR-A03 |
| FR-A03 | Webhook 接入（Todoist） | Phase 1 | — |
| FR-B01 | GitHub 数据拉取 | Phase 2 | FR-B06 |
| FR-B05 | 噪音过滤（规则+LLM） | Phase 2 | FR-B01~B04 |
| FR-B06 | 增量同步 | Phase 2 | — |
| FR-C01 | RAG 语义检索 | Phase 3 | FR-B05 |
| FR-C02 | 模板套用 | Phase 3 | FR-C01 |
| FR-C03 | LLM 润色生成 | Phase 3 | FR-C02 |
| FR-C04 | [待补充] 标记 | Phase 3 | FR-C03 |
| FR-C05 | SSE 流式输出 | Phase 3 | FR-C03 |
| FR-D01 | Notion 页面创建 | Phase 4 | FR-C04 |
| FR-D04 | 任务备注更新 | Phase 4 | FR-D01 |
| FR-D05 | 通知推送 | Phase 4 | FR-D04 |

### 2.2 MVP-PLUS 清单（5 项 — 资源允许则交付）

| 需求编号 | 名称 | 阶段 | 备注 |
| --- | --- | --- | --- |
| FR-A02 | DDL 触发 | Phase 1 | 与关键词触发共用触发引擎 |
| FR-A04 | 手动触发（"帮我起跑"按钮） | Phase 1 | 前端工作量大时延后到 V1.5 |
| FR-B02 | 日历数据拉取 | Phase 2 | 至少支持 Google Calendar |
| FR-B03 | IM 消息拉取 | Phase 2 | MVP 阶段只做飞书 |
| FR-B04 | 笔记数据读取 | Phase 2 | MVP 阶段只做 Obsidian 本地 |

### 2.3 V1.5 清单（5 项）

| 需求编号 | 名称 | 触发条件 |
| --- | --- | --- |
| FR-B02 扩展 | Outlook / Slack / Notion 数据源 | MVP 反馈缺口 |
| FR-D02 | Obsidian 文件写入 | 用户画像-张远场景验证 |
| FR-D03 | 飞书文档创建 | 林薇场景验证 |
| 自定义 Prompt 编辑器 | MVP OUT 推迟项 | 模板复用率 < 60% 时启动 |
| 移动端推送（APNs/FCM） | MVP OUT 推迟项 | 桌面端日活 ≥ 100 时启动 |

### 2.4 V2.0 清单（4 项 — 战略级，单独规划）

| 需求 | 名称 | 触发条件 |
| --- | --- | --- |
| 语音唤醒 | MVP OUT | ASR 准确率 ≥ 95% 且有商业价值 |
| 多用户协作 | MVP OUT | 团队版需求验证 |
| 数据分析报表 | MVP OUT | 日活 ≥ 1000 |
| 插件市场 | MVP OUT | 第三方适配器 > 10 个 |

### 2.5 Explicit Non-Goals（明确不做）

为防止范围蔓延，**以下功能在 V2.0 之前一律不做**：

- ❌ 移动端原生 App（仅做桌面通知）
- ❌ 自研 LLM / 自部署 Embedding 模型
- ❌ 多用户实时协作编辑
- ❌ 用户数据用于模型训练/微调
- ❌ 跨境数据同步（默认数据本地优先）
- ❌ 自定义工作流引擎（仅支持预设模板）
- ❌ 离线模式（必须联网调用 LLM）

---

## 3. 优先级边界（Requirement ↔ Phase 矩阵）

| 需求 | P0/P1/P2 | MVP 归属 | Phase |
| --- | --- | --- | --- |
| FR-A01 关键词触发 | P0 | CORE | 1 |
| FR-A02 DDL 触发 | P1 | PLUS | 1 |
| FR-A03 Webhook 接入 | P0 | CORE | 1 |
| FR-A04 手动触发 | P1 | PLUS | 1 |
| FR-B01 GitHub | P0 | CORE | 2 |
| FR-B02 日历 | P1 | PLUS | 2 |
| FR-B03 IM | P1 | PLUS（仅飞书） | 2 |
| FR-B04 笔记 | P1 | PLUS（仅 Obsidian） | 2 |
| FR-B05 噪音过滤 | P0 | CORE | 2 |
| FR-B06 增量同步 | P0 | CORE | 2 |
| FR-C01 RAG | P0 | CORE | 3 |
| FR-C02 模板套用 | P1 | CORE | 3 |
| FR-C03 LLM 润色 | P0 | CORE | 3 |
| FR-C04 [待补充] | P1 | CORE | 3 |
| FR-C05 SSE | P0 | CORE | 3 |
| FR-D01 Notion 写入 | P0 | CORE | 4 |
| FR-D02 Obsidian 写入 | P1 | V1.5 | 4 |
| FR-D03 飞书写入 | P2 | V1.5 | 4 |
| FR-D04 任务备注 | P0 | CORE | 4 |
| FR-D05 通知 | P1 | CORE | 4 |

**决策规则**：
- P0 缺一项 → MVP 不通过验收
- P1 缺一项 → 走 MVP-PLUS 评估，资源紧张可顺延到 V1.5
- P2 → 不进 MVP 排期

---

## 4. 模块边界（4 模块职责契约）

### 4.1 模块 A：触发引擎（Trigger Engine）

**唯一职责**：检测外部信号并创建 AgentRun。

| 上游 | 下游 | 边界接口 |
| --- | --- | --- |
| Webhook / 轮询 / fsnotify | AgentRun 创建 | `TriggerEvent{event_id, source, type, payload, occurred_at}` |

**与其他模块的边界**：
- ✅ 不知道也不关心上下文数据
- ✅ 不知道也不关心 LLM Prompt
- ✅ 不知道也不关心交付目标
- ❌ 不做任务识别（识别交给模块 C 模板选择）

### 4.2 模块 B：上下文搜集（Context Harvesting）

**唯一职责**：按数据源适配器拉取并清洗上下文。

| 上游 | 下游 | 边界接口 |
| --- | --- | --- |
| AgentRun + 用户数据源配置 | 模块 C | `ContextSnapshot{commits[], meetings[], messages[], notes[], filter_meta}` |

**与其他模块的边界**：
- ✅ 不知道也不关心 LLM Prompt
- ✅ 不知道也不关心模板类型
- ❌ 不做语义检索（语义检索交给模块 C RAG）

### 4.3 模块 C：草稿生成（Synthesis）

**唯一职责**：基于上下文 + 模板生成结构化草稿并流式推送。

| 上游 | 下游 | 边界接口 |
| --- | --- | --- |
| 上下文快照 + 任务类型 | 模块 D + 前端 | `Draft{id, markdown, marks[], completeness, sse_stream}` |

**与其他模块的边界**：
- ✅ 不知道也不关心第三方 API
- ✅ 不知道也不关心交付目标
- ❌ 不做内容交付（交付交给模块 D）

### 4.4 模块 D：交付管道（Delivery Pipeline）

**唯一职责**：将草稿写入目标平台并回写链接 + 通知。

| 上游 | 下游 | 边界接口 |
| --- | --- | --- |
| 草稿 + 交付配置 | 第三方平台 + 任务工具 | `Delivery{delivery_id, target_url, status}` |

**与其他模块的边界**：
- ✅ 不知道也不关心 LLM 细节
- ✅ 不知道也不关心上下文数据
- ❌ 不做内容修改（修改由用户在前端完成）

### 4.5 跨模块依赖图

```
模块A ──► AgentRun ──► 模块B ──► ContextSnapshot ──► 模块C ──► Draft ──► 模块D ──► Delivery
  │                                              │
  └────────── SSE 事件流（贯穿 B/C/D）◄──────────┘
```

**关键约束**：模块 A→B→C→D 是严格单向依赖，**禁止反向调用**。

---

## 5. 平台边界

### 5.1 技术栈分层

| 层 | 选型 | 边界 | 责任 |
| --- | --- | --- | --- |
| 客户端 | Tauri v2 + React + Lobe UI | 与后端通过 REST + SSE 通信 | UI 渲染、草稿编辑、桌面通知 |
| API 网关 | Go + Gin | 对外暴露 /api/v1/* 端点 | 认证、限流、请求路由 |
| Agent 编排 | Eino（DAG） | 内部流程，不对外暴露 | 4 阶段流水线编排 |
| 数据层 | PostgreSQL + pgvector | 仅 API 网关可访问 | 结构化数据 + 向量数据 |
| 缓存/队列 | Redis（Asynq） | 内部通信 | 任务队列、SSE 断线缓存 |
| 第三方 API | GitHub/Calendar/IM/Notion | 通过适配器隔离 | 外部数据源 + 交付目标 |

### 5.2 平台职责边界

| 平台 | 负责 | 不负责 |
| --- | --- | --- |
| 桌面客户端 | 草稿展示、编辑、[待补充] 高亮、桌面通知 | 触发检测、LLM 调用、数据拉取 |
| Go 后端 | 触发解析、上下文拉取、LLM 调用、草稿生成、交付写入、SSE 推送 | UI 渲染、本地文件 IO（Obsidian 除外） |
| 第三方平台 | 数据提供、文档托管 | Agent 逻辑、用户配置 |

### 5.3 客户端方案边界

| 方案 | 包含 | 不包含 |
| --- | --- | --- |
| Tauri v2（推荐） | 桌面端 Windows/macOS/Linux | iOS/Android（V1.5+ 再评估） |
| Flutter（备选） | 桌面 + 移动端统一 | 不在 MVP 选型范围 |

**决策**：MVP 优先 Tauri v2，移动端延后到 V1.5。

---

## 6. 时间边界（13 周 WBS）

### 6.1 Phase 划分

| Phase | 周次 | 目标 | 交付物 | 退出标准 |
| --- | --- | --- | --- | --- |
| **Phase 0** | W1-W2 | 基础设施 | 脚手架、Schema、SSE、OAuth、CI/CD | `/health` 200，可登录 1 个 OAuth |
| **Phase 1** | W3-W4 | 触发引擎 | 关键词 + DDL + Webhook | 触发事件成功创建 AgentRun |
| **Phase 2** | W4-W7 | 上下文搜集 | GitHub/日历/IM/笔记 + 过滤 | 多源数据可拉取且过滤后无噪音 |
| **Phase 3** | W7-W9 | 草稿生成 | 模板 + Eino DAG + LLM + [待补充] + SSE | 草稿流式输出且完成度 ≥ 30% |
| **Phase 4** | W10-W13 | 前端 + 交付 | 编辑器 + Notion + 通知 + 集成测试 | 端到端 US-01~05 全部跑通 |

### 6.2 关键里程碑

- **M0（W2 末）**：基础设施就绪，Demo: 手动 POST → 200
- **M1（W4 末）**：触发器可工作，Demo: Todoist 新建任务 → 触发事件
- **M2（W7 末）**：上下文可拉取，Demo: 拉取 4 个数据源数据
- **M3（W9 末）**：草稿可生成，Demo: SSE 流式生成周报草稿
- **M4（W13 末）**：MVP 发布，Demo: 端到端 5 个用户故事

### 6.3 并行开发窗口

| 并行任务 | 主路径 | 启动时间 |
| --- | --- | --- |
| 单元测试编写 | 各 Phase 同步 | Phase 0 起 |
| API 文档（OpenAPI） | 与代码同步 | Phase 0 起 |
| 前端原型设计 | Phase 1 启动时 | W3 |
| 集成测试 | Phase 4 | W10 |

---

## 7. 数据边界

### 7.1 数据归属

| 数据类型 | 存储位置 | 加密 | 保留策略 |
| --- | --- | --- | --- |
| 用户配置 | PostgreSQL | 否 | 与账号同生命周期 |
| Token / API Key | PostgreSQL（config 字段） | AES-256-GCM | 撤销即删 |
| 原始上下文（commit/msg/note） | PostgreSQL + pgvector | 否 | 90 天，可配置 |
| Embedding 向量 | pgvector | AES-256 | 90 天 |
| 草稿 | PostgreSQL | 否 | 永久（用户可删） |
| 交付 URL | PostgreSQL | 否 | 永久 |
| 模板 | 文件系统 | 否 | 版本控制 |
| SSE 缓存 | Redis | 否 | 30 分钟 |

### 7.2 数据流向边界

```
第三方 API ──► 模块 B（拉取+过滤）──► PostgreSQL（落库+向量化）
                                                │
                                                ▼
模块 C（LLM 调用）──► 敏感信息脱敏 ──► LLM Provider
                                                │
                                                ▼
模块 D（写入）──► Notion / Obsidian / 飞书
```

**关键约束**：
- LLM 调用前必须对邮箱/手机号/身份证脱敏
- 不使用用户数据训练/微调任何模型
- 用户可一键清除全部数据（含缓存和向量）

---

## 8. 风险边界

| 风险 | 等级 | 边界（缓解） | 责任人 |
| --- | --- | --- | --- |
| 第三方 API 限流 | 高 | 令牌桶 + 指数退避 + 缓存兜底 + 单源失败跳过 | 模块 B 适配器作者 |
| OAuth Token 失效 | 高 | 5min 提前 refresh + 失效通知 + 缓存兜底 | 数据源管理 |
| LLM 质量不稳定 | 中 | temperature=0.3 + 多轮润色 + 用户反馈闭环 | 模块 C 编排 |
| 数据隐私合规 | 中 | 本地优先 + 敏感脱敏 + 90 天清理 | 数据层 |
| 单点故障 | 中 | 健康检查 + supervisor + 优雅降级 + 断路器 | 运维 |
| 第三方 API 升级 | 低 | 适配器隔离 + 版本锁 + changelog 监控 | 适配器作者 |

---

## 9. 验收边界（MVP 成功指标）

| 指标 | 目标 | 测量方法 | Phase 截止 |
| --- | --- | --- | --- |
| 草稿完成度 | ≥ 30% | Agent 文本 token / 最终文档 token | W9 |
| 用户修改率 | < 50% | 用户修改段落 / 总段落 | W13 |
| 端到端延迟 | < 30s | AgentRun 创建 → 交付完成 | W9 |
| 数据源覆盖 | ≥ 3 个 | GitHub + Cal + IM（笔记可选） | W7 |
| 用户满意度 | ≥ 4.0/5 | 1-5 分评分弹窗 | W13 |
| SSE 首字节 | < 3s | 网络抓包测量 | W9 |
| Notion 写入成功率 | > 99% | 集成测试 100 次 | W13 |

**MVP 退出标准**：
- ✅ 5 个用户故事（US-01~05）全部通过验收测试
- ✅ 13 项 CORE 需求全部实现
- ✅ 5 项 NFR 全部达标
- ✅ 0 个 P0 缺陷遗留
- ✅ 关键指标（草稿完成度、修改率、延迟）达标

---

## 10. 变更控制

### 10.1 范围变更流程

```
发现新需求 → 影响评估（4 维度）→ 分类（CORE / PLUS / V1.5 / V2.0 / 拒绝）
                                          │
                                          ▼
                              更新本文件 + 同步 requirement-spec + 同步 mvp-definition
```

**4 维度影响评估**：
- 工期影响（>3 天需重新评审时间线）
- 依赖影响（新增外部 API 需重做风险评估）
- 数据影响（涉及新数据源需更新隐私边界）
- 验收影响（新增指标需补充测试用例）

### 10.2 决策记录（ADR 入口）

重大边界决策需追加 ADR（Architecture Decision Record），包含：
- 决策日期
- 上下文与备选方案
- 决策理由
- 后果与权衡

---

## 11. 下一步行动

1. **W1 启动前**：
   - [ ] 评审本规划文档
   - [ ] 拆分 Phase 0 任务到 task-tracker.html
   - [ ] 确认 Tauri + Go 技术栈的版本号与依赖锁定
   - [ ] 申请 OAuth App（GitHub / Google / Notion / 飞书）

2. **Phase 0 交付前**：
   - [ ] 完成数据库 Schema 评审
   - [ ] 完成 SSE 协议草案
   - [ ] 完成 API 路由骨架

3. **持续进行**：
   - [ ] 每 Phase 结束更新本文件对应章节
   - [ ] 每周同步 task-tracker.html 进度
   - [ ] 风险表每月评审

---

## 附录 A：需求编号 → MVP 归属速查表

| 编号 | 名称 | 档位 | Phase | 状态 |
| --- | --- | --- | --- | --- |
| FR-A01 | 关键词触发 | CORE | 1 | 待开发 |
| FR-A02 | DDL 触发 | PLUS | 1 | 待开发 |
| FR-A03 | Webhook 接入 | CORE | 1 | 待开发 |
| FR-A04 | 手动触发 | PLUS | 1 | 待开发 |
| FR-B01 | GitHub 拉取 | CORE | 2 | 待开发 |
| FR-B02 | 日历拉取 | PLUS | 2 | 待开发 |
| FR-B03 | IM 拉取 | PLUS | 2 | 待开发 |
| FR-B04 | 笔记读取 | PLUS | 2 | 待开发 |
| FR-B05 | 噪音过滤 | CORE | 2 | 待开发 |
| FR-B06 | 增量同步 | CORE | 2 | 待开发 |
| FR-C01 | RAG 检索 | CORE | 3 | 待开发 |
| FR-C02 | 模板套用 | CORE | 3 | 待开发 |
| FR-C03 | LLM 润色 | CORE | 3 | 待开发 |
| FR-C04 | [待补充] 标记 | CORE | 3 | 待开发 |
| FR-C05 | SSE 流式输出 | CORE | 3 | 待开发 |
| FR-D01 | Notion 创建 | CORE | 4 | 待开发 |
| FR-D02 | Obsidian 写入 | V1.5 | — | 暂缓 |
| FR-D03 | 飞书创建 | V1.5 | — | 暂缓 |
| FR-D04 | 任务备注 | CORE | 4 | 待开发 |
| FR-D05 | 通知推送 | CORE | 4 | 待开发 |

## 附录 B：术语对照

| 文档 A 用语 | 文档 B 用语 | 含义 |
| --- | --- | --- |
| 模块A 触发引擎 | 触发引擎 | Trigger Engine |
| 模块B 上下文搜集 | 上下文搜集器 | Context Harvesting / Collector |
| 模块C 草稿生成 | 草稿生成器 | Synthesis / Draft Generator |
| 模块D 交付管道 | 交付管道 | Delivery Pipeline |
| AgentRun | 任务 | 单次端到端执行实例 |
| Draft | 草稿 | LLM 生成的半成品文档 |
| Delivery | 交付 | 草稿写入目标平台的动作 |
| [待补充] | 标记 | LLM 置信度低时的占位符 |
| 噪音过滤 | — | 规则 + LLM 双层过滤 |
