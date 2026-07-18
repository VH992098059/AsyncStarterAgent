# AsyncStarterAgent

**在你打开任务之前，先把前 30% 做完。**

AsyncStarterAgent 会盯着一件事"即将开始"的信号——群里提到的关键词、逼近的截止日期、来自各类工具的 webhook 事件——然后自动从这件事相关的各处（GitHub、日历、IM、笔记）聚合上下文，用 LLM 生成草稿，再把草稿实时推给你，或者直接写进 Notion、Obsidian、飞书文档。你打开任务时，它已经开始了。

[English](./README.md) · **简体中文**

---

## 目录

- [为什么做这个](#为什么做这个)
- [工作原理](#工作原理)
- [特性](#特性)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [项目结构](#项目结构)
- [文档](#文档)
- [进展状态](#进展状态)
- [参与贡献](#参与贡献)

## 为什么做这个

大多数"AI 任务"工具都在等你打开对话框、描述需求。可到那时候，你已经花了力气去回忆上下文、找相关链接、组织怎么问。

AsyncStarterAgent 反过来做：在后台监听触发信号，在你开口之前就把相关上下文收集好，直接给你一份已经完成 30% 的草稿。你要做的只是审核、补齐空白、发出去——而不是从一张白纸开始。

## 工作原理

四阶段流水线，全程通过 SSE 流式输出：

```
 触发（Trigger）      聚合（Harvest）         生成（Synthesize）      交付（Deliver）
┌─────────────┐      ┌───────────────┐      ┌──────────────┐      ┌─────────────────┐
│ 关键词命中    │ ──▶  │ GitHub        │ ──▶  │ RAG 检索      │ ──▶  │ Notion 页面      │
│ DDL 临近     │      │ 日历          │      │ Eino Agent   │      │ Obsidian 笔记库  │
│ Webhook 事件 │      │ IM 消息       │      │ LLM 草稿生成  │      │ 飞书文档         │
└─────────────┘      │ 笔记          │      │ SSE 流式输出  │      │ 评论回写         │
                      └───────────────┘      └──────────────┘      └─────────────────┘
```

- **触发**（`internal/trigger`）——关键词匹配、DDL 轮询调度器、带签名校验的 Webhook 接入（Todoist、飞书）
- **聚合**（`internal/harvesting`）——可插拔的数据源适配器 + 增量 ETL 落库为向量
- **生成**（`internal/synthesis`）——pgvector 向量检索 + Eino DAG/Workflow Agent 编排 + LLM 生成草稿，通过 SSE 流式返回
- **交付**（`internal/delivery`）——把草稿写回 Notion、Obsidian 或飞书文档，并在原始讨论串里回写评论

## 特性

- **多通道触发** —— 关键词规则、DDL 轮询检测、签名校验 Webhook，三路统一汇入去重后的任务队列
- **上下文聚合** —— GitHub、日历、IM、笔记的适配器，增量同步而非每次全量拉取
- **RAG + Agent 生成草稿** —— pgvector 检索为 [Eino](https://github.com/cloudwego/eino) DAG/Workflow Agent 提供上下文，调用 OpenAI 兼容模型生成草稿
- **流式输出** —— 草稿边生成边通过 SSE 推送，而不是等一次长阻塞调用结束
- **写回式交付** —— 直接落地到 Notion、Obsidian 或飞书文档，AI 没把握的地方标记为 `[待补充]`
- **按用户的飞书凭证** —— 每个用户可以注册自己的飞书自建应用（App ID/Secret 通过 pgcrypto 加密存储），不再共用一个全局应用
- **桌面客户端** —— Tauri + React 壳子，本地审核和编辑草稿

## 技术栈

| 层次 | 技术 |
|---|---|
| 后端 | Go 1.26 · Gin · [Eino](https://github.com/cloudwego/eino)（LLM/Agent 编排）· PostgreSQL 16 + pgvector · Redis 7 · Asynq |
| 集成 | GitHub API · 飞书（Lark）开放平台 SDK · Google API（日历）· OpenAI 兼容模型 |
| 鉴权与安全 | JWT · pgcrypto 加密凭证存储 · OAuth2 |
| 桌面客户端 | Tauri v2（Rust 壳）· React 19 · TypeScript · Tailwind CSS 4 · assistant-ui · Tiptap |

## 快速开始

```bash
make docker-up    # 启动 PostgreSQL + Redis
make build        # 编译
make run          # 运行 API 服务
```

健康检查：`curl http://localhost:8080/health`

其他常用命令：`make test`、`make migrate-up`、`make docker-logs` —— 完整列表见 `Makefile`。

## 项目结构

```
cmd/            程序入口（API 服务、依赖装配）
internal/
  trigger/      关键词 / DDL / Webhook 触发引擎
  harvesting/   数据源适配器 + ETL
  synthesis/    RAG + Eino Agent + LLM 生成草稿
  delivery/     Notion / Obsidian / 飞书 写回
  feishu/       飞书 OAuth、Token 与按用户 client 管理
  handler/      HTTP 处理器
migrations/     SQL 迁移脚本
web/            Tauri + React 桌面客户端
doc/            需求规格、MVP 定义、任务追踪、决策日志
docs/superpowers/  各功能分支的 spec-driven 实现计划
```

## 文档

- 需求: `doc/requirement-spec.html`
- MVP 定义: `doc/mvp-definition.html`
- 实现计划: `doc/plans/00-index.md`
- AI 编码边界规范: `doc/ai-coding-boundary.md`
- 任务追踪（权威进度来源）: `doc/task-tracker.html`

## 进展状态

核心流水线（触发 → 聚合 → 生成 → 交付）与飞书集成已实现，并有通过的单元测试覆盖。作为备选评估的 Flutter 前端方案尚未开始。飞书用户自建应用凭证功能已在功能分支上实现，待最终验收。以 `doc/task-tracker.html` 里逐任务的进度为准——它比仓库里其他地方的进展说明更新、更权威。

## 参与贡献

本项目采用 spec-driven 的工作方式：设计规格与实现计划放在 `docs/superpowers/` 下，改动按任务逐步提交 diff，AI 辅助的改动需遵循 `doc/ai-coding-boundary.md`。
