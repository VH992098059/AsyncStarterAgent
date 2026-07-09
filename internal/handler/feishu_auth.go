package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/feishu"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/jackc/pgx/v5"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FeishuAuthHandler 暴露飞书 OAuth 授权流程的四个端点：
//   - StartAuth  GET  /api/v1/auth/feishu/start    前端拉起授权（生成 state，服务端存储）
//   - Callback   GET  /api/v1/auth/feishu/callback 飞书回调（无 authMW，靠 state 恢复 user_id）
//   - Status     GET  /api/v1/auth/feishu/status   查询当前用户授权状态
//   - Revoke     POST /api/v1/auth/feishu/revoke   撤销授权（删除 token）
//
// CSRF 防护采用服务端 state 存储（替代 cookie 方案）：
// state 通过 URL 在飞书和 Callback 间传递，不依赖浏览器 cookie，
// 适用于 Tauri/跨源部署（授权在系统浏览器进行，cookie 存在 webview，无法跨进程共享）。
type FeishuAuthHandler struct {
	authClient feishuOAuthClient
	tokenStore feishu.TokenStore
	states     *oauthStateStore
}

// feishuOAuthClient 抽象飞书 OAuth 客户端方法，便于测试 mock。
// 生产环境由 *feishu.AuthClient 实现。
type feishuOAuthClient interface {
	AuthorizeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*feishu.TokenResponse, error)
}

func NewFeishuAuthHandler(authClient feishuOAuthClient, tokenStore feishu.TokenStore) *FeishuAuthHandler {
	return &FeishuAuthHandler{
		authClient: authClient,
		tokenStore: tokenStore,
		states:     newOAuthStateStore(),
	}
}

// oauthStateEntry 存储一次 OAuth 授权流程的上下文（StartAuth 写入，Callback 消费后删除）
type oauthStateEntry struct {
	userID    uuid.UUID
	expiresAt time.Time
}

// oauthStateStore 是 OAuth state 的服务端存储（内存 map + mutex + TTL）。
// 替代 cookie 方案：state 通过 URL 传递，不依赖浏览器 cookie，适用于 Tauri/跨源部署。
type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]oauthStateEntry
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{states: make(map[string]oauthStateEntry)}
}

// Set 存储 state → userID 映射，TTL 后自动过期。
// 同时清理已过期的旧 entry（机会式清理，避免内存泄漏）。
func (s *oauthStateStore) Set(state string, userID uuid.UUID, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state] = oauthStateEntry{userID: userID, expiresAt: time.Now().Add(ttl)}
	now := time.Now()
	for k, v := range s.states {
		if v.expiresAt.Before(now) {
			delete(s.states, k)
		}
	}
}

// Consume 查找并删除 state（一次性使用）。
// 返回 userID 和是否有效（存在且未过期）。
func (s *oauthStateStore) Consume(state string) (uuid.UUID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.states[state]
	if !ok {
		return uuid.Nil, false
	}
	delete(s.states, state) // 一次性使用，防重放
	if e.expiresAt.Before(time.Now()) {
		return uuid.Nil, false
	}
	return e.userID, true
}

// StartAuth GET /api/v1/auth/feishu/start
//
// 流程：
//  1. 取 userID（auth.MustUserID，未登录直接 401）
//  2. 生成 csrf=uuid（随机串）
//  3. state = buildOAuthState(csrf, userID)（user_id 编码进 state，Callback 无 session 时也能恢复）
//  4. 服务端存储 state → userID（10 分钟 TTL，不依赖 cookie）
//  5. 返回 {authorize_url} 给前端，前端跳转
func (h *FeishuAuthHandler) StartAuth(c *gin.Context) {
	userID, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 401, "unauthorized")
		return
	}

	csrf := uuid.New().String()
	state := buildOAuthState(csrf, userID)
	h.states.Set(state, userID, 10*time.Minute) // 服务端存储，不依赖 cookie

	url := h.authClient.AuthorizeURL(state)
	httpx.OK(c, gin.H{"authorize_url": url})
}

// Callback GET /api/v1/auth/feishu/callback
//
// 飞书回跳到此端点（无 authMW，因为飞书跳过来时浏览器没有 JWT）。
// state 通过 URL 传递，服务端消费 state 恢复 user_id。
//
// 流程：
//  1. 校验 code/state 非空
//  2. Consume(state) → userID（一次性消费，防重放）
//  3. ExchangeCode → ToTokenRecord(userID) → tokenStore.Save
//  4. 成功：返回 HTML 成功页（浏览器导航，JSON 不适用）
//     失败：返回 HTML 错误页（log 记录完整错误，返回泛化文案）
func (h *FeishuAuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		h.renderCallbackError(c, http.StatusBadRequest, "缺少授权参数")
		return
	}

	userID, ok := h.states.Consume(state)
	if !ok {
		h.renderCallbackError(c, http.StatusBadRequest, "授权状态无效或已过期，请重新授权")
		return
	}

	tr, err := h.authClient.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		log.Printf("[feishu-auth] exchange code failed for user %s: %v", userID, err)
		h.renderCallbackError(c, http.StatusBadGateway, "飞书授权码交换失败，请重试")
		return
	}

	rec := tr.ToTokenRecord(userID)
	if err := h.tokenStore.Save(c.Request.Context(), rec); err != nil {
		log.Printf("[feishu-auth] save token failed for user %s: %v", userID, err)
		h.renderCallbackError(c, http.StatusInternalServerError, "保存授权信息失败，请重试")
		return
	}
	log.Printf("[feishu-auth] user %s authorized successfully (open_id=%s)", userID, tr.OpenID)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(feishuAuthSuccessHTML))
}

// Status GET /api/v1/auth/feishu/status
//
// 返回当前用户飞书授权状态：
//   - 已授权：{status:"authorized", name:"<飞书用户名>"}
//   - 未授权：{status:"not_authorized", name:""}
//
// 仅检查 token 记录是否存在（不区分过期，过期可由 refresh 恢复）。
func (h *FeishuAuthHandler) Status(c *gin.Context) {
	userID, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 401, "unauthorized")
		return
	}

	rec, err := h.tokenStore.Get(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.OK(c, gin.H{"status": "not_authorized", "name": ""})
			return
		}
		log.Printf("[feishu-auth] status query failed for user %s: %v", userID, err)
		httpx.Fail(c, http.StatusInternalServerError, 500, "status query failed")
		return
	}
	httpx.OK(c, gin.H{"status": "authorized", "name": rec.Name})
}

// Revoke POST /api/v1/auth/feishu/revoke
//
// 撤销授权：删除 tokenStore 中的记录。
// 返回 {status:"revoked"}。
func (h *FeishuAuthHandler) Revoke(c *gin.Context) {
	userID, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 401, "unauthorized")
		return
	}

	if err := h.tokenStore.Delete(c.Request.Context(), userID); err != nil {
		log.Printf("[feishu-auth] revoke failed for user %s: %v", userID, err)
		httpx.Fail(c, http.StatusInternalServerError, 500, "revoke failed")
		return
	}
	httpx.OK(c, gin.H{"status": "revoked"})
}

// buildOAuthState 构造 OAuth state 参数：`<csrf>_<userID>`。
// csrf 是随机串用于增加熵，user_id 直接编码进 state，
// 让 Callback 即使飞书跳转过来时无登录 session，也能从 state 恢复 user_id。
func buildOAuthState(csrf string, userID uuid.UUID) string {
	return csrf + "_" + userID.String()
}

// parseOAuthState 解析 state 字符串，找第一个 "_"，前半 csrf，后半 uuid.Parse。
// 注意必须找"第一个"下划线，因为 csrf (uuid) 本身不含下划线，但用 SplitN 更稳健。
//
// 注意：state 的 CSRF 防护现已由服务端 oauthStateStore 承担（Consume 校验存在性），
// 本函数仅作为 state 格式的文档化工具保留，parseOAuthState 自身不再参与鉴权决策。
func parseOAuthState(state string) (string, uuid.UUID, error) {
	idx := strings.Index(state, "_")
	if idx < 0 {
		return "", uuid.Nil, errors.New("invalid state: missing underscore")
	}
	csrf := state[:idx]
	userIDStr := state[idx+1:]
	if csrf == "" || userIDStr == "" {
		return "", uuid.Nil, errors.New("invalid state: empty segment")
	}
	uid, err := uuid.Parse(userIDStr)
	if err != nil {
		return "", uuid.Nil, errors.New("invalid state: bad user id")
	}
	return csrf, uid, nil
}

// feishuAuthSuccessHTML 是 Callback 成功时返回给浏览器的页面（深色主题，与前端风格一致）。
const feishuAuthSuccessHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>飞书授权</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;background:#0a0a0a;color:#e5e5e5}
.card{text-align:center;padding:2rem}
.icon{width:48px;height:48px;margin:0 auto 1rem;border-radius:50%;background:rgba(16,185,129,0.1);display:flex;align-items:center;justify-content:center;color:#10b981}
h1{font-size:1.125rem;font-weight:600;color:#10b981;margin-bottom:0.5rem}
p{font-size:0.875rem;color:#a3a3a3}
</style>
</head>
<body>
<div class="card">
<div class="icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5"/></svg></div>
<h1>授权成功</h1>
<p>飞书账号已绑定，请返回应用</p>
</div>
</body>
</html>`

// feishuAuthErrorHTML 是 Callback 失败时返回给浏览器的页面。
// 含一个 {{MSG}} 占位符，由 renderCallbackError 用 strings.Replace 填入用户可见的泛化错误文案。
// 不用 fmt.Sprintf：HTML/CSS 中的 % 字符（如 border-radius:50%）会被 go vet 误判为格式动词。
const feishuAuthErrorHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>飞书授权</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;background:#0a0a0a;color:#e5e5e5}
.card{text-align:center;padding:2rem}
.icon{width:48px;height:48px;margin:0 auto 1rem;border-radius:50%;background:rgba(239,68,68,0.1);display:flex;align-items:center;justify-content:center;color:#ef4444}
h1{font-size:1.125rem;font-weight:600;color:#ef4444;margin-bottom:0.5rem}
p{font-size:0.875rem;color:#a3a3a3}
</style>
</head>
<body>
<div class="card">
<div class="icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg></div>
<h1>授权失败</h1>
<p>{{MSG}}</p>
</div>
</body>
</html>`

// renderCallbackError 返回 HTML 错误页给浏览器（Callback 是浏览器导航，JSON 不适用）
func (h *FeishuAuthHandler) renderCallbackError(c *gin.Context, httpStatus int, msg string) {
	body := strings.Replace(feishuAuthErrorHTML, "{{MSG}}", msg, 1)
	c.Data(httpStatus, "text/html; charset=utf-8", []byte(body))
}
