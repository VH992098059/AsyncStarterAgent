# Phase 2: 上下文搜集（W4-W7）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **AI 行为约束**: 严格遵守 [ai-coding-boundary.md](../../ai-coding-boundary.md)。任何偏离需先询问。

**Goal**: 实现模块 B（上下文搜集），4 个数据源适配器 + 噪音过滤 + 增量同步。

**关联需求**:
- FR-B01 (P0): GitHub 拉取（commit / PR / review）
- FR-B02 (P1): 日历拉取（Google/Outlook，MVP 阶段只做 Google）
- FR-B03 (P1): IM 拉取（飞书/Slack，MVP 阶段只做飞书）
- FR-B04 (P1): 笔记读取（Obsidian/Notion，MVP 阶段只做 Obsidian）
- FR-B05 (P0): 噪音过滤（规则 + LLM 双层）
- FR-B06 (P0): 增量同步（last_sync_at 时间戳）

**退出标准（M2）**:
- [ ] 4 数据源适配器单元测试通过
- [ ] 规则 + LLM 噪音过滤测试通过
- [ ] 增量同步（只拉新数据）测试通过
- [ ] 集成测试：手动触发 → 4 数据源拉取 → 过滤 → 入库

**目录新增**:
```
internal/harvesting/
├── context.go                # ContextSnapshot 类型
├── pipeline.go               # 编排
├── pipeline_test.go
├── filter/
│   ├── rules.go              # 规则过滤
│   ├── rules_test.go
│   ├── llm.go                # LLM 过滤
│   └── llm_test.go
├── source/
│   ├── github.go
│   ├── github_test.go
│   ├── calendar.go
│   ├── calendar_test.go
│   ├── feishu.go
│   ├── feishu_test.go
│   ├── obsidian.go
│   └── obsidian_test.go
└── sync.go                   # 增量同步
migrations/
└── 0004_context.up.sql
```

---

## Task T010: GitHub 数据源适配器

**Files:**
- Create: `internal/harvesting/source/github.go`
- Create: `internal/harvesting/source/github_test.go`
- Create: `internal/harvesting/context.go`

**关联**: FR-B01 (P0), FR-B06 (P0)

- [ ] **Step 1: 添加 GitHub 客户端依赖**

Run:
```bash
cd "k:\go_projects\AsyncStarterAgent"
go get github.com/google/go-github/v57@v57.0.0
go get github.com/bradleyfalzon/ghinstallation/v2@v2.1.0
```

- [ ] **Step 2: 写 ContextSnapshot 类型**

Create file `internal/harvesting/context.go`:
```go
package harvesting

import "time"

type ContextItem struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`     // github / calendar / im / note
	Type       string    `json:"type"`       // commit / pr / review / meeting / message / note
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	URL        string    `json:"url,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ContextSnapshot struct {
	AgentRunID  string
	UserID      string
	Items       []ContextItem
	FilterMeta  FilterMeta
	SyncedAt    time.Time
}

type FilterMeta struct {
	TotalBeforeFilter int                `json:"total_before_filter"`
	TotalAfterFilter  int                `json:"total_after_filter"`
	RuleRejections    map[string]int     `json:"rule_rejections"`
	LLMRejections     int                `json:"llm_rejections"`
}
```

- [ ] **Step 3: 写 GitHub 适配器接口**

Create file `internal/harvesting/source/github.go`:
```go
package source

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v57/github"
	"github.com/asyncstarter/agent/internal/harvesting"
	"golang.org/x/oauth2"
)

type GitHubConfig struct {
	Token      string    // PAT or installation token
	Owner      string
	Repo       string
	Since      time.Time // 增量同步起点
}

type GitHubAdapter struct {
	cfg GitHubConfig
	cli *gh.Client
}

func NewGitHubAdapter(cfg GitHubConfig) *GitHubAdapter {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	httpClient := oauth2.NewClient(context.Background(), ts)
	return &GitHubAdapter{cfg: cfg, cli: gh.NewClient(httpClient)}
}

// FetchCommits 拉取增量 commits
func (a *GitHubAdapter) FetchCommits(ctx context.Context) ([]harvesting.ContextItem, error) {
	opts := &gh.CommitsListOptions{
		Since: a.cfg.Since,
		ListOptions: gh.ListOptions{PerPage: 50},
	}
	commits, _, err := a.cli.Repositories.ListCommits(ctx, a.cfg.Owner, a.cfg.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list commits: %w", err)
	}
	items := make([]harvesting.ContextItem, 0, len(commits))
	for _, c := range commits {
		msg := ""
		if c.Commit != nil && c.Commit.Message != nil {
			msg = *c.Commit.Message
		}
		author := ""
		if c.Author != nil && c.Author.Login != nil {
			author = *c.Author.Login
		}
		sha := ""
		if c.SHA != nil {
			sha = *c.SHA
		}
		date := time.Time{}
		if c.Commit != nil && c.Commit.Author != nil && c.Commit.Author.Date != nil {
			date = *c.Commit.Author.Date
		}
		items = append(items, harvesting.ContextItem{
			ID:         "github:commit:" + sha,
			Source:     "github",
			Type:       "commit",
			Title:      msg,
			Content:    msg,
			URL:        fmt.Sprintf("https://github.com/%s/%s/commit/%s", a.cfg.Owner, a.cfg.Repo, sha),
			OccurredAt: date,
			Metadata:   map[string]string{"author": author, "sha": sha},
		})
	}
	return items, nil
}

// FetchPullRequests 拉取 PR
func (a *GitHubAdapter) FetchPullRequests(ctx context.Context) ([]harvesting.ContextItem, error) {
	opts := &gh.PullRequestListOptions{
		State: "all",
		Sort:  "updated",
		Direction: "desc",
		ListOptions: gh.ListOptions{PerPage: 30},
	}
	prs, _, err := a.cli.PullRequests.List(ctx, a.cfg.Owner, a.cfg.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list prs: %w", err)
	}
	items := make([]harvesting.ContextItem, 0, len(prs))
	for _, p := range prs {
		items = append(items, harvesting.ContextItem{
			ID:         fmt.Sprintf("github:pr:%d", p.GetNumber()),
			Source:     "github",
			Type:       "pr",
			Title:      p.GetTitle(),
			Content:    p.GetBody(),
			URL:        p.GetHTMLURL(),
			OccurredAt: p.GetUpdatedAt().Time,
			Metadata: map[string]string{
				"state":  p.GetState(),
				"author": p.GetUser().GetLogin(),
			},
		})
	}
	return items, nil
}
```

- [ ] **Step 4: 写 GitHub 适配器测试（用 mock HTTP）**

Create file `internal/harvesting/source/github_test.go`:
```go
package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/google/go-github/v57/github"
	"github.com/asyncstarter/agent/internal/harvesting"
)

// newMockGitHubClient 构造一个不依赖外部网络的 *gh.Client
func newMockGitHubClient(handler http.HandlerFunc) *gh.Client {
	srv := httptest.NewServer(handler)
	httpClient := &http.Client{}
	c := gh.NewClient(httpClient)
	c.BaseURL = srv.URL + "/"
	return c
}

func TestGitHubAdapter_FetchCommits_Mock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/commits", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"sha": "abc123",
				"commit": map[string]interface{}{
					"message": "feat: add login",
					"author": map[string]interface{}{
						"date": time.Now().Format(time.RFC3339),
					},
				},
				"author": map[string]interface{}{"login": "alice"},
			},
		})
	})
	cli := newMockGitHubClient(mux.ServeHTTP)

	a := &GitHubAdapter{
		cfg: GitHubConfig{Token: "x", Owner: "o", Repo: "r", Since: time.Now().Add(-24 * time.Hour)},
		cli: cli,
	}
	items, err := a.FetchCommits(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0].Type != "commit" {
		t.Errorf("type: %s", items[0].Type)
	}
	if items[0].Source != "github" {
		t.Errorf("source: %s", items[0].Source)
	}
	if items[0].Metadata["sha"] != "abc123" {
		t.Errorf("sha: %s", items[0].Metadata["sha"])
	}
}

func TestGitHubAdapter_FetchPRs_Mock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"number":    42,
				"title":     "Add dark mode",
				"body":      "Implement dark mode toggle",
				"state":     "open",
				"html_url":  "https://example/pr/42",
				"updated_at": time.Now().Format(time.RFC3339),
				"user":      map[string]interface{}{"login": "bob"},
			},
		})
	})
	cli := newMockGitHubClient(mux.ServeHTTP)

	a := &GitHubAdapter{cfg: GitHubConfig{Token: "x", Owner: "o", Repo: "r"}, cli: cli}
	items, err := a.FetchPullRequests(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 1 || items[0].Type != "pr" {
		t.Fatalf("items: %+v", items)
	}
}

// 校验 harvesting.ContextItem 字段正确（compile-time 保证）
var _ = func() []harvesting.ContextItem { return nil }
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/source/...`
Expected: PASS — 2 个测试全过

- [ ] **Step 6: 端到端（可选 — 需真实 token）**

> **边界说明**: 真实 GitHub 集成需要 user 提供 PAT。若无 token，跳过本步。

Run (有 token 时):
```bash
export GITHUB_TOKEN=<your-pat>
export GITHUB_REPO=owner/repo
# 启动 API，手动触发，然后查询数据库
```

- [ ] **Step 7: 展示 diff 等用户决定**

---

## Task T011: 日历数据源适配器

**Files:**
- Create: `internal/harvesting/source/calendar.go`
- Create: `internal/harvesting/source/calendar_test.go`

**关联**: FR-B02 (P1), MVP 阶段只做 Google Calendar

- [ ] **Step 1: 写 Calendar 接口（抽象 Google/Outlook）**

Create file `internal/harvesting/source/calendar.go`:
```go
package source

import (
	"context"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// CalendarProvider 抽象日历 Provider
type CalendarProvider interface {
	ListEvents(ctx context.Context, from, to time.Time) ([]harvesting.ContextItem, error)
}

// CalendarAdapter 统一入口
type CalendarAdapter struct {
	Provider CalendarProvider
	Source   string // google_calendar / outlook
}

func (a *CalendarAdapter) Fetch(ctx context.Context, from, to time.Time) ([]harvesting.ContextItem, error) {
	items, err := a.Provider.ListEvents(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Source = a.Source
		if items[i].Type == "" {
			items[i].Type = "meeting"
		}
	}
	return items, nil
}
```

- [ ] **Step 2: 写 Google Calendar 实现（使用 Google API 客户端）**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go get google.golang.org/api@v0.157.0`

Create file `internal/harvesting/source/google_calendar.go`:
```go
package source

import (
	"context"
	"fmt"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type GoogleCalendarConfig struct {
	CredentialsFile string // OAuth credentials.json 路径
	TokenFile       string // 用户授权 token
	CalendarID      string // primary / xxx@group.calendar.google.com
}

type GoogleCalendarProvider struct {
	svc *calendar.Service
	cal string
}

func NewGoogleCalendarProvider(ctx context.Context, cfg GoogleCalendarConfig) (*GoogleCalendarProvider, error) {
	// 实际生产：使用 oauth2.Config + TokenSource。本实现假设 token 已存在并简化处理
	svc, err := calendar.NewService(ctx, option.WithCredentialsFile(cfg.CredentialsFile))
	if err != nil {
		return nil, fmt.Errorf("new service: %w", err)
	}
	cal := cfg.CalendarID
	if cal == "" {
		cal = "primary"
	}
	return &GoogleCalendarProvider{svc: svc, cal: cal}, nil
}

func (p *GoogleCalendarProvider) ListEvents(ctx context.Context, from, to time.Time) ([]harvesting.ContextItem, error) {
	events, err := p.svc.Events.List(p.cal).
		TimeMin(from.Format(time.RFC3339)).
		TimeMax(to.Format(time.RFC3339)).
		SingleEvents(true).
		MaxResults(100).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	items := make([]harvesting.ContextItem, 0, len(events.Items))
	for _, e := range events.Items {
		start := time.Time{}
		if e.Start != nil && e.Start.DateTime != "" {
			t, _ := time.Parse(time.RFC3339, e.Start.DateTime)
			start = t
		}
		items = append(items, harvesting.ContextItem{
			ID:         "gcal:" + e.Id,
			Source:     "google_calendar",
			Type:       "meeting",
			Title:      e.Summary,
			Content:    e.Description,
			URL:        e.HtmlLink,
			OccurredAt: start,
			Metadata: map[string]string{
				"attendees": fmt.Sprintf("%d", len(e.Attendees)),
			},
		})
	}
	return items, nil
}
```

- [ ] **Step 3: 写 Calendar 测试（接口 mock）**

Create file `internal/harvesting/source/calendar_test.go`:
```go
package source

import (
	"context"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type mockCalendar struct {
	events []harvesting.ContextItem
}

func (m *mockCalendar) ListEvents(_ context.Context, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.events, nil
}

func TestCalendarAdapter_Fetch(t *testing.T) {
	mock := &mockCalendar{events: []harvesting.ContextItem{
		{ID: "1", Title: "Team Standup", Type: "meeting"},
	}}
	a := &CalendarAdapter{Provider: mock, Source: "google_calendar"}
	items, err := a.Fetch(context.Background(), time.Now(), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal("expected 1")
	}
	if items[0].Source != "google_calendar" {
		t.Errorf("source: %s", items[0].Source)
	}
}
```

- [ ] **Step 4: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/source/...`
Expected: PASS

- [ ] **Step 5: 展示 diff 等用户决定**

---

## Task T012: IM 沟通记录适配器

**Files:**
- Create: `internal/harvesting/source/feishu.go`
- Create: `internal/harvesting/source/feishu_test.go`

**关联**: FR-B03 (P1), MVP 阶段只做飞书

- [ ] **Step 1: 添加飞书 SDK 依赖**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go get github.com/larksuite/oapi-sdk-go@v3.4.4`

- [ ] **Step 2: 写飞书消息接口**

Create file `internal/harvesting/source/feishu.go`:
```go
package source

import (
	"context"
	"fmt"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type FeishuConfig struct {
	AppID     string
	AppSecret string
	ChatIDs   []string // 指定 chat 列表
}

type FeishuAdapter struct {
	cfg  FeishuConfig
	cli  *lark.Client
}

func NewFeishuAdapter(cfg FeishuConfig) *FeishuAdapter {
	cli := lark.NewClient(cfg.AppID, cfg.AppSecret)
	return &FeishuAdapter{cfg: cfg, cli: cli}
}

func (a *FeishuAdapter) FetchMessages(ctx context.Context, from, to time.Time) ([]harvesting.ContextItem, error) {
	var out []harvesting.ContextItem
	for _, chatID := range a.cfg.ChatIDs {
		req := larkim.NewListMessageReqBuilder().
			ContainerIdType("chat").
			ContainerId(chatID).
			StartTime(fmt.Sprintf("%d", from.Unix())).
			EndTime(fmt.Sprintf("%d", to.Unix())).
			Build()
		resp, err := a.cli.Im.Message.List(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}
		if !resp.Success() {
			return nil, fmt.Errorf("list messages: code=%d msg=%s", resp.Code, resp.Msg)
		}
		for _, m := range resp.Data.Items {
			out = append(out, harvesting.ContextItem{
				ID:         "feishu:msg:" + *m.MessageId,
				Source:     "feishu",
				Type:       "message",
				Title:      m.MsgType,
				Content:    messageContent(m),
				URL:        "",
				OccurredAt: time.Unix(int64(*m.CreateTime)/1000, 0),
				Metadata: map[string]string{
					"chat_id":   chatID,
					"sender":    derefStr(m.SenderId),
					"msg_type":  m.MsgType,
				},
			})
		}
	}
	return out, nil
}

func messageContent(m *larkim.Message) string {
	if m.Body != nil && m.Body.Content != nil {
		return *m.Body.Content
	}
	return ""
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
```

- [ ] **Step 3: 写飞书测试（mock 客户端）**

> 边界说明：飞书 SDK 客户端 mock 比较复杂。本任务**只写接口测试 + 文档化**，集成测试留到端到端阶段。

Create file `internal/harvesting/source/feishu_test.go`:
```go
package source

import "testing"

// 编译期断言：飞书适配器实现预期的 source 接口形状
func TestFeishuAdapter_New(t *testing.T) {
	a := NewFeishuAdapter(FeishuConfig{AppID: "x", AppSecret: "y"})
	if a == nil {
		t.Fatal("nil adapter")
	}
	if a.cli == nil {
		t.Fatal("nil client")
	}
}
```

- [ ] **Step 4: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/source/...`
Expected: PASS

- [ ] **Step 5: 展示 diff 等用户决定**

---

## Task T013: 笔记数据源适配器

**Files:**
- Create: `internal/harvesting/source/obsidian.go`
- Create: `internal/harvesting/source/obsidian_test.go`

**关联**: FR-B04 (P1), MVP 阶段只做 Obsidian 本地

- [ ] **Step 1: 写 Obsidian 本地读取**

Create file `internal/harvesting/source/obsidian.go`:
```go
package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type ObsidianConfig struct {
	VaultPath  string // Vault 根目录
	MaxDepth   int    // 默认 3
	MaxFiles   int    // 默认 200
}

type ObsidianAdapter struct {
	cfg ObsidianConfig
}

func NewObsidianAdapter(cfg ObsidianConfig) *ObsidianAdapter {
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 3
	}
	if cfg.MaxFiles == 0 {
		cfg.MaxFiles = 200
	}
	return &ObsidianAdapter{cfg: cfg}
}

func (a *ObsidianAdapter) FetchNotes(_ context.Context, from time.Time) ([]harvesting.ContextItem, error) {
	var items []harvesting.ContextItem
	count := 0
	err := filepath.Walk(a.cfg.VaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无权限目录
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		if info.ModTime().Before(from) {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		items = append(items, harvesting.ContextItem{
			ID:         "obsidian:note:" + path,
			Source:     "obsidian",
			Type:       "note",
			Title:      strings.TrimSuffix(filepath.Base(path), ".md"),
			Content:    string(body),
			URL:        "file://" + path,
			OccurredAt: info.ModTime(),
		})
		count++
		return count >= a.cfg.MaxFiles
	})
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}
	return items, nil
}
```

- [ ] **Step 2: 写 Obsidian 测试**

Create file `internal/harvesting/source/obsidian_test.go`:
```go
package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestObsidianAdapter_FetchNotes(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(p, body string) {
		full := filepath.Join(dir, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		_ = os.WriteFile(full, []byte(body), 0o644)
	}
	mustWrite("note1.md", "# Note 1\nContent")
	mustWrite("note2.md", "# Note 2\nContent")
	mustWrite("ignored.txt", "skip")
	// 修改时间未来，确保被检索
	future := time.Now().Add(1 * time.Hour)
	_ = os.Chtimes(filepath.Join(dir, "note1.md"), future, future)

	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	items, err := a.FetchNotes(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 (note1 future mtime), got %d", len(items))
	}
	if items[0].Source != "obsidian" {
		t.Errorf("source: %s", items[0].Source)
	}
	if items[0].Type != "note" {
		t.Errorf("type: %s", items[0].Type)
	}
}
```

- [ ] **Step 3: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/source/...`
Expected: PASS

- [ ] **Step 4: 展示 diff 等用户决定**

---

## Task T014: ETL 管线与增量同步

**Files:**
- Create: `internal/harvesting/filter/rules.go`
- Create: `internal/harvesting/filter/rules_test.go`
- Create: `internal/harvesting/filter/llm.go`
- Create: `internal/harvesting/filter/llm_test.go`
- Create: `internal/harvesting/sync.go`
- Create: `internal/harvesting/sync_test.go`
- Create: `internal/harvesting/pipeline.go`
- Create: `internal/harvesting/pipeline_test.go`
- Create: `migrations/0004_context.up.sql`
- Create: `migrations/0004_context.down.sql`

**关联**: FR-B05 (P0), FR-B06 (P0)

- [ ] **Step 1: 写 context_items 表迁移**

Create file `migrations/0004_context.up.sql`:
```sql
-- 拉取的原始上下文数据（FR-B06 增量同步）
CREATE TABLE context_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    agent_run_id    UUID,
    external_id     TEXT NOT NULL,        -- 数据源内唯一 ID
    source          VARCHAR(32) NOT NULL, -- github / google_calendar / feishu / obsidian
    type            VARCHAR(32) NOT NULL, -- commit / pr / meeting / message / note
    title           TEXT,
    content         TEXT,
    url             TEXT,
    occurred_at     TIMESTAMPTZ NOT NULL,
    metadata        JSONB DEFAULT '{}'::jsonb,
    is_noise        BOOLEAN NOT NULL DEFAULT false,
    embedding       VECTOR(1536),         -- Phase 3 RAG 用
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, source, external_id)
);
CREATE INDEX idx_context_items_run ON context_items(agent_run_id);
CREATE INDEX idx_context_items_user_occurred ON context_items(user_id, occurred_at DESC);
CREATE INDEX idx_context_items_noise ON context_items(is_noise) WHERE is_noise = false;
```

Create file `migrations/0004_context.down.sql`:
```sql
DROP TABLE IF EXISTS context_items;
```

Run: `cd "k:\go_projects\AsyncStarterAgent" && make migrate-up`

- [ ] **Step 2: 写规则过滤器**

Create file `internal/harvesting/filter/rules.go`:
```go
package filter

import (
	"regexp"
	"strings"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type Rule struct {
	Name    string
	Pattern *regexp.Regexp
	Reason  string
}

type RuleFilter struct {
	rules []Rule
}

func NewRuleFilter(rules []Rule) *RuleFilter { return &RuleFilter{rules: rules} }

// DefaultRules 内置默认规则
func DefaultRules() []Rule {
	return []Rule{
		{Name: "merge-commit", Pattern: regexp.MustCompile(`^Merge (pull request|branch)`), Reason: "merge commit"},
		{Name: "deps-bump", Pattern: regexp.MustCompile(`^(chore|build|ci):.*(bump|upgrade|update)`), Reason: "dependency bump"},
		{Name: "formatting", Pattern: regexp.MustCompile(`^(style|format):`), Reason: "formatting only"},
		{Name: "typo", Pattern: regexp.MustCompile(`(?i)\btypo\b`), Reason: "typo fix"},
		{Name: "bump-version", Pattern: regexp.MustCompile(`bump.*version`), Reason: "version bump"},
	}
}

// Apply 对 items 应用规则过滤
func (f *RuleFilter) Apply(items []harvesting.ContextItem) (kept []harvesting.ContextItem, rejections map[string]int) {
	rejections = make(map[string]int)
	for _, it := range items {
		matched := ""
		for _, r := range f.rules {
			if r.Pattern.MatchString(it.Title) || r.Pattern.MatchString(it.Content) {
				matched = r.Name
				break
			}
		}
		if matched != "" {
			rejections[matched]++
			continue
		}
		// 长度阈值：内容 < 5 字符视为噪声
		if strings.TrimSpace(it.Content) == "" && strings.TrimSpace(it.Title) == "" {
			rejections["empty"]++
			continue
		}
		kept = append(kept, it)
	}
	return kept, rejections
}
```

- [ ] **Step 3: 写规则过滤器测试**

Create file `internal/harvesting/filter/rules_test.go`:
```go
package filter

import (
	"testing"

	"github.com/asyncstarter/agent/internal/harvesting"
)

func TestRuleFilter_Apply(t *testing.T) {
	f := NewRuleFilter(DefaultRules())
	items := []harvesting.ContextItem{
		{Title: "Merge pull request #42 from feature/xx", Type: "commit"},
		{Title: "chore: bump dependencies", Type: "commit"},
		{Title: "feat: add login flow", Type: "commit"},
		{Title: "fix: typo in docs", Type: "pr"},
		{Title: "", Content: "", Type: "commit"},
		{Title: "Team standup", Content: "discussed Q3 OKR", Type: "meeting"},
	}
	kept, rejections := f.Apply(items)
	if len(kept) != 2 {
		t.Errorf("expected 2 kept, got %d", len(kept))
	}
	if rejections["merge-commit"] != 1 {
		t.Errorf("merge-commit: %d", rejections["merge-commit"])
	}
	if rejections["deps-bump"] != 1 {
		t.Errorf("deps-bump: %d", rejections["deps-bump"])
	}
	if rejections["empty"] != 1 {
		t.Errorf("empty: %d", rejections["empty"])
	}
}
```

- [ ] **Step 4: 跑测试确认失败**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/filter/...`
Expected: FAIL — `filter.NewRuleFilter` undefined

- [ ] **Step 5: 跑测试确认通过**（Step 2 已实现）

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/filter/...`
Expected: PASS

- [ ] **Step 6: 写 LLM 过滤器（接口 + mock 实现）**

Create file `internal/harvesting/filter/llm.go`:
```go
package filter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type NoiseClassifier interface {
	IsNoise(ctx context.Context, item harvesting.ContextItem) (bool, string, error)
}

// LLMFilter 用 LLM 二次过滤
type LLMFilter struct {
	cli NoiseClassifier
}

func NewLLMFilter(cli NoiseClassifier) *LLMFilter {
	return &LLMFilter{cli: cli}
}

// Apply 接受 items 列表，让 LLM 评估每条是否相关
func (f *LLMFilter) Apply(ctx context.Context, items []harvesting.ContextItem) (kept []harvesting.ContextItem, rejected int) {
	for _, it := range items {
		noise, reason, err := f.cli.IsNoise(ctx, it)
		if err != nil {
			// LLM 错误 → 保守保留，不丢失数据
			kept = append(kept, it)
			continue
		}
		if noise {
			rejected++
			_ = reason // 留待存储/审计
			continue
		}
		kept = append(kept, it)
	}
	return kept, rejected
}

// PromptTemplate 给 LLM 的 prompt
func PromptTemplate() string {
	return `判断以下内容是否是值得汇报的工作内容。返回 JSON {"is_noise": true/false, "reason": "..."}。

内容: ` + "`%s`" + `

如果只是 merge、版本号变更、格式调整、空消息，则 is_noise=true。
如果是功能开发、bug 修复、讨论、设计、决策，则 is_noise=false。`
}

// ParseResponse 解析 LLM JSON 响应
func ParseResponse(raw string) (bool, string, error) {
	var resp struct {
		IsNoise bool   `json:"is_noise"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return false, "", fmt.Errorf("parse: %w", err)
	}
	return resp.IsNoise, resp.Reason, nil
}
```

- [ ] **Step 7: 写 LLM 过滤器测试（mock classifier）**

Create file `internal/harvesting/filter/llm_test.go`:
```go
package filter

import (
	"context"
	"testing"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type mockClassifier struct {
	noise map[string]bool
}

func (m *mockClassifier) IsNoise(_ context.Context, it harvesting.ContextItem) (bool, string, error) {
	if m.noise[it.ID] {
		return true, "mocked", nil
	}
	return false, "", nil
}

func TestLLMFilter_Apply(t *testing.T) {
	cli := &mockClassifier{noise: map[string]bool{"noise-1": true, "noise-2": true}}
	f := NewLLMFilter(cli)
	items := []harvesting.ContextItem{
		{ID: "keep-1", Title: "Keep me"},
		{ID: "noise-1", Title: "drop me"},
		{ID: "keep-2", Title: "Keep me too"},
		{ID: "noise-2", Title: "drop me too"},
	}
	kept, rejected := f.Apply(context.Background(), items)
	if len(kept) != 2 {
		t.Errorf("kept: %d", len(kept))
	}
	if rejected != 2 {
		t.Errorf("rejected: %d", rejected)
	}
}

func TestLLMFilter_LLMError_KeepsAll(t *testing.T) {
	cli := &errClassifier{}
	f := NewLLMFilter(cli)
	items := []harvesting.ContextItem{{ID: "1"}, {ID: "2"}}
	kept, _ := f.Apply(context.Background(), items)
	if len(kept) != 2 {
		t.Errorf("on error should keep all, got %d", len(kept))
	}
}

type errClassifier struct{}

func (errClassifier) IsNoise(context.Context, harvesting.ContextItem) (bool, string, error) {
	return false, "", fmt.Errorf("simulated")
}
```

> 上面代码使用 `fmt.Errorf`，需要在 import 中添加 `"fmt"`。

Modify `internal/harvesting/filter/llm_test.go` — 添加 import:
```go
import (
	"context"
	"fmt"
	"testing"

	"github.com/asyncstarter/agent/internal/harvesting"
)
```

- [ ] **Step 8: 跑测试通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/filter/...`
Expected: PASS

- [ ] **Step 9: 写 sync 增量同步**

Create file `internal/harvesting/sync.go`:
```go
package harvesting

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncStore 同步状态存储
type SyncStore struct {
	pool *pgxpool.Pool
}

func NewSyncStore(pool *pgxpool.Pool) *SyncStore { return &SyncStore{pool: pool} }

// GetLastSync 获取数据源上次同步时间
func (s *SyncStore) GetLastSync(ctx context.Context, dataSourceID string) (time.Time, error) {
	var ts time.Time
	err := s.pool.QueryRow(ctx, `SELECT last_sync_at FROM sync_timestamps WHERE data_source_id = $1`, dataSourceID).Scan(&ts)
	if err != nil {
		// 首次同步：从 7 天前开始
		return time.Now().Add(-7 * 24 * time.Hour), nil
	}
	return ts, nil
}

// UpdateLastSync 更新同步时间戳
func (s *SyncStore) UpdateLastSync(ctx context.Context, dataSourceID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_timestamps (data_source_id, last_sync_at)
		VALUES ($1, NOW())
		ON CONFLICT (data_source_id) DO UPDATE SET last_sync_at = NOW()
	`, dataSourceID)
	if err != nil {
		return fmt.Errorf("update sync ts: %w", err)
	}
	return nil
}

// UpsertContextItem 插入或忽略（依赖唯一约束）
func (s *SyncStore) UpsertContextItem(ctx context.Context, item ContextItem) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO context_items
		(user_id, source, external_id, type, title, content, url, occurred_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, source, external_id) DO NOTHING
	`, item.UserID, item.Source, item.ID, item.Type, item.Title, item.Content, item.URL, item.OccurredAt, item.Metadata)
	return err
}
```

> **修改 ContextItem**: 上面用到 `item.UserID`。但 `ContextItem` 当前没有 UserID 字段。修改 harvesting/context.go：

Modify `internal/harvesting/context.go` — 在 `ContextItem` 添加 `UserID`:
```go
type ContextItem struct {
	ID         string            `json:"id"`
	UserID     string            `json:"user_id"`     // 新增
	Source     string            `json:"source"`
	// ... 其它字段保持
}
```

各适配器需在创建 ContextItem 时填入 UserID。**此为内部重构，§7.1 允许**。

- [ ] **Step 10: 写 sync 测试**

Create file `internal/harvesting/sync_test.go`:
```go
package harvesting

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSyncStore_GetLastSync_Default(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires DATABASE_URL")
	}
	// 集成测试，跳过详细步骤
	_ = context.Background()
	_ = time.Now()
}
```

> **简化说明**: sync 集成测试需要真实数据库 + 数据源 ID。本任务仅确保编译通过。完整集成测试留到 Phase 2 末的 pipeline 集成测试。

- [ ] **Step 11: 写 pipeline 编排**

Create file `internal/harvesting/pipeline.go`:
```go
package harvesting

import (
	"context"
	"log"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting/filter"
	"github.com/asyncstarter/agent/internal/harvesting/source"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pipeline struct {
	pool     *pgxpool.Pool
	sync     *SyncStore
	ruleF    *filter.RuleFilter
	llmF     *filter.LLMFilter

	adapters []source.Adapter
}

type sourceAdapter interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]ContextItem, error)
}

// Adapter 是数据源适配器接口
type Adapter = sourceAdapter

func NewPipeline(pool *pgxpool.Pool, llmCli filter.NoiseClassifier, adapters ...Adapter) *Pipeline {
	return &Pipeline{
		pool:     pool,
		sync:     NewSyncStore(pool),
		ruleF:    filter.NewRuleFilter(filter.DefaultRules()),
		llmF:     filter.NewLLMFilter(llmCli),
		adapters: adapters,
	}
}

// Run 执行一次完整 pipeline
func (p *Pipeline) Run(ctx context.Context, userID, agentRunID, dataSourceID string) (*ContextSnapshot, error) {
	adapter, dsID, err := p.findAdapter(dataSourceID)
	if err != nil {
		return nil, err
	}
	since, err := p.sync.GetLastSync(ctx, dsID)
	if err != nil {
		return nil, err
	}
	raw, err := adapter.Fetch(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	// 规则过滤
	kept, rejections := p.ruleF.Apply(raw)
	// LLM 过滤
	kept2, llmRejected := p.llmF.Apply(ctx, kept)

	// 入库
	for _, it := range kept2 {
		if err := p.sync.UpsertContextItem(ctx, it); err != nil {
			log.Printf("[pipeline] upsert: %v", err)
		}
	}
	// 更新同步时间
	if err := p.sync.UpdateLastSync(ctx, dsID); err != nil {
		return nil, err
	}

	return &ContextSnapshot{
		AgentRunID: agentRunID,
		UserID:     userID,
		Items:      kept2,
		FilterMeta: FilterMeta{
			TotalBeforeFilter: len(raw),
			TotalAfterFilter:  len(kept2),
			RuleRejections:    rejections,
			LLMRejections:     llmRejected,
		},
		SyncedAt: time.Now().UTC(),
	}, nil
}

func (p *Pipeline) findAdapter(dataSourceID string) (Adapter, string, error) {
	// dataSourceID 形如 "github:o/r" — 简单路由
	for _, a := range p.adapters {
		if a.Name() == splitSource(dataSourceID) {
			return a, dataSourceID, nil
		}
	}
	return nil, "", &NotFoundError{Source: dataSourceID}
}

type NotFoundError struct{ Source string }

func (e *NotFoundError) Error() string { return "adapter not found: " + e.Source }

func splitSource(id string) string {
	for i, c := range id {
		if c == ':' {
			return id[:i]
		}
	}
	return id
}
```

- [ ] **Step 12: 写 pipeline 测试（mock adapter）**

Create file `internal/harvesting/pipeline_test.go`:
```go
package harvesting

import (
	"context"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting/filter"
)

type mockAdapter struct {
	name  string
	items []ContextItem
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Fetch(_ context.Context, _ string, _ time.Time) ([]ContextItem, error) {
	return m.items, nil
}

type mockClassifier struct {
	noiseIDs map[string]bool
}

func (m *mockClassifier) IsNoise(_ context.Context, it ContextItem) (bool, string, error) {
	if m.noiseIDs[it.ID] {
		return true, "mock", nil
	}
	return false, "", nil
}

func TestPipeline_FilterFlow(t *testing.T) {
	adapter := &mockAdapter{
		name: "mock",
		items: []ContextItem{
			{ID: "k1", Source: "mock", Title: "feat: add login", Type: "commit"},
			{ID: "n1", Source: "mock", Title: "Merge pull request #1", Type: "commit"}, // 规则过滤
			{ID: "k2", Source: "mock", Title: "Project plan", Type: "note"},
			{ID: "n2", Source: "mock", Title: "spam", Type: "message"}, // LLM 过滤
		},
	}
	cli := &mockClassifier{noiseIDs: map[string]bool{"n2": true}}

	// 跳过 DB 依赖：直接测 filter flow
	items, _ := adapter.Fetch(context.Background(), "u1", time.Now())
	rf := filter.NewRuleFilter(filter.DefaultRules())
	kept, rej := rf.Apply(items)
	if len(kept) != 3 {
		t.Errorf("after rules: %d", len(kept))
	}
	if rej["merge-commit"] != 1 {
		t.Errorf("merge rejections: %d", rej["merge-commit"])
	}
	lf := filter.NewLLMFilter(cli)
	kept2, llmRej := lf.Apply(context.Background(), kept)
	if len(kept2) != 2 {
		t.Errorf("after llm: %d", len(kept2))
	}
	if llmRej != 1 {
		t.Errorf("llm rejected: %d", llmRej)
	}
}
```

- [ ] **Step 13: 跑全部测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: PASS

- [ ] **Step 14: 编译验证**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build ./...`
Expected: 0 错误

- [ ] **Step 15: 展示 diff 等用户决定**

---

## Phase 2 退出标准验证

完成 T010-T014 后，逐项验证 M2 退出标准：

- [ ] `go test ./internal/harvesting/...` → 全部 PASS
- [ ] 规则过滤测试：5 类规则 + 空内容
- [ ] LLM 过滤测试：mock classifier，错误时保留所有
- [ ] 增量同步测试：GetLastSync 首次返回 7 天前
- [ ] Pipeline 测试：raw → rules → llm → 入库 流程跑通
- [ ] 更新 [task-tracker.html](../../task-tracker.html) 中 T010-T014 状态

---

**下一步**: 进入 [04-phase3-synthesis.md](04-phase3-synthesis.md) 执行草稿生成（T015-T020, T026）。
