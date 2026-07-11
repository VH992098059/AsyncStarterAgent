# AsyncStarterAgent 代码审查报告 (2026-07-10)

审查范围：`cmd/`、`internal/`、`pkg/`、`migrations/`、`go.mod`、`Dockerfile`、`docker-compose.yml`（未深入 `doc/`、`docs/`）。

本次为只读代码审查，未做任何代码改动。以下按严重程度从高到低列出发现，均基于实际读取的代码文件。

---

## 高危 / Build-breaking

### 1. ✅ Dockerfile Go 版本与 go.mod 不匹配，构建可能失败
- **文件**: `Dockerfile:2`，对照 `go.mod:3`
- **问题**: `go.mod` 声明 `go 1.26.3`，但 `Dockerfile` 用 `FROM golang:1.24-alpine AS build`。Go 1.21+ 的 toolchain 机制在发现 go.mod 要求的版本高于本地工具链时，会尝试自动下载对应版本的 go 工具链（需联网），如果构建环境网络受限或镜像被固定为离线构建，`go build` 会直接失败。
- **影响**: CI/CD 或本地 `docker build` 可能在无网络环境下直接失败，属于可直接复现的构建阻断问题。
- **建议**: 把基础镜像升级到 `golang:1.26-alpine`（或匹配 go.mod 的具体版本），并在 CI 中加一步 `go build ./...` 做纯净环境验证。

### 2. ✅ Todoist webhook 用 `uuid.Nil` 作为所有触发的 AgentRun 属主，且"未匹配"判断逻辑已失效（死代码路径）
- **文件**: `internal/handler/webhook.go:29`, `:69-78`；对照 `internal/trigger/service.go` 的 `ProcessKeyword`
- **问题**:
  1. `ProcessKeyword` 现在的实现是「没匹配到关键词就回退成 `taskType = "message"`」，**不再返回 `"no rule matched"` 错误**。但 `webhook.go:29` 定义的 `noRuleMatchedErr` 常量、以及 `:72` 处 `if err.Error() == noRuleMatchedErr` 分支据此判断"未匹配"，这条分支现在永远不会命中——**任何非空 content 的 Todoist webhook 都会创建 AgentRun**，即使完全不匹配任何关键词规则。这与代码注释里描述的"不匹配返回 `matched:false`"的设计意图不一致。
  2. `webhook.go:69` 硬编码 `uid := uuid.Nil` 作为 run 的 owner。由于 `GetAgentRunByID` 对 `uuid.Nil` 有特殊放行逻辑（`repository/agent_run.go:52` `if userID != uuid.Nil`），这些 run 实质上"无主"，前端 `/api/v1/agent-runs` 等按登录用户过滤的列表接口永远看不到它们，造成数据库里堆积孤儿记录，且无法被清理/管理。
- **影响**: 数据完整性问题（垂钓/垃圾 Todoist 事件会不断产生无主 AgentRun，占用存储、污染统计），且实际行为与代码注释描述的设计不符，容易误导后续维护者。
- **建议**: 在 `trigger` 包中定义 `var ErrNoRuleMatched = errors.New(...)`，`ProcessKeyword` 匹配失败时返回该 sentinel error 而不是静默回退成 `"message"` 类型（若"回退成 message"是既定业务决策，则删除 webhook.go 中已经死掉的分支和常量，避免误导）；`uuid.Nil` 属主问题需要接入真实的 user 身份映射，或至少标记这些 run 为"unowned"并在清理任务中处理。

### 3. ✅ GitHub adapter 增量同步只拉第一页，可能静默丢失数据
- **文件**: `internal/harvesting/source/github.go:32-71`（`FetchCommits`）、`:74-112`（`FetchPullRequests`）
- **问题**: `FetchCommits` 用 `ListOptions{PerPage: 50}`，`FetchPullRequests` 用 `PerPage: 30`，两者都只请求了 GitHub API 的第一页，没有分页循环直到遇到早于 `since` 的记录。如果某个仓库在同步窗口内的 commit/PR 数量超过 50/30，超出第一页的记录会被直接漏掉，且**没有任何错误或日志提示**，调用方无法感知数据不完整。
- **影响**: 高活跃度仓库的增量同步会静默丢数据，后续生成的周报/总结会缺内容，且此问题不会在日志或返回值中体现，排查成本高。
- **建议**: 用 `go-github` 的 `Response.NextPage` 做分页循环，直到 `NextPage == 0` 或遇到早于 `since` 的记录再提前退出。

### 4. ✅ `DB_ENCRYPTION_KEY` 未在启动时校验，飞书功能可静默失败
- **文件**: `internal/config/config.go:66-70`（读取 `FeishuAppID`/`FeishuAppSecret`/`DBEncryptionKey`，无联合校验）；`internal/feishu/token_store.go:44-46,67-69`（`Save`/`Get` 内部检查 `encKey == ""` 才报错）
- **问题**: `cmd/api/wire.go:85` 只检查 `cfg.FeishuAppID != "" && cfg.FeishuAppSecret != ""` 就初始化飞书 client factory 和 auth handler，完全没检查 `DB_ENCRYPTION_KEY` 是否配置。如果运维人员配置了飞书 App ID/Secret 但忘记配 `DB_ENCRYPTION_KEY`，服务会正常启动、飞书授权路由也会正常注册，但**第一个用户走 OAuth 回调时才会报错**（`token_store.go` 的 `fmt.Errorf("feishu token store: DB_ENCRYPTION_KEY is empty")`）。
- **影响**: 配置错误无法在启动阶段被发现，只能等到用户实际点击"连接飞书"才报错，排查成本高，且用户体验差（授权流程走到最后一步才失败）。
- **建议**: 在 `config.Load()` 里加一条：`if FeishuAppID != "" && FeishuAppSecret != "" && DBEncryptionKey == "" { return error }`，Fail Fast。

---

## 中危

### 5. ✅ 飞书 token 刷新的"等待中"分支存在竞态窗口
- **文件**: `internal/feishu/client_factory.go:87-131`（`refreshToken`）
- **问题**: 当检测到同一用户已有并发刷新在进行时（`f.refresh[userID]` 存在），代码固定 `sleep 200ms` 后重读一次 store（`:96`），并没有真正等待刷新协程完成（无 channel/条件变量同步）。如果刷新耗时超过 200ms（网络抖动、飞书接口慢），等待方会读到仍然过期的旧 token，直接把过期 token 返给调用方，导致下游请求收到 401。代码注释里已经承认这是已知限制（"V1.5 可改为 condition variable / channel 等待刷新完成"），但目前仍是生产环境的真实风险点。
- **影响**: 高并发场景下（同一用户多个请求同时触发 token 过期）会有一定概率返回过期 token，导致下游飞书 API 调用失败。
- **建议**: 用 `sync.WaitGroup` 或每个 userID 一个 `chan struct{}` 来真正阻塞等待刷新完成，而不是固定时间 sleep 后猜测。

### 6. ✅ `Revoke` 与并发 `refreshToken` 之间的竞态可能"复活"已撤销的授权
- **文件**: `internal/feishu/client_factory.go:141-145`（`Revoke`）
- **问题**: 若用户点击"撤销授权"的同时有一个正在进行的 `refreshToken` 调用，`Revoke` 删除记录后，`refreshToken` 完成时仍会调用 `f.store.Save(...)` 把新 token 写回，相当于撤销被悄悄撤销。代码注释同样承认此为已知限制。
- **影响**: 用户认为已撤销授权，但系统仍持有并可能继续使用其飞书 token，属于权限/隐私层面的隐患。
- **建议**: `Revoke` 时同时置一个"撤销标记"或用 `context.CancelFunc` 取消正在进行的 in-flight refresh，或在 `Save` 前二次检查记录是否已被删除（用事务/版本号做 CAS）。

### 7. ✅ 飞书 webhook 无事件幂等去重，重试会产生重复 AgentRun
- **文件**: `internal/handler/webhook.go:142-143`（注释已自述："MVP 限制：未做 event_id 幂等去重，飞书重试可能创建重复 AgentRun"）
- **问题**: 飞书事件订阅在网络异常/服务响应慢时会自动重试推送相同事件，当前代码对 `header.event_id` 没有做任何去重记录，每次收到都会创建新的 AgentRun。
- **影响**: 用户可能看到同一个任务生成多份重复草稿，浪费 LLM 调用配额，体验混乱。
- **建议**: 增加一张短期去重表（`event_id` + TTL），或直接对 `agent_runs.trigger_source` 加唯一索引（如 `feishu:task:<guid>` 类型的 source 可以做 `(trigger_type, trigger_source)` 唯一约束防重复创建）。

### 8. ✅ 登录接口无限流/失败次数限制
- **文件**: `internal/handler/auth.go:75-101`（`Login`）
- **问题**: `Login` 只做用户名密码校验（bcrypt），没有失败次数计数、锁定或 IP 限流。虽然错误信息统一为"用户名或密码错误"（防止用户名枚举，这点做得对），但没有任何机制阻止暴力破解密码。
- **影响**: 弱密码账户存在被暴力破解的风险，尤其该服务面向公网（`AllowOriginFunc` 里包含 tauri 客户端场景，说明是面向真实用户使用的产品）。
- **建议**: 在网关层或应用层加基于 IP/用户名的限流（如 golang.org/x/time/rate 或 Redis 计数器），超过阈值临时锁定或要求验证码。

### 9. ✅ JWT 黑名单纯内存实现，重启丢失且无法多实例共享
- **文件**: `internal/auth/jwt.go:78-119`（`Blacklist`）
- **问题**: 代码注释已自述"MVP 单实例内存足够；跨实例部署后续替换为 Redis"。当前实现是 `map[string]time.Time` + mutex，服务重启后所有已登出但未过期的 token 会重新变为有效，多实例部署时黑名单也无法共享（在实例 A 登出的 token 在实例 B 仍可用）。
- **影响**: 只要服务水平扩展到多实例，或者服务重启，"登出"功能就会实际失效，属于安全功能失效而非仅性能问题。
- **建议**: 若近期有多实例部署计划，需要提前把黑名单迁到 Redis（已经引入 Redis 依赖，可复用）。

### 10. ✅ `UpdateDraft` 等接口对请求体大小无限制，存在资源耗尽风险
- **文件**: `internal/handler/draft_detail.go:93-132`（`UpdateDraft`），`c.ShouldBindJSON(&req)` 直接绑定 `Markdown string`
- **问题**: Gin 默认不对 JSON body 大小做限制（除非显式设置 `http.MaxBytesReader` 或类似中间件），当前路由链（`server.go`）也没有全局的 body size 限制中间件。已登录用户可以提交任意大小的 `markdown` 字段（比如几百 MB 的字符串），直接进入数据库写入流程。
- **影响**: 恶意或误操作的巨大请求体会占用大量内存/DB 带宽，可能造成服务资源耗尽（DoS 风险），虽然需要先登录（有一定门槛）。
- **建议**: 在 `server.go` 的全局中间件里加一个 `http.MaxBytesReader` 包装（如限制 1-5MB），拒绝过大请求体。

### 11. ✅ asynq 队列基础设施已完整搭建但似乎未被业务流程实际使用
- **文件**: `internal/queue/queue.go`、`internal/queue/handler.go`；对照 `cmd/api/wire.go` 全文
- **问题**: 项目引入了完整的 `asynq` 生产者（`queue.NewClient`）、消费者（`queue.NewServer`/`Mux`）基础设施，但通读 `wire.go` 和 `handler` 层代码，`Enqueue` 方法**只在 `queue.go` 自身和集成测试里出现**，没有在任何 handler/service 的业务路径中被调用；`queue.NewServer`（worker 端）在 `cmd/` 下也没有任何入口去启动它。当前的草稿生成/交付流程实际是同步处理（HTTP 请求内直接跑 workflow），并非通过队列异步化。
- **影响**: 这是一套已经写好但未接入主流程的基础设施——增加了依赖面（redis、asynq 相关的多个间接依赖）和维护成本，却没有产生实际价值，容易让后来者误以为系统是异步处理的（实际是同步阻塞在 HTTP handler 里跑完整 LLM workflow，`DraftStreamHandler.Stream` 就是个例子）。
- **建议**: 明确这套队列基础设施的去留——如果计划后续把耗时的 harvesting/synthesis 任务迁移到异步 worker（更符合"AsyncStarterAgent"这个项目名的定位），应尽快把 `queue.Enqueue` 接入实际触发路径；如果暂无计划，考虑先移除未使用的 worker 端代码，减少认知负担。

### 12. ✅ Harvesting pipeline 对每条 context item 单独执行一次 DB upsert（N+1）
- **文件**: `internal/harvesting/pipeline.go:80-85`（`Run` 方法中的 `for _, it := range kept2 { p.sync.UpsertContextItem(...) }`）
- **问题**: 循环内逐条调用 `UpsertContextItem`，每条都是一次独立的 `pool.Exec`。如果一次同步拉取到几百上千条 context item（比如活跃 GitHub 仓库或大量飞书任务），会产生等量的数据库往返。
- **影响**: 同步耗时随条目数线性增长且伴随大量小事务开销，在数据量变大后会成为明显的性能瓶颈。
- **建议**: 用 `pgx.Batch`（批量 pipeline 发送）或多值 `INSERT ... VALUES (...),(...),... ON CONFLICT DO NOTHING` 一次性写入。

### 13. ⏭️ 依赖版本管理：存在指向未发布 commit 的伪版本号（已确认跳过）
- **文件**: `go.mod:7`
- **问题**: `github.com/cloudwego/eino-ext/components/embedding/openai v0.0.0-20260616080858-ab17b7308bf8` 是一个指向具体 commit 的伪版本（pseudo-version），而不是打了 tag 的正式 release，其余大多数依赖（包括同仓库的 `model/openai v0.1.13`）都用了正式版本号。
- **影响**: 伪版本意味着该依赖代码未经过正式发布流程的审核/测试，供应链风险略高于打 tag 的版本，且后续如果上游历史被重写（force push）该 commit 有理论上不可复现的风险（不过 Go module proxy 一般会缓存，实际风险较低但仍值得关注）。
- **建议**: 待上游发布正式 tag 后尽快切换，或在依赖审查流程中标记此项定期复查。
- **处理结果**: 已确认上游该子模块 (`embedding/openai`) 尚无正式 tag（仅 `model/openai`、`libs/acl/openai` 等其他子模块有 tag），当前无法切换到正式版本号，经用户确认跳过，待上游发布 tag 后再处理。

---

## 低危 / 可维护性

### 14. `extractPageID` 用手写字符串解析提取 Notion page ID，脆弱
- **文件**: `internal/delivery/service.go:206-223`
- **问题**: 该函数用两次反向 `for` 循环手工解析 URL（先按 `/` 分割取最后一段，再按 `-` 分割取最后一段）来提取 Notion page ID，隐含假设 Notion 返回的 URL 格式固定为 `.../{title}-{32位hex}`。一旦 Notion 更换 URL 格式，此函数会静默返回错误的 ID 而不报错。
- **建议**: 优先直接使用 `NotionAdapter.CreatePage` 返回时 API 响应里的原始 `result.ID` 字段（该字段在 `notion.go:81` 已经拿到），而不是从拼好的 URL 反向解析；如果一定要从 URL 解析，至少加上格式校验（如长度/hex 校验），解析失败时返回 error 而不是啞默降级。

### 15. `parseInt` 用不相关的 `http.ErrBodyNotAllowed` 作为通用错误哨兵
- **文件**: `internal/handler/lists.go:163-173`
- **问题**: 自定义的 `parseInt` 小工具函数在遇到非数字字符时返回 `http.ErrBodyNotAllowed`，这是 net/http 包里专门表示"不允许读取 body"的错误，与"字符串不是合法数字"语义完全不相关，容易误导以后读代码或做 `errors.Is` 判断的人。
- **建议**: 用 `strconv.Atoi` 替换整个手写实现（标准库已经做好了这件事），或者至少定义一个本地的 `ErrInvalidInt` sentinel。

### 16. 已标记 `Deprecated` 的 `SSEWriter` 仍留在代码库中
- **文件**: `internal/synthesis/sse.go`（整个文件标注 `Deprecated: use A2UIWriter instead`）
- **问题**: 确认已有替代实现（`A2UIWriter`）后，`SSEWriter` 应该已经没有生产调用点（值得用 grep 确认一遍所有引用后清理）。
- **建议**: 确认无引用后删除该文件及对应测试，减少维护面。

### 17. CORS 允许的 Origin 列表硬编码在代码里
- **文件**: `internal/server/server.go:19-25`
- **问题**: `allowedOrigins` 是写死在源码里的一个 slice，包含开发环境地址（`localhost:1420`、`localhost:5173`）和 Tauri 客户端地址，没有通过环境变量注入生产域名的能力。
- **建议**: 把这个列表改为从 `config.Config` 读取（逗号分隔的环境变量），保留当前硬编码值做为开发环境默认值，避免每次上生产/新增前端域名都要改代码重新编译。

### 18. 数据库连接池大小硬编码
- **文件**: `internal/repository/db.go:33`（`cfg.MaxConns = 20`）
- **问题**: 连接池上限硬编码为 20，不同部署环境（数据库规格、并发量）可能需要不同的池大小，目前只能改代码重新编译才能调整。
- **建议**: 从环境变量读取（比如 `DB_MAX_CONNS`），给个合理默认值即可。

---

## 未发现问题的方面（值得一提）

- 所有 SQL 查询都使用了参数化占位符（`$1, $2...`），没有发现字符串拼接 SQL 导致注入的情况；唯一一处动态拼接 SQL（`repository/db.go:81` 的 `CREATE DATABASE`）也正确使用了 `pgxstd.Identifier{}.Sanitize()` 做标识符转义。
- Todoist webhook 签名校验用了 `hmac.Equal`（恒定时间比较），做法正确（`internal/trigger/webhook.go:17-26`）。
- `.env` 文件已被 `.gitignore` 排除，`git ls-files` 确认仓库里只有 `.env.example`，没有真实密钥泄露到版本库。
- 密码存储用 bcrypt，且 `NewService` 对 cost 值做了下限校验，登录/注册错误信息统一处理避免用户名枚举，处理得比较规范。
- 优雅退出（`cmd/api/main.go`）处理了 SIGINT/SIGTERM，先停 HTTP 再 cancel context 停 DDL scheduler，顺序和注释都清晰，是本次审查中写得比较扎实的部分。

---

## 备注

本报告为只读审查产出，未对代码做任何改动。如需处理任意一项，按 `doc/ai-coding-boundary.md` 的红线规则，涉及设计决策的条目（如第 9 项黑名单迁移方案、第 11 项队列去留）需先确认方向再动手，不能直接改。
