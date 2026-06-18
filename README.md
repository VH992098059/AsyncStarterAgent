# AsyncStarterAgent

"完成前 30%" 异步行动起跑器 Agent — 后端服务。

## 快速开始

```bash
make docker-up    # 启动 PostgreSQL + Redis
make build        # 编译
make run          # 运行
```

健康检查: `curl http://localhost:8080/health`

## 文档

- 需求: `doc/requirement-spec.html`
- MVP 定义: `doc/mvp-definition.html`
- 计划: `doc/plans/00-index.md`
- AI 编码规范: `doc/ai-coding-boundary.md`

## 状态

- [x] Phase 0 / T001: Go 项目初始化与目录结构
- [ ] Phase 0 / T002: Gin 框架搭建与中间件配置
- [ ] Phase 0 / T003: 数据库 Schema 设计与迁移
- [ ] Phase 0 / T004: Redis 任务队列集成
- [ ] Phase 0 / T005: Docker 开发环境配置
