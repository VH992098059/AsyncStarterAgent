# Phase 4 / T024 交接文档 — 给下一窗口

> **生成时间**: 2026-06-21 (Asia/Taipei)
> **当前任务**: T024 — Obsidian Vault 交付集成（FR-D02 P1, T024）
> **状态**: ✅ **DONE**（实现 + 测试 + build + 全量测试通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014-T020 + T026 + T023 + T024 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T024 做完了什么内容）

按 `doc/plans/05-phase4-delivery.md` §Task T024 执行。

### 1.1 关联需求

- **FR-D02 (P1)**: Obsidian Vault 写入

### 1.2 新建/修改文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/delivery/service.go` | **修改** | Service 新增 `obs *ObsidianAdapter` 字段 + NewService 签名新增 obs 参数 + Deliver 添加 "obsidian" case + updateSourceComment 添加 obsidian 分支 |
| `internal/delivery/service_test.go` | **修改** | 已有测试适配新 NewService 签名 + 新增 3 个 obsidian 测试 |
| `cmd/api/wire.go` | **修改** | 条件初始化 ObsidianAdapter + 注入到 delivery.Service |

> **修改 3 个文件 = T024 总变更 3 个对象**。未新建文件（ObsidianAdapter 已在 T024 前序存在）。未新增依赖。

### 1.3 架构设计说明

T024 将已有的 ObsidianAdapter 集成到交付管道，使 `Service.Deliver` 支持 "obsidian" 目标类型：

| 层 | 变更 | 职责 |
|---|---|---|
| 适配层 | `ObsidianAdapter` (已有) | 写入本地 Vault .md 文件 |
| 编排层 | `Service.obs` 字段 (新增) | 持有 ObsidianAdapter 引用 |
| 路由层 | `Deliver` switch "obsidian" case (新增) | 路由到 `obs.WriteFile` |
| 备注层 | `updateSourceComment` obsidian 分支 (新增) | Obsidian 无远程 API，直接返回 nil |
| 注入层 | `wire.go` 条件初始化 (修改) | `OBSIDIAN_VAULT_PATH` 非空时创建适配器 |

**关键设计决策**：

1. **Obsidian 不需要 updateSourceComment**：Obsidian 写入本地文件，没有远程评论 API。`updateSourceComment` 的 obsidian 分支直接返回 nil。

2. **条件初始化与 Notion 一致**：仅在 `OBSIDIAN_VAULT_PATH` 非空时创建 ObsidianAdapter，否则 `Service.obs` 为 nil，Deliver 对 "obsidian" 类型返回 "obsidian adapter not configured" 错误。

3. **NewService 签名变更**：从 `NewService(pool, notion, notif)` 变为 `NewService(pool, notion, obs, notif)`，所有调用方（wire.go、service_test.go）已同步更新。

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | ObsidianAdapter 在 T024 前序已存在 | 计划中 T024 包含 obsidian.go 创建，但实际已在更早任务中创建 | ✅ 合规（不重复创建） |
| 2 | `updateSourceComment` obsidian 分支返回 nil | Obsidian 无远程评论 API | ✅ 合规改进 |
| 3 | wire.go Notion + Obsidian 分别条件初始化 | 两个适配器独立配置，互不依赖 | ✅ 合规改进 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8 合规）。

### 1.5 测试覆盖

| # | 测试名 | 包 | 覆盖场景 |
|---|---|---|---|
| 1 | `TestDeliver_ObsidianNotConfigured` | delivery | nil Obsidian 适配器返回错误 |
| 2 | `TestDeliver_ObsidianWriteFile` | delivery | Obsidian 适配器已配置，路由正确（DB nil 时返回 database 错误而非 adapter 错误） |
| 3 | `TestNewService_WithObsidian` | delivery | Service 构造时 obs 字段正确设置 |

已有测试（T023）已全部适配新签名：
- `TestNewService`: `NewService(nil, nil, nil, notif)`
- `TestNewService_NilNotifier`: `NewService(nil, nil, nil, nil)`
- `TestDeliver_UnsupportedTarget`: `NewService(nil, nil, nil, nil)`
- `TestDeliver_NotionNotConfigured`: `NewService(nil, nil, nil, nil)`

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误 |
| `go test ./...` | ✅ 全部 PASS | 无回归 |
| `go test -v ./internal/delivery/...` | ✅ 22/22 PASS | 含 T023 的 12 个 + T024 的 3 个 + obsidian adapter 的 7 个 |

---

## 2. 本窗口**没做**什么

- ❌ **没有**实现飞书文档交付（T025 范围）
- ❌ **没有**实现桌面通知（T021 Tauri 范围，Notifier 接口已定义）
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**做端到端验证（按用户红线）
- ❌ **没有**新增任何依赖

---

## 3. 下一步需要实现什么 — Phase 4: T025

### 3.1 T025: 飞书文档交付

来源：`doc/plans/05-phase4-delivery.md` §Task T025

- FR-D03 (P2) — 标注 V1.5 推迟项
- 需要创建 `internal/delivery/feishu_doc.go` — 飞书文档适配器
- 需要创建 `internal/delivery/feishu_doc_test.go`
- 需要在 `Service.Deliver` 中添加 "feishu" case
- 需要在 `wire.go` 中注入 FeishuDocAdapter
- 需要在 `Service` 结构体中添加 `feishu *FeishuDocAdapter` 字段
- 飞书 SDK `github.com/larksuite/oapi-sdk-go/v3` 已在 T012 引入

> **注意**：FR-D03 标为 P2/V1.5 推迟项。是否在 MVP 阶段实现需用户确认。

### 3.2 T024 完成后 Phase 4 进度

| 任务 | 状态 |
|---|---|
| T023: Notion API 文档交付 | ✅ 完成 |
| T024: Obsidian Vault 交付 | ✅ 完成 |
| T025: 飞书文档交付 | ⬜ 待实现（P2/V1.5 推迟项） |
| T021: Tauri 前端原型 | ⬜ 待实现（需安装 Rust + Node.js 工具链） |
| T022: Flutter 备选 | ⬜ 默认跳过 |

---

## 4. 给下一窗口的提示

1. **`Service` 现在支持 Notion + Obsidian**：`NewService(pool, notion, obs, notif)`。两个适配器均可为 nil。

2. **`POST /api/v1/drafts/:id/deliver`** 支持两种 target_type：
   - `"notion"` — 调用 NotionAdapter.CreatePage
   - `"obsidian"` — 调用 ObsidianAdapter.WriteFile

3. **`wire.go` 条件初始化**：
   - `NOTION_API_KEY` 非空 → 创建 NotionAdapter
   - `OBSIDIAN_VAULT_PATH` 非空 → 创建 ObsidianAdapter
   - 两者独立，可同时启用

4. **`ObsidianWriter` 接口**已定义在 `obsidian.go`：`WriteFile(ctx, title, markdown) (string, error)`。`ObsidianAdapter` 实现了该接口。

5. **`updateSourceComment` obsidian 分支**：直接返回 nil（Obsidian 无远程评论 API）。

6. **T025 飞书文档交付**需注意：
   - 飞书 SDK `github.com/larksuite/oapi-sdk-go/v3` 已在 T012 引入
   - 需要在 `Service` 结构体添加 `feishu *FeishuDocAdapter` 字段
   - 需要在 `NewService` 签名中添加 feishu 参数
   - 需要在 `Deliver` switch 中添加 "feishu" case
   - FR-D03 标为 P2/V1.5 推迟项，是否实现需问用户

7. **T021 Tauri 前端**需注意：
   - 需要安装 Rust + Node.js 工具链
   - 计划中 Step 0 要求先询问用户是否安装
   - Tauri notification 插件实现 FR-D05 桌面通知
   - Notifier 接口已定义：`Notify(ctx, title, body) error`

8. **ai-coding-boundary P1 红线继续生效**：不得 `git commit`；由用户在主窗口决定是否 commit。

9. **用户已确认的选型决策**（累计）：
   - Notion SDK: 直接 HTTP 调用（用户选择，因第三方 SDK 不可用）
   - Obsidian: 本地文件写入（无第三方 SDK）
   - 其余决策同 T026-handoff.md §4.8

10. **依赖变更**（T024 累计）：
    - T024 **净新增 0 个 Go 依赖**（ObsidianAdapter 使用 os 标准库）

---

## 5. 当前文件结构（Phase 4 / T024 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009+T020+T026+T023+T024
│   └── wire.go                    ✅ T024 修改（注入 ObsidianAdapter）
├── internal/
│   ├── config/                    ✅ T001+T006+T026 (未动)
│   ├── handler/                   ✅ T002+T006+T009+T020+T023 (未动)
│   │   ├── health.go              ✅ T002
│   │   ├── health_test.go         ✅ T002
│   │   ├── webhook.go             ✅ T006
│   │   ├── webhook_test.go        ✅ T006
│   │   ├── trigger.go             ✅ T009
│   │   ├── trigger_test.go        ✅ T009
│   │   ├── util.go                ✅ T002
│   │   ├── draft.go               ✅ T020
│   │   ├── draft_test.go          ✅ T020
│   │   └── delivery.go            ✅ T023
│   ├── delivery/                  ✅ T023+T024
│   │   ├── notion.go              ✅ T023（NotionAdapter）
│   │   ├── notion_test.go         ✅ T023
│   │   ├── obsidian.go            ✅ T024（ObsidianAdapter + ObsidianWriter）
│   │   ├── obsidian_test.go       ✅ T024
│   │   ├── service.go             ✅ T024 修改（obs 字段 + obsidian case）
│   │   └── service_test.go        ✅ T024 修改（obsidian 测试 + 签名适配）
│   ├── harvesting/                ✅ T010+T011+T012+T013+T014 (未动)
│   ├── middleware/                ✅ T002 (未动)
│   ├── queue/                     ✅ T004 (未动)
│   ├── repository/                ✅ T003+T007 (未动)
│   ├── server/                    ✅ T002+T009+T020+T023 (未动)
│   ├── synthesis/                 ✅ T015+T016+T017+T018+T019+T020+T026 (未动)
│   └── trigger/                   ✅ T006-T008 (未动)
├── pkg/httpx/                     ✅ T002 (未动)
├── templates/                     ✅ T017 (未动)
├── migrations/                    ✅ T003+T006+T008+T014+T015 (未动)
├── doc/handoff/
│   ├── ...                        (untracked)
│   └── T024-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T023 修改（Go 1.26.3）
└── go.sum                         ✅ T023 修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015-T020 + T026 + T023 + T024 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T025 飞书文档交付是否在 MVP 阶段实现**：FR-D03 标为 P2/V1.5 推迟项。是否跳过直接进入 T021？

4. **T021 Tauri 前端是否启动**：需要安装 Rust + Node.js 工具链，需用户确认。

5. **Go 版本升级**：Go 从 1.25.8 升级到 1.26.3（因 go-api-libs/notion 要求），该 SDK 已移除但版本保留。是否回退 Go 版本？
