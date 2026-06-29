# Phase 4 / T023 交接文档 — 给下一窗口

> **生成时间**: 2026-06-21 (Asia/Taipei)
> **当前任务**: T023 — Notion API 文档交付（FR-D01 P0, FR-D04 P0）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014-T020 + T026 + T023 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T023 做完了什么内容）

按 `doc/plans/05-phase4-delivery.md` §Task T023 执行。

### 1.1 关联需求

- **FR-D01 (P0)**: Notion 文档创建
- **FR-D04 (P0)**: 任务备注更新

### 1.2 新建/修改文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/delivery/notion.go` | **新建** | NotionAdapter — 直接 HTTP 调用 Notion REST API（CreatePage + UpdateTaskComment） |
| `internal/delivery/notion_test.go` | **新建** | 6 个测试（markdown 转换 + 适配器构造） |
| `internal/delivery/service.go` | **新建** | Delivery Service — 交付编排（Deliver + updateSourceComment + extractPageID） |
| `internal/delivery/service_test.go` | **新建** | 5 个测试（extractPageID + Service 构造 + 错误路径） |
| `internal/handler/delivery.go` | **新建** | DeliveryHandler — HTTP 层（POST /api/v1/drafts/:id/deliver） |
| `internal/server/server.go` | **修改** | 新增 `delivSvc` 参数 + 注册 delivery 路由 |
| `cmd/api/wire.go` | **修改** | 注入 delivery.Service（Notion 适配器条件初始化） |
| `internal/delivery/obsidian.go` | **修改** | 添加 ObsidianWriter 接口定义（修复已有测试编译错误） |

> **新建 5 个文件 + 修改 3 个文件 = T023 总变更 8 个对象**。未新增 Go 依赖（Notion 使用直接 HTTP 调用）。

### 1.3 架构设计说明

T023 实现了 Notion 文档交付管道，将草稿从 DB 推送到 Notion 平台：

| 层 | 类型 | 职责 |
|---|---|---|
| 适配层 | `NotionAdapter` | 直接 HTTP 调用 Notion REST API（CreatePage + UpdateTaskComment） |
| 编排层 | `delivery.Service` | 交付编排：查草稿 → 创建 delivery 记录 → 调适配器 → 更新状态 → 更新备注 → 通知 |
| 通知层 | `Notifier` 接口 | 桌面通知抽象（FR-D05），T021 Tauri 将实现 |
| 调度层 | `DeliveryHandler` | HTTP 参数提取 → 调用 Service → 错误处理 |
| 注入层 | `cmd/api/wire.go` | 条件初始化 Notion 适配器（NOTION_API_KEY 非空时） |

**关键设计决策**：

1. **直接 HTTP 调用替代第三方 SDK**：计划指定 `github.com/mattn/go-notion`，但该包不存在。`go-api-libs/notion` 只有读取方法无 CreatePage。改用 `net/http` 直接调用 Notion REST API，零新增依赖。

2. **Notion 适配器条件初始化**：仅在 `NOTION_API_KEY` 非空时创建 NotionAdapter，否则 Service 中 notion 为 nil，Deliver 对 "notion" 类型返回错误。

3. **markdownToBlocks 支持 heading/list**：计划中只有段落块，实现扩展支持 `#`/`##`/`###` heading 和 `-`/`*` 列表项。

4. **extractPageID 处理 Notion URL 格式**：Notion URL 格式为 `{Title}-{PageID}`，需要提取最后一个 `-` 后的部分作为 PageID。

5. **Deliver nil pool 保护**：`Service.Deliver` 在 pool 为 nil 时返回错误而非 panic。

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | 使用直接 HTTP 调用替代 `github.com/mattn/go-notion` | 指定 SDK 不存在，替代 SDK 无写入功能 | ✅ 合规改进（零新增依赖） |
| 2 | `markdownToBlocks` 支持 heading + list | 计划只有段落块，扩展支持更完整的 markdown | ✅ 合规改进 |
| 3 | `extractPageID` 处理 Notion Title-ID 格式 | Notion URL 格式特殊，需要提取 ID 部分 | ✅ 合规改进 |
| 4 | `service.go` 使用 `log.Printf` 替代 `fmt.Printf` | C4 红线：禁止调试输出 | ✅ 合规改进 |
| 5 | `service.go` 使用 `*string` 替代 `sql.NullString` | 计划中建议的改进 | ✅ 合规改进 |
| 6 | `Deliver` 添加 nil pool 检查 | 防止测试和未配置场景 panic | ✅ 合规改进 |
| 7 | 添加 `ObsidianWriter` 接口 | 修复已有 obsidian_test.go 编译错误 | ✅ 合规改进 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增 Go 依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestMarkdownToBlocks_Paragraphs` | delivery | 段落 + heading 块生成 |
| 2 | `TestMarkdownToBlocks_EmptyLines` | delivery | 空行跳过 |
| 3 | `TestMarkdownToBlocks_Headings` | delivery | H1/H2/H3 检测 |
| 4 | `TestMarkdownToBlocks_ListItems` | delivery | 列表项检测 |
| 5 | `TestParagraphBlock` | delivery | paragraphBlock 辅助函数 |
| 6 | `TestNewNotionAdapter` | delivery | 适配器构造 |
| 7 | `TestExtractPageID` | delivery | Notion URL ID 提取 |
| 8 | `TestExtractPageID_NoSlash` | delivery | 无斜杠 URL |
| 9 | `TestNewService` | delivery | Service 构造 |
| 10 | `TestNewService_NilNotifier` | delivery | nil 通知器 |
| 11 | `TestDeliver_UnsupportedTarget` | delivery | 不支持的目标类型 |
| 12 | `TestDeliver_NotionNotConfigured` | delivery | Notion 未配置 |

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./...` | ✅ 全部 PASS | 无回归（Phase 1-3 + T023 测试仍通过） |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现 Obsidian 交付路由（T024 已有适配器，但 Service.Deliver 未添加 "obsidian" case）
- ❌ **没有**实现飞书文档交付（T025 范围）
- ❌ **没有**实现桌面通知（T021 Tauri 范围，Notifier 接口已定义）
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**做端到端验证（按用户红线）

---

## 3. 下一步需要实现什么 — Phase 4: T024

### 3.1 T024: Obsidian Vault 交付

来源：`doc/plans/05-phase4-delivery.md` §Task T024

- ObsidianAdapter 已存在（T024 前序任务已完成）
- 需要在 `Service.Deliver` 中添加 "obsidian" case
- 需要在 `wire.go` 中注入 ObsidianAdapter
- FR-D02 (P1) — V1.5 推迟项，但最小可工作实现已完成

### 3.2 T023 完成后 Phase 4 进度

| 任务 | 状态 |
|---|---|
| T023: Notion API 文档交付 | ✅ 完成 |
| T024: Obsidian Vault 交付 | ⬜ 适配器已存在，需集成到 Service |
| T025: 飞书文档交付 | ⬜ 待实现 |
| T021: Tauri 前端原型 | ⬜ 待实现 |
| T022: Flutter 备选 | ⬜ 默认跳过 |

---

## 4. 给下一窗口的提示

1. **`NotionAdapter` 使用直接 HTTP 调用**：`NewNotionAdapter(cfg)` 创建，`CreatePage(ctx, title, md)` 创建页面，`UpdateTaskComment(ctx, pageID, comment)` 添加评论。不需要第三方 SDK。

2. **`delivery.Service` 是交付编排入口**：`NewService(pool, notion, notif)`，`Deliver(ctx, draftID, targetType)` 执行交付。当前只支持 "notion" 类型。

3. **`POST /api/v1/drafts/:id/deliver` 路由已注册**：Body `{"target_type": "notion"}`。

4. **`Notifier` 接口已定义**：`Notify(ctx, title, body) error`。T021 Tauri 将实现此接口。

5. **`ObsidianAdapter` 已存在**：`WriteFile(ctx, title, md)` 写入本地 vault。T024 需要在 `Service.Deliver` 中添加 "obsidian" case。

6. **`wire.go` 已注入 delivery.Service**：`Deps.Deliv` 为 `*delivery.Service`。如果 `NOTION_API_KEY` 未设置，Notion 适配器为 nil。

7. **Go 版本已升级到 1.26.4**：`go-api-libs/notion` 依赖要求 Go >= 1.26.3，但该 SDK 已被移除（只有读取方法）。`go.mod` 中 Go 版本为 1.26.3，实际使用 1.26.4。

8. **T024 需注意**：
   - `ObsidianAdapter` 已实现 `WriteFile`，需要在 `Service.Deliver` switch 中添加 "obsidian" case
   - `wire.go` 需要注入 ObsidianAdapter（使用 `cfg.ObsidianVaultPath`）
   - FR-D02 标为 P1/V1.5 推迟项，但最小可工作实现已完成

9. **ai-coding-boundary P1 红线继续生效**：不得 `git commit`；由用户在主窗口决定是否 commit。

10. **用户已确认的选型决策**（累计）：
    - Notion SDK: 直接 HTTP 调用（用户选择，因第三方 SDK 不可用）
    - 其余决策同 T026-handoff.md §4.8

11. **依赖变更**（T023 累计）：
    - `github.com/go-api-libs/notion` — **已添加后移除**（只有读取方法，无 CreatePage）
    - Go 版本升级 1.25.8 → 1.26.3（因 go-api-libs/notion 要求，已保留）
    - T023 **净新增 0 个 Go 依赖**（Notion 使用 net/http 直接调用）

---

## 5. 当前文件结构（Phase 4 / T023 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009+T020+T026+T023
│   └── wire.go                    ✅ T023 修改（注入 delivery.Service）
├── internal/
│   ├── config/                    ✅ T001+T006+T026 (未动)
│   ├── handler/                   ✅ T002+T006+T009+T020+T023
│   │   ├── health.go              ✅ T002
│   │   ├── health_test.go         ✅ T002
│   │   ├── webhook.go             ✅ T006
│   │   ├── webhook_test.go        ✅ T006
│   │   ├── trigger.go             ✅ T009
│   │   ├── trigger_test.go        ✅ T009
│   │   ├── util.go                ✅ T002
│   │   ├── draft.go               ✅ T020
│   │   ├── draft_test.go          ✅ T020
│   │   └── delivery.go            ✅ T023（DeliveryHandler）
│   ├── delivery/                  ✅ T023+T024
│   │   ├── notion.go              ✅ T023（NotionAdapter）
│   │   ├── notion_test.go         ✅ T023
│   │   ├── obsidian.go            ✅ T024（ObsidianAdapter + ObsidianWriter）
│   │   ├── obsidian_test.go       ✅ T024
│   │   ├── service.go             ✅ T023（Delivery Service）
│   │   └── service_test.go        ✅ T023
│   ├── harvesting/                ✅ T010+T011+T012+T013+T014 (未动)
│   ├── middleware/                ✅ T002 (未动)
│   ├── queue/                     ✅ T004 (未动)
│   ├── repository/                ✅ T003+T007 (未动)
│   ├── server/                    ✅ T002+T009+T020+T023
│   │   └── server.go              ✅ T023 修改（新增 delivSvc 参数 + delivery 路由）
│   ├── synthesis/                 ✅ T015+T016+T017+T018+T019+T020+T026 (未动)
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── templates/                     ✅ T017 (未动)
├── migrations/                    ✅ T003+T006+T008+T014+T015 (未动)
├── doc/handoff/
│   ├── ...                        (untracked)
│   └── T023-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T023 修改（Go 1.26.3）
└── go.sum                         ✅ T023 修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015-T020 + T026 + T023 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T024 启动前需确认**：
   - Obsidian 交付是否在 MVP 阶段实现（FR-D02 标为 P1/V1.5 推迟项）
   - 飞书文档交付（FR-D03 P2）是否跳过

4. **Go 版本升级**：Go 从 1.25.8 升级到 1.26.3（因 go-api-libs/notion 要求），该 SDK 已移除但版本保留。是否回退 Go 版本？
