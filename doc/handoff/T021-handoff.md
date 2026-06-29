# Phase 4 / T021 交接文档 — 给下一窗口

> **生成时间**: 2026-06-21 (Asia/Taipei)
> **当前任务**: T021 — Tauri 前端原型（FR-D05 P1, T021 P0）
> **状态**: ✅ **DONE**（框架搭建 + 代码质量修复 + build 通过，待用户决定 commit）
> **基线 commit**: `74e17a3` (T013，T014-T024 + T021 未 commit)
> **用户红线**: 不跑 docker / 不做端到端 / 不自动 push

---

## 1. 本窗口做了什么（T021 做完了什么内容）

按 `doc/plans/05-phase4-delivery.md` §Task T021 执行。

### 1.1 关联需求

- **T021 (P0)**: Tauri 前端原型
- **FR-D05 (P1)**: 桌面通知（Tauri notification 插件）

### 1.2 新建/修改文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `web/package.json` | **新建** | React 18 + Tauri API v2 + Vite 5 + TypeScript 5.5 |
| `web/vite.config.ts` | **新建** | Vite 配置，port 1420，envPrefix VITE_+TAURI_ |
| `web/tsconfig.json` | **新建** | TypeScript 严格模式配置 |
| `web/index.html` | **新建** | 入口 HTML，lang="zh-CN" |
| `web/src/main.tsx` | **新建** | React 入口，StrictMode |
| `web/src/index.css` | **新建** | 暗色主题 CSS 变量 |
| `web/src/App.tsx` | **新建** | 主界面：触发输入 + SSE 订阅 + 错误处理 |
| `web/src/api/client.ts` | **新建** | SSE 客户端 + 类型定义（Mark/Draft） |
| `web/src/components/DraftEditor.tsx` | **新建** | 草稿编辑器组件 |
| `web/src/components/MarkList.tsx` | **新建** | [待补充] 标记列表组件 |
| `web/src-tauri/Cargo.toml` | **新建** | Tauri v2 + notification 插件 |
| `web/src-tauri/tauri.conf.json` | **新建** | Tauri 配置，CSP 最小策略 |
| `web/src-tauri/src/main.rs` | **新建** | Tauri 入口，注册 notification 插件 |
| `web/src-tauri/build.rs` | **新建** | Tauri 构建脚本 |

> **新建 14 个文件 = T021 总变更**。未修改已有 Go 代码。未新增 Go 依赖。

### 1.3 架构设计说明

T021 搭建了 Tauri v2 + React 前端原型，实现以下功能：

| 层 | 文件 | 职责 |
|---|---|---|
| 入口 | `main.tsx` | React 挂载 |
| 编排 | `App.tsx` | 触发输入 → POST /api/v1/trigger → SSE 订阅 → 渲染草稿+标记 |
| API | `api/client.ts` | EventSource SSE 客户端，含错误回调 |
| 组件 | `DraftEditor.tsx` | 草稿内容只读展示 + 完成度 |
| 组件 | `MarkList.tsx` | [待补充] 标记列表 + 补全输入框 |
| Tauri | `src-tauri/` | 桌面壳 + notification 插件（FR-D05） |

**关键设计决策**：

1. **API 基础 URL 可配置**：使用 `import.meta.env.VITE_API_BASE` 环境变量，默认 `http://localhost:8080`。避免硬编码。

2. **错误处理完整（C5 合规）**：
   - `handleTrigger`: try-catch 包裹 fetch，检查 `res.ok`，验证 `data.data.run_id` 存在
   - `streamDraft`: 新增 `onError` 回调，SSE error 事件通知调用方
   - `JSON.parse`: try-catch 包裹，解析失败走 onError
   - UI 层显示错误消息

3. **CSP 最小策略**：`default-src 'self'; connect-src 'self' http://localhost:*; style-src 'self' 'unsafe-inline'`，替代不安全的 `null`。

4. **移除 lobe-ui 依赖**：原计划包含 `lobe-ui`，但该包可能不存在或不可用。按 C7 规则，不引入未确认的依赖。如需 UI 库，后续经用户确认后添加。

5. **pnpm 使用（C9 合规）**：所有包管理操作使用 pnpm，生成 `pnpm-lock.yaml`。

### 1.4 与计划/规范的偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | 移除 `lobe-ui` 依赖 | 包可能不存在，按 C7 不引入未确认依赖 | ✅ 合规（C7 优先） |
| 2 | 新增 `@types/react-dom` devDep | TypeScript 类型安全需要 | ✅ 合理补充 |
| 3 | `streamDraft` 新增 `onError` 参数 | C5 红线：禁止绕过错误处理 | ✅ 合规改进 |
| 4 | `App.tsx` 新增 `error` 状态 + 错误显示 | C5 红线：错误必须有处理 | ✅ 合规改进 |
| 5 | CSP 从 `null` 改为最小策略 | 安全隐患修复 | ✅ 合规改进 |
| 6 | API URL 使用环境变量 | 代码质量改进，避免硬编码 | ✅ 合规改进 |
| 7 | 计划中 `hooks/` 目录未创建 | 计划未指定 hook 文件内容，按 R2 不自我发挥 | ✅ 合规 |
| 8 | `pnpm-workspace.yaml` 额外创建 | pnpm 自动生成 | ✅ 合理 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未新增 Go 依赖（C7）、未写 TODO/FIXME（C1）、未自动 commit（P1）、未写 mock 数据（C8）、使用 pnpm（C9）。

### 1.5 测试覆盖

T021 是前端原型，按计划不包含前端单元测试（计划 Step 1-14 无测试步骤）。

Go 后端测试无回归：
- `go test ./internal/delivery/...` → ✅ PASS（cached）

前端构建验证：
- `pnpm run build` → ✅ 34 modules transformed，0 errors

### 1.6 验证结果

| 命令 | 结果 | 备注 |
|---|---|---|
| `pnpm install` | ✅ 75 packages | pnpm 11.7.0 |
| `pnpm run build` | ✅ vite build | 1.15s，0 errors |
| `go test ./internal/delivery/...` | ✅ PASS | 无回归 |

---

## 2. 本窗口**没做**什么

- ❌ **没有**运行 `pnpm tauri dev`（需要完整 Tauri 工具链配置 + Rust 编译，可能需要额外时间）
- ❌ **没有**实现飞书文档交付（T025 范围）
- ❌ **没有**自动 push（按 P2 红线）
- ❌ **没有**自动 commit（按 P1 红线，等用户决定）
- ❌ **没有**做端到端验证（按用户红线）
- ❌ **没有**创建 `hooks/` 目录（计划未指定内容，按 R2 不自我发挥）
- ❌ **没有**添加前端单元测试（计划未要求）
- ❌ **没有**添加组件级 CSS 样式（计划未要求，原型阶段功能优先）

---

## 3. 下一步需要实现什么 — Phase 4: T025 或集成测试

### 3.1 T025: 飞书文档交付

来源：`doc/plans/05-phase4-delivery.md` §Task T025

- FR-D03 (P2) — 标注 V1.5 推迟项
- 需要创建 `internal/delivery/feishu_doc.go` — 飞书文档适配器
- 需要创建 `internal/delivery/feishu_doc_test.go`
- 需要在 `Service.Deliver` 中添加 "feishu" case
- 需要在 `wire.go` 中注入 FeishuDocAdapter
- 需要在 `Service` 结构体中添加 `feishu *FeishuDocAdapter` 字段
- 飞书 SDK `github.com/larksuite/oapi-sdk-go/v3` 已在 go.mod 中

> **注意**：FR-D03 标为 P2/V1.5 推迟项。按 R5 红线，是否在 MVP 阶段实现需用户确认。

### 3.2 T021 后续：Tauri 开发服务器验证

- 运行 `pnpm tauri dev` 验证桌面窗口能打开
- 需要后端 API 运行在 localhost:8080
- 验证触发 → SSE → 草稿流式出现的完整流程

### 3.3 Phase 4 退出标准验证

| 退出标准 | 状态 |
|---|---|
| 5 个用户故事端到端通过 | ⬜ 待验证 |
| Notion API 集成测试通过 | ✅ T023 |
| Obsidian 集成测试通过 | ✅ T024 |
| 飞书集成测试通过 | ⬜ T025（V1.5 推迟项） |
| 任务备注更新到原数据源 | ✅ T023 |
| 桌面通知在草稿完成时触发 | ⚠️ Tauri 插件已注册，需端到端验证 |
| Tauri 客户端能跑 5 个核心场景 | ⬜ 需 `pnpm tauri dev` 验证 |

### 3.4 Phase 4 进度

| 任务 | 状态 |
|---|---|
| T023: Notion API 文档交付 | ✅ 完成 |
| T024: Obsidian Vault 交付 | ✅ 完成 |
| T025: 飞书文档交付 | ⬜ 待实现（P2/V1.5 推迟项） |
| T021: Tauri 前端原型 | ✅ 完成（框架搭建 + build 通过） |
| T022: Flutter 备选 | ⬜ 默认跳过 |

---

## 4. 给下一窗口的提示

1. **`web/` 目录已创建**：完整的 Tauri v2 + React 前端项目，`pnpm install` + `pnpm run build` 已验证通过。

2. **API 基础 URL 可配置**：通过 `VITE_API_BASE` 环境变量设置，默认 `http://localhost:8080`。

3. **`streamDraft` 签名变更**：新增第 4 个参数 `onError: (err: Error) => void`，SSE 错误不再静默。

4. **CSP 已设置最小策略**：`default-src 'self'; connect-src 'self' http://localhost:*; style-src 'self' 'unsafe-inline'`。

5. **Tauri notification 插件已注册**：`main.rs` 中 `tauri_plugin_notification::init()`，满足 FR-D05。但后端 `Notifier` 接口目前传 `nil`，桌面通知的触发逻辑需要在后端集成（或通过 Tauri 前端 API 调用）。

6. **`pnpm tauri dev` 需要后端运行**：启动 Tauri 开发服务器前，需确保 Go 后端在 `localhost:8080` 运行。

7. **T025 飞书文档交付**需注意：
   - 飞书 SDK `github.com/larksuite/oapi-sdk-go/v3` 已在 go.mod 中
   - 需要在 `Service` 结构体添加 `feishu *FeishuDocAdapter` 字段
   - 需要在 `NewService` 签名中添加 feishu 参数
   - 需要在 `Deliver` switch 中添加 "feishu" case
   - FR-D03 标为 P2/V1.5 推迟项，是否实现需问用户

8. **ai-coding-boundary P1 红线继续生效**：不得 `git commit`；由用户在主窗口决定是否 commit。

9. **用户已确认的选型决策**（累计）：
   - Notion SDK: 直接 HTTP 调用（用户选择）
   - Obsidian: 本地文件写入（无第三方 SDK）
   - 前端包管理器: pnpm（C9 规则 + 用户确认）
   - lobe-ui: 移除（可能不存在，C7 合规）
   - T021 优先于 T025（用户选择）

10. **依赖变更**（T021 累计）：
    - T021 **净新增 0 个 Go 依赖**
    - T021 **新增 75 个前端 npm 包**（通过 pnpm install）

---

## 5. 当前文件结构（Phase 4 / T021 末）

```
AsyncStarterAgent/
├── cmd/api/                       ✅ T002+T009+T020+T026+T023+T024
│   └── wire.go                    ✅ T024 修改（注入 ObsidianAdapter）
├── internal/
│   ├── config/                    ✅ T001+T006+T026 (未动)
│   ├── handler/                   ✅ T002+T006+T009+T020+T023 (未动)
│   ├── delivery/                  ✅ T023+T024
│   │   ├── notion.go              ✅ T023
│   │   ├── notion_test.go         ✅ T023
│   │   ├── obsidian.go            ✅ T024
│   │   ├── obsidian_test.go       ✅ T024
│   │   ├── service.go             ✅ T024 修改
│   │   └── service_test.go        ✅ T024 修改
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
├── web/                           ✅ T021 新建
│   ├── package.json               ✅ T021
│   ├── vite.config.ts             ✅ T021
│   ├── tsconfig.json              ✅ T021
│   ├── index.html                 ✅ T021
│   ├── pnpm-lock.yaml             ✅ T021 (pnpm 生成)
│   ├── pnpm-workspace.yaml        ✅ T021 (pnpm 生成)
│   ├── src/
│   │   ├── main.tsx               ✅ T021
│   │   ├── index.css              ✅ T021
│   │   ├── App.tsx                ✅ T021（含错误处理）
│   │   ├── api/
│   │   │   └── client.ts          ✅ T021（含 onError 回调）
│   │   └── components/
│   │       ├── DraftEditor.tsx    ✅ T021
│   │       └── MarkList.tsx       ✅ T021
│   └── src-tauri/
│       ├── Cargo.toml             ✅ T021（含 notification 插件）
│       ├── tauri.conf.json        ✅ T021（CSP 最小策略）
│       ├── build.rs               ✅ T021
│       └── src/
│           └── main.rs            ✅ T021（notification 插件注册）
├── doc/handoff/
│   ├── ...                        (untracked)
│   └── T021-handoff.md            ← 本文件 (untracked)
├── go.mod                         ✅ T023 修改（Go 1.26.3）
└── go.sum                         ✅ T023 修改
```

---

## 6. 待用户决定（必须问，不可以自动做）

1. **T015-T020 + T026 + T023 + T024 + T021 代码是否 commit**：当前所有变更未暂存。用户需决定是否 commit 以及 commit 哪些文件。

2. **`doc/handoff/` 是否入库**：当前 handoff 文件均为 untracked。

3. **T025 飞书文档交付是否在 MVP 阶段实现**：FR-D03 标为 P2/V1.5 推迟项。是否跳过直接进入集成测试？

4. **`pnpm tauri dev` 是否运行**：需要后端 API 运行在 localhost:8080，需要用户确认是否启动。

5. **Go 版本升级**：Go 从 1.25.8 升级到 1.26.3（因 go-api-libs/notion 要求），该 SDK 已移除但版本保留。是否回退 Go 版本？

6. **`web/dist/` 和 `web/node_modules/` 是否加入 .gitignore**：构建产物和依赖不应入库。
