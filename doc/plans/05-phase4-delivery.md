# Phase 4: 前端与交付（W10-W13）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **AI 行为约束**: 严格遵守 [ai-coding-boundary.md](../../ai-coding-boundary.md)。任何偏离需先询问。

**Goal**: 实现模块 D（交付管道）+ Tauri v2 桌面客户端，跑通 5 个用户故事（US-01~05）。

**关联需求**:
- FR-D01 (P0): Notion 文档创建
- FR-D02 (P1): Obsidian Vault 写入
- FR-D03 (P2): 飞书文档创建
- FR-D04 (P0): 任务备注更新
- FR-D05 (P1): 通知推送（桌面通知）
- T021 (P0): Tauri 前端
- T022 (P1, 可选): Flutter 备选
- T023-T025: 3 个交付实现

**退出标准（M4）**:
- [ ] 5 个用户故事端到端通过
- [ ] Notion API 集成测试通过
- [ ] Obsidian 集成测试通过
- [ ] 飞书集成测试通过（如 T025 仍属 V1.5 范围则跳过）
- [ ] 任务备注更新到原数据源
- [ ] 桌面通知在草稿完成时触发
- [ ] Tauri 客户端能跑 5 个核心场景

**目录新增**:
```
internal/delivery/
├── service.go            # 交付服务
├── service_test.go
├── notion.go             # Notion 适配器
├── notion_test.go
├── obsidian.go           # Obsidian 适配器
├── obsidian_test.go
├── feishu_doc.go         # 飞书文档适配器
├── feishu_doc_test.go
├── notifier.go           # 桌面通知
└── notifier_test.go
web/                     # Tauri 前端
├── src-tauri/
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── components/
│   ├── hooks/
│   └── api/
├── package.json
├── vite.config.ts
└── index.html
```

---

## Task T023: Notion API 文档交付

**Files:**
- Create: `internal/delivery/notion.go`
- Create: `internal/delivery/notion_test.go`
- Create: `internal/delivery/service.go`
- Create: `internal/delivery/service_test.go`

**关联**: FR-D01 (P0), FR-D04 (P0)

- [ ] **Step 1: 添加 Notion SDK 依赖**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go get github.com/mattn/go-notion@v0.0.0-20240714132300-32ee69a0c6e8`

> 边界说明：精确版本号在安装时确认。如版本不存在，按 §3.2 询问用户。

- [ ] **Step 2: 写 Notion 适配器**

Create file `internal/delivery/notion.go`:
```go
package delivery

import (
	"context"
	"fmt"
	"strings"
	"time"

	notion "github.com/mattn/go-notion"
)

type NotionConfig struct {
	APIKey       string
	ParentPageID string
}

type NotionAdapter struct {
	cfg NotionConfig
	cli *notion.Client
}

func NewNotionAdapter(cfg NotionConfig) *NotionAdapter {
	cli := notion.NewClient(cfg.APIKey)
	return &NotionAdapter{cfg: cfg, cli: cli}
}

// CreatePage 创建 Notion 页面
func (a *NotionAdapter) CreatePage(ctx context.Context, title, markdown string) (string, error) {
	blocks := markdownToBlocks(markdown)
	page := &notion.Page{
		Parent: notion.Parent{
			PageID: a.cfg.ParentPageID,
		},
		Properties: notion.PageProperties{
			Title: []notion.RichText{
				{Type: "text", Text: notion.Text{Content: title}},
			},
		},
		Children: blocks,
	}
	p, err := a.cli.CreatePage(ctx, page)
	if err != nil {
		return "", fmt.Errorf("notion create: %w", err)
	}
	return p.URL, nil
}

// markdownToBlocks 将 markdown 转换为 Notion blocks（简化版：每行一段落）
func markdownToBlocks(md string) []notion.Block {
	lines := strings.Split(md, "\n")
	blocks := make([]notion.Block, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		blocks = append(blocks, notion.Block{
			Object: "block",
			Type:   "paragraph",
			Paragraph: &notion.Paragraph{
				Text: []notion.RichText{
					{Type: "text", Text: notion.Text{Content: line}},
				},
			},
		})
	}
	return blocks
}

// UpdatePageComment 在原 Todoist 任务下加备注（实际为 Notion 页面 comment）
func (a *NotionAdapter) UpdateTaskComment(ctx context.Context, pageID, comment string) error {
	// 简化：post comment 到 Notion 页面
	_, err := a.cli.CreateComment(ctx, &notion.Comment{
		Parent: notion.Parent{PageID: pageID},
		RichText: []notion.RichText{
			{Type: "text", Text: notion.Text{Content: comment}},
		},
	})
	return err
}

// Time helper (used in tests)
var _ = time.Now
```

- [ ] **Step 3: 写 Notion 测试（mock HTTP）**

Create file `internal/delivery/notion_test.go`:
```go
package delivery

import "testing"

func TestMarkdownToBlocks(t *testing.T) {
	md := "# Title\n\nFirst paragraph.\n\nSecond paragraph.\n"
	blocks := markdownToBlocks(md)
	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "paragraph" {
		t.Errorf("type: %s", blocks[0].Type)
	}
}

func TestMarkdownToBlocks_EmptyLines(t *testing.T) {
	md := "\n\n# Title\n\n\n"
	blocks := markdownToBlocks(md)
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}
}
```

- [ ] **Step 4: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/delivery/...`
Expected: PASS

- [ ] **Step 5: 写 Delivery service**

Create file `internal/delivery/service.go`:
```go
package delivery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool    *pgxpool.Pool
	notion  *NotionAdapter
	obs     *ObsidianAdapter
	feishu  *FeishuDocAdapter
	notif   Notifier
}

func NewService(pool *pgxpool.Pool, n *NotionAdapter, o *ObsidianAdapter, f *FeishuDocAdapter, nf Notifier) *Service {
	return &Service{pool: pool, notion: n, obs: o, feishu: f, notif: nf}
}

// DeliverResult 交付结果
type DeliverResult struct {
	DeliveryID string
	TargetURL  string
	Status     string
}

// Deliver 草稿交付
func (s *Service) Deliver(ctx context.Context, draftID, targetType string) (*DeliverResult, error) {
	// 1. 查询草稿
	row := s.pool.QueryRow(ctx, `SELECT agent_run_id, title, markdown_content FROM drafts WHERE id = $1`, draftID)
	var runID uuid.UUID
	var title, md string
	if err := row.Scan(&runID, &title, &md); err != nil {
		return nil, fmt.Errorf("load draft: %w", err)
	}

	// 2. 创建 delivery 记录（pending）
	deliveryID := uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO deliveries (id, draft_id, target_type, status)
		VALUES ($1, $2, $3, 'pending')
	`, deliveryID, draftID, targetType)
	if err != nil {
		return nil, err
	}

	// 3. 根据 target_type 路由
	var targetURL string
	switch targetType {
	case "notion":
		targetURL, err = s.notion.CreatePage(ctx, title, md)
	case "obsidian":
		targetURL, err = s.obs.WriteFile(ctx, title, md)
	case "feishu":
		targetURL, err = s.feishu.CreateDoc(ctx, title, md)
	default:
		return nil, fmt.Errorf("unknown target: %s", targetType)
	}

	// 4. 更新 delivery 状态
	status := "success"
	if err != nil {
		status = "failed"
		_, _ = s.pool.Exec(ctx, `UPDATE deliveries SET status = $1, error_message = $2, updated_at = $3 WHERE id = $4`,
			status, err.Error(), time.Now(), deliveryID)
		return &DeliverResult{DeliveryID: deliveryID.String(), Status: status}, err
	}
	_, err = s.pool.Exec(ctx, `UPDATE deliveries SET target_url = $1, status = $2, updated_at = $3 WHERE id = $4`,
		targetURL, status, time.Now(), deliveryID)
	if err != nil {
		return nil, err
	}

	// 5. FR-D04: 更新原任务备注
	if err := s.updateSourceComment(ctx, runID, targetType, targetURL, title); err != nil {
		// 备注失败不阻塞主交付
		fmt.Printf("[delivery] update source comment: %v\n", err)
	}

	// 6. FR-D05: 桌面通知
	if s.notif != nil {
		_ = s.notif.Notify(ctx, "草稿已交付", title)
	}

	return &DeliverResult{DeliveryID: deliveryID.String(), TargetURL: targetURL, Status: status}, nil
}

func (s *Service) updateSourceComment(ctx context.Context, runID uuid.UUID, targetType, url, title string) error {
	// 简化：直接查 agent_run 的 trigger_source，加 comment
	row := s.pool.QueryRow(ctx, `SELECT trigger_source FROM agent_runs WHERE id = $1`, runID)
	var src sql.NullString
	if err := row.Scan(&src); err != nil {
		return err
	}
	if !src.Valid {
		return errors.New("no source")
	}
	switch targetType {
	case "notion":
		return s.notion.UpdateTaskComment(ctx, extractPageID(url), fmt.Sprintf("已生成: %s", title))
	}
	return nil
}

func extractPageID(url string) string {
	// 简化：取 URL 最后一段
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			return url[i+1:]
		}
	}
	return url
}
```

> **修复 import**: 上面 `service.go` 用到 `sql` 是为 sql.NullString。考虑去掉，改为 `*string` 或直接 `string` + 零值。改为：

Modify `internal/delivery/service.go` — 删除 `"database/sql"`，改 Scan:
```go
var src string
_ = row.Scan(&src)
```

- [ ] **Step 6: 写 service 测试（mock 所有 adapter）**

Create file `internal/delivery/service_test.go`:
```go
package delivery

import (
	"context"
	"testing"
)

type mockNotifier struct{ lastTitle string }

func (m *mockNotifier) Notify(_ context.Context, _, title string) error {
	m.lastTitle = title
	return nil
}

func TestExtractPageID(t *testing.T) {
	cases := map[string]string{
		"https://notion.so/Page-abc123":          "abc123",
		"https://notion.so/abc123":                "abc123",
		"https://example.com/x/y/xyz-987":         "xyz-987",
	}
	for in, want := range cases {
		if got := extractPageID(in); got != want {
			t.Errorf("%s: want %s, got %s", in, want, got)
		}
	}
}
```

- [ ] **Step 7: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 8: 展示 diff 等用户决定**

---

## Task T024: Obsidian Vault 交付

**Files:**
- Create: `internal/delivery/obsidian.go`
- Create: `internal/delivery/obsidian_test.go`

**关联**: FR-D02 (P1) — 标注 V1.5 推迟项

> **边界说明**: 本任务在 `plan-boundary.md` 中标为 V1.5 推迟项。task-tracker.html 标 P1 但延期到 MVP 后。本任务给最小可工作实现，**不与 Notion 优先级冲突**。

- [ ] **Step 1: 写 Obsidian 适配器**

Create file `internal/delivery/obsidian.go`:
```go
package delivery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ObsidianConfig struct {
	VaultPath string
	Subdir    string // 默认 "Generated"
}

type ObsidianAdapter struct {
	cfg ObsidianConfig
}

func NewObsidianAdapter(cfg ObsidianConfig) *ObsidianAdapter {
	if cfg.Subdir == "" {
		cfg.Subdir = "Generated"
	}
	return &ObsidianAdapter{cfg: cfg}
}

func (a *ObsidianAdapter) WriteFile(_ context.Context, title, markdown string) (string, error) {
	dir := filepath.Join(a.cfg.VaultPath, a.cfg.Subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	filename := sanitizeFilename(title) + ".md"
	path := filepath.Join(dir, filename)
	body := "---\ncreated: " + time.Now().Format(time.RFC3339) + "\n---\n\n" + markdown
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	return "file://" + path, nil
}

func sanitizeFilename(s string) string {
	invalid := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	for _, c := range invalid {
		s = strings.ReplaceAll(s, c, "_")
	}
	return strings.TrimSpace(s)
}
```

- [ ] **Step 2: 写 Obsidian 测试**

Create file `internal/delivery/obsidian_test.go`:
```go
package delivery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestObsidianAdapter_WriteFile(t *testing.T) {
	dir := t.TempDir()
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	url, err := a.WriteFile(context.Background(), "My Draft", "# Hello\n\nContent")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(filepath.FromSlash(url[7:])) {
		t.Errorf("not absolute: %s", url)
	}
	// 验证文件存在
	path := filepath.Join(dir, "Generated", "My Draft.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "# Hello") {
		t.Errorf("body: %s", body)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"valid name":      "valid name",
		"with/slash":      "with_slash",
		"with:colon":      "with_colon",
		"with?question":   "with_question",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("%q: want %q, got %q", in, want, got)
		}
	}
}
```

- [ ] **Step 3: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/delivery/...`
Expected: PASS

- [ ] **Step 4: 展示 diff 等用户决定**

---

## Task T025: 飞书文档交付

**Files:**
- Create: `internal/delivery/feishu_doc.go`
- Create: `internal/delivery/feishu_doc_test.go`

**关联**: FR-D03 (P2) — 标注 V1.5 推迟项

> **边界说明**: 本任务在 `plan-boundary.md` 中标为 V1.5 推迟项。MVP 阶段给接口骨架，集成测试留到 V1.5。

- [ ] **Step 1: 写飞书文档适配器接口**

Create file `internal/delivery/feishu_doc.go`:
```go
package delivery

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdoc "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

type FeishuDocConfig struct {
	AppID     string
	AppSecret string
	FolderToken string
}

type FeishuDocAdapter struct {
	cfg FeishuDocConfig
	cli *lark.Client
}

func NewFeishuDocAdapter(cfg FeishuDocConfig) *FeishuDocAdapter {
	cli := lark.NewClient(cfg.AppID, cfg.AppSecret)
	return &FeishuDocAdapter{cfg: cfg, cli: cli}
}

func (a *FeishuDocAdapter) CreateDoc(ctx context.Context, title, markdown string) (string, error) {
	// 1. 创建空白文档
	req := larkdoc.NewCreateDocumentReqBuilder().
		Title(title).
		FolderToken(a.cfg.FolderToken).
		Build()
	resp, err := a.cli.Docx.Document.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create doc: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("create doc: code=%d msg=%s", resp.Code, resp.Msg)
	}
	docID := *resp.Data.Document.DocumentId
	url := *resp.Data.Document.URL

	// 2. 写入内容（简化：调用 raw_content API）
	// 真实实现按飞书 v1 协议转换 markdown → block
	// 本 MVP 接口骨架先返回 URL，V1.5 补内容写入
	return url, nil
}
```

- [ ] **Step 2: 写飞书测试（构造测试）**

Create file `internal/delivery/feishu_doc_test.go`:
```go
package delivery

import "testing"

func TestFeishuDocAdapter_New(t *testing.T) {
	a := NewFeishuDocAdapter(FeishuDocConfig{AppID: "x", AppSecret: "y"})
	if a == nil {
		t.Fatal("nil adapter")
	}
}
```

- [ ] **Step 3: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/delivery/...`
Expected: PASS

- [ ] **Step 4: 展示 diff 等用户决定**

---

## Task T021: 方案一 Tauri 前端原型

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/components/DraftEditor.tsx`
- Create: `web/src/components/MarkList.tsx`
- Create: `web/src/api/client.ts`
- Create: `web/src-tauri/Cargo.toml`
- Create: `web/src-tauri/tauri.conf.json`
- Create: `web/src-tauri/src/main.rs`

**关联**: T021 (P0), 客户端 MVP

> **边界说明**: Tauri 前端是 T021 的核心，但启动 T021 需先安装 Tauri 工具链。执行前应询问用户是否安装。

> **Tauri 安装询问 (按 ai-coding-boundary §3.5)**:

- [ ] **Step 0: 询问用户是否安装 Tauri**

AskUserQuestion:
- 题目：Tauri 前端需要安装 Rust + Node.js 工具链，是否现在安装？
- 选项 A: 安装完整工具链（Rust + Node + Tauri CLI）— 推荐
- 选项 B: 仅写代码不安装，由用户自行构建
- 选项 C: 改用方案二 Flutter（重写 plan）

> 此询问是 ai-coding-boundary §3.5 "外部依赖"触发的必问项，必须在执行前得到答复。

- [ ] **Step 1: 写 web/package.json**

Create file `web/package.json`:
```json
{
  "name": "asyncstarter-web",
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "tauri": "tauri"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "@tauri-apps/api": "^2.0.0",
    "lobe-ui": "^1.5.0"
  },
  "devDependencies": {
    "@tauri-apps/cli": "^2.0.0",
    "@types/react": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.0",
    "typescript": "^5.5.0",
    "vite": "^5.4.0"
  }
}
```

- [ ] **Step 2: 写 web/vite.config.ts**

Create file `web/vite.config.ts`:
```typescript
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 1420,
    strictPort: true,
  },
  envPrefix: ["VITE_", "TAURI_"],
  build: {
    target: "es2021",
    sourcemap: true,
  },
});
```

- [ ] **Step 3: 写 web/index.html**

Create file `web/index.html`:
```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>AsyncStarterAgent</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 4: 写 main.tsx**

Create file `web/src/main.tsx`:
```typescript
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

Create file `web/src/index.css`:
```css
:root {
  --bg: #0b1120;
  --surface: #172033;
  --text: #e2e8f0;
  --accent: #6366f1;
}

* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font-family: -apple-system, "Segoe UI", sans-serif;
  min-height: 100vh;
}
```

- [ ] **Step 5: 写 App.tsx（主界面：草稿编辑器 + 标记列表）**

Create file `web/src/App.tsx`:
```typescript
import { useEffect, useState } from "react";
import { DraftEditor } from "./components/DraftEditor";
import { MarkList } from "./components/MarkList";
import { streamDraft, type Draft, type Mark } from "./api/client";

export default function App() {
  const [draft, setDraft] = useState<Draft>({ content: "", marks: [], completeness: 0 });
  const [runId, setRunId] = useState<string | null>(null);
  const [userId] = useState(crypto.randomUUID());

  const handleTrigger = async (text: string) => {
    // 1. 创建 AgentRun
    const res = await fetch("http://localhost:8080/api/v1/trigger", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ user_id: userId, text }),
    });
    const data = await res.json();
    setRunId(data.data.run_id);
  };

  useEffect(() => {
    if (!runId) return;
    // 2. 订阅 SSE
    const cleanup = streamDraft(runId, (chunk) => {
      setDraft((d) => ({ ...d, content: d.content + chunk }));
    }, (marks: Mark[]) => {
      setDraft((d) => ({ ...d, marks }));
    });
    return cleanup;
  }, [runId]);

  return (
    <div className="app">
      <header>
        <h1>"完成前 30%" 异步起跑器</h1>
      </header>
      <main>
        <div className="trigger-bar">
          <input
            type="text"
            placeholder="说点什么触发：'写周报'、'总结'、'规划'..."
            onKeyDown={(e) => {
              if (e.key === "Enter") handleTrigger((e.target as HTMLInputElement).value);
            }}
          />
        </div>
        <div className="content">
          <DraftEditor draft={draft} />
          <MarkList marks={draft.marks} />
        </div>
      </main>
    </div>
  );
}
```

- [ ] **Step 6: 写 API client（含 SSE 订阅）**

Create file `web/src/api/client.ts`:
```typescript
export interface Mark {
  id: string;
  hint: string;
  position: number;
  resolved: boolean;
}

export interface Draft {
  content: string;
  marks: Mark[];
  completeness: number;
}

export function streamDraft(
  runId: string,
  onDelta: (text: string) => void,
  onComplete: (marks: Mark[]) => void
): () => void {
  const es = new EventSource(`http://localhost:8080/api/v1/drafts/${runId}/stream`);
  es.addEventListener("delta", (e) => {
    const data = JSON.parse((e as MessageEvent).data);
    onDelta(data.text);
  });
  es.addEventListener("complete", (e) => {
    const data = JSON.parse((e as MessageEvent).data);
    onComplete(data.marks ?? []);
    es.close();
  });
  es.addEventListener("error", () => {
    es.close();
  });
  return () => es.close();
}
```

- [ ] **Step 7: 写 DraftEditor 组件**

Create file `web/src/components/DraftEditor.tsx`:
```typescript
import type { Draft } from "../api/client";

export function DraftEditor({ draft }: { draft: Draft }) {
  return (
    <div className="draft-editor">
      <div className="completeness">
        完成度: {Math.round(draft.completeness * 100)}%
      </div>
      <textarea
        value={draft.content}
        readOnly
        rows={20}
        placeholder="草稿将在这里流式出现..."
      />
    </div>
  );
}
```

- [ ] **Step 8: 写 MarkList 组件**

Create file `web/src/components/MarkList.tsx`:
```typescript
import type { Mark } from "../api/client";

export function MarkList({ marks }: { marks: Mark[] }) {
  if (marks.length === 0) return null;
  return (
    <div className="mark-list">
      <h3>[待补充] 标记 ({marks.length})</h3>
      <ul>
        {marks.map((m) => (
          <li key={m.id}>
            <span className="hint">{m.hint}</span>
            <input type="text" placeholder="补全内容" />
          </li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 9: 写 Tauri 配置**

Create file `web/src-tauri/Cargo.toml`:
```toml
[package]
name = "asyncstarter"
version = "0.1.0"
edition = "2021"

[build-dependencies]
tauri-build = { version = "2", features = [] }

[dependencies]
tauri = { version = "2", features = [] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"

[[bin]]
name = "asyncstarter"
path = "src/main.rs"
```

Create file `web/src-tauri/tauri.conf.json`:
```json
{
  "$schema": "https://schema.tauri.app/config/2",
  "productName": "AsyncStarter",
  "version": "0.1.0",
  "identifier": "com.asyncstarter.app",
  "build": {
    "devUrl": "http://localhost:1420",
    "frontendDist": "../dist"
  },
  "app": {
    "windows": [
      {
        "title": "AsyncStarterAgent",
        "width": 1200,
        "height": 800,
        "resizable": true
      }
    ],
    "security": {
      "csp": null
    }
  }
}
```

Create file `web/src-tauri/src/main.rs`:
```rust
fn main() {
    tauri::Builder::default()
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

- [ ] **Step 10: 写 Tauri 通知插件（FR-D05 桌面通知）**

Modify `web/src-tauri/Cargo.toml` — 添加 notification 插件:
```toml
[dependencies]
tauri = { version = "2", features = [] }
tauri-plugin-notification = "2"
```

Modify `web/src-tauri/src/main.rs`:
```rust
fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_notification::init())
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

- [ ] **Step 11: 安装依赖**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent\web"
npm install
```
Expected: 依赖安装成功。

- [ ] **Step 12: 启动开发服务器**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent\web"
npm run tauri dev
```
Expected: Tauri 窗口打开，显示主界面

- [ ] **Step 13: 端到端：输入"写周报" → 草稿流式出现**

Manual: 在 Tauri 窗口输入框输入 "写周报"，按 Enter → 等待几秒 → 草稿内容出现

- [ ] **Step 14: 展示 diff 等用户决定**

---

## Task T022: 方案二 Flutter 前端原型（可选）

**Files:** 暂无（资源紧张时跳过）

**关联**: T022 (P1, 备选)

> **边界说明**: 本任务为 MVP-PLUS 项。MVP 阶段 T021 完成即可，T022 视 T021 进度决定是否启动。默认跳过。

---

## Phase 4 退出标准验证

完成 T021-T025 后，逐项验证 M4 退出标准：

- [ ] `go test ./internal/delivery/...` → 全部 PASS
- [ ] Notion 适配器单元测试通过
- [ ] Obsidian 适配器单元测试通过
- [ ] 飞书文档适配器接口骨架就绪
- [ ] 桌面通知（Tauri notification plugin）就绪
- [ ] Tauri 客户端能跑：触发 → SSE 订阅 → 草稿流式出现
- [ ] 5 个用户故事（US-01~05）端到端跑通
- [ ] 更新 [task-tracker.html](../../task-tracker.html) 中 T021-T025 状态

---

## 5 个用户故事端到端验证

| 故事 | 场景 | 验证步骤 |
|---|---|---|
| US-01 | 独立开发者收到 GitHub commit 推送 | 1) 手动触发 "写本周周报" 2) 等待 RAG 检索 3) 草稿出现 commit 摘要 4) 标记 [待补充:影响]  5) 提交到 Notion |
| US-02 | 项目经理整理会议纪要 | 1) 手动触发 "整理会议纪要" 2) 拉取飞书消息 3) 草稿生成 4) 提交到 Notion |
| US-03 | 产品经理做下周规划 | 1) DDL 触发 24h 内的"规划下季度"任务 2) 拉取 Obsidian 笔记 3) 草稿生成 4) 提交到 Obsidian |
| US-04 | 关键词误触发检测 | 1) 输入"买菜" 2) 关键词不匹配 3) 400 错误 |
| US-05 | [待补充] 标记定位 | 1) 草稿生成后 2) MarkList 显示 3) 用户输入补全 4) draft 更新 |

---

**Phase 4 完成 = MVP 发布**。所有 13 项 CORE 需求 + 5 项 NFR 已实现，5 个用户故事跑通。

**发布前 checklist**:
- [ ] 全部 26 个任务状态更新为"已完成"
- [ ] task-tracker.html 状态同步
- [ ] 7 项关键指标（plan-boundary §9）达到
- [ ] 0 个 P0 缺陷
- [ ] 编写 release notes
- [ ] 文档归档到 `doc/release/v0.1.0/`

**下一步**: 进入 V1.5 迭代规划（不在本计划范围）。
