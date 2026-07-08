# 飞书集成（Feishu Integration）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **AI 行为约束**: 严格遵守 [ai-coding-boundary.md](../../../doc/ai-coding-boundary.md)。任何偏离需先询问。本计划基于 [决策 #7](../../../doc/decision-log.md)。

**Goal:** 以用户身份（user_access_token）接入飞书 API，实现 IM 消息接线、飞书任务拉取、任务事件触发、任务备注回写、飞书文档交付五项能力。

**Architecture:** OAuth 2.0 授权码流程获取 user_access_token，pgcrypto 对称加密存入独立 `feishu_tokens` 表，settings.Factory 提供 per-user 的 `*lark.Client`（自动刷新 token）。所有飞书 API 调用走 `lark.WithUserAccessToken(token)`。

**Tech Stack:** Go 1.22+ / Gin / larksuite/oapi-sdk-go/v3 (升级到 v3.4.25) / PostgreSQL 16 + pgcrypto / React + Tauri

**关联决策:** [决策 #7](../../../doc/decision-log.md) — FR-D03 扩入 MVP + user_access_token + pgcrypto

---

## 文件结构（File Structure）

### 新建文件
| 文件 | 职责 |
|---|---|
| `migrations/0010_feishu_tokens.up.sql` | feishu_tokens 表 + pgcrypto 扩展 |
| `migrations/0010_feishu_tokens.down.sql` | 回滚 |
| `internal/feishu/auth.go` | OAuth 流程：code 换 token、refresh token |
| `internal/feishu/auth_test.go` | OAuth 测试 |
| `internal/feishu/token_store.go` | token 存储层（pgcrypto 加解密） |
| `internal/feishu/token_store_test.go` | token 存储测试 |
| `internal/feishu/client_factory.go` | per-user client 工厂（检查过期 + 刷新） |
| `internal/feishu/client_factory_test.go` | client 工厂测试 |
| `internal/harvesting/source/feishu_task.go` | 飞书任务拉取适配器 |
| `internal/harvesting/source/feishu_task_test.go` | 任务拉取测试 |
| `internal/delivery/feishu.go` | 飞书文档交付适配器（注：feishu_doc.go 骨架不存在，本文件为全新创建） |
| `internal/delivery/feishu_test.go` | 文档交付测试 |
| `internal/handler/feishu_auth.go` | OAuth 回调 handler |
| `internal/handler/feishu_auth_test.go` | OAuth 回调测试 |

### 修改文件
| 文件 | 改动 |
|---|---|
| `internal/config/config.go` | 加 FeishuAppID/AppSecret/RedirectURL/DBEncryptionKey 字段 |
| `.env.example` | 加 4 个环境变量 |
| `internal/settings/factory.go` | 加 GetFeishuClient 方法 + feishuClientCache |
| `internal/harvesting/source/feishu.go` | 适配 per-user client（不再用全局 AppID/AppSecret 构造） |
| `internal/handler/webhook.go` | 加飞书事件处理器（challenge + 签名校验 + 事件分发） |
| `internal/delivery/service.go` | updateSourceComment 加 feishu case + GetFeishuAdapter |
| `internal/delivery/factory.go` | AdapterFactory 加 GetFeishuAdapter |
| `internal/server/server.go` | 注册飞书 OAuth 回调路由 + webhook 路由 |
| `cmd/api/wire.go` | 装配 feishu 模块 + IM 适配器接线 |
| `web/src/components/Settings.tsx` | 加飞书授权区块 |
| `web/src/api/feishu.ts` | 飞书授权 API client |

---

## Phase A — 基础设施（OAuth + Token 存储 + Client 工厂）

### Task F001: 数据库迁移 — feishu_tokens 表 + pgcrypto

**Files:**
- Create: `migrations/0010_feishu_tokens.up.sql`
- Create: `migrations/0010_feishu_tokens.down.sql`

**关联**: 决策 #7 / 数据层基础设施

- [ ] **Step 1: 写 up 迁移**

Create file `migrations/0010_feishu_tokens.up.sql`:
```sql
-- 启用 pgcrypto 扩展（提供 pgp_sym_encrypt / pgp_sym_decrypt）
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 飞书用户 token 存储（决策 #7）
-- 独立于 user_settings 表，避免 token 频繁刷新污染 settings 缓存
-- access_token / refresh_token 用 BYTEA：pgp_sym_encrypt 返回 bytea，
-- 直接存 BYTEA 避免依赖 bytea→text 隐式转换（生产环境可能禁用）
CREATE TABLE feishu_tokens (
    user_id        UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_token   BYTEA NOT NULL,       -- pgp_sym_encrypt 加密后的密文
    refresh_token  BYTEA NOT NULL,       -- pgp_sym_encrypt 加密后的密文
    expires_at     TIMESTAMPTZ NOT NULL, -- access_token 过期时间
    open_id        TEXT,                 -- 飞书用户标识
    name           TEXT,                 -- 飞书用户名（展示用）
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_feishu_tokens_expires ON feishu_tokens(expires_at);
```

- [ ] **Step 2: 写 down 迁移**

Create file `migrations/0010_feishu_tokens.down.sql`:
```sql
DROP TABLE IF EXISTS feishu_tokens;
-- 不 DROP pgcrypto 扩展，可能被其他表使用
```

- [ ] **Step 3: 执行迁移**

Run: `cd "k:\go_projects\AsyncStarterAgent" && make migrate-up`
Expected: migration 0010 应用成功

- [ ] **Step 4: 验证表结构**

Run: `psql "$DATABASE_URL" -c "\d feishu_tokens"`
Expected: 表结构包含 user_id / access_token (bytea) / refresh_token (bytea) / expires_at / open_id 字段

---

### Task F002: 配置层 — config.go + .env.example

**Files:**
- Modify: `internal/config/config.go`
- Modify: `.env.example`

**关联**: 决策 #7 §4 配置层

- [ ] **Step 1: 扩展 Config 结构**

Modify `internal/config/config.go` — 在 `NotionParentPageID` 字段后追加:
```go
type Config struct {
    // ... 现有字段保持不变 ...
    NotionParentPageID        string
    FeishuAppID               string // 决策 #7: 飞书应用 AppID
    FeishuAppSecret           string // 决策 #7: 飞书应用 AppSecret
    FeishuRedirectURL         string // 决策 #7: OAuth 回调地址
    FeishuVerificationToken   string // 决策 #7: 飞书事件订阅 Verification Token（webhook 校验）
    DBEncryptionKey           string // 决策 #7: pgcrypto 对称加密密钥
}
```

- [ ] **Step 2: 扩展 Load 函数**

Modify `internal/config/config.go` — 在 `return &Config{...}` 中追加字段:
```go
return &Config{
    // ... 现有赋值保持不变 ...
    NotionParentPageID:        getEnv("NOTION_PARENT_PAGE_ID", ""),
    FeishuAppID:               getEnv("FEISHU_APP_ID", ""),
    FeishuAppSecret:           getEnv("FEISHU_APP_SECRET", ""),
    FeishuRedirectURL:         getEnv("FEISHU_REDIRECT_URL", "http://localhost:8080/api/v1/auth/feishu/callback"),
    FeishuVerificationToken:   getEnv("FEISHU_VERIFICATION_TOKEN", ""),
    DBEncryptionKey:           getEnv("DB_ENCRYPTION_KEY", ""),
}, nil
```

- [ ] **Step 3: 更新 .env.example**

Modify `.env.example` — 在文件末尾追加:
```env
# 决策 #7: 飞书集成（user_access_token OAuth 流程）
FEISHU_APP_ID=
FEISHU_APP_SECRET=
FEISHU_REDIRECT_URL=http://localhost:8080/api/v1/auth/feishu/callback
# 飞书事件订阅 Verification Token（开放平台 → 事件订阅页获取）
FEISHU_VERIFICATION_TOKEN=
# pgcrypto 对称加密密钥（用于加密飞书 token）。生产环境必须 ≥ 32 字节随机串。
DB_ENCRYPTION_KEY=change-me-in-production-min-32-bytes-random
```

- [ ] **Step 4: 验证配置加载**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build ./internal/config/...`
Expected: 编译通过，无错误

---

### Task F003: Token 存储层 — pgcrypto 加解密

**Files:**
- Create: `internal/feishu/token_store.go`
- Create: `internal/feishu/token_store_test.go`

**关联**: 决策 #7 §3 Token 存储

- [ ] **Step 1: 写 TokenStore 接口与实现**

Create file `internal/feishu/token_store.go`:
```go
package feishu

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TokenRecord 存储在 feishu_tokens 表中的 token 记录（解密后）
type TokenRecord struct {
	UserID       uuid.UUID
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	OpenID       string
	Name         string
}

// TokenStore 抽象 token 存储层，便于测试 mock
type TokenStore interface {
	Save(ctx context.Context, rec TokenRecord) error
	Get(ctx context.Context, userID uuid.UUID) (*TokenRecord, error)
	Delete(ctx context.Context, userID uuid.UUID) error
}

// pgTokenStore 基于 PostgreSQL + pgcrypto 的实现
type pgTokenStore struct {
	pool *pgxpool.Pool
	// encKey 是 pgcrypto pgp_sym_encrypt 的对称密钥（来自 DB_ENCRYPTION_KEY 环境变量）
	encKey string
}

// NewTokenStore 创建 token 存储实例
// encKey 不能为空，否则 Save/Get 会返回错误
func NewTokenStore(pool *pgxpool.Pool, encKey string) TokenStore {
	return &pgTokenStore{pool: pool, encKey: encKey}
}

// Save 加密并存储 token 记录（UPSERT）
func (s *pgTokenStore) Save(ctx context.Context, rec TokenRecord) error {
	if s.encKey == "" {
		return fmt.Errorf("feishu token store: DB_ENCRYPTION_KEY is empty")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO feishu_tokens (user_id, access_token, refresh_token, expires_at, open_id, name, updated_at)
		VALUES ($1, pgp_sym_encrypt($2, $3), pgp_sym_encrypt($4, $3), $5, $6, $7, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			expires_at = EXCLUDED.expires_at,
			open_id = EXCLUDED.open_id,
			name = EXCLUDED.name,
			updated_at = NOW()
	`, rec.UserID, rec.AccessToken, s.encKey, rec.RefreshToken, rec.ExpiresAt, rec.OpenID, rec.Name)
	if err != nil {
		return fmt.Errorf("feishu token save: %w", err)
	}
	return nil
}

// Get 读取并解密 token 记录
// 列类型为 BYTEA（见 F001 迁移），无需 ::bytea cast
func (s *pgTokenStore) Get(ctx context.Context, userID uuid.UUID) (*TokenRecord, error) {
	if s.encKey == "" {
		return nil, fmt.Errorf("feishu token store: DB_ENCRYPTION_KEY is empty")
	}
	rec := &TokenRecord{UserID: userID}
	err := s.pool.QueryRow(ctx, `
		SELECT
			pgp_sym_decrypt(access_token, $2) AS access_token,
			pgp_sym_decrypt(refresh_token, $2) AS refresh_token,
			expires_at,
			COALESCE(open_id, ''),
			COALESCE(name, '')
		FROM feishu_tokens WHERE user_id = $1
	`, userID, s.encKey).Scan(&rec.AccessToken, &rec.RefreshToken, &rec.ExpiresAt, &rec.OpenID, &rec.Name)
	if err != nil {
		return nil, fmt.Errorf("feishu token get: %w", err)
	}
	return rec, nil
}

// Delete 删除 token 记录（用户撤销授权时调用）
func (s *pgTokenStore) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM feishu_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("feishu token delete: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: 写 mock TokenStore 用于测试**

Create file `internal/feishu/token_store_test.go`:
```go
package feishu

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// memTokenStore 是内存版 TokenStore，用于单测
type memTokenStore struct {
	mu   sync.Mutex
	data map[uuid.UUID]*TokenRecord
	err  error
}

func newMemTokenStore() *memTokenStore {
	return &memTokenStore{data: make(map[uuid.UUID]*TokenRecord)}
}

func (m *memTokenStore) Save(_ context.Context, rec TokenRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	recCopy := rec
	m.data[rec.UserID] = &recCopy
	return nil
}

func (m *memTokenStore) Get(_ context.Context, userID uuid.UUID) (*TokenRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	rec, ok := m.data[userID]
	if !ok {
		return nil, errors.New("not found")
	}
	return rec, nil
}

func (m *memTokenStore) Delete(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, userID)
	return nil
}

func TestTokenStore_SaveAndGet(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	rec := TokenRecord{
		UserID:       uid,
		AccessToken:  "acc-123",
		RefreshToken: "ref-456",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		OpenID:       "ou_abc",
		Name:         "张三",
	}
	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.Get(context.Background(), uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AccessToken != "acc-123" {
		t.Errorf("access token: %s", got.AccessToken)
	}
	if got.RefreshToken != "ref-456" {
		t.Errorf("refresh token: %s", got.RefreshToken)
	}
	if got.OpenID != "ou_abc" {
		t.Errorf("open id: %s", got.OpenID)
	}
}

func TestTokenStore_Get_NotFound(t *testing.T) {
	store := newMemTokenStore()
	_, err := store.Get(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestTokenStore_Delete(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{UserID: uid, AccessToken: "x"})
	if err := store.Delete(context.Background(), uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := store.Get(context.Background(), uid)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestTokenStore_Save_Overwrite(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{UserID: uid, AccessToken: "old"})
	_ = store.Save(context.Background(), TokenRecord{UserID: uid, AccessToken: "new"})
	got, _ := store.Get(context.Background(), uid)
	if got.AccessToken != "new" {
		t.Errorf("expected overwrite to 'new', got %s", got.AccessToken)
	}
}
```

- [ ] **Step 3: 跑测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/feishu/...`
Expected: PASS — 4 个测试全过

- [ ] **Step 4: 展示 diff 等用户决定**

---

### Task F004: OAuth 流程 — code 换 token + refresh

**Files:**
- Create: `internal/feishu/auth.go`
- Create: `internal/feishu/auth_test.go`

**关联**: 决策 #7 §2 OAuth 流程

- [ ] **Step 1: 写 OAuth client**

Create file `internal/feishu/auth.go`:
```go
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthConfig 飞书 OAuth 应用配置
type OAuthConfig struct {
	AppID       string
	AppSecret   string
	RedirectURL string
	// HTTPClient 可注入用于测试；nil 时用 http.DefaultClient
	HTTPClient HTTPDoer
}

// HTTPDoer 抽象 http.Client.Do，便于测试 mock
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// TokenResponse 飞书 OAuth token 响应
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"` // 秒
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
	OpenID           string `json:"open_id"`
	Name             string `json:"name"`
}

// AuthError 飞书 OAuth 错误响应
type AuthError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("feishu oauth: code=%d msg=%s", e.Code, e.Msg)
}

const (
	feishuTokenURL    = "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token"
	feishuRefreshURL  = "https://open.feishu.cn/open-apis/authen/v1/oidc/refresh_access_token"
	feishuAuthorizeURL = "https://open.feishu.cn/open-apis/authen/v1/index"
)

// AuthClient 飞书 OAuth 客户端
type AuthClient struct {
	cfg OAuthConfig
}

func NewAuthClient(cfg OAuthConfig) *AuthClient {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &AuthClient{cfg: cfg}
}

// AuthorizeURL 构造飞书授权页 URL（前端跳转用）
// state 用于 CSRF 防护，调用方生成随机串并暂存（如 session）
func (c *AuthClient) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("app_id", c.cfg.AppID)
	q.Set("redirect_uri", c.cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	return feishuAuthorizeURL + "?" + q.Encode()
}

// ExchangeCode 用授权码换 token
func (c *AuthClient) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, feishuTokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build exchange req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// 飞书 v2 OIDC: 需在 body 里带 app_id 和 app_secret
	body.Set("app_id", c.cfg.AppID)
	body.Set("app_secret", c.cfg.AppSecret)
	req.Body = io.NopCloser(strings.NewReader(body.Encode()))
	req.ContentLength = int64(len(body.Encode()))

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange http: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode exchange resp: %w", err)
	}
	if raw.Code != 0 {
		return nil, &AuthError{Code: raw.Code, Msg: raw.Msg}
	}

	var tr TokenResponse
	if err := json.Unmarshal(raw.Data, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal token data: %w", err)
	}
	return &tr, nil
}

// RefreshToken 用 refresh_token 刷新 access_token
func (c *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	body := url.Values{}
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", refreshToken)
	body.Set("app_id", c.cfg.AppID)
	body.Set("app_secret", c.cfg.AppSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, feishuRefreshURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build refresh req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh http: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode refresh resp: %w", err)
	}
	if raw.Code != 0 {
		return nil, &AuthError{Code: raw.Code, Msg: raw.Msg}
	}

	var tr TokenResponse
	if err := json.Unmarshal(raw.Data, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal refreshed token: %w", err)
	}
	return &tr, nil
}

// ToTokenRecord 把 TokenResponse 转成 TokenRecord（计算过期时间）
func (t *TokenResponse) ToTokenRecord(userID interface{ Bytes() []byte }) TokenRecord {
	_ = userID
	return TokenRecord{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(t.ExpiresIn) * time.Second),
		OpenID:       t.OpenID,
		Name:         t.Name,
	}
}
```

> **注意**: `ExchangeCode` 中用了 `io.NopCloser`，需在 import 加 `"io"`。`ToTokenRecord` 的 userID 参数类型有问题，下面测试会暴露，Step 2 修复。

- [ ] **Step 2: 修复 import 和 ToTokenRecord 签名**

Modify `internal/feishu/auth.go` — 修改 import 块和 ToTokenRecord:
```go
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)
```

替换 `ToTokenRecord` 为:
```go
// ToTokenRecord 把 TokenResponse 转成 TokenRecord（计算过期时间）
func (t *TokenResponse) ToTokenRecord(userID uuid.UUID) TokenRecord {
	return TokenRecord{
		UserID:       userID,
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(t.ExpiresIn) * time.Second),
		OpenID:       t.OpenID,
		Name:         t.Name,
	}
}
```

- [ ] **Step 3: 写测试（mock HTTP）**

Create file `internal/feishu/auth_test.go`:
```go
package feishu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// mockHTTPDoer 记录请求并返回预设响应
type mockHTTPDoer struct {
	respBody   string
	respStatus int
	lastURL    string
	lastBody   string
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	m.lastURL = req.URL.String()
	bodyBytes, _ := io.ReadAll(req.Body)
	m.lastBody = string(bodyBytes)
	status := m.respStatus
	if status == 0 {
		status = 200
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(m.respBody)),
	}, nil
}

func TestAuthorizeURL(t *testing.T) {
	c := NewAuthClient(OAuthConfig{
		AppID:       "cli_xxx",
		RedirectURL: "http://localhost:8080/api/v1/auth/feishu/callback",
	})
	u := c.AuthorizeURL("random-state-123")
	if !strings.Contains(u, "app_id=cli_xxx") {
		t.Errorf("missing app_id: %s", u)
	}
	if !strings.Contains(u, "state=random-state-123") {
		t.Errorf("missing state: %s", u)
	}
	if !strings.Contains(u, "response_type=code") {
		t.Errorf("missing response_type: %s", u)
	}
}

func TestExchangeCode_Success(t *testing.T) {
	doer := &mockHTTPDoer{
		respBody: `{
			"code": 0,
			"msg": "ok",
			"data": {
				"access_token": "u-xxx",
				"refresh_token": "ur-yyy",
				"token_type": "Bearer",
				"expires_in": 7200,
				"open_id": "ou_abc",
				"name": "张三"
			}
		}`,
	}
	c := NewAuthClient(OAuthConfig{
		AppID:     "cli_xxx",
		AppSecret: "sec_yyy",
		HTTPClient: doer,
	})
	tr, err := c.ExchangeCode(context.Background(), "code-123")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if tr.AccessToken != "u-xxx" {
		t.Errorf("access token: %s", tr.AccessToken)
	}
	if tr.RefreshToken != "ur-yyy" {
		t.Errorf("refresh token: %s", tr.RefreshToken)
	}
	if tr.OpenID != "ou_abc" {
		t.Errorf("open id: %s", tr.OpenID)
	}
	if tr.ExpiresIn != 7200 {
		t.Errorf("expires in: %d", tr.ExpiresIn)
	}
	if !strings.Contains(doer.lastBody, "code=code-123") {
		t.Errorf("request body missing code: %s", doer.lastBody)
	}
}

func TestExchangeCode_Error(t *testing.T) {
	doer := &mockHTTPDoer{
		respBody: `{"code": 10001, "msg": "invalid code"}`,
	}
	c := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y", HTTPClient: doer})
	_, err := c.ExchangeCode(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error")
	}
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}
	if authErr.Code != 10001 {
		t.Errorf("code: %d", authErr.Code)
	}
}

func TestRefreshToken_Success(t *testing.T) {
	doer := &mockHTTPDoer{
		respBody: `{
			"code": 0,
			"msg": "ok",
			"data": {
				"access_token": "u-new",
				"refresh_token": "ur-new",
				"expires_in": 7200
			}
		}`,
	}
	c := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y", HTTPClient: doer})
	tr, err := c.RefreshToken(context.Background(), "ur-old")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if tr.AccessToken != "u-new" {
		t.Errorf("access token: %s", tr.AccessToken)
	}
	if !strings.Contains(doer.lastBody, "refresh_token=ur-old") {
		t.Errorf("body missing refresh_token: %s", doer.lastBody)
	}
}

func TestTokenResponse_ToTokenRecord(t *testing.T) {
	uid := uuid.New()
	tr := &TokenResponse{
		AccessToken:  "u-xxx",
		RefreshToken: "ur-yyy",
		ExpiresIn:    7200,
		OpenID:       "ou_abc",
		Name:         "张三",
	}
	rec := tr.ToTokenRecord(uid)
	if rec.UserID != uid {
		t.Errorf("user id mismatch")
	}
	if rec.AccessToken != "u-xxx" {
		t.Errorf("access token: %s", rec.AccessToken)
	}
	if rec.OpenID != "ou_abc" {
		t.Errorf("open id: %s", rec.OpenID)
	}
	// 过期时间应是 now + 7200s 左右
	if rec.ExpiresAt.IsZero() {
		t.Error("expires_at is zero")
	}
}

// 编译期断言：确保 AuthClient 实现预期接口形状
var _ HTTPDoer = (*mockHTTPDoer)(nil)
var _ = json.Marshal
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/feishu/...`
Expected: PASS — 全部测试通过

- [ ] **Step 5: 展示 diff 等用户决定**

---

### Task F005: Client 工厂 — per-user lark.Client + 自动刷新

**Files:**
- Create: `internal/feishu/client_factory.go`
- Create: `internal/feishu/client_factory_test.go`

**关联**: 决策 #7 §4 settings.Factory.GetFeishuClient

- [ ] **Step 1: 写 ClientFactory**

Create file `internal/feishu/client_factory.go`:
```go
package feishu

import (
	"context"
	"fmt"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/google/uuid"
)

// ClientFactory 提供 per-user 的 *lark.Client，自动检查 token 过期并刷新
type ClientFactory struct {
	appID      string
	appSecret  string
	store      TokenStore
	authClient *AuthClient

	mu      sync.Mutex
	refresh map[uuid.UUID]struct{} // 正在刷新的用户集合，防止并发刷新
}

func NewClientFactory(appID, appSecret string, store TokenStore, authClient *AuthClient) *ClientFactory {
	return &ClientFactory{
		appID:      appID,
		appSecret:  appSecret,
		store:      store,
		authClient: authClient,
		refresh:    make(map[uuid.UUID]struct{}),
	}
}

// ErrNotAuthorized 用户未授权飞书
type ErrNotAuthorized struct{ UserID uuid.UUID }

func (e *ErrNotAuthorized) Error() string {
	return fmt.Sprintf("feishu: user %s not authorized, please connect feishu account first", e.UserID)
}

// ErrRefreshFailed token 刷新失败（refresh_token 过期等）
type ErrRefreshFailed struct {
	UserID uuid.UUID
	Cause  error
}

func (e *ErrRefreshFailed) Error() string {
	return fmt.Sprintf("feishu: refresh failed for user %s: %v", e.UserID, e.Cause)
}

// GetClient 返回带 user_access_token 的 lark.Client
// 流程：读 token → 检查过期（提前 5 分钟） → 需要则刷新 → 构造 client
func (f *ClientFactory) GetClient(ctx context.Context, userID uuid.UUID) (*lark.Client, string, error) {
	rec, err := f.store.Get(ctx, userID)
	if err != nil {
		return nil, "", &ErrNotAuthorized{UserID: userID}
	}

	// 提前 5 分钟判定过期，避免调用时刚好失效
	if time.Until(rec.ExpiresAt) < 5*time.Minute {
		rec, err = f.refreshToken(ctx, userID, rec.RefreshToken)
		if err != nil {
			return nil, "", err
		}
	}

	// 构造 lark.Client（用 WithUserAccessToken 选项包装每次请求）
	cli := lark.NewClient(f.appID, f.appSecret)
	return cli, rec.AccessToken, nil
}

// refreshToken 用 refresh_token 刷新并存储
// 使用 mutex 防止同一用户并发刷新
func (f *ClientFactory) refreshToken(ctx context.Context, userID uuid.UUID, refreshToken string) (*TokenRecord, error) {
	f.mu.Lock()
	if _, ongoing := f.refresh[userID]; ongoing {
		// 已有刷新在进行，等待释放后重新读
		f.mu.Unlock()
		// 简单等待 200ms 后重读
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return f.store.Get(ctx, userID)
	}
	f.refresh[userID] = struct{}{}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.refresh, userID)
		f.mu.Unlock()
	}()

	tr, err := f.authClient.RefreshToken(ctx, refreshToken)
	if err != nil {
		// 刷新失败：refresh_token 可能已过期，删除记录，要求重新授权
		_ = f.store.Delete(ctx, userID)
		return nil, &ErrRefreshFailed{UserID: userID, Cause: err}
	}

	rec := tr.ToTokenRecord(userID)
	if err := f.store.Save(ctx, rec); err != nil {
		return nil, fmt.Errorf("feishu: save refreshed token: %w", err)
	}
	return &rec, nil
}

// IsAuthorized 检查用户是否已授权（不刷新 token）
func (f *ClientFactory) IsAuthorized(ctx context.Context, userID uuid.UUID) bool {
	_, err := f.store.Get(ctx, userID)
	return err == nil
}

// Revoke 撤销授权（删除 token）
func (f *ClientFactory) Revoke(ctx context.Context, userID uuid.UUID) error {
	return f.store.Delete(ctx, userID)
}
```

- [ ] **Step 2: 写测试**

Create file `internal/feishu/client_factory_test.go`:
```go
package feishu

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClientFactory_GetClient_NotAuthorized(t *testing.T) {
	store := newMemTokenStore() // 空 store
	auth := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y"})
	factory := NewClientFactory("x", "y", store, auth)

	_, _, err := factory.GetClient(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected ErrNotAuthorized")
	}
	if _, ok := err.(*ErrNotAuthorized); !ok {
		t.Fatalf("expected ErrNotAuthorized, got %T: %v", err, err)
	}
}

func TestClientFactory_GetClient_ValidToken(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	// 存一个未过期的 token
	_ = store.Save(context.Background(), TokenRecord{
		UserID:       uid,
		AccessToken:  "u-valid",
		RefreshToken: "ur-valid",
		ExpiresAt:    time.Now().Add(2 * time.Hour), // 未过期
	})

	auth := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y"})
	factory := NewClientFactory("x", "y", store, auth)

	cli, token, err := factory.GetClient(context.Background(), uid)
	if err != nil {
		t.Fatalf("get client: %v", err)
	}
	if cli == nil {
		t.Fatal("nil client")
	}
	if token != "u-valid" {
		t.Errorf("token: %s", token)
	}
}

func TestClientFactory_GetClient_Expired_RefreshSuccess(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{
		UserID:       uid,
		AccessToken:  "u-old",
		RefreshToken: "ur-old",
		ExpiresAt:    time.Now().Add(-1 * time.Minute), // 已过期
	})

	// mock auth client 返回刷新成功
	doer := &mockHTTPDoer{
		respBody: `{
			"code": 0,
			"msg": "ok",
			"data": {
				"access_token": "u-new",
				"refresh_token": "ur-new",
				"expires_in": 7200
			}
		}`,
	}
	auth := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y", HTTPClient: doer})
	factory := NewClientFactory("x", "y", store, auth)

	cli, token, err := factory.GetClient(context.Background(), uid)
	if err != nil {
		t.Fatalf("get client: %v", err)
	}
	if cli == nil {
		t.Fatal("nil client")
	}
	if token != "u-new" {
		t.Errorf("expected refreshed token u-new, got %s", token)
	}

	// 验证 store 已更新
	rec, _ := store.Get(context.Background(), uid)
	if rec.AccessToken != "u-new" {
		t.Errorf("store not updated: %s", rec.AccessToken)
	}
}

func TestClientFactory_GetClient_RefreshFailed_DeletesToken(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{
		UserID:       uid,
		AccessToken:  "u-old",
		RefreshToken: "ur-bad",
		ExpiresAt:    time.Now().Add(-1 * time.Minute),
	})

	// mock auth client 返回错误
	doer := &mockHTTPDoer{
		respBody: `{"code": 10003, "msg": "refresh token invalid"}`,
	}
	auth := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y", HTTPClient: doer})
	factory := NewClientFactory("x", "y", store, auth)

	_, _, err := factory.GetClient(context.Background(), uid)
	if err == nil {
		t.Fatal("expected refresh error")
	}
	if _, ok := err.(*ErrRefreshFailed); !ok {
		t.Fatalf("expected ErrRefreshFailed, got %T: %v", err, err)
	}

	// 验证 token 已被删除
	_, getErr := store.Get(context.Background(), uid)
	if getErr == nil {
		t.Error("token should be deleted after refresh failure")
	}
}

func TestClientFactory_IsAuthorized(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	auth := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y"})
	factory := NewClientFactory("x", "y", store, auth)

	if factory.IsAuthorized(context.Background(), uid) {
		t.Error("should not be authorized initially")
	}
	_ = store.Save(context.Background(), TokenRecord{
		UserID:      uid,
		AccessToken: "x",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	})
	if !factory.IsAuthorized(context.Background(), uid) {
		t.Error("should be authorized after save")
	}
}

func TestClientFactory_Revoke(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{UserID: uid, AccessToken: "x"})
	auth := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y"})
	factory := NewClientFactory("x", "y", store, auth)

	if err := factory.Revoke(context.Background(), uid); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if factory.IsAuthorized(context.Background(), uid) {
		t.Error("should not be authorized after revoke")
	}
}
```

- [ ] **Step 3: 跑测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/feishu/...`
Expected: PASS — 全部测试通过

- [ ] **Step 4: 展示 diff 等用户决定**

---

### Task F006: settings.Factory 集成 GetFeishuClient

**Files:**
- Modify: `internal/settings/factory.go`

**关联**: 决策 #7 §4 / 与现有 Notion/Obsidian adapter 缓存模式一致

- [ ] **Step 1: 扩展 Factory 结构**

Modify `internal/settings/factory.go` — 在 `obsMap` 字段后加:
```go
type Factory struct {
	// ... 现有字段 ...
	obsMap    map[uuid.UUID]cachedObsidian
	feishuCli *feishu.ClientFactory // 决策 #7: 飞书 per-user client 工厂
}
```

并在 `NewFactory` 函数中初始化。但为避免 settings 包直接依赖 feishu 包（可能产生循环），改为**注入接口**。

修改 `NewFactory` 签名，加 feishu factory 参数:
```go
// FeishuClientGetter 由 feishu 包实现，settings 包通过接口依赖，避免循环依赖
type FeishuClientGetter interface {
	GetClient(ctx context.Context, userID uuid.UUID) (*lark.Client, string, error)
	IsAuthorized(ctx context.Context, userID uuid.UUID) bool
}

func NewFactory(pool *pgxpool.Pool, repo *Repo, defaults *config.Config, feishuCli FeishuClientGetter) *Factory {
	return &Factory{
		pool:      pool,
		repo:      repo,
		defaults:  defaults,
		llmCache:  make(map[uuid.UUID]cachedLLM),
		embCache:  make(map[uuid.UUID]cachedEmbedder),
		notionMap: make(map[uuid.UUID]cachedNotion),
		obsMap:    make(map[uuid.UUID]cachedObsidian),
		feishuCli: feishuCli,
	}
}
```

> **注意**: 需要在 factory.go import 加 `lark "github.com/larksuite/oapi-sdk-go/v3"` 和 `"github.com/asyncstarter/agent/internal/feishu"`。但因为用了接口，不需要直接 import feishu 包。`*lark.Client` 类型需要 import lark。

- [ ] **Step 2: 加 GetFeishuClient 方法**

Modify `internal/settings/factory.go` — 在文件末尾追加:
```go
// GetFeishuClient 返回 per-user 的飞书 lark.Client + user_access_token
// 决策 #7: 所有飞书 API 调用必须走此方法，确保以用户身份调用
func (f *Factory) GetFeishuClient(ctx context.Context, userID uuid.UUID) (*lark.Client, string, error) {
	if f.feishuCli == nil {
		return nil, "", fmt.Errorf("飞书集成未启用（未配置 FEISHU_APP_ID）")
	}
	return f.feishuCli.GetClient(ctx, userID)
}

// IsFeishuAuthorized 检查用户是否已授权飞书
func (f *Factory) IsFeishuAuthorized(ctx context.Context, userID uuid.UUID) bool {
	if f.feishuCli == nil {
		return false
	}
	return f.feishuCli.IsAuthorized(ctx, userID)
}
```

- [ ] **Step 3: 更新 wire.go 装配**

Modify `cmd/api/wire.go` — 在 `Build` 函数中（settingsFactory 创建前）插入飞书工厂构造:
```go
// 决策 #7: 飞书 OAuth + token 工厂
var feishuFactory *feishu.ClientFactory
if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
	tokenStore := feishu.NewTokenStore(pool, cfg.DBEncryptionKey)
	authClient := feishu.NewAuthClient(feishu.OAuthConfig{
		AppID:       cfg.FeishuAppID,
		AppSecret:   cfg.FeishuAppSecret,
		RedirectURL: cfg.FeishuRedirectURL,
	})
	feishuFactory = feishu.NewClientFactory(cfg.FeishuAppID, cfg.FeishuAppSecret, tokenStore, authClient)
	log.Println("[feishu] client factory initialized")
} else {
	log.Println("[feishu] disabled (FEISHU_APP_ID not set)")
}

settingsFactory := settings.NewFactory(pool, settingsRepo, cfg, feishuFactory)
```

并更新 import 加 `"github.com/asyncstarter/agent/internal/feishu"`。

- [ ] **Step 4: 更新 NewFactory 调用点**

由于 `NewFactory` 签名变了，需更新 `cmd/api/wire.go` 中所有调用。当前只有一处（Step 3 已改）。

- [ ] **Step 5: 跑编译**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build ./...`
Expected: 编译通过

- [ ] **Step 6: 跑全量测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./...`
Expected: 既有测试全过（settings 测试可能需更新 NewFactory 调用，加 nil 参数）

- [ ] **Step 7: 修复 settings 测试中的 NewFactory 调用**

如有 settings 包测试调用了 `NewFactory`，把第 4 个参数补 `nil`:
```go
f := NewFactory(pool, repo, cfg, nil)
```

- [ ] **Step 8: 展示 diff 等用户决定**

---

## Phase B — IM 消息适配器接线

### Task F007: wire.go 装配 FeishuAdapter + 数据源注册

**Files:**
- Modify: `internal/harvesting/source/feishu.go`
- Modify: `cmd/api/wire.go`

**关联**: FR-B03 (P1) 收尾 / 现有孤儿代码接线

> **边界说明**: 现有 feishu.go 用全局 `LarkConfig{AppID, AppSecret}` 构造 client（tenant 模式）。决策 #7 改用 user_access_token，需调整适配器签名：Fetch 时从 Factory 获取 per-user client + token。

- [ ] **Step 1: 重构 FeishuAdapter 支持 per-user client**

Modify `internal/harvesting/source/feishu.go` — 替换整个文件:
```go
package source

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuProvider 抽象飞书消息 provider（决策 #7: 以用户身份调用）
type FeishuProvider interface {
	ListMessages(ctx context.Context, userToken, chatID string, from, to time.Time) ([]harvesting.ContextItem, error)
}

// FeishuAdapter 是统一入口，实现 sourceAdapter 接口
// 决策 #7: 不再持有全局 client，每次 Fetch 时接收 per-user client + token
type FeishuAdapter struct {
	Provider FeishuProvider
	Source   string   // feishu / lark
	ChatIDs  []string // chat IDs to fetch messages from
}

func (a *FeishuAdapter) Name() string { return a.Source }

// Fetch 实现 sourceAdapter 接口
// 注意：标准 sourceAdapter 签名是 Fetch(ctx, userID, since)
// 决策 #7 需 user token，但接口未携带。这里通过 ctx 注入 token（见 ClientFactory.InjectToken）
func (a *FeishuAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	userToken, ok := UserTokenFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("feishu adapter: user access token not found in context")
	}
	to := time.Now()
	var out []harvesting.ContextItem
	for _, chatID := range a.ChatIDs {
		items, err := a.Provider.ListMessages(ctx, userToken, chatID, since, to)
		if err != nil {
			return nil, fmt.Errorf("feishu adapter fetch: %w", err)
		}
		for i := range items {
			items[i].UserID = userID
			items[i].Source = a.Source
			if items[i].Type == "" {
				items[i].Type = "message"
			}
		}
		out = append(out, items...)
	}
	return out, nil
}

// --- Lark SDK Provider ---

// LarkProvider 用 lark SDK 实现 FeishuProvider
type LarkProvider struct {
	cli *lark.Client
}

// NewLarkProvider 创建 provider。cli 是用 AppID/AppSecret 构造的基础 client，
// 实际 API 调用时通过 lark.WithUserAccessToken(userToken) 以用户身份调用
func NewLarkProvider(cli *lark.Client) *LarkProvider {
	return &LarkProvider{cli: cli}
}

func (p *LarkProvider) ListMessages(ctx context.Context, userToken, chatID string, from, to time.Time) ([]harvesting.ContextItem, error) {
	req := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		StartTime(strconv.FormatInt(from.Unix(), 10)).
		EndTime(strconv.FormatInt(to.Unix(), 10)).
		SortType("ByCreateTimeAsc").
		PageSize(50).
		Build()

	iter, err := p.cli.Im.V1.Message.ListByIterator(ctx, req, lark.WithUserAccessToken(userToken))
	if err != nil {
		return nil, fmt.Errorf("lark list messages: %w", err)
	}

	var items []harvesting.ContextItem
	for {
		ok, msg, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("lark iterate messages: %w", err)
		}
		if !ok {
			break
		}
		items = append(items, convertMessage(msg, chatID))
	}
	return items, nil
}

func convertMessage(m *larkim.Message, chatID string) harvesting.ContextItem {
	id := derefStr(m.MessageId)
	msgType := derefStr(m.MsgType)
	content := ""
	if m.Body != nil {
		content = derefStr(m.Body.Content)
	}
	senderID := ""
	if m.Sender != nil {
		senderID = derefStr(m.Sender.Id)
	}
	occurredAt := time.Time{}
	if m.CreateTime != nil {
		ts, err := strconv.ParseInt(*m.CreateTime, 10, 64)
		if err == nil {
			occurredAt = time.UnixMilli(ts)
		}
	}
	return harvesting.ContextItem{
		ID:         "feishu:msg:" + id,
		Source:     "feishu",
		Type:       "message",
		Title:      msgType,
		Content:    content,
		OccurredAt: occurredAt,
		Metadata: map[string]string{
			"chat_id":  chatID,
			"sender":   senderID,
			"msg_type": msgType,
		},
	}
}

// UserTokenFromContext 从 ctx 读取 user_access_token（由调用方注入）
type ctxKey struct{}

var userTokenKey = ctxKey{}

// WithUserToken 把飞书 user_access_token 注入 ctx
func WithUserToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, userTokenKey, token)
}

// UserTokenFromContext 从 ctx 取出 user_access_token
func UserTokenFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(userTokenKey).(string)
	return t, ok
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// 确保 LarkProvider 满足 FeishuProvider 接口
var _ FeishuProvider = (*LarkProvider)(nil)
```

- [ ] **Step 2: 更新 feishu_test.go 适配新接口**

Modify `internal/harvesting/source/feishu_test.go` — 替换整个文件。变更点：
1. `mockFeishu.ListMessages` 签名加 `userToken` 参数（位置在 chatID 前）
2. `mockFeishuFunc.ListMessages` 同步加 `userToken` 参数
3. 6 个调用 `a.Fetch` 的测试，ctx 都改为 `WithUserToken(context.Background(), "fake-token")`
4. 删除 `TestNewLarkProvider_MissingAppID` / `TestNewLarkProvider_MissingAppSecret`（NewLarkProvider 签名从 `(LarkConfig) (*LarkProvider, error)` 改为 `(*lark.Client) *LarkProvider`，不再有 AppID/AppSecret 校验）
5. 新增 `TestFeishuAdapter_Fetch_NoToken` 验证缺 token 时返回错误

```go
package source

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type mockFeishu struct {
	messages []harvesting.ContextItem
	err      error
}

// 签名变更：新增 userToken 参数（决策 #7）
func (m *mockFeishu) ListMessages(_ context.Context, _ string, _ string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.messages, m.err
}

func TestFeishuAdapter_Fetch(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "1", Title: "text", Type: "message", Content: "hello"},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0].Source != "feishu" {
		t.Errorf("source: %s", items[0].Source)
	}
	if items[0].Type != "message" {
		t.Errorf("type: %s", items[0].Type)
	}
}

func TestFeishuAdapter_Name(t *testing.T) {
	a := &FeishuAdapter{Source: "feishu"}
	if a.Name() != "feishu" {
		t.Errorf("expected feishu, got %s", a.Name())
	}
}

func TestFeishuAdapter_Fetch_DefaultType(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "2", Title: "text", Type: ""},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal("expected 1")
	}
	if items[0].Type != "message" {
		t.Errorf("expected default type message, got %s", items[0].Type)
	}
}

func TestFeishuAdapter_Fetch_EmptyResult(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0, got %d", len(items))
	}
}

func TestFeishuAdapter_Fetch_ProviderError(t *testing.T) {
	mock := &mockFeishu{err: errors.New("api unavailable")}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	_, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, mock.err) {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestFeishuAdapter_Fetch_MultipleChats(t *testing.T) {
	callCount := 0
	mock := &mockFeishuFunc{
		fn: func(chatID string) ([]harvesting.ContextItem, error) {
			callCount++
			return []harvesting.ContextItem{
				{ID: "msg-" + chatID, Title: "text", Type: "message"},
			}, nil
		},
	}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-a", "chat-b"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestFeishuAdapter_Fetch_NoChatIDs(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "1", Title: "text", Type: "message"},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: nil}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 with no chat IDs, got %d", len(items))
	}
}

// 新增：缺 token 时 Fetch 应返回错误（决策 #7）
func TestFeishuAdapter_Fetch_NoToken(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "1", Title: "text", Type: "message"},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	_, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected error for missing user token")
	}
}

// Compile-time check: FeishuAdapter satisfies the sourceAdapter interface
var _ interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)
} = (*FeishuAdapter)(nil)

// mockFeishuFunc is a function-based mock for testing multiple chat scenarios
type mockFeishuFunc struct {
	fn func(chatID string) ([]harvesting.ContextItem, error)
}

// 签名变更：新增 userToken 参数（决策 #7）
func (m *mockFeishuFunc) ListMessages(_ context.Context, _ string, chatID string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.fn(chatID)
}
```

> **注意**: 现有 `TestNewLarkProvider_MissingAppID` / `TestNewLarkProvider_MissingAppSecret` 两个测试在新签名下失去意义（NewLarkProvider 不再校验 AppID/AppSecret），直接删除。校验逻辑上移到 wire.go 的 `if cfg.FeishuAppID != ""` 判断。

- [ ] **Step 3: 跑测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/source/...`
Expected: PASS

- [ ] **Step 4: wire.go 装配（IM 适配器注册到 Pipeline）**

> **边界说明**: 当前 wire.go 并未构造 Pipeline（Pipeline 在别处或未接线）。本步骤只在 feishuFactory 就绪时构造 IM 适配器实例并存入 Deps，供后续 Pipeline 装配使用。

Modify `cmd/api/wire.go` — 在 `Deps` 结构加字段:
```go
type Deps struct {
	// ... 现有字段 ...
	SettingsFactory *settings.Factory
	FeishuFactory   *feishu.ClientFactory // 决策 #7
}
```

并在 `Build` 返回值中加 `FeishuFactory: feishuFactory`。

- [ ] **Step 5: 跑编译 + 全量测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build ./... && go test ./...`
Expected: 全部通过

- [ ] **Step 6: 展示 diff 等用户决定**

---

## Phase C — 飞书任务拉取适配器

### Task F008: feishu_task.go — 拉取用户任务作为上下文

**Files:**
- Create: `internal/harvesting/source/feishu_task.go`
- Create: `internal/harvesting/source/feishu_task_test.go`

**关联**: FR-B03 扩展 / 拉取用户负责的任务

- [ ] **Step 1: 写飞书任务适配器**

Create file `internal/harvesting/source/feishu_task.go`:
```go
package source

import (
	"context"
	"fmt"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

// TaskProvider 抽象飞书任务拉取
type TaskProvider interface {
	ListTasks(ctx context.Context, userToken string, from, to time.Time) ([]harvesting.ContextItem, error)
}

// FeishuTaskAdapter 拉取用户负责的飞书任务作为上下文
type FeishuTaskAdapter struct {
	Provider TaskProvider
	Source   string
}

func (a *FeishuTaskAdapter) Name() string { return a.Source }

func (a *FeishuTaskAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	userToken, ok := UserTokenFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("feishu task adapter: user access token not found in context")
	}
	items, err := a.Provider.ListTasks(ctx, userToken, since, time.Now())
	if err != nil {
		return nil, fmt.Errorf("feishu task fetch: %w", err)
	}
	for i := range items {
		items[i].UserID = userID
		items[i].Source = a.Source
		if items[i].Type == "" {
			items[i].Type = "task"
		}
	}
	return items, nil
}

// LarkTaskProvider 用 lark SDK 实现 TaskProvider
type LarkTaskProvider struct {
	cli *lark.Client
}

func NewLarkTaskProvider(cli *lark.Client) *LarkTaskProvider {
	return &LarkTaskProvider{cli: cli}
}

// ListTasks 调用 task/v2 接口拉取任务
// 飞书 task/v2 的 List 接口不支持按时间范围过滤，只能列出当前用户负责的任务
// 增量过滤在客户端做（按 updated_at 过滤 since）
func (p *LarkTaskProvider) ListTasks(ctx context.Context, userToken string, from, to time.Time) ([]harvesting.ContextItem, error) {
	req := larktask.NewListTaskReqBuilder().
		PageSize(50).
		Build()

	iter, err := p.cli.Task.V2.Task.ListByIterator(ctx, req, lark.WithUserAccessToken(userToken))
	if err != nil {
		return nil, fmt.Errorf("lark list tasks: %w", err)
	}

	var items []harvesting.ContextItem
	for {
		ok, task, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("lark iterate tasks: %w", err)
		}
		if !ok {
			break
		}
		item, include := convertTask(task, from, to)
		if !include {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func convertTask(t *larktask.Task, from, to time.Time) (harvesting.ContextItem, bool) {
	guid := derefStr(t.Guid)
	summary := derefStr(t.Summary)
	desc := ""
	if t.Description != nil {
		desc = derefStr(t.Description)
	}

	// 解析更新时间用于增量过滤
	updated := time.Time{}
	if t.UpdatedAt != nil {
		// 飞书时间戳是毫秒
		var ms int64
		fmt.Sscanf(*t.UpdatedAt, "%d", &ms)
		updated = time.UnixMilli(ms)
	}

	// 增量过滤：只保留 since 之后更新的任务
	if !updated.IsZero() && updated.Before(from) {
		return harvesting.ContextItem{}, false
	}

	// 解析截止时间
	dueStr := ""
	if t.Due != nil && t.Due.Timestamp != nil {
		var dueMs int64
		fmt.Sscanf(*t.Due.Timestamp, "%d", &dueMs)
		dueStr = time.UnixMilli(dueMs).Format(time.RFC3339)
	}

	// 完成状态
	completed := ""
	if t.CompletedAt != nil && *t.CompletedAt != "0" {
		completed = "true"
	}

	content := desc
	if dueStr != "" {
		content += fmt.Sprintf("\n\n截止时间: %s", dueStr)
	}
	if completed == "true" {
		content += "\n状态: 已完成"
	} else {
		content += "\n状态: 进行中"
	}

	return harvesting.ContextItem{
		ID:         "feishu:task:" + guid,
		Source:     "feishu",
		Type:       "task",
		Title:      summary,
		Content:    content,
		OccurredAt: updated,
		Metadata: map[string]string{
			"task_guid": guid,
			"due":       dueStr,
			"completed": completed,
		},
	}, true
}

var _ TaskProvider = (*LarkTaskProvider)(nil)
```

- [ ] **Step 2: 写测试（mock provider）**

Create file `internal/harvesting/source/feishu_task_test.go`:
```go
package source

import (
	"context"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type mockTaskProvider struct {
	items []harvesting.ContextItem
	err   error
}

func (m *mockTaskProvider) ListTasks(_ context.Context, _ string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.items, m.err
}

func TestFeishuTaskAdapter_Fetch(t *testing.T) {
	mock := &mockTaskProvider{items: []harvesting.ContextItem{
		{ID: "t1", Title: "写周报", Type: "task", Content: "本周工作总结"},
	}}
	a := &FeishuTaskAdapter{Provider: mock, Source: "feishu"}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0].Source != "feishu" {
		t.Errorf("source: %s", items[0].Source)
	}
	if items[0].Type != "task" {
		t.Errorf("type: %s", items[0].Type)
	}
	if items[0].UserID != "user-1" {
		t.Errorf("user id: %s", items[0].UserID)
	}
}

func TestFeishuTaskAdapter_Fetch_NoToken(t *testing.T) {
	mock := &mockTaskProvider{}
	a := &FeishuTaskAdapter{Provider: mock, Source: "feishu"}
	_, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestFeishuTaskAdapter_Fetch_DefaultType(t *testing.T) {
	mock := &mockTaskProvider{items: []harvesting.ContextItem{
		{ID: "t1", Title: "task", Type: ""},
	}}
	a := &FeishuTaskAdapter{Provider: mock, Source: "feishu"}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, _ := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if items[0].Type != "task" {
		t.Errorf("expected default type task, got %s", items[0].Type)
	}
}

func TestFeishuTaskAdapter_Name(t *testing.T) {
	a := &FeishuTaskAdapter{Source: "feishu"}
	if a.Name() != "feishu" {
		t.Errorf("expected feishu, got %s", a.Name())
	}
}

// 编译期断言
var _ interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)
} = (*FeishuTaskAdapter)(nil)
```

- [ ] **Step 3: 跑测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/harvesting/source/...`
Expected: PASS

- [ ] **Step 4: 展示 diff 等用户决定**

---

## Phase D — 飞书任务事件触发（Webhook）

### Task F009: 飞书事件订阅 webhook 处理器

**Files:**
- Modify: `internal/handler/webhook.go`
- Modify: `internal/handler/webhook_test.go`
- Modify: `internal/server/server.go`

**关联**: FR-A03 (P0) / 飞书任务事件触发 AgentRun

> **边界说明**:
> 1. 飞书事件订阅 v2 有两种验证模式：(1) URL 校验 challenge（明文）；(2) 事件加密推送（需 EncryptKey 解密 + 签名校验）。MVP 阶段先实现 challenge 校验 + 明文事件接收；加密模式作为加固项。
> 2. 现有 `WebhookHandler` 结构为 `{Secret, Svc}`（[webhook.go:14-17](../../../internal/handler/webhook.go#L14-L17)），server.go 内联构造（[server.go:85-88](../../../internal/server/server.go#L85-L88)）。本任务**只追加字段，不重命名、不加构造函数**，保持 Todoist handler 既有调用 `h.Secret` / `h.Svc` 不破坏。
> 3. `FeishuVerificationToken` 配置字段已在 F002 添加，本任务不再重复。

- [ ] **Step 1: WebhookHandler 追加 Pool / Cfg 字段 + 飞书 webhook 处理器**

Modify `internal/handler/webhook.go` — 在现有 `WebhookHandler` 结构追加两个字段（不删除 Secret/Svc），并在文件末尾追加飞书处理器:

结构体改动（[webhook.go:14-17](../../../internal/handler/webhook.go#L14-L17)）:
```go
type WebhookHandler struct {
	Secret string
	Svc    *trigger.Service
	Pool   *pgxpool.Pool  // 决策 #7: 查 feishu_tokens 表
	Cfg    *config.Config // 决策 #7: 读 FeishuVerificationToken
}
```

在文件末尾追加（同时需要更新 import 块，加 `"context"` / `"fmt"` / `"github.com/asyncstarter/agent/internal/config"` / `"github.com/jackc/pgx/v5/pgxpool"` / `"github.com/google/uuid"`）:
```go
// HandleFeishuWebhook 处理飞书事件订阅推送
// 路由: POST /api/v1/webhook/feishu
// 决策 #7: 飞书任务事件触发 AgentRun
func (h *WebhookHandler) HandleFeishuWebhook(c *gin.Context) {
	var payload struct {
		Challenge string `json:"challenge"` // URL 校验时飞书下发
		Token     string `json:"token"`     // 事件订阅 Verification Token（顶层兼容旧格式）
		Type      string `json:"type"`      // url_verification / event_callback
		Header    struct {
			EventID    string `json:"event_id"`
			EventType  string `json:"event_type"`
			Token      string `json:"token"`
			CreateTime string `json:"create_time"`
		} `json:"header"`
		Event map[string]interface{} `json:"event"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// 1. URL 校验：返回 challenge
	if payload.Type == "url_verification" {
		c.JSON(http.StatusOK, gin.H{"challenge": payload.Challenge})
		return
	}

	// 2. Token 校验（防伪造）。header.token 优先，回退顶层 token（飞书旧版格式）
	token := payload.Header.Token
	if token == "" {
		token = payload.Token
	}
	if h.Cfg != nil && h.Cfg.FeishuVerificationToken != "" && token != h.Cfg.FeishuVerificationToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid verification token"})
		return
	}

	// 3. 事件分发：只处理任务相关事件
	eventType := payload.Header.EventType
	switch eventType {
	case "task.v2.task.created", "task.v2.task.updated":
		h.handleFeishuTaskEvent(c, payload.Event)
	default:
		// 非任务事件，确认接收但不处理
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ignored"})
	}
}

// handleFeishuTaskEvent 处理飞书任务事件，创建 AgentRun
// MVP 限制：open_id → user_id 映射依赖 feishu_tokens 表的 open_id 字段。
// 若用户未授权过飞书（表里无此 open_id），事件被忽略（返回 200 + user not mapped），
// 避免飞书端因业务不匹配无限重试。
func (h *WebhookHandler) handleFeishuTaskEvent(c *gin.Context, event map[string]interface{}) {
	summary, _ := event["summary"].(string)
	if summary == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "no summary"})
		return
	}

	// 安全导航：operator_id 可能不存在或不是 map
	openID := ""
	if operator, ok := event["operator_id"].(map[string]interface{}); ok {
		openID, _ = operator["open_id"].(string)
	}
	if openID == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "no operator"})
		return
	}

	// 通过 open_id 查找系统用户（feishu_tokens 表的 open_id 字段，F001 已建）
	userID, err := h.findUserByOpenID(c.Request.Context(), openID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "user not mapped"})
		return
	}

	// 创建 AgentRun。h.Svc 是 *trigger.Service，ProcessKeyword 签名 (ctx, userID uuid.UUID, text string)
	if h.Svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trigger service not configured"})
		return
	}
	if _, err := h.Svc.ProcessKeyword(c.Request.Context(), userID, summary); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// findUserByOpenID 通过飞书 open_id 查找系统用户
func (h *WebhookHandler) findUserByOpenID(ctx context.Context, openID string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := h.Pool.QueryRow(ctx, `SELECT user_id FROM feishu_tokens WHERE open_id = $1`, openID).Scan(&userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user not found for open_id %s: %w", openID, err)
	}
	return userID, nil
}
```

- [ ] **Step 2: server.go 更新 WebhookHandler 构造 + 注册飞书 webhook 路由**

Modify `internal/server/server.go` — 把现有的 `wh` 构造点（[server.go:85-88](../../../internal/server/server.go#L85-L88)）改为追加 Pool/Cfg 字段，并在其后追加飞书 webhook 路由:

现有代码:
```go
wh := &handler.WebhookHandler{
    Secret: cfg.TodoistWebhookSecret,
    Svc:    trigSvc,
}
r.POST("/api/v1/webhook/todoist", wh.Todoist)
```

改为:
```go
wh := &handler.WebhookHandler{
    Secret: cfg.TodoistWebhookSecret,
    Svc:    trigSvc,
    Pool:   pool, // 决策 #7: 查 feishu_tokens
    Cfg:    cfg,  // 决策 #7: 读 FeishuVerificationToken
}
r.POST("/api/v1/webhook/todoist", wh.Todoist)
// 决策 #7: 飞书事件订阅 webhook（无需登录中间件，飞书直接推送）
r.POST("/api/v1/webhook/feishu", wh.HandleFeishuWebhook)
```

- [ ] **Step 3: 写 webhook 测试（handler_test 包，遵循现有模式）**

Modify `internal/handler/webhook_test.go` — 在文件末尾追加飞书测试。注意现有测试用 `package handler_test` 外部包 + `&handler.WebhookHandler{...}` 构造 + `gin.New()` + `httptest.NewRecorder()` 模式:

```go
// 飞书 webhook 测试（决策 #7）

func TestWebhook_Feishu_Challenge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{
		Cfg: &config.Config{FeishuVerificationToken: "tok"},
	}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	body := `{"challenge":"ajls384kdjx98XX","type":"url_verification","token":"tok"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp["challenge"] != "ajls384kdjx98XX" {
		t.Errorf("challenge: %s", resp["challenge"])
	}
}

func TestWebhook_Feishu_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{
		Cfg: &config.Config{FeishuVerificationToken: "expected-token"},
	}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	body := `{"type":"event_callback","header":{"event_type":"task.v2.task.created","token":"wrong-token"},"event":{}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestWebhook_Feishu_NoOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Cfg.FeishuVerificationToken 为空 → 跳过 token 校验
	h := &handler.WebhookHandler{}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	// 事件无 operator_id 字段 → 走"no operator"分支
	body := `{"type":"event_callback","header":{"event_type":"task.v2.task.created","token":""},"event":{"summary":"写周报"}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no operator") {
		t.Errorf("expected 'no operator' msg, got %s", w.Body.String())
	}
}

func TestWebhook_Feishu_NonTaskEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.WebhookHandler{}

	r := gin.New()
	r.POST("/api/v1/webhook/feishu", h.HandleFeishuWebhook)

	// 非任务事件 → "ignored"
	body := `{"type":"event_callback","header":{"event_type":"im.message.receive_v1","token":""},"event":{}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/webhook/feishu", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ignored") {
		t.Errorf("expected 'ignored' msg, got %s", w.Body.String())
	}
}
```

> **import 提示**: webhook_test.go 已有 `bytes` / `context` / `encoding/json` / `net/http` / `net/http/httptest` / `testing` / `handler` / `gin` / `uuid`。需追加 `"strings"` 和 `"github.com/asyncstarter/agent/internal/config"`。

- [ ] **Step 4: 跑测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/handler/...`
Expected: PASS — 含新增 4 个飞书 webhook 测试

- [ ] **Step 5: 展示 diff 等用户决定**

---

## Phase E — 回写草稿链接到飞书任务备注

### Task F010: delivery service 扩展 feishu case

**Files:**
- Modify: `internal/delivery/service.go`
- Modify: `internal/delivery/factory.go`

**关联**: FR-D04 (P0) / 飞书任务评论 API

- [ ] **Step 1: AdapterFactory 加 GetFeishuAdapter**

Modify `internal/delivery/factory.go`:
```go
type AdapterFactory interface {
	GetNotionAdapter(ctx context.Context, userID uuid.UUID) (*NotionAdapter, error)
	GetObsidianAdapter(ctx context.Context, userID uuid.UUID) (*ObsidianAdapter, error)
	// 决策 #7: 飞书文档交付适配器
	GetFeishuAdapter(ctx context.Context, userID uuid.UUID) (*FeishuAdapter, error)
}
```

- [ ] **Step 2: settings.Factory 实现 GetFeishuAdapter**

Modify `internal/settings/factory.go` — 追加:
```go
// GetFeishuAdapter 返回飞书交付适配器（per-user）
func (f *Factory) GetFeishuAdapter(ctx context.Context, userID uuid.UUID) (*delivery.FeishuAdapter, error) {
	cli, token, err := f.GetFeishuClient(ctx, userID)
	if err != nil {
		return nil, err
	}
	return delivery.NewFeishuAdapter(cli, token), nil
}
```

- [ ] **Step 3: delivery/service.go 的 updateSourceComment 加 feishu case**

Modify `internal/delivery/service.go` — 把现有 `updateSourceComment` 函数（当前只有 notion/obsidian case）替换为:
```go
func (s *Service) updateSourceComment(ctx context.Context, userID uuid.UUID, runID, targetType, targetURL, title string) error {
	switch targetType {
	case "notion":
		notion, err := s.factory.GetNotionAdapter(ctx, userID)
		if err != nil {
			return nil
		}
		pageID := extractPageID(targetURL)
		return notion.UpdateTaskComment(ctx, pageID, fmt.Sprintf("已生成: %s", title))
	case "obsidian":
		return nil
	case "feishu":
		// 决策 #7: 回写草稿链接到原始飞书任务备注
		// 需要从 agent_run 的 trigger_source 取 task_guid
		taskGUID, err := s.getFeishuTaskGUID(ctx, runID)
		if err != nil {
			return nil // 没有关联的飞书任务，静默跳过
		}
		adapter, err := s.factory.GetFeishuAdapter(ctx, userID)
		if err != nil {
			return fmt.Errorf("get feishu adapter: %w", err)
		}
		return adapter.CreateTaskComment(ctx, taskGUID, fmt.Sprintf("起跑器草稿: %s\n%s", title, targetURL))
	}
	return nil
}

// getFeishuTaskGUID 从 agent_run.trigger_source 解析飞书任务 GUID
func (s *Service) getFeishuTaskGUID(ctx context.Context, runID string) (string, error) {
	var src string
	err := s.pool.QueryRow(ctx, `SELECT trigger_source FROM agent_runs WHERE id = $1`, runID).Scan(&src)
	if err != nil {
		return "", err
	}
	// trigger_source 格式假设为 "feishu:task:<guid>"
	if !strings.HasPrefix(src, "feishu:task:") {
		return "", fmt.Errorf("not a feishu task trigger")
	}
	return strings.TrimPrefix(src, "feishu:task:"), nil
}
```

> **说明**: 现有 notion case 逻辑来自 `internal/delivery/service.go:132-145`，本次只是在末尾追加 feishu case + 新增 `getFeishuTaskGUID` 方法。`extractPageID` 保持不变。

- [ ] **Step 4: 跑测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/delivery/... ./internal/settings/...`
Expected: PASS（需补充 feishu case 的单测，mock factory）

- [ ] **Step 5: 展示 diff 等用户决定**

---

## Phase F — 飞书文档交付（FR-D03，扩入 MVP）

### Task F011: delivery/feishu.go — 创建飞书云文档

**Files:**
- Create: `internal/delivery/feishu.go`
- Create: `internal/delivery/feishu_test.go`
- ~~Delete: `internal/delivery/feishu_doc.go`~~ — 经核对，该骨架文件在当前代码库中不存在（T025 从未落地），Step 3 改为"确认不存在并跳过"

**关联**: FR-D03 (P2, 扩入 MVP 决策 #7)

- [ ] **Step 1: 写飞书文档适配器**

Create file `internal/delivery/feishu.go`:
```go
package delivery

import (
	"context"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdoc "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

// FeishuAdapter 飞书交付适配器（决策 #7）
// 职责：1) 创建飞书云文档 2) 在任务下添加评论
type FeishuAdapter struct {
	cli        *lark.Client
	userToken  string
}

// NewFeishuAdapter 创建适配器。cli 是基础 client，userToken 是 user_access_token
func NewFeishuAdapter(cli *lark.Client, userToken string) *FeishuAdapter {
	return &FeishuAdapter{cli: cli, userToken: userToken}
}

// CreateDoc 创建飞书云文档并写入 markdown 内容
// 返回文档 URL
func (a *FeishuAdapter) CreateDoc(ctx context.Context, title, markdown string) (string, error) {
	// 1. 创建空白文档
	createReq := larkdoc.NewCreateDocumentReqBuilder().
		Title(title).
		Build()

	createResp, err := a.cli.Docx.Document.Create(ctx, createReq, lark.WithUserAccessToken(a.userToken))
	if err != nil {
		return "", fmt.Errorf("feishu create doc: %w", err)
	}
	if !createResp.Success() {
		return "", fmt.Errorf("feishu create doc: code=%d msg=%s", createResp.Code, createResp.Msg)
	}

	docID := *createResp.Data.Document.DocumentId
	docURL := ""
	if createResp.Data.Document.URL != nil {
		docURL = *createResp.Data.Document.URL
	}

	// 2. 把 markdown 转换为飞书 block 并写入
	blocks := markdownToFeishuBlocks(markdown)
	if len(blocks) > 0 {
		blockReq := larkdoc.NewCreateDocumentBlockChildrenReqBuilder().
			DocumentId(docID).
			Index(0).
			Children(blocks).
			Build()
		_, err = a.cli.Docx.DocumentBlockChildren.Create(ctx, blockReq, lark.WithUserAccessToken(a.userToken))
		if err != nil {
			// 内容写入失败不阻断，返回已创建的文档 URL
			fmt.Printf("[feishu] write blocks failed (doc still created): %v\n", err)
		}
	}

	if docURL == "" {
		docURL = fmt.Sprintf("https://feishu.cn/docx/%s", docID)
	}
	return docURL, nil
}

// CreateTaskComment 在飞书任务下添加评论（FR-D04）
func (a *FeishuAdapter) CreateTaskComment(ctx context.Context, taskGUID, content string) error {
	req := larktask.NewCreateCommentReqBuilder().
		TaskGuid(taskGUID).
		Body(larktask.NewCommentBuilder().
			Content(content).
			Build()).
		Build()

	resp, err := a.cli.Task.V2.Comment.Create(ctx, req, lark.WithUserAccessToken(a.userToken))
	if err != nil {
		return fmt.Errorf("feishu create comment: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu create comment: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// markdownToFeishuBlocks 把 markdown 转为飞书 docx block 数组
// MVP 阶段做基础转换：段落 + 标题。复杂块（表格/代码块）V1.5 补
func markdownToFeishuBlocks(md string) []*larkdoc.Block {
	lines := strings.Split(md, "\n")
	var blocks []*larkdoc.Block
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// 标题识别（# ## ###）
		if strings.HasPrefix(trimmed, "### ") {
			blocks = append(blocks, larkdoc.NewBlockBuilder().
				BlockType(3).
				Heading3(larkdoc.NewTextBlockBuilder().
					Elements([]*larkdoc.TextElement{
						larkdoc.NewTextElementBuilder().
							Content(trimmed[4:]).
							Build(),
					}).
					Build()).
				Build())
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			blocks = append(blocks, larkdoc.NewBlockBuilder().
				BlockType(2).
				Heading2(larkdoc.NewTextBlockBuilder().
					Elements([]*larkdoc.TextElement{
						larkdoc.NewTextElementBuilder().
							Content(trimmed[3:]).
							Build(),
					}).
					Build()).
				Build())
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			blocks = append(blocks, larkdoc.NewBlockBuilder().
				BlockType(1).
				Heading1(larkdoc.NewTextBlockBuilder().
					Elements([]*larkdoc.TextElement{
						larkdoc.NewTextElementBuilder().
							Content(trimmed[2:]).
							Build(),
					}).
					Build()).
				Build())
			continue
		}
		// 普通段落
		blocks = append(blocks, larkdoc.NewBlockBuilder().
			BlockType(2).
			Paragraph(larkdoc.NewTextBlockBuilder().
				Elements([]*larkdoc.TextElement{
					larkdoc.NewTextElementBuilder().
						Content(trimmed).
						Build(),
				}).
				Build()).
			Build())
	}
	return blocks
}
```

> **注意**: 上面的 SDK builder 方法名（如 `NewBlockBuilder`、`BlockType`、`Heading1`）是基于飞书 SDK 约定的推断，实际名称需对照 `service/docx/v1` 包。Step 2 验证编译，如有偏差调整方法名。

- [ ] **Step 2: 验证 SDK API 名称**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go doc github.com/larksuite/oapi-sdk-go/v3/service/docx/v1 Block`
Expected: 输出 Block 类型定义，确认 builder 方法名。如方法名不符，修正 Step 1 代码。

> **边界说明**: 飞书 docx SDK 的 block 构造 API 较复杂，如编译失败，先简化为只创建空文档（不写内容），内容写入留到后续迭代。简化版 CreateDoc 只返回文档 URL，跳过 blocks 写入。

- [ ] **Step 3: 确认旧 feishu_doc.go 骨架不存在（跳过删除）**

经核对项目实际状态（`Glob internal/delivery/feishu*.go` 返回空），`internal/delivery/feishu_doc.go` 骨架在当前代码库中**并不存在**——`doc/plans/05-phase4-delivery.md` 的 T025 骨架从未落地。因此本步骤无需删除任何文件，直接跳过。

> **C2 红线说明**: 既然目标文件不存在，"删除前确认"动作自然失效。F011 Step 1 创建的 `internal/delivery/feishu.go` 是该路径下的首个飞书交付文件，不存在覆盖风险。

Run: `cd "k:\go_projects\AsyncStarterAgent" && dir internal\delivery\feishu*.go`
Expected: 找不到文件（或在 Step 1 之后仅出现 `feishu.go` + `feishu_test.go`）

- [ ] **Step 4: 写测试**

Create file `internal/delivery/feishu_test.go`:
```go
package delivery

import (
	"testing"
)

func TestMarkdownToFeishuBlocks_Heading(t *testing.T) {
	md := "# Title\n\n## Section\n\nContent"
	blocks := markdownToFeishuBlocks(md)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
}

func TestMarkdownToFeishuBlocks_EmptyLines(t *testing.T) {
	md := "\n\n# Title\n\n\n"
	blocks := markdownToFeishuBlocks(md)
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}
}

func TestMarkdownToFeishuBlocks_Paragraph(t *testing.T) {
	md := "First paragraph.\nSecond paragraph."
	blocks := markdownToFeishuBlocks(md)
	if len(blocks) != 2 {
		t.Errorf("expected 2 paragraphs, got %d", len(blocks))
	}
}

func TestNewFeishuAdapter(t *testing.T) {
	a := NewFeishuAdapter(nil, "fake-token")
	if a == nil {
		t.Fatal("nil adapter")
	}
	if a.userToken != "fake-token" {
		t.Errorf("token: %s", a.userToken)
	}
}
```

- [ ] **Step 5: delivery/service.go 的 Deliver 加 feishu case**

Modify `internal/delivery/service.go` — `Deliver` 函数的 switch 加:
```go
case "feishu":
	feishu, ferr := s.factory.GetFeishuAdapter(ctx, uid)
	if ferr != nil {
		adapterErr = ferr
		break
	}
	targetURL, err = feishu.CreateDoc(ctx, title, md)
```

- [ ] **Step 6: 跑测试**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go test ./internal/delivery/...`
Expected: PASS

- [ ] **Step 7: 展示 diff 等用户决定**

---

## Phase G — 前端

### Task F012: 前端飞书授权 UI + API client

**Files:**
- Create: `internal/handler/feishu_auth.go`
- Create: `internal/handler/feishu_auth_test.go`
- Modify: `internal/server/server.go`
- Modify: `cmd/api/wire.go`
- Create: `web/src/api/feishu.ts`
- Modify: `web/src/components/Settings.tsx`

**关联**: 决策 #7 §前端 / 设置页飞书授权区块

> **关键设计**（修复 B1）: state 格式为 `<csrfRandom>_<userID>`。前半段 CSRF 随机串存 cookie 用于防伪造，后半段 user_id 直接从 state 解析。这样 Callback 即使飞书跳转过来时无登录 session，也能从 state 恢复 user_id。

- [ ] **Step 1: 后端 OAuth handler**

Create file `internal/handler/feishu_auth.go`:
```go
package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/feishu"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FeishuAuthHandler struct {
	cfg        *config.Config
	authClient *feishu.AuthClient
	tokenStore feishu.TokenStore
}

func NewFeishuAuthHandler(cfg *config.Config, authClient *feishu.AuthClient, tokenStore feishu.TokenStore) *FeishuAuthHandler {
	return &FeishuAuthHandler{cfg: cfg, authClient: authClient, tokenStore: tokenStore}
}

// StartAuth GET /api/v1/auth/feishu/start
// 生成 state（格式: <csrfRandom>_<userID>），CSRF 部分存 cookie，重定向到飞书授权页
func (h *FeishuAuthHandler) StartAuth(c *gin.Context) {
	userID, ok := auth.MustUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	csrf := uuid.New().String()
	state := buildOAuthState(csrf, userID)
	// CSRF 部分存 cookie（10 分钟有效），用于 Callback 校验
	c.SetCookie("feishu_oauth_csrf", csrf, int(10*time.Minute.Seconds()), "/", "", false, true)

	url := h.authClient.AuthorizeURL(state)
	c.JSON(http.StatusOK, gin.H{"authorize_url": url})
}

// Callback GET /api/v1/auth/feishu/callback
// 飞书授权后回调，校验 CSRF + 解析 user_id + code 换 token + 存储
func (h *FeishuAuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code or state"})
		return
	}

	// 校验 CSRF：cookie 中的 csrf 必须与 state 前半段一致
	csrfFromCookie, err := c.Cookie("feishu_oauth_csrf")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing csrf cookie"})
		return
	}
	csrfFromState, userID, err := parseOAuthState(state)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state format"})
		return
	}
	if csrfFromCookie != csrfFromState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "csrf mismatch"})
		return
	}
	c.SetCookie("feishu_oauth_csrf", "", -1, "/", "", false, true) // 清除

	// code 换 token
	tr, err := h.authClient.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	rec := tr.ToTokenRecord(userID)
	if err := h.tokenStore.Save(c.Request.Context(), rec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 重定向回前端设置页
	c.Redirect(http.StatusFound, "/?feishu_auth=success")
}

// Status GET /api/v1/auth/feishu/status
// 查询当前用户飞书授权状态
func (h *FeishuAuthHandler) Status(c *gin.Context) {
	userID, ok := auth.MustUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rec, err := h.tokenStore.Get(c.Request.Context(), userID)
	status := "not_authorized"
	name := ""
	if err == nil {
		status = "authorized"
		name = rec.Name
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "name": name})
}

// Revoke POST /api/v1/auth/feishu/revoke
// 撤销飞书授权
func (h *FeishuAuthHandler) Revoke(c *gin.Context) {
	userID, ok := auth.MustUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.tokenStore.Delete(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// buildOAuthState 构造 state: "<csrfRandom>_<userID>"
func buildOAuthState(csrf string, userID uuid.UUID) string {
	return csrf + "_" + userID.String()
}

// parseOAuthState 解析 state: "<csrfRandom>_<userID>" → (csrf, userID, error)
func parseOAuthState(state string) (string, uuid.UUID, error) {
	idx := strings.Index(state, "_")
	if idx <= 0 || idx == len(state)-1 {
		return "", uuid.Nil, fmt.Errorf("invalid state format")
	}
	csrf := state[:idx]
	userID, err := uuid.Parse(state[idx+1:])
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("invalid user id in state: %w", err)
	}
	return csrf, userID, nil
}
```

- [ ] **Step 2: server.go 注册飞书 auth 路由 + wire.go 装配**

Modify `internal/server/server.go` — server.go 当前用 `r.GET("/api/v1/...", authMW, handler)` 直注册（无 `v1.Group`，见 [server.go:79-127](../../../internal/server/server.go#L79-L127)）。飞书 auth 路由遵循同一模式。

扩展 `server.New` 签名（[server.go:39-51](../../../internal/server/server.go#L39-L51)）追加参数:
```go
func New(
    cfg *config.Config,
    pool *pgxpool.Pool,
    trigSvc *trigger.Service,
    synthSvc *synthesis.Service,
    delivSvc *delivery.Service,
    authSvc *auth.Service,
    authMgr *auth.Manager,
    authBL *auth.Blacklist,
    matcher *trigger.Matcher,
    settingsRepo *settings.Repo,
    settingsFactory *settings.Factory,
    feishuAuthHandler *handler.FeishuAuthHandler, // 决策 #7: 可空，nil 时不注册路由
) *gin.Engine {
```

在 `return r` 之前追加路由注册:
```go
// 飞书 OAuth（决策 #7）
if feishuAuthHandler != nil {
    r.GET("/api/v1/auth/feishu/start", authMW, feishuAuthHandler.StartAuth)
    r.GET("/api/v1/auth/feishu/status", authMW, feishuAuthHandler.Status)
    r.POST("/api/v1/auth/feishu/revoke", authMW, feishuAuthHandler.Revoke)
    // callback 不需登录中间件（飞书跳转过来时靠 state 恢复 user_id）
    r.GET("/api/v1/auth/feishu/callback", feishuAuthHandler.Callback)
}
```

Modify `cmd/api/wire.go` — `Deps` 结构加字段（[wire.go:23-36](../../../cmd/api/wire.go#L23-L36)）:
```go
type Deps struct {
    // ... 现有字段 ...
    SettingsFactory   *settings.Factory
    FeishuFactory     *feishu.ClientFactory     // 决策 #7
    FeishuAuthHandler *handler.FeishuAuthHandler // 决策 #7
}
```

在 `Build` 函数中（紧跟 feishuFactory 构造之后，F006 Step 3 已有 feishuFactory 构造逻辑）追加 FeishuAuthHandler 构造:
```go
// 决策 #7: 飞书 OAuth handler
var feishuAuthHandler *handler.FeishuAuthHandler
if feishuFactory != nil {
    // 复用同一 encKey 构造 tokenStore（与 feishuFactory 内部共享同一存储）
    tokenStore := feishu.NewTokenStore(pool, cfg.DBEncryptionKey)
    authClient := feishu.NewAuthClient(feishu.OAuthConfig{
        AppID:       cfg.FeishuAppID,
        AppSecret:   cfg.FeishuAppSecret,
        RedirectURL: cfg.FeishuRedirectURL,
    })
    feishuAuthHandler = handler.NewFeishuAuthHandler(cfg, authClient, tokenStore)
    log.Println("[feishu] auth handler initialized")
}
```

在 `return &Deps{...}` 中加 `FeishuAuthHandler: feishuAuthHandler`。

更新 `Deps.Server()` 方法（[wire.go:103-105](../../../cmd/api/wire.go#L103-L105)）传入新参数:
```go
func (d *Deps) Server() *gin.Engine {
    return server.New(d.Cfg, d.Pool, d.Trigger, d.Syn, d.Deliv, d.Auth, d.AuthMgr, d.AuthBL, d.Matcher, d.SettingsRepo, d.SettingsFactory, d.FeishuAuthHandler)
}
```

- [ ] **Step 3: 写 feishu_auth handler 测试**

Create file `internal/handler/feishu_auth_test.go`。测试放在 `handler` 内部包（因为要测 `parseOAuthState` / `buildOAuthState` 私有函数）:
```go
package handler

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildAndParseOAuthState(t *testing.T) {
	csrf := "csrf-random-abc"
	uid := uuid.New()
	state := buildOAuthState(csrf, uid)

	parsedCSRF, parsedUID, err := parseOAuthState(state)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsedCSRF != csrf {
		t.Errorf("csrf: want %s, got %s", csrf, parsedCSRF)
	}
	if parsedUID != uid {
		t.Errorf("uid: want %s, got %s", uid, parsedUID)
	}
}

func TestParseOAuthState_InvalidFormat(t *testing.T) {
	cases := []string{
		"no-underscore",
		"_userid_only", // csrf 为空
		"csrf_only_",   // userid 为空
		"csrf_not-a-uuid",
		"",
	}
	for _, s := range cases {
		if _, _, err := parseOAuthState(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

func TestParseOAuthState_ValidUUID(t *testing.T) {
	csrf := "abc123XYZ"
	uid := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	state := buildOAuthState(csrf, uid)
	parsedCSRF, parsedUID, err := parseOAuthState(state)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsedCSRF != csrf || parsedUID != uid {
		t.Errorf("roundtrip mismatch: csrf=%s uid=%s", parsedCSRF, parsedUID)
	}
}
```

> **说明**: Status/Revoke/StartAuth 的 HTTP 层测试需要 mock auth middleware 注入 user_id，较重。MVP 阶段只测 `parseOAuthState` / `buildOAuthState` 的纯函数逻辑（覆盖 B1 修复点）。HTTP 层测试留到集成测试阶段。

- [ ] **Step 4: 前端 API client**

Create file `web/src/api/feishu.ts`:
```typescript
import { apiFetch } from "./client";

export interface FeishuAuthStatus {
  status: "authorized" | "not_authorized";
  name: string;
}

export async function getFeishuStatus(): Promise<FeishuAuthStatus> {
  const res = await apiFetch("/api/v1/auth/feishu/status");
  const data = await res.json();
  return data.data ?? data;
}

export async function startFeishuAuth(): Promise<string> {
  const res = await apiFetch("/api/v1/auth/feishu/start", { method: "GET" });
  const data = await res.json();
  return data.data.authorize_url;
}

export async function revokeFeishuAuth(): Promise<void> {
  await apiFetch("/api/v1/auth/feishu/revoke", { method: "POST" });
}
```

- [ ] **Step 5: 前端 Settings 页加飞书区块**

Modify `web/src/components/Settings.tsx` — 在 Notion/Obsidian 配置后追加飞书授权区块:
```tsx
// 飞书集成（决策 #7）
<div className="settings-section">
  <h3>飞书集成</h3>
  <p className="hint">授权后可拉取飞书任务、接收任务事件、交付到飞书文档</p>
  {feishuStatus?.status === "authorized" ? (
    <div className="feishu-authorized">
      <span>已授权: {feishuStatus.name}</span>
      <button onClick={handleRevokeFeishu}>解除授权</button>
    </div>
  ) : (
    <button onClick={handleStartFeishuAuth}>授权飞书账号</button>
  )}
</div>
```

并加对应 state 和 handler:
```typescript
const [feishuStatus, setFeishuStatus] = useState<FeishuAuthStatus | null>(null);

useEffect(() => {
  getFeishuStatus().then(setFeishuStatus).catch(() => {});
}, []);

const handleStartFeishuAuth = async () => {
  const url = await startFeishuAuth();
  window.location.href = url; // 跳转飞书授权页
};

const handleRevokeFeishu = async () => {
  await revokeFeishuAuth();
  setFeishuStatus({ status: "not_authorized", name: "" });
};
```

- [ ] **Step 6: 跑全量测试 + 编译**

Run: `cd "k:\go_projects\AsyncStarterAgent" && go build ./... && go test ./...`
Expected: 全部通过

- [ ] **Step 7: 端到端验证（手动）**

1. 在飞书开放平台创建应用，配置回调 URL 为 `http://localhost:8080/api/v1/auth/feishu/callback`
2. 申请权限：`task:task:read` / `task:task:write` / `task:comment:write` / `docx:document:create`
3. 配置 .env 的 `FEISHU_APP_ID` / `FEISHU_APP_SECRET` / `DB_ENCRYPTION_KEY`
4. 启动后端 + 前端
5. 设置页点击"授权飞书账号" → 跳转飞书 → 授权 → 回调 → 显示"已授权"
6. 手动触发"写周报" → 验证拉取飞书任务作为上下文

- [ ] **Step 8: 展示 diff 等用户决定**

---

## 退出标准验证

完成 F001-F012 后逐项验证:

- [ ] `go build ./...` 编译通过
- [ ] `go test ./...` 全部 PASS
- [ ] feishu_tokens 表创建成功，pgcrypto 扩展启用
- [ ] OAuth 流程跑通：前端授权 → 回调 → token 存储
- [ ] IM 消息适配器接线到 wire.go（不再是孤儿代码）
- [ ] 飞书任务拉取适配器单测通过
- [ ] 飞书 webhook challenge 校验通过
- [ ] 飞书任务评论回写（FR-D04）单测通过
- [ ] 飞书文档创建（FR-D03）单测通过
- [ ] 前端设置页飞书授权区块显示正常
- [ ] 更新 [task-tracker.html](../../../doc/task-tracker.html) 中飞书集成状态
- [ ] 更新 [decision-log.md](../../../doc/decision-log.md) 记录实现完成

---

## 风险与未决问题

| # | 问题 | 缓解 |
|---|---|---|
| 1 | 飞书 docx SDK block builder API 名称未完全验证 | F011 Step 2 验证，如不符先做空文档创建 |
| 2 | 飞书事件订阅加密模式（EncryptKey）未实现 | MVP 先明文 + token 校验，加密模式作为加固项 |
| 3 | state → user_id 映射用 cookie + state 编码，不够安全 | MVP 单用户可接受，多用户改 Redis |
| 4 | lark SDK 升级 v3.4.4 → v3.4.25 可能有 breaking change | 升级后跑全量测试，记录 diff |
| 5 | 飞书任务 List 接口不支持时间范围过滤 | 客户端按 updated_at 增量过滤 |
| 6 | 多用户并发刷新 token | ClientFactory 用 mutex 防并发（F005 已实现） |

---

**下一步**: 本计划评审通过后，按 `superpowers:subagent-driven-development` 或 `superpowers:executing-plans` 执行。
