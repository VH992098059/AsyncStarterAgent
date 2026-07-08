package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
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
	feishuTokenURL     = "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token"
	feishuRefreshURL   = "https://open.feishu.cn/open-apis/authen/v1/oidc/refresh_access_token"
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
	body.Set("app_id", c.cfg.AppID)
	body.Set("app_secret", c.cfg.AppSecret)
	encoded := body.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, feishuTokenURL, strings.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("build exchange req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
