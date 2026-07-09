# 决策日志 — Decision Log

> **作用**: 记录所有"需要用户决定才能继续"或"修改规范本身"的关键决策
> **创建日期**: 2026-06-18
> **维护规则**: 按 [ai-coding-boundary.md §5](./ai-coding-boundary.md) 维护
> **格式**: 每条决策用 `## [YYYY-MM-DD HH:MM] 决策 #N` 标题

---

## 修订记录

| 日期 | 决策 # | 修订内容 |
|---|---|---|
| 2026-06-18 21:45 | #1 | 初始创建：依赖最新稳定版规则（M8） |
| 2026-06-25 11:20 | #2 | 前端修复：登录/JWT/仪表盘/触发配置真实化 |
| 2026-06-25 12:00 | #3 | 新增 Superpower Skills 使用规范 §8 |
| 2026-06-25 12:30 | #4 | 新增持续对齐机制 §8.8（项目无关 / 可移植） |
| 2026-06-25 13:00 | #5 | 前端修复收尾：触发器改用 JWT 上下文 / SSE 走 query token |
| 2026-06-25 14:40 | #6 | CORS 中间件（dev 跨域预检 404 修复） |
| 2026-07-08 19:00 | #7 | 飞书集成范围扩展：FR-D03 扩入 MVP + user_access_token + pgcrypto |
| 2026-07-09 22:30 | #7 | 飞书集成实现完成：F001-F012 全部交付（12 提交），build/vet/test 全绿 |

---

## [2026-06-18 21:45] 决策 #1 — 依赖版本策略

**问题**: go.mod 和前端的组件包是否需要使用最新稳定版？该规则应写入 ai-coding-boundary.md 吗？

**用户原话**:
> "还有增加一个条件就是 go mod 和前端的组件包需要最新的，加上这个 ai-coding-boundary.md 上"

**触发场景**:
- 上一窗口完成 T001（go.mod 当前为空，无第三方依赖）
- T002 计划写 `go get github.com/gin-gonic/gin@v1.10.0`（具体版本）
- 用户主动要求修改 ai-coding-boundary.md 加入"最新依赖"约束

**执行方案**:
在 [ai-coding-boundary.md §2](./ai-coding-boundary.md) 新增 **M8 规则**：
- Go 依赖一律 `go get <pkg>@latest`（不锁版本到具体 tag）
- 前端组件包一律 `pnpm/npm/yarn add <pkg>@latest`
- 排除 pre-release (alpha/beta/rc)
- 每次升级后必须跑测试验证
- major 版本号变化时必须在本文件记录原因与影响
- **M8 与 plans/*.md 锁定版本冲突时，M8 优先**

**影响范围**:

| 范围 | 影响 |
|---|---|
| T002 计划 | 计划 `gin@v1.10.0` → M8 改为 `gin@latest` |
| T003 计划 | 计划 `pgx@v5.5.5` / `migrate@v4.17.1` → M8 改为 `@latest` |
| T004 计划 | 计划 `asynq@v0.24.1` → M8 改为 `@latest` |
| Phase 4 前端 | React / Lobe UI / Tauri 插件 → 全用 `@latest` |
| T001 → go.mod | 无影响（当前无第三方依赖） |
| 现有用户代码 | 无（项目为空） |

**未决问题**（可在后续窗口澄清）:
1. "最新"是否包含 pre-release → 当前规则：不包含
2. 前端包管理器选型（pnpm/npm/yarn） → 当前规则：按项目选定，T021 起决定
3. 何时批量升级已有依赖到 latest → 当前规则：每次新增时；批量升级建议单独开 ADR
4. 是否所有依赖都用 @latest（包括 indirect） → 当前规则：是

**后续窗口注意事项**:
- 任何"按计划文件的具体版本"执行命令前，先看 M8 是否优先
- `go get <pkg>@latest` 后必须 `go mod tidy` 并展示 `go.mod` diff
- 前端依赖升级需 `pnpm-lock.yaml` / `package-lock.json` 同步更新

---

## [2026-06-25 11:20] 决策 #2 — 前端修复（登录/JWT/仪表盘/触发配置真实化）

**问题**: 用户报告 4 个前端问题需要修复，且要求遵循 ai-coding-boundary.md：
1. 「自动登录」需要 JWT 黑名单（前后端）
2. 仪表盘页面不是仪表盘
3. 触发配置不清晰
4. 不能有 mock 数据，必须连后端 API

**用户原话**:
> "前端需要修复内容：1.我没有登录就自动登录上了，需要设置JWT黑名单，后端也是一样看看有没有这个功能 2.仪表盘页面有问题，仪表盘页面没有仪表盘，而是这些不知道是什么东西 3.触发配置是什么，这个不清晰 4.都需要连接后端API不能有mock数据 5.遵循这个 ai-coding-boundary.md 编码规则"

**触发场景**:
- 上一窗口完成 T026（Phase 3 / Phase 4 部分），前端页面已存在但无 auth、无 dashboard 真实数据、无 keywords/datasources GET 端点
- 调研发现：
  - 后端 5 个端点全部无认证（`server.go` 全文确认）
  - `config.go:16` 已有 `JWTSecret` 字段但无人使用
  - `App.tsx:17-21` 用 `crypto.randomUUID()` 把 userId 写入 localStorage — 这就是"自动登录"现象的根因
  - `TriggerConfig.tsx:44,119-124` 硬编码关键词列表与数据源状态 — 违反 C8
- MVP 定义明确"多用户协作"在 OUT 范围

**3 项子决策**（已通过 AskUserQuestion 询问并获得用户明确选择）:

### 子决策 A — 登录方案
**用户回复**: "加完整登录 + JWT + 黑名单（需要扩 MVP）"
- 在 `mvp-definition.html` 已锁定的 OUT 范围外新增 auth 模块
- 影响：plan-boundary.md 需要追加"扩 MVP 范围"标记

### 子决策 B — 仪表盘内容
**用户回复**: "近期运行列表 + 状态统计（推荐）"
- 走 `GET /api/v1/agent-runs?limit=20` + 今日统计聚合
- 需要后端新增列表端点

### 子决策 C — 触发配置数据
**用户回复**: "新增后端 GET 端点 + 前端调用（推荐）"
- 新增 `GET /api/v1/keywords`（来自 `trigger.Matcher` 的 `Rules()`）
- `GET /api/v1/datasources`（来自 `data_sources` 表）

**AI 自决项**（按 §6 "用户说'你自己决定'等价"路径，仅限非红线）:

| 决策点 | 选择 | 理由 |
|---|---|---|
| 密码哈希 | bcrypt (cost 10) | Go 生态标准；`golang.org/x/crypto/bcrypt` 已随 gin 间接依赖 |
| JWT 算法 | HS256 + JWT_SECRET | config.go 已有字段；单 MVP 实例无需 RS256 |
| JWT 黑名单存储 | 内存 map + mutex | MVP 单实例；后续可换 Redis；不需新建依赖 |
| 前端 token 存储 | localStorage | 单页应用 + 跨刷新持久化；不用 httpOnly cookie（避免后端 SSR 复杂度） |
| 前端 HTTP 客户端 | 继续用 fetch + 包 wrapper | 已有 fetch 客户端；不引新依赖（避免 §3.5 新增依赖询问） |
| 用户表 | 新建 `users` 表(id, username UNIQUE, password_hash, created_at) | MVP 单用户场景扩为多账号；不自建 oauth 流程 |
| 注册端点 | 新建 `POST /api/v1/auth/register` | 让首个用户能注册；后续可关闭 |

**执行方案**:

**Step 1** — 决策落库（commit 1）: ADR + plan-boundary.md 同步

**Step 2** — 后端 auth 模块（commits 2-5，TDD）:
- migration `0006_auth.up/down.sql` 建 `users` 表
- `internal/auth/jwt.go` + `internal/auth/jwt_test.go`：签发/解析/校验
- `internal/auth/middleware.go` + `internal/auth/middleware_test.go`：Bearer 解析 + 黑名单检查
- `internal/auth/service.go` + `internal/auth/service_test.go`：register/login/logout
- `internal/handler/auth.go` + `internal/handler/auth_test.go`：4 个端点
- `internal/server/server.go` 接入 + 给受保护端点加中间件
- `cmd/api/wire.go` 注入 auth service

**Step 3** — 后端列表端点（commits 6-8，TDD）:
- `GET /api/v1/keywords`：`trigger.Matcher.Rules()` 导出为列表
- `GET /api/v1/datasources`：`repository.ListDataSources(ctx, pool, userID)` 新增
- `GET /api/v1/agent-runs?limit=N&status=...`：`repository.ListAgentRuns` 新增

**Step 4** — 前端（commits 9-13）:
- 新建 `web/src/api/auth.ts`：login/register/logout/getMe
- 改造 `web/src/api/client.ts`：fetch wrapper 注入 Authorization + 401 跳转
- 新建 `web/src/components/Login.tsx`：登录/注册页（双 tab）
- 改造 `web/src/App.tsx`：路由守卫 + 登出按钮
- 改造 `web/src/components/Dashboard.tsx`：近期运行列表 + 状态统计
- 改造 `web/src/components/TriggerConfig.tsx`：删硬编码，走 API

**影响范围**:

| 范围 | 影响 |
|---|---|
| 规范 | MVP 范围扩展（auth + dashboard + 列表 API），plan-boundary.md 需要追加章节标注扩 MVP |
| 数据库 | 新增 `users` 表（migration 0006） |
| 配置文件 | .env.example 需追加 JWT_SECRET 示例；新增示例用户文档 |
| 受保护端点 | /api/v1/trigger、/api/v1/drafts/:id/stream、/api/v1/drafts/:id/deliver 都走 JWT 中间件 |
| 不受保护 | /health、/api/v1/webhook/todoist（webhook 已有 secret 验证）、/api/v1/auth/* 自身 |
| 前端存储 | localStorage 增加 `asa_token`；移除 `asa_user_id`（改为 token 解析的 user_id） |
| 测试 | 后端 service/middleware/handler 三层单测；前端暂不引入 e2e（按 C9 规则不引新依赖） |
| 部署 | docker-compose / Makefile 不变；新增 0006 migration 自动跟随 |

**未决问题**（后续窗口澄清）:
1. 是否需要"刷新 token"机制 → 当前不做，单 MVP token 7 天过期即重登
2. 跨实例部署时黑名单同步 → MVP 单实例内存足够；后续换 Redis
3. 注册是否要 admin 审核 → 当前开放注册，首个用户即 owner
4. dashboard 列表分页 → 当前 limit=20 上限，需要时分页

**后续窗口注意事项**:
- 修改任何后端 handler 前先读最新版本（[P5 必读]）
- server.go 的中间件顺序：recovery → logger → cors → auth → handler
- wire.go 注入新 service 时按现有 `cmd/api/wire.go` 风格
- 任何"按规范里没列的功能"先问再做
- C9: 前端不引 npm/yarn；本窗口不新增前端依赖，仅用现有 react/axios/tiptap

---

## [2026-06-25 12:00] 决策 #3 — 新增 Superpower Skills 使用规范

**问题**: 用户要求在 `ai-coding-boundary.md` 中新增"使用 superpower 技能"的规范，让编码和任务推进更合理。

**用户原话**:
> "帮我把 `k:\go_projects\AsyncStarterAgent\doc\ai-coding-boundary.md` 新增一个需要使用superpower技能的规范使用，让编码和任务更加合理"

**触发场景**:
- 上一窗口完成前端 4 项修复（决策 #2）
- 用户主动要求把"使用 Skill"提升为正式规范
- 背景：项目可用的 Skill 体系已就绪（`using-superpowers` / `brainstorming` / `TDD` / `verification-before-completion` 等），但尚无强制调用规则，导致 AI 习惯用通用能力直接动手

**执行方案**:

在 [ai-coding-boundary.md §8](./ai-coding-boundary.md) 新增 **Superpower Skills 使用规范**（文档版本 v0.1.0 → v0.2.0）：

1. **§8.1 核心原则**：Skill 是第一性资源；通用能力不替代专业 Skill
2. **§8.2 何时调用 Skill**：12 类触发场景（S-T1~S-T12）表 + 判定口诀 + 未匹配时的处理
3. **§8.3 标准工作流**：`using-superpowers → brainstorming → writing-plans → TDD → verification → code-review`
4. **§8.4 红线 SP1-SP5**：跳过 Skill / 通用替代 / 加载前回复 / 忽略询问提示 / 懒得调用
5. **§8.5 与现有规范的关系**：C1/C8/M1-M3/§3/§5/§9 的协同
6. **§8.6 自检清单**：6 个 yes/no 自问
7. **§8.7 何时不调用 Skill**：3 类豁免场景

**配套更新**:
- 后续章节重新编号：原 §8→§9、§9→§10、§10→§11
- §7.3 🚫 列表追加"跳过 Skill 直接动手（见 §8）"
- §11 生效与变更追踪 v0.2.0 增补说明
- 文档版本 v0.1.0 → v0.2.0

**12 类必调 Skill 场景**（S-T1~S-T12）:
| 场景 | 必调 Skill |
|---|---|
| 任何对话开始 | using-superpowers |
| 创意 / 设计 / 新功能 | brainstorming |
| 复杂 Bug | systematic-debugging + TRAE-debugger |
| 任何功能 / 修复 | test-driven-development |
| 3+ 步任务 | TodoWrite + writing-plans |
| 执行实现计划 | executing-plans / subagent-driven-development |
| 2+ 独立任务 | dispatching-parallel-agents |
| 创建 / 优化 Skill | skill-creator + writing-skills |
| 完成时 | verification-before-completion |
| 代码审查 | requesting-code-review / receiving-code-review |
| 安全敏感代码 | TRAE-security-review |
| 隔离工作区 | using-git-worktrees |

**影响范围**:

| 范围 | 影响 |
|---|---|
| 所有 AI 编码活动 | 强制要求每轮先评估 Skill 调用 |
| 现有代码 | 无（纯规范文档变更） |
| 测试 | 无（不需要新增测试） |
| 部署 | 无 |
| 后续窗口 | 每窗口开始时 AI 应主动声明"已评估 Skill 调用"或在自检清单中确认 |

**未决问题**（后续窗口澄清）:
1. "未调 Skill 的理由"是否每次都强制写 ADR → 当前规则：非强制，但漏调被指出时需要
2. 是否有 Skill 加载失败的重试机制 → 当前规则：失败就 ADR 记录，下次重试
3. 用户说"不用 Skill"是否要二次确认 → 当前规则：不重复发问，ADR 即可

**后续窗口注意事项**:
- 每窗口开始时若发现未调 `using-superpowers`，需在最终自检清单中说明
- 任何"按计划文件的具体版本"执行命令前，仍按 M8 优先
- 红色线 SP1-SP5 与 C1-C9/P1-P6 同级，违反触发 §9（违规处理）

---

## [2026-06-25 12:30] 决策 #4 — 新增持续对齐机制 §8.8（项目无关 / 可移植）

**问题**: 用户要求：
1. 编码过程中已实现的功能必须持续与 MVP / 需求 / 当前任务做对齐
2. 若发现 AI 想象 / 偏离任务，立即停止
3. 该机制必须项目无关，可平移到任何项目，不能局限于 AsyncStarterAgent
4. 顺便检查前面 §8 增补内容有无冲突

**用户原话**:
> "还有 `k:\go_projects\AsyncStarterAgent\doc\ai-coding-boundary.md` 增加让AI在编码的过程中已经实现的功能进行对MVP、需求分析和当前任务进行对比是否有AI想象出来偏离任务需求的，有的话则立即停止，顺便看看我以上的增加内容是否有冲突，然后这个需要这个编码过程可以适用任何一个项目，不能局限于某个项目"

**触发场景**:
- 上一窗口完成 §8 增补（决策 #3）
- 用户复盘时发现：§8 强调"使用 Skill"，但**没有强制编码过程持续对齐需求**
- §8.7 中残留项目特定文件名引用（`requirement-spec` / `mvp-definition`），违反项目无关原则
- §8.5 中残留（原 §8）标注

**冲突点排查**（按用户要求）:

| # | 位置 | 冲突类型 | 修复 |
|---|---|---|---|
| 1 | §8.5 表格 line 290 | 残留"（原 §8）"标注 | 改为"§9（违规处理）" |
| 2 | §8.7 第 3 项 | 项目特定文件名（`requirement-spec` / `mvp-definition`） | 改为通用术语"当前任务 / 需求 / 范围明确不在范围" |

**执行方案**:

升级 [ai-coding-boundary.md](./ai-coding-boundary.md) 到 **v0.3.0**：
1. **§1.3 流程红线追加 P7**：「禁止带偏移继续编码」（最高级别红线）
2. **§2 MUST DO 追加 M9**：「持续对齐检查」（流程细则）
3. **§7.3 🚫 追加**：「写完不复盘 / 出现偏移不停止」
4. **§8.8 新增《持续对齐机制（项目无关 / 可移植）》**：包括
   - §8.8.1 三个对比维度（任务 D1 / 需求 D2 / 范围 D3）
   - §8.8.2 检查时机（6 类强制时机）
   - §8.8.3 偏移类型（6 类：任务/需求/范围/AI 想象/接口/技术）
   - §8.8.4 标准动作（流程图 + 3 问）
   - §8.8.5 红线 A1-A5（与 P7/C1-C9/SP1-SP5 同级）
   - §8.8.6 与现有 10 项规范条款的协同
   - §8.8.7 自检清单（6 个 yes/no）
   - §8.8.8 项目无关性说明 + 移植步骤
   - §8.8.9 与 §8.4 / §8.6 的协同时机

**项目无关性设计**（用户重点要求）:
- §8.8 全部内容**不引用任何具体项目名 / 文件名 / FR 编号 / 业务术语**
- §8.8.1 用"任务 / 需求 / 范围"三个通用维度，"映射示例"列只列概念名（如 PRD / FRD / MVP / Phase）
- §8.8.8 明确说明"移植到任何项目只需替换 §8.8.1 映射表"
- 适用项目类型：电商 / 金融 / 工具 / 游戏 / 任何有 Sprint + 需求 + 范围划分的项目

**6 类偏移类型**:

| 类型 | 典型信号 | 处理 |
|---|---|---|
| 任务偏移 | 当前任务 A 还在进行，代码却处理 B | 立即停止 + 询问切换 |
| 需求偏移 | 找不到对应的 FR / Story / 验收点 | 立即停止 + 询问 |
| 范围偏移 | 命中 OUT 范围 / V1.5+ 功能 | 立即删除 + 询问 |
| AI 想象 | 加缓存 / 加日志 / 加重试，需求未列 | 强制 §3 询问，不允许自决 |
| 接口偏移 | 改了已锁定的 API 路径 / 表名 / 字段 | 立即停止 + 询问 |
| 技术偏移 | 引入技术栈外的依赖 | 立即停止 + 询问 |

**3 问自检**（每完成原子改动）:
1. D1 任务：对应哪个 Task / Issue / Todo？
2. D2 需求：对应哪条 FR / Story / 验收点？
3. D3 范围：在 MVP / Phase / IN 矩阵内吗？
4. AI 想象检查：有没有"顺手"加规范没有的东西？

**影响范围**:

| 范围 | 影响 |
|---|---|
| 所有 AI 编码活动 | 强制每原子改动做 3 问对齐 |
| §8 既有内容 | §8.5 清理残留 + §8.7 泛化引用 |
| §1.3 红线 | 追加 P7 流程红线 |
| §2 MUST DO | 追加 M9 流程规则 |
| §7.3 🚫 列表 | 追加对齐相关禁止项 |
| 现有代码 | 无（纯规范文档变更） |
| 测试 | 无（不需要新增测试） |
| 部署 | 无 |
| 后续窗口 | 每窗口开始时 AI 应主动声明"已评估对齐检查" |

**未决问题**（后续窗口澄清）:
1. "AI 想象"具体如何量化检查 → 当前规则：通过 §3 强制询问兜底
2. 偏移检查是否每 30 分钟强制 → 当前规则：是（见 §8.8.2）
3. 跨窗口 / 跨会话时对齐检查如何传承 → 当前规则：每个新窗口从 §8.8.7 自检开始

**后续窗口注意事项**:
- 任何写代码前先过 §8.8.4 的 3 问
- 偏移未解决时禁止任何形式的 commit / push
- §8.8 复制到其他项目时按 §8.8.8 步骤操作
- 红色线 A1-A5 与 C1-C9 / P1-P7 / SP1-SP5 同级，违反触发 §9
- §8.8 + §8.6 + M9 + P7 + A1-A5 五重保险：开始前 / 过程中 / 完成后 / 提交前 都要过

---

## [2026-06-25 13:00] 决策 #5 — 前端修复收尾（触发器改用 JWT / SSE query token）

**问题**: 在执行决策 #2 的前端 4 项修复时，发现两个后端配套问题需同步修：
1. `internal/handler/trigger.go` 的 `ManualTrigger` 仍从 body 读 `user_id`，但该端点已挂 `auth.Middleware`，user_id 应来自 JWT — 现状下前端用 JWT 触发的请求会 400（"invalid user_id"）。
2. SSE 端点 `GET /api/v1/drafts/:id/stream` 也挂 `auth.Middleware`，但浏览器 `EventSource` 不支持自定义 Header — token 无法用 Authorization 头传，401 一直挂。

**用户原话**: （承接决策 #2 的 5 项修复指令；这两个是收尾时的必要配套修改）

**触发场景**:
- 跑 `go test ./...` 暴露 trigger_test 失败（401 vs 期望 400）
- 改造 `client.ts` 时为 `streamDraft` 设计 token 传递方式，发现 EventSource API 限制
- §8.8 持续对齐检查：D1（修复任务）→ D2（FR-A04 / FR-C05 涉及）→ D3（在扩 MVP 范围）→ 命中 OK

**执行方案**:

### 1. TriggerHandler 改用 JWT 上下文
- 文件：`internal/handler/trigger.go`
- 改动：`triggerRequest.UserID` 删除，body 只剩 `text binding:"required"`
- user_id 改用 `auth.MustUserID(c)` 从 context 取
- 同步更新 `internal/handler/trigger_test.go`：router 前置 middleware 模拟已注入 user_id
- 原因：避免 body 中 user_id 与 JWT user_id 不一致时的越权（安全修复）
- 字段从 body 迁移到 context 是"安全"改进而非范围偏移（C3 不触发，因 body 字段是删除）

### 2. auth.Middleware 支持 query string token（用于 SSE）
- 文件：`internal/auth/middleware.go`
- 改动：提取 `extractToken(c)` 辅助函数 — 优先 `Authorization: Bearer`，回退 `?token=`
- SSE 端点 URL：`/api/v1/drafts/:id/stream?token=<JWT>`
- 风险：`?token=` 形式会进 access log / 浏览器历史 — SSE 端点 token 7 天过期，可接受
- 替代方案（已拒绝）：用 `event-source-polyfill`（需新增前端依赖 → 触发 C7 询问）

**AI 自决项**（按 §6 "用户说'你自己决定'等价"路径，仅限非红线）:

| 决策点 | 选择 | 理由 |
|---|---|---|
| 删除 body 中 user_id 字段 | 彻底删除（不是 deprecated） | 防越权；前后端同步改 |
| SSE token 传 query 而非 polyfill | query string | 不新增依赖（C7 询问可省） |
| token 是否记录到 ADR | 记录 | SSE 鉴权是 C3 范畴（接口机制变更） |

**影响范围**:

| 范围 | 影响 |
|---|---|
| 接口契约 | `POST /api/v1/trigger` 请求体：`{user_id, text}` → `{text}`（breaking，但前端同步改） |
| 鉴权机制 | `auth.Middleware` 新增 `?token=` 回退；不影响其他端点 |
| 测试 | trigger_test 更新；auth_test 不变（middleware 行为向后兼容） |
| 前端 | `triggerAgent(text)` 不再带 userId（已生效） |
| 安全 | 触发器越权面消除（user_id 不再可被 body 覆盖） |
| 部署 | 无（环境变量不变） |

**未决问题**（后续窗口澄清）:
1. SSE query token 是否进 access log → 当前接受（7 天过期足够短；如需严格合规可后续改用 polyfill）
2. trigger 端点是否要支持匿名 → 否，决策 #2 已确认必须登录

**后续窗口注意事项**:
- 任何后续 handler 改 user_id 流程时，优先从 `auth.MustUserID(c)` 取，不再读 body
- SSE 鉴权如要再加固，可改 short-lived token（5 分钟）
- 改 trigger 端点时记得同步前端（`apiFetch("/api/v1/trigger", { body: { text } })`）

---

## [2026-06-25 14:40] 决策 #6 — CORS 中间件（dev 跨域预检 404 修复）

**问题**: 前端 dev 时跑在 `http://localhost:1420`（Vite）/ `tauri://localhost`（Tauri），后端在 `http://localhost:8080`。浏览器跨域 POST 会先发 OPTIONS 预检；后端没配 CORS → 404 OPTIONS，浏览器拦截实际请求 → 前端注册/登录发不出去。

**用户原话**:
> "404 OPTIONS 怎么修？" → 用户选「后端加 CORS 中间件」

**触发场景**:
- 用户跑前端注册测试，devtools 看到 `404 OPTIONS /api/v1/auth/register 0s`
- §8.8 持续对齐检查：D1（修复 dev 跨域）→ D2（dev 环境基础设施，不改接口契约）→ D3（决策 #2 范围内）→ 命中 OK
- 触发 C7（新加 go 依赖 `gin-contrib/cors`）→ 已通过 AskUserQuestion 获得用户批准

**执行方案**:

### 1. 新增依赖
- `github.com/gin-contrib/cors v1.7.7`（M8 latest 规则）
- 仅 1 个新依赖，最小化

### 2. server.go 接入 cors 中间件
- 位置：先于 `Recovery/Logger`，确保 OPTIONS 预检快速响应
- 白名单 origin（白名单显式列出，不 `AllowAllOrigins` 避免安全风险）：
  - `http://localhost:1420`（Vite 默认）
  - `http://localhost:5173`（Vite 备用）
  - `tauri://localhost`、`http(s)://tauri.localhost`（Tauri 桌面）
- 用 `AllowOriginFunc` 而非 `AllowOrigins`：gin-contrib/cors 要求 `AllowOrigins` 中所有 origin 必须带 `http://`/`https://`，无法直接表达 `tauri://` 协议
- 显式列出 `AllowMethods`/`AllowHeaders`/`ExposeHeaders`：避免默认 `*` 行为
- `AllowCredentials: false`：JWT 在 header 传，不需要 cookie
- `MaxAge: 12h`：浏览器缓存预检结果

### 3. 测试
- TDD（红→绿）
- 新增 `internal/server/cors_test.go`：
  - `TestCORS_Preflight_RegisterEndpoint`：Vite origin OPTIONS 预检 → 204 + 头齐全
  - `TestCORS_Preflight_TauriOrigin`：Tauri origin OPTIONS 预检
  - `TestCORS_ActualRequest_HealthEndpoint`：实际 GET 请求带 ACAO 头

**AI 自决项**（按 §6 "用户说'你自己决定'等价"路径，仅限非红线）:

| 决策点 | 选择 | 理由 |
|---|---|---|
| AllowAllOrigins 还是白名单 | 白名单 | dev/桌面 origin 已知；不暴露给任意域 |
| 用 AllowOriginFunc 还是 AllowOrigins | AllowOriginFunc | 兼容 `tauri://` 非 http 协议 |
| AllowCredentials | false | JWT 在 header；无需 cookie |
| MaxAge | 12h | 平衡预检性能与配置变更生效速度 |
| 中间件顺序 | cors 在最前 | OPTIONS 快速响应，无需走业务中间件 |

**影响范围**:

| 范围 | 影响 |
|---|---|
| 接口契约 | 不变（请求/响应未改） |
| 部署 | 不变（环境变量未改） |
| 安全 | 收窄而非放宽（白名单替代默认行为） |
| 依赖 | +1 (`gin-contrib/cors`) |
| 测试 | +3（CORS 三个用例） |
| 旧行为 | dev 跨域 404 → dev 跨域 200/204 |

**未决问题**（后续窗口澄清）:
1. 生产部署用反代还是直接暴露 → 当前文档未约定；如直接暴露需要扩白名单
2. 是否需要支持自定义 origin 列表（环境变量） → 否，硬编码足够

**后续窗口注意事项**:
- 新增 dev/桌面 origin 时，更新 `allowedOrigins` 常量
- 生产部署前确认反向代理配置（如果直接 8080 暴露，需要扩白名单）
- §6 §7 §8 检验：本决策 = "加 CORS 解决 dev 404"，未改接口 / 数据 / 业务逻辑，不属 §6.2 红色线

---

## [2026-07-08 19:00] 决策 #7 — 飞书集成范围扩展（FR-D03 扩入 MVP + user_access_token + pgcrypto）

**问题**: 用户要求接入飞书 API 的"任务等功能"。经盘点，现有 `internal/harvesting/source/feishu.go` 仅做 IM 消息拉取（FR-B03）且未接线 wire.go；FR-D04（飞书任务备注回写，P0）未实现；FR-D03（飞书文档交付）在 plan-boundary.md 中标为 P2/V1.5（OUT of MVP）。用户选择全量接入并批准 FR-D03 扩入 MVP。

**用户原话**:
> "`k:\go_projects\AsyncStarterAgent` 这个我需要接入飞书的API，飞书的任务等功能，需要你使用AI编码规则技能，然后我们讨论一下"

经 AskUserQuestion 三轮澄清，用户明确选择：
1. 集成范围：拉取飞书任务 + 任务事件触发 AgentRun + 回写草稿链接到任务备注 + 交付到飞书文档（FR-D03）+ IM 适配器接线
2. 鉴权模型：user_access_token（用户身份，OAuth 流程）
3. FR-D03 范围处理：批准扩入 MVP
4. Token 加密：pgcrypto 对称加密（DB_ENCRYPTION_KEY 环境变量 + pgp_sym_encrypt/pgp_sym_decrypt）

**触发场景**:
- 现有 feishu.go 是"孤儿代码"（写了未装配）
- FR-D04（P0）规范要求但未实现
- FR-D03（P2/OUT）用户主动要求扩入 → 触发 R5 红线（禁止实现超范围功能），需正式批准 + 更新 plan-boundary.md
- user_access_token 方案涉及 OAuth 流程、token 存储、自动刷新，架构复杂度高

**执行方案**:

### 1. 范围扩展（plan-boundary.md 同步）
- FR-D03 从 §2.3 V1.5 清单移入 §2.2 MVP-PLUS 清单，标注"扩入"
- §3 矩阵 FR-D03 行：V1.5 → MVP-PLUS（扩）
- 附录 A FR-D03 状态：暂缓 → 扩入 MVP

### 2. 鉴权架构（user_access_token）
- OAuth 2.0 授权码流程：前端跳转飞书授权页 → 回调 `GET /api/v1/auth/feishu/callback` → code 换 token → 存储
- token 有效期 ~2h，refresh_token ~30d，过期自动刷新
- SDK 用法：`client.Request(ctx, ..., lark.WithUserAccessToken(token))`

### 3. Token 存储（独立表 + pgcrypto）
- 新建 `feishu_tokens` 表（不扩展 user_settings，避免污染 settings 缓存）
- 字段：user_id, access_token(加密), refresh_token(加密), expires_at, open_id, updated_at
- 加密：pgcrypto 的 pgp_sym_encrypt/pgp_sym_decrypt，密钥从 `DB_ENCRYPTION_KEY` 环境变量

### 4. 配置层
- config.go + .env.example 新增：`FEISHU_APP_ID` / `FEISHU_APP_SECRET` / `FEISHU_REDIRECT_URL` / `DB_ENCRYPTION_KEY`
- settings.Factory 新增 `GetFeishuClient(ctx, userID)` 方法（检查过期 → 刷新 → 返回带 user token 的 client）

### 5. 五项能力实现路径
| 能力 | 文件 | SDK 调用 |
|---|---|---|
| IM 消息接线 | wire.go 装配现有 feishu.go | 已有 larkim |
| 拉取飞书任务 | internal/harvesting/source/feishu_task.go | client.Task.V2.Task.List |
| 任务事件触发 | internal/handler/webhook.go 加飞书处理器 | 飞书事件订阅 v2 |
| 回写任务备注 | delivery/service.go updateSourceComment 加 feishu case | client.Task.V2.Comment.Create |
| 交付飞书文档 | internal/delivery/feishu.go（替代现有 feishu_doc.go 骨架） | docx API |

**AI 自决项**（按 §6 路径）:

| 决策点 | 选择 | 理由 |
|---|---|---|
| token 存独立表 vs 扩 settings 表 | 独立 feishu_tokens 表 | token 2h 刷新频繁，独立表不干扰 settings 缓存；语义分离（凭证≠配置） |
| 加密方案 | pgcrypto 对称加密 | 用户选定；无新 Go 依赖；DB 层加密便于审计 |
| OAuth 回调路径 | /api/v1/auth/feishu/callback | 与现有 /api/v1/auth/* 风格一致 |
| 前端授权入口 | 设置页内"飞书集成"区块 | 不新建独立页，复用现有 Settings 组件 |
| 计划文件位置 | docs/superpowers/plans/2026-07-08-feishu-integration.md | 遵循 writing-plans skill 默认 + 项目已有 superpowers/plans/ 先例 |

**影响范围**:

| 范围 | 影响 |
|---|---|
| 规范 | FR-D03 从 V1.5 扩入 MVP-PLUS；plan-boundary.md 更新 |
| 数据库 | 新增 feishu_tokens 表（migration 0010）+ pgcrypto 扩展 |
| 配置 | .env.example 新增 4 个环境变量 |
| 依赖 | 无新增（larksuite/oapi-sdk-go/v3 已有，需升级到 v3.4.25） |
| 后端 | 新建 feishu_task.go / feishu.go(delivery) / feishu_auth.go；扩展 webhook.go / service.go / factory.go / wire.go / config.go |
| 前端 | Settings 页新增飞书授权区块；交付选项加飞书文档；触发器配置加飞书事件源 |
| 安全 | user_access_token 加密存储；OAuth state 防 CSRF |
| 测试 | OAuth 流程 mock 测试 + 各适配器单测 + webhook 签名校验测试 |

**未决问题**（实现时澄清）:
1. 飞书事件订阅 challenge 验证方式（v1 明文 vs v2 加密）→ 实现时查最新文档
2. 飞书文档 markdown → docx block 转换的完整度 → MVP 阶段做基础段落转换，复杂块（表格/代码块）V1.5 补
3. token 刷新失败的降级策略 → 返回特定错误码，前端引导重新授权
4. 多用户并发刷新 token 的锁 → MVP 单实例用 mutex，后续换 Redis 锁

**后续窗口注意事项**:
- 所有飞书 API 调用必须走 `lark.WithUserAccessToken(token)`，不能用 tenant token
- feishu_doc.go 现有骨架（T025）将被本计划的完整实现替代
- webhook 飞书处理器需处理事件签名校验（飞书 v2 事件用 X-Lark-Signature 头）
- DB_ENCRYPTION_KEY 丢失 = 所有飞书 token 不可解密，需在部署文档强调备份
- 升级 lark SDK v3.4.4 → v3.4.25 后需跑全量测试（M8 规则）

### 实现完成记录（2026-07-09）

**状态**: ✅ 全部交付。分支 `feat/feishu-integration`，基线 `94e120e`，HEAD `c07b1cf`，共 12 个提交。

**交付清单**（plan: docs/superpowers/plans/2026-07-08-feishu-integration.md，F001-F012）:

| 任务 | 内容 | 关键文件 | 提交 |
|---|---|---|---|
| F001 | feishu_tokens 表（pgcrypto） | migrations/0010_feishu_tokens.up.sql | fe8d4a9 |
| F002 | config 飞书字段 | internal/config/config.go | ef85bf7 |
| F003 | token store（加解密 + %w 包装） | internal/feishu/token_store.go | e87c980 |
| F004 | OAuth client（code exchange + refresh） | internal/feishu/auth.go | 2d6e0b1 |
| F005 | per-user client factory（自动刷新） | internal/feishu/client_factory.go | c06d0b8 |
| F006 | settings factory 接线 | internal/settings/factory.go | 6aac646 |
| F007 | per-user token 适配器 + wire Deps | internal/harvesting/source/feishu.go | 40b0ac5 |
| F008 | 飞书任务拉取适配器 | internal/harvesting/source/feishu_task.go | 77b400e |
| F009 | 飞书事件订阅 webhook | internal/handler/webhook.go | 9edf390 |
| F010 | 飞书文档交付适配器 | internal/delivery/feishu.go | ef11396 |
| F011 | 任务评论回写 | internal/delivery/service.go | ef11396 |
| F012 | 前端授权 UI + API client | web/src/components/Settings.tsx, web/src/api/feishu.ts | a01cd3e |

**集成修复**（最终集成审查发现）: `c07b1cf` — webhook 未提取 task guid 导致 trigger_source 断链，评论回写失效。新增 `ProcessKeywordWithSource` + `parseFeishuTaskGUID` 纯函数 + 10 单测打通数据流。

**实现期关键架构修正**（代码质量审查 ❌ → 已修复）:
1. **CSRF cookie → 服务端 state store**: 原 cookie 方案在 Tauri/跨源部署根本失效（cookie 在 webview，授权在系统浏览器，回调读不到）。改为服务端 `oauthStateStore`（map+mutex+TTL+一次性 Consume），state 走 URL 传递，全拓扑可用。
2. **Callback 响应 JSON/重定向 → HTML 页面**: API-only 后端 `/?feishu_auth=success` 会 404，JSON 在浏览器导航下显示为裸文本。改为返回深色主题 HTML 成功/错误页，前端用 `visibilitychange` 感知返回并刷新授权状态。
3. **raw c.JSON → httpx 信封**: 前端 `apiFetch<T>` 期望 `{code,message,data}`，授权相关端点统一改用 `httpx.OK`/`httpx.Fail`。
4. **Status 错误归类**: `errors.Is(err, pgx.ErrNoRows)` 区分"未授权"与"服务端错误"，避免 DB 故障误报未授权。
5. **内部错误泄露**: Callback/Revoke 失败路径改 `log.Printf` 记录完整错误 + 对外泛化文案。

**验证**: `go build ./...` = 0；`go vet ./...` = 0；`go test ./...` 全 PASS（feishu/handler/delivery/trigger/server 等包）。

**遗留/延期项**（非阻塞）:
- OAuth E2E 手动验证（需真实飞书应用凭证，留给用户）
- `oauthStateStore` 为内存存储（单实例 Tauri 够用；多实例需换 Redis）
- webhook 未做 event_id 幂等去重（飞书重试可能创建重复 AgentRun，加固项独立任务）
- 飞书文档 markdown→docx 仅基础段落转换（表格/代码块 V1.5 补）
- `harvesting.Pipeline` 未在 wire.go 构造（项目预存模式，所有源适配器共用，非飞书特有）
