# AsyncStarterAgent 代码质量审查报告

> 审查日期：2026-06-21
> 审查范围：全项目 Go 源码（internal/*, cmd/api, pkg/httpx）
> 合格标准：无不合理设计/循环、无无意义代码、无逻辑不通畅、无架构不合理

---

## 总览

| 严重度 | 数量 |
|--------|------|
| Critical | 7 |
| Major | 31 |
| Minor | 30 |

---

## Critical 问题（必须修复）

### C1. PGVectorStore.Upsert 名不副实 — 静默丢失数据

- **文件**: [pgvector.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/pgvector.go#L59-L73)
- **描述**: `Upsert` 方法名暗示"INSERT OR UPDATE"，但实际只执行 `UPDATE ... WHERE external_id = $3`。如果 `external_id` 不存在，UPDATE 影响 0 行但不返回错误。调用方（`RAG.Index`）认为索引成功，数据实际从未入库。
- **修复**: 使用真正的 UPSERT：
  ```sql
  INSERT INTO context_items (external_id, embedding, metadata, ...)
  VALUES ($3, $1, $2, ...)
  ON CONFLICT (external_id) DO UPDATE SET embedding = $1, metadata = $2
  ```
  或至少检查 `RowsAffected()`，影响 0 行时返回错误。

### C2. Service.StreamDraft 未检查 pool 为 nil — 运行时 panic

- **文件**: [service.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/service.go#L68)
- **描述**: `StreamDraft` 直接调用 `s.pool.QueryRow(...)` 无 nil 检查。对比同文件 `GenerateDraft`（第 50 行有 `if s.pool != nil`），`StreamDraft` 在 `pool == nil` 时直接 panic。
- **修复**: 在方法开头添加 `if s.pool == nil { return fmt.Errorf("database not configured") }`

### C3. DDL Scheduler 并发重复处理风险

- **文件**: [ddl_scheduler.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/ddl_scheduler.go#L20-L52)
- **描述**: `run()` 流程为：查询未触发任务 → 调用 handler → 更新 `triggered_at`。多实例部署下，两个实例可能同时查到同一条未触发任务，都调用 handler，导致同一任务被处理两次。`RowsAffected() == 0` 只是事后检测，handler 已被调用两次。
- **修复**: 使用 `UPDATE ... WHERE triggered_at IS NULL RETURNING ...` 实现行级锁，先抢占再执行 handler。

### C4. GitHubAdapter.Fetch 并发不安全 — 修改共享状态

- **文件**: [github.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/github.go#L119)
- **描述**: `Fetch` 方法直接修改结构体字段 `a.cfg.Since = since`。如果 `GitHubAdapter` 被并发调用，会产生数据竞争。即使非并发场景，也永久修改了适配器配置状态。
- **修复**: 不修改 `a.cfg.Since`，将 `since` 作为参数传入 `FetchCommits` 和 `FetchPullRequests`。

### C5. TriggerEvent 事件模型与 Service 层完全脱节

- **文件**: [event.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/event.go#L22-L28) vs [service.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/service.go#L22-L28)
- **描述**: 包内定义了完整的 `TriggerEvent` 标准化事件模型和 `NormalizeTodoist`/`NormalizeFeishu` 标准化函数，但 `Service.ProcessKeyword` 只接受原始 `text string`，完全不接受 `TriggerEvent`。handler 层调用了 `NormalizeTodoist` 但只用了 `ev.EventID`，业务逻辑完全绕过事件模型。`Source`、`EventType`、`Payload`、`OccurredAt` 字段在业务流程中从未被消费。
- **修复**: 重构 Service 层，使其接受 `TriggerEvent` 作为输入（如 `ProcessEvent(ctx, TriggerEvent) (uuid.UUID, error)`），在 Service 内部根据 `Source` 和 `EventType` 分发处理逻辑。

### C6. webhook.go content 为空时返回 matched: true — 逻辑 Bug

- **文件**: [webhook.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/webhook.go#L52-L75)
- **描述**: 当 `content == ""` 时，`if content != ""` 整个分支被跳过，代码直接落入第 75 行返回 `matched: true`，但实际没有进行任何关键词匹配。这会误导调用方认为匹配成功。
- **修复**:
  ```go
  if content == "" {
      httpx.OK(c, gin.H{"received": true, "event_id": ev.EventID, "matched": false})
      return
  }
  ```

### C7. Pipeline.Run 完全没有单元测试 — 核心编排逻辑零覆盖

- **文件**: [pipeline_test.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/pipeline_test.go)
- **描述**: 测试文件中的函数手动调用 `adapter.Fetch → ruleF.Apply → llmF.Apply`，绕过了 `Pipeline.Run` 的编排逻辑（适配器查找、同步时间戳、持久化、更新时间戳）。根因是 `Pipeline` 直接依赖 `*pgxpool.Pool`，无法在无数据库环境下实例化。
- **修复**: 将 `SyncStore` 抽象为接口（`SyncManager`），使 `Pipeline.Run` 可被完整单元测试。

---

## Major 问题（应当修复）

### M1. Recovery 中间件硬编码 500，未走统一响应格式

- **文件**: [recovery.go](file:///k:/go_projects/AsyncStarterAgent/internal/middleware/recovery.go#L14)
- **描述**: panic 后直接 `c.AbortWithStatus(500)` 返回空 body，与项目统一的 `httpx.Fail()` JSON 格式不一致。
- **修复**: 使用 `httpx.Fail(c, http.StatusInternalServerError, 5000, "internal server error")`

### M2. Recovery 不记录堆栈信息

- **文件**: [recovery.go](file:///k:/go_projects/AsyncStarterAgent/internal/middleware/recovery.go#L13)
- **描述**: 捕获 panic 时仅记录 `err`、`Method`、`Path`，没有 `runtime.Stack`，无法定位根因。
- **修复**: 使用 `debug.Stack()` 获取并记录堆栈信息。

### M3. server.go 通过 c.Set 字符串 key 注入配置 — 设计脆弱

- **文件**: [server.go](file:///k:/go_projects/AsyncStarterAgent/internal/server/server.go#L19-L22)
- **描述**: 通过匿名中间件 `c.Set("env", cfg.Env)` 注入配置，仅被 health handler 使用。字符串 key 无类型安全保证，且每个请求都执行无用的 context 赋值。
- **修复**: 直接将 `cfg.Env` 传给 `HealthHandler` 构造函数，删除匿名中间件。

### M4. 业务码定义散落、语义模糊、与 HTTP 状态码混用

- **文件**: [response.go](file:///k:/go_projects/AsyncStarterAgent/pkg/httpx/response.go#L9-L13)
- **描述**: `delivery.go` 使用 HTTP 状态码（400, 500, 503）作为业务码，其他 handler 用自定义码（4001, 4003, 5001, 5002）。没有业务码注册表或枚举定义。
- **修复**: 在 `pkg/httpx` 中定义业务错误码常量，统一管理；修正 `delivery.go` 中的业务码。

### M5. webhook.go 通过字符串比较判断错误类型

- **文件**: [webhook.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/webhook.go#L66)
- **描述**: `err.Error() == noRuleMatchedErr` 与 `trigger/service.go` 的 `fmt.Errorf("no rule matched")` 形成隐式耦合。一旦错误消息文本变化，此处判断静默失效。
- **修复**: 在 `trigger` 包中定义 `var ErrNoRuleMatched = errors.New("no rule matched")`，使用 `errors.Is()` 判断。

### M6. config.Load() 缺少对关键配置的校验

- **文件**: [config.go](file:///k:/go_projects/AsyncStarterAgent/internal/config/config.go#L24-L49)
- **描述**: 仅校验生产环境 `JWT_SECRET`，`DATABASE_URL` 为空时应用启动后所有数据库操作都会失败，但没有明确报错。
- **修复**: 至少对 `DATABASE_URL` 在非开发环境下做非空校验。

### M7. trigger.go 服务层错误一律映射为 400 Bad Request

- **文件**: [trigger.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/trigger.go#L36-L39)
- **描述**: `ProcessKeyword` 的错误被统一返回为 400，但数据库连接失败等属于服务端内部错误，应返回 500。
- **修复**: 区分错误类型，至少区分"业务不匹配"和"内部错误"。

### M8. TriggerHandler 缺少 Svc == nil 防护

- **文件**: [trigger.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/trigger.go#L25-L41)
- **描述**: 其他 handler 都有 nil 检查并返回 503，但 `TriggerHandler.ManualTrigger` 没有，`h.Svc` 为 nil 时直接 panic。
- **修复**: 添加 `if h.Svc == nil` 检查。

### M9. util.go 无意义包装函数

- **文件**: [util.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/util.go#L5-L8)
- **描述**: `jsonUnmarshal` 是 `json.Unmarshal` 的 1:1 包装，`webhook.go` 中的 `bindJSON` 又包装 `jsonUnmarshal`，三层包装零额外逻辑。
- **修复**: 删除 `util.go`，直接使用 `json.Unmarshal`。

### M10. NormalizeTodoist 返回值大部分被浪费

- **文件**: [webhook.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/webhook.go#L41)
- **描述**: 创建了完整的 `TriggerEvent`，但只用了 `ev.EventID`。`ev.Source`、`ev.EventType`、`ev.Payload`、`ev.OccurredAt` 全部被丢弃。handler 还直接从原始 payload 提取 content，绕过了标准化结果。
- **修复**: 要么从 `ev.Payload` 提取 content，要么不调用 `NormalizeTodoist`，直接用 `getStr(payload, "event_id")`。

### M11. AgentRunInput 死代码

- **文件**: [event.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/event.go#L41-L47)
- **描述**: `AgentRunInput` 结构体定义后从未使用。`Service.createRun` 直接构造 `repository.AgentRun`。
- **修复**: 删除 `AgentRunInput`。

### M12. ShouldTrigger 的 lead 参数是死参数

- **文件**: [ddl.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/ddl.go#L23-L24)
- **描述**: `ShouldTrigger(deadline, lead, now)` 接受 `lead` 参数但 `_ = lead` 显式忽略，始终使用 `d.defaultLead`。调用方误以为可自定义 lead time。
- **修复**: 从签名中移除 `lead` 参数。

### M13. AddRule 使用 regexp.MustCompile 会 panic

- **文件**: [matcher.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/matcher.go#L32)
- **描述**: 如果 pattern 无效会直接 panic 导致进程崩溃。如果规则来自配置文件或用户输入，这是安全隐患。
- **修复**: 改为 `regexp.Compile` 并返回 error。

### M14. Service 直接依赖 *pgxpool.Pool — 不可单元测试

- **文件**: [service.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/service.go#L12-L18)
- **描述**: `Service` 直接持有 `*pgxpool.Pool`，无法为 Service 编写不依赖数据库的单元测试。
- **修复**: 定义 `RunRepository` 接口，让 Service 依赖接口。

### M15. NormalizeFeishu 未被使用且无标准化逻辑

- **文件**: [webhook.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/webhook.go#L41-L44)
- **描述**: 整个代码库中没有被调用，且实现只是透传 payload，没有任何标准化处理。
- **修复**: 删除此函数，待 Feishu 集成实现时再添加。

### M16. Pipeline 对 PostgreSQL 硬依赖

- **文件**: [pipeline.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/pipeline.go#L43)
- **描述**: `NewPipeline` 直接接收 `*pgxpool.Pool`，导致 Pipeline 与 PostgreSQL 紧耦合。
- **修复**: 让 `NewPipeline` 接收 `SyncManager` 接口。

### M17. Pipeline.Run 静默吞掉 Upsert 错误

- **文件**: [pipeline.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/pipeline.go#L82-L84)
- **描述**: `UpsertContextItem` 返回错误时仅 `log.Printf` 后继续，调用方无法知道部分数据持久化失败，返回的 `ContextSnapshot` 包含了未入库的条目。
- **修复**: 收集错误并在返回时告知调用方。

### M18. sync.go json.Marshal 错误被静默忽略

- **文件**: [sync.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/sync.go#L53)
- **描述**: `metaJSON, _ := json.Marshal(item.Metadata)` 忽略错误。且 `Metadata` 为 nil 时 `json.Marshal` 返回 `"null"` 而非 `"{}"`，导致数据库存入字符串 `"null"`。
- **修复**: 正确处理错误，对 nil map 做显式检查。

### M19. sourceAdapter 未导出接口，子包无法显式引用

- **文件**: [pipeline.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/pipeline.go#L14)
- **描述**: `sourceAdapter` 是小写开头，所有实现在 `source` 子包中，无法做编译期接口检查，只能在测试中重新构造接口字面量。
- **修复**: 导出为 `SourceAdapter`。

### M20. 各 Source Adapter 重复的 UserID/Source/Type 赋值逻辑

- **文件**: [calendar.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/calendar.go#L31-L36), [feishu.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/feishu.go#L37-L43), [github.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/github.go#L129-L131), [obsidian.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/obsidian.go#L111-L115)
- **描述**: 每个适配器都有几乎相同的遍历 items 设置 `UserID`、`Source`、默认 `Type` 的逻辑，违反 DRY 原则。
- **修复**: 提取公共辅助函数 `applyDefaults(items, userID, source, defaultType)`。

### M21. GitHubAdapter 使用 context.Background() 创建 HTTP 客户端

- **文件**: [github.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/github.go#L26-L28)
- **描述**: `oauth2.NewClient(context.Background(), ts)` 导致底层 HTTP 传输无法被取消或设置超时。
- **修复**: 接受 `context.Context` 参数。

### M22. FetchPullRequests 与 FetchCommits 增量同步策略不一致

- **文件**: [github.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/github.go#L32-L112)
- **描述**: `FetchCommits` 使用 API 端 `Since` 参数过滤，`FetchPullRequests` 拉取全部后在客户端过滤，效率低且可能遗漏。
- **修复**: 统一增量同步策略，优先使用 API 端过滤。

### M23. LLM 响应解析未处理 Markdown 代码围栏

- **文件**: [llm.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/filter/llm.go#L106-L114)
- **描述**: `ParseResponse` 直接 `json.Unmarshal`，但 LLM 经常在 JSON 外包裹 Markdown 代码围栏，导致解析失败，过滤器完全失效。
- **修复**: 解析前剥离 Markdown 代码围栏。

### M24. EinoClassifier.IsNoise 格式化字符串注入风险

- **文件**: [llm.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/filter/llm.go#L86)
- **描述**: `fmt.Sprintf(c.promptTmpl, content)` 如果 `content` 包含 `%` 字符会导致格式化错误或 panic。
- **修复**: 使用 `text/template` 或 `strings.ReplaceAll`。

### M25. delivery.Service 依赖具体类型而非接口 — 不可测试

- **文件**: [service.go](file:///k:/go_projects/AsyncStarterAgent/internal/delivery/service.go#L21-L26)
- **描述**: `notion` 和 `obs` 字段分别是 `*NotionAdapter` 和 `*ObsidianAdapter` 具体类型，无法 mock。讽刺的是 `obsidian.go` 中已定义了 `ObsidianWriter` 接口但未被使用。
- **修复**: 定义 `NotionPageCreator` 和 `ObsidianWriter` 接口，让 Service 依赖接口。

### M26. Service.llm 字段从未使用 — 死代码

- **文件**: [service.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/service.go#L28)
- **描述**: `Service` 持有 `llm LLMClient` 字段，但 `GenerateDraft` 和 `StreamDraft` 均未使用。LLM 调用已封装在 `DraftGenerator` 内部。
- **修复**: 从结构体和 `NewService` 参数中移除。

### M27. scoredItemToContextItem 伪造时间戳

- **文件**: [workflow.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/agent/workflow.go#L168)
- **描述**: `OccurredAt: time.Now()` 并非事件实际发生时间，向下游传递虚假时间信息。
- **修复**: 在 `ScoredItem` 中添加时间字段或设为零值。

### M28. State.Errors 和 AppendError 是死代码

- **文件**: [dag.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/agent/dag.go#L27)
- **描述**: 定义了但整个 DAG 执行流程中没有任何节点调用 `AppendError`，"部分失败继续执行"的设计意图未实现。
- **修复**: 实际使用或移除。

### M29. RAG.Index 不设置 Source 字段 — 数据不完整

- **文件**: [rag.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/rag.go#L76-L86)
- **描述**: 创建 `ScoredItem` 时遗漏了 `Source` 字段，导致检索结果来源信息始终为空。
- **修复**: 在 `Index` 方法签名中添加 `source` 参数。

### M30. Deliver 方法同时返回非 nil 结果和错误

- **文件**: [service.go](file:///k:/go_projects/AsyncStarterAgent/internal/delivery/service.go#L86-L93)
- **描述**: 交付失败时同时返回 `DeliverResult{Status: "failed"}` 和 `error`，违反 Go 惯例。
- **修复**: 失败时只返回 error，不返回 DeliverResult。

### M31. sync_test.go 中的测试是空壳

- **文件**: [sync_test.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/sync_test.go#L17-L18)
- **描述**: 三个测试函数在缺少 `DATABASE_URL` 时直接跳过，且 `_ = context.Background()` 和 `_ = time.Now()` 是无意义死代码。
- **修复**: 通过抽象接口编写真正的单元测试，或删除空壳测试。

---

## Minor 问题（建议修复）

| # | 文件 | 行号 | 描述 |
|---|------|------|------|
| m1 | [logger.go](file:///k:/go_projects/AsyncStarterAgent/internal/middleware/logger.go#L4) | 4,14 | 使用标准库 log，缺乏结构化日志能力 |
| m2 | [logger.go](file:///k:/go_projects/AsyncStarterAgent/internal/middleware/logger.go#L10) | 10-15 | 未记录 request ID，无法关联请求链路 |
| m3 | [response.go](file:///k:/go_projects/AsyncStarterAgent/pkg/httpx/response.go#L12) | 12,15,19 | 使用 `interface{}` 而非 `any`，风格过时 |
| m4 | [response.go](file:///k:/go_projects/AsyncStarterAgent/pkg/httpx/response.go#L19) | 19 | `Fail` 函数 httpStatus/code 两个 int 参数易混淆 |
| m5 | [server.go](file:///k:/go_projects/AsyncStarterAgent/internal/server/server.go#L13) | 13 | `New()` 函数签名接收具体类型，扩展性差 |
| m6 | [health.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/health.go#L8) | 8-11 | `HealthResponse` 不必要地导出 |
| m7 | [delivery.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/delivery.go#L42) | 42 | 服务层错误一律返回 500，缺少错误分类 |
| m8 | [webhook.go](file:///k:/go_projects/AsyncStarterAgent/internal/handler/webhook.go#L25) | 25-76 | handler 职责过重，混合协议解析、业务逻辑和错误处理 |
| m9 | [matcher.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/matcher.go#L20) | 20 | `compiledRule.raw` 字段存储后从未读取 |
| m10 | [event.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/event.go#L15) | 15-16 | `SourceNotion` 和 `SourceManual` 常量未被使用 |
| m11 | [webhook.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/webhook.go#L9) | 9-14 | `SignHMAC` 是导出的测试辅助函数，不应在生产代码中 |
| m12 | [webhook.go](file:///k:/go_projects/AsyncStarterAgent/internal/trigger/webhook.go#L29) | 29-38 | `NormalizeTodoist` 浅拷贝 event_data，存在并发风险 |
| m13 | [pipeline.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/pipeline.go#L108) | 108-116 | `findAdapter` 返回冗余的第二个值 |
| m14 | [github.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/github.go#L18) | 18 | `GitHubConfig.Since` 字段冗余，被 Fetch 参数覆盖 |
| m15 | [google_calendar.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/google_calendar.go#L63) | 63,70 | 时间解析失败时静默回退到 time.Now() |
| m16 | [llm.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/filter/llm.go#L36) | 36 | `IsNoise` 返回的 reason 被显式丢弃 |
| m17 | [feishu.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/feishu.go#L123) | 123 | `convertMessage` 硬编码 Source 为 "feishu" |
| m18 | [rules.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/filter/rules.go#L28) | 28-35 | `DefaultRules()` 每次调用重新编译正则 |
| m19 | [rules.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/filter/rules.go#L53) | 53-56 | "empty" 规则硬编码，与其他规则不一致 |
| m20 | [obsidian.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/obsidian.go#L103) | 103 | 无文件大小限制，大文件可能导致内存暴涨 |
| m21 | [obsidian.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/source/obsidian.go#L112) | 112 | ID 包含完整文件路径，暴露服务器结构 |
| m22 | [context.go](file:///k:/go_projects/AsyncStarterAgent/internal/harvesting/context.go#L10) | 10-12 | `NoiseClassifier` 接口定义位置不当，应由消费者定义 |
| m23 | [workflow.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/agent/workflow.go#L19) | 19,54 | `Workflow.templateDir` 存储后从未使用 |
| m24 | [service.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/service.go#L99) | 99-113 | `marshalMarks` 创建与 `Mark` 完全相同的冗余类型 |
| m25 | [workflow.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/agent/workflow.go#L83) | 83,90,117 | 硬编码魔法数字（topK=20, Completeness=0.3/0.5, markPenalty=20） |
| m26 | [db.go](file:///k:/go_projects/AsyncStarterAgent/internal/repository/db.go#L20) | 20 | 连接池 MaxConns 硬编码为 20 |
| m27 | [queue.go](file:///k:/go_projects/AsyncStarterAgent/internal/queue/queue.go#L48) | 48 | 重试次数 MaxRetry 硬编码为 3 |
| m28 | [obsidian.go](file:///k:/go_projects/AsyncStarterAgent/internal/delivery/obsidian.go#L42) | 42 | `WriteFile` 忽略 context，不支持取消 |
| m29 | [notion.go](file:///k:/go_projects/AsyncStarterAgent/internal/delivery/notion.go#L29) | 29 | HTTP 客户端无超时配置 |
| m30 | [pgvector.go](file:///k:/go_projects/AsyncStarterAgent/internal/synthesis/pgvector.go#L50) | 50 | `Search` 静默忽略 json.Unmarshal 错误 |

---

## 架构级问题总结

### 1. 数据库硬依赖贯穿全栈

`trigger.Service`、`harvesting.Pipeline`、`delivery.Service`、`synthesis.Service` 均直接持有 `*pgxpool.Pool`，导致：
- 无法编写不依赖数据库的单元测试
- 违反依赖倒置原则
- 与 PostgreSQL 紧耦合

**统一修复方向**: 为每个 Service 定义 Repository 接口，通过依赖注入传入。

### 2. 事件模型形同虚设

`TriggerEvent` 定义了完整的事件标准化体系（Source、EventType、Payload、OccurredAt），但 Service 层只接受原始字符串，导致：
- 事件标准化做了无用功
- Source 常量无法参与路由
- 未来扩展新 Source 时缺乏框架支撑

**统一修复方向**: 重构 Service 层接受 `TriggerEvent`，按 Source 分发处理逻辑。

### 3. 错误处理缺乏体系

- 无 sentinel error 定义，依赖字符串比较
- 业务码散落各处，无统一规范
- 多处静默忽略错误（json.Marshal、Upsert、Unmarshal）
- handler 层错误映射粗暴（一律 400 或一律 500）

**统一修复方向**: 定义项目级错误码规范和 sentinel error 体系。

### 4. 测试覆盖严重不足

- `Pipeline.Run`、`trigger.Service`、`delivery.Service` 核心逻辑零单元测试
- 多个测试是空壳（skip on no DB）
- `draft_test.go` 测试的不是 handler 层逻辑
- 测试基础设施重复（mock embedder、vector store 等）

**统一修复方向**: 引入接口抽象 → 编写 mock → 补充单元测试。

---

## 优先修复建议

**第一优先级（数据安全 + 运行时稳定性）**:
1. C1: PGVectorStore.Upsert 假 upsert → 数据静默丢失
2. C6: webhook content 为空返回 matched: true → 逻辑 Bug
3. C2: StreamDraft nil panic → 运行时崩溃
4. C4: GitHubAdapter 并发不安全 → 数据竞争

**第二优先级（架构改进）**:
5. C5 + M5: 事件模型脱节 + 字符串错误比较 → 架构级重构
6. C3: DDL Scheduler 并发重复处理 → 多实例部署风险
7. C7 + M14 + M16 + M25: 数据库硬依赖 → 不可测试

**第三优先级（代码质量）**:
8. M1 + M2: Recovery 中间件补齐
9. M4: 业务码统一
10. M20: 重复代码提取
