# Loop Engineering 集成设计方案 — AsyncStarterAgent

> **文档版本**: v0.1.0
> **创建日期**: 2026-06-29
> **状态**: 设计确认，MVP 阶段不实现代码，仅预留扩展点

---

## 0. 设计目标

将 Loop Engineering 理念融入 AsyncStarterAgent 项目，使 Agent 从「单次触发、线性流水线」升级为「持续运行、自我修正、不断学习的闭环系统」。

**核心原则**：
- MVP 阶段保持线性流水线，仅预留扩展点，零运行时成本
- V1.5 阶段逐步完整实现 Loop Engineering 五大能力
- 基于 Eino 框架原生能力（Aspect / Graph / State），不重复造轮子

---

## 1. 架构总览

### 1.1 四层扩展架构

```
┌─────────────────────────────────────────┐
│  Loop Engine (V1.5c/d/e)               │
│  主动发现 / 跨任务记忆 / 子Agent调度   │
├─────────────────────────────────────────┤
│  Graph-Level Middleware (V1.5a/b)      │
│  全局验证 / 全局重试 / 全局记忆注入    │
├─────────────────────────────────────────┤
│  Node-Level Middleware (MVP 预留)      │
│  节点级验证 / 节点级钩子 / 质量检查     │
├─────────────────────────────────────────┤
│  Eino 原生 DAG + Aspect 机制         │
│  现有四阶段流水线                       │
└─────────────────────────────────────────┘
```

### 1.2 与现有系统的关系

- 现有 Eino Workflow（四阶段线性流水线）保持不变
- Middleware 层在节点外层包装，不侵入节点内部逻辑
- Loop Engine 是独立调度器，在 Workflow 外层运行

---

## 2. 核心概念定义

### 2.1 Loop Engineering 五大能力（V1.5 完整实现）

| 编号 | 能力 | 说明 | 实现阶段 |
| --- | --- | --- | --- |
| A | 草稿自验证与修正循环 | 生成后自动检查质量，不达标时自动补充上下文重试 | V1.5a |
| B | 跨任务记忆与学习 | 记住用户偏好和修改模式，后续生成自动应用 | V1.5b |
| C | 主动发现与持续巡检 | 定期扫描环境，主动发现需要生成文档的场景 | V1.5c |
| D | 子 Agent 分工协作 | 多子 Agent 并行执行，结果汇总 | V1.5d |
| E | 用户反馈闭环 | 从用户修改中学习，越用越准 | V1.5e |

### 2.2 中间件体系（两层）

**全局工作流中间件（Graph Middleware）**：
- 作用于整个工作流的执行前后
- 用途：全局日志、全局记忆注入、全局错误处理
- 执行顺序：按注册顺序 Pre，逆序 Post

**节点级中间件（Node Middleware）**：
- 作用于单个节点的执行前后
- 用途：节点验证、节点重试、节点级日志
- 执行顺序：按注册顺序 Pre，逆序 Post

---

## 3. MVP 阶段扩展点（零成本预留）

### 3.1 代码扩展点

#### 3.1.1 新增文件

| 文件路径 | 用途 | MVP 状态 |
| --- | --- | --- |
| `internal/workflow/loop_extension.go` | Loop 扩展接口定义、类型定义 | 定义类型和空实现 |

#### 3.1.2 类型定义

```go
// NodeValidationResult 节点验证结果
type NodeValidationResult struct {
    Passed    bool
    Score     float64    // 0-100
    Issues    []string   // 问题列表
    Retryable bool       // 是否可通过重试改进
}

// LoopContext Loop 运行上下文
type LoopContext struct {
    Iteration      int                    // 当前迭代次数
    MaxIterations  int                    // 最大迭代次数
    Validation     *NodeValidationResult  // 最近一次验证结果
    Memory         map[string]any         // 跨迭代记忆
    UserPrefs      map[string]any         // 用户偏好（从记忆系统加载）
}

// GraphMiddleware 全局图中间件
type GraphMiddleware func(next GraphHandler) GraphHandler
type GraphHandler func(ctx context.Context, state workflowState) (workflowState, error)

// NodeMiddleware 节点级中间件
type NodeMiddleware func(next NodeHandler) NodeHandler
type NodeHandler func(ctx context.Context, state workflowState) (workflowState, error)
```

#### 3.1.3 workflowState 扩展

在现有 `workflowState` 中增加 Loop 相关字段：

```go
type workflowState struct {
    // --- 现有字段 ---
    Trigger    triggerPayload
    Context    *contextPayload
    Outline    *outlinePayload
    Draft      *draftPayload
    Delivery   *deliveryPayload

    // --- Loop 扩展字段（MVP 阶段为零值，V1.5 启用）---
    LoopCtx *LoopContext
}
```

#### 3.1.4 中间件注册与应用

```go
// 全局中间件列表（MVP 阶段为空切片）
var graphMiddlewares = []GraphMiddleware{}

// 每个节点的中间件映射（MVP 阶段为空 map）
var nodeMiddlewares = map[string][]NodeMiddleware{}

func applyNodeMiddleware(key string, fn nodeFunc) nodeFunc {
    middles, ok := nodeMiddlewares[key]
    if !ok || len(middles) == 0 {
        return fn // 无中间件，直接返回原函数，零成本
    }
    // 从右往左包装
    wrapped := fn
    for i := len(middles) - 1; i >= 0; i-- {
        wrapped = middles[i](wrapped)
    }
    return wrapped
}
```

**MVP 运行时成本**：
- `graphMiddlewares` 为空切片 → 不执行任何额外逻辑
- `nodeMiddlewares` 为空 map → `applyNodeMiddleware` 直接返回原函数
- `LoopCtx` 为 nil → 不占用额外内存（指针零值）

### 3.2 数据库扩展预留

在 `drafts` 表预留字段（MVP 阶段不使用，但 schema 中预留）：

| 字段名 | 类型 | 默认值 | 用途 | MVP 状态 |
| --- | --- | --- | --- | --- |
| `iteration_count` | INT | 1 | 迭代次数 | 总是 1 |
| `quality_score` | FLOAT | NULL | 质量评分 | NULL |
| `validation_issues` | TEXT | NULL | 验证问题列表（JSON） | NULL |

### 3.3 UI 扩展预留

#### 3.3.1 看板列预留

- 「巡检发现」列：在「待触发」列左侧，MVP 阶段 `display: none`
- V1.5c 接入 LoopScheduler 的发现结果后显示

#### 3.3.2 任务卡片预留字段

MVP 阶段以下字段数据为 nil，UI 不渲染：
- `iteration_count`：迭代次数标识
- `quality_score`：质量评分
- `validation_issues`：验证问题列表

#### 3.3.3 对话区预留折叠面板

| 预留区域 | 位置 | MVP 状态 | V1.5 用途 |
| --- | --- | --- | --- |
| 验证结果面板 | 对话区底部 | 折叠隐藏 | V1.5a 显示草稿质量评分、问题列表 |
| 记忆面板 | 对话区右侧边栏 | 折叠隐藏 | V1.5b 显示用户偏好、历史修改模式 |
| 循环控制 | 对话区顶部操作栏 | 隐藏 | V1.5a 暂停循环、调整最大迭代数 |

#### 3.3.4 侧边栏预留入口

左侧边栏底部预留「Loop 控制台」入口：
- MVP 阶段隐藏或置灰
- V1.5c 点击进入 Loop 管理面板

---

## 4. V1.5 分期实现路径

### 4.1 V1.5a — 草稿自验证循环（A）

**目标**：实现生成-验证-修正的内部循环，提高草稿一次通过率。

**实现内容**：
1. 实现 `ValidationMiddleware`（节点级，挂在 generate 节点后）
2. 验证策略：
   - 确定性验证（优先）：结构检查、[待补充] 标记比例统计、Markdown 格式校验
   - LLM 验证（补充）：内容相关性、完整性评估
3. 利用 Eino Graph 的 `AddBranch` 实现条件跳转
4. 停止条件：验证通过 OR 达到最大迭代次数（默认 2 次）OR 质量分不再提升

**接入方式**：
```go
nodeMiddlewares["generate"] = []NodeMiddleware{
    ValidationMiddleware(maxIterations: 2),
}
```

### 4.2 V1.5b — 跨任务记忆与学习（B）

**目标**：Agent 能记住用户偏好，越用越准。

**实现内容**：
1. 实现 `MemoryMiddleware`（全局 Graph Middleware）
2. 记忆内容：
   - 用户修改模式（diff 分析）
   - 风格偏好（语气、结构、详略程度）
   - 数据源权重（哪些数据源对哪类文档更有用）
3. 存储：`agent_memory` 表
4. 生成前注入记忆偏好到上下文，生成后更新记忆

**接入方式**：
```go
graphMiddlewares = append(graphMiddlewares, MemoryMiddleware())
```

### 4.3 V1.5c — 主动发现与巡检（C）

**目标**：Agent 不只是被动响应，还能主动发现工作。

**实现内容**：
1. 实现独立的 `LoopScheduler`（外层调度器，不在 Eino Graph 内）
2. 巡检触发：定时扫描日历、任务列表、笔记
3. 发现策略：
   - 高置信度 → 自动创建 AgentRun，进入流水线
   - 低置信度 → 只提示用户，不自动执行
4. 新增「巡检发现」看板列
5. 侧边栏 Loop 控制台：配置巡检规则、查看运行记录

### 4.4 V1.5d — 子 Agent 协作（D）

**目标**：复杂任务拆分为多子 Agent 并行执行，提高质量和速度。

**实现内容**：
1. 搜集阶段：利用 Eino 的并发节点能力实现多数据源并行抓取
2. 审查阶段：独立的 Reviewer Agent（可配置不同模型）
3. 生成-审查-修正三阶段循环（用 Graph 有环图实现）
4. 子 Agent 状态监控面板

### 4.5 V1.5e — 用户反馈闭环（E）

**目标**：从用户修改中学习，形成大循环。

**实现内容**：
1. 实现 `FeedbackLoop`
2. 检测用户修改 → 分析 diff → 提取模式 → 更新记忆
3. 长期优化生成策略，形成「生成-使用-修改-学习」的大循环
4. 与 V1.5b 的记忆系统联动

---

## 5. 技术选型与依据

### 5.1 为什么用中间件模式而不是钩子模式？

| 对比项 | 中间件模式 | 钩子模式 |
| --- | --- | --- |
| 灵活性 | 高，可组合、可插拔 | 中，固定位置固定 |
| 复杂度 | 中，需理解包装模式 | 低，简单直接 |
| 与 Eino 契合度 | 高，Eino Aspect 也是类似模式 | 中 |
| 测试性 | 好，每个中间件独立测试 | 好 |

**选择中间件模式的理由**：
- 与 Eino 原生 Aspect 机制理念一致
- 两层中间件（全局+节点级）功能最全
- 可组合性强，V1.5 各阶段可以独立添加中间件
- MVP 阶段零成本（空中间件链）

### 5.2 为什么基于 Eino 原生能力？

- Eino 已内置 Aspect/Callback 机制，无需自研
- Eino Graph 支持有环图，可实现验证-重试循环
- Eino 有 State / Checkpoint 机制，支持中断恢复
- 减少自研代码，降低维护成本

### 5.3 Graph vs Workflow 选型

- **MVP 阶段**：继续使用 Workflow（DAG，字段级数据映射），改动最小
- **V1.5a**：在 Workflow 外层包一个循环控制器（验证失败则重新跑部分节点）
- **V1.5d 评估**：根据子 Agent 复杂度决定是否切换到 Graph API

---

## 6. 风险和权衡

### 6.1 主要风险

| 风险 | 影响 | 概率 | 缓解措施 |
| --- | --- | --- | --- |
| Eino Graph 的有环图能力不如预期 | 高 | 中 | MVP 阶段不依赖有环图，V1.5a 前先做技术验证，不行就用外层循环控制 |
| 验证循环导致 LLM token 消耗翻倍 | 中 | 高 | 控制最大迭代次数（默认 2 次）；优先用确定性验证，LLM 验证只在必要时触发 |
| 中间件层增加调试复杂度 | 中 | 中 | 保持中间件数量精简；统一日志格式；提供中间件链路追踪工具 |
| 主动巡检可能产生噪音任务 | 高 | 中 | V1.5c 默认关闭巡检；高置信度才自动创建任务，低置信度只提示 |
| 记忆系统可能学到错误模式 | 中 | 中 | 用户修改需超过阈值才计入记忆；保留人工审核机制 |

### 6.2 关键权衡

**权衡 1：验证循环的 token 成本 vs 质量提升
- 策略：确定性验证优先，LLM 验证为辅
- 停止条件：验证通过 OR 达到最大迭代次数 OR 质量分不再提升
- 默认最大迭代：2 次（可配置）

**权衡 2：MVP 扩展点的侵入程度**
- 最小侵入原则：只加字段、加空切片、加包装函数（空路径直接返回）
- 不改变现有节点函数的签名和逻辑
- 运行时零成本

**权衡 3：主动巡检的自动化程度**
- 保守策略：默认关闭，需用户手动开启
- 分级处理：高置信度自动执行，低置信度只提示
- 可回退：用户可随时关闭巡检功能

---

## 7. MVP 开发注意事项

在 MVP 开发过程中，为了便于 V1.5 平滑接入，需注意：

1. **节点函数保持纯粹**：每个节点函数只做一件事，输入输出清晰，便于后续用中间件包装
2. **状态集中管理**：所有状态放在 `workflowState` 中，不散落各处
3. **错误处理规范**：统一错误类型，便于中间件捕获和重试判断
4. **日志结构化**：使用统一的日志格式，便于 Loop 系统分析
5. **配置外置**：阈值、最大迭代次数等可配置，不硬编码

---

## 8. 参考资料

- Eino 官方文档：https://eino.dev/
- Loop Engineering (Addy Osmani)：https://addyosmani.com/blog/loop-engineering/
- Loop Engineering (Morten Minde)：https://mortenminde.substack.com/p/the-patterns-of-loop-engineering

---

## 9. 变更记录

| 版本 | 日期 | 变更内容 |
| --- | --- | --- |
| v0.1.0 | 2026-06-29 | 初始版本，完成架构设计和 V1.5 分期规划 |
