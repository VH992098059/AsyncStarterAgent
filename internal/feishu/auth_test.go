package feishu

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mockHTTPDoer 记录请求并返回预设响应
type mockHTTPDoer struct {
	respBody   string
	respStatus int
	lastURL    string
	lastBody   string
	// delay 在返回响应前等待的时长，用于测试并发场景下真实耗时的刷新
	delay time.Duration
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	m.lastURL = req.URL.String()
	bodyBytes, _ := io.ReadAll(req.Body)
	m.lastBody = string(bodyBytes)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
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
		RedirectURL: "http://localhost:8080/api/v1/auth/feishu/callback",
	})
	u := c.AuthorizeURL("cli_xxx", "random-state-123")
	if !strings.Contains(u, "app_id=cli_xxx") {
		t.Errorf("missing app_id: %s", u)
	}
	if !strings.Contains(u, "state=random-state-123") {
		t.Errorf("missing state: %s", u)
	}
	if !strings.Contains(u, "response_type=code") {
		t.Errorf("missing response_type: %s", u)
	}
	if !strings.Contains(u, "redirect_uri=") {
		t.Errorf("missing redirect_uri: %s", u)
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
		HTTPClient: doer,
	})
	tr, err := c.ExchangeCode(context.Background(), "cli_xxx", "sec_yyy", "code-123")
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
	if !strings.Contains(doer.lastBody, "app_id=cli_xxx") {
		t.Errorf("request body missing app_id: %s", doer.lastBody)
	}
	if !strings.Contains(doer.lastBody, "app_secret=sec_yyy") {
		t.Errorf("request body missing app_secret: %s", doer.lastBody)
	}
}

func TestExchangeCode_Error(t *testing.T) {
	doer := &mockHTTPDoer{
		respBody: `{"code": 10001, "msg": "invalid code"}`,
	}
	c := NewAuthClient(OAuthConfig{HTTPClient: doer})
	_, err := c.ExchangeCode(context.Background(), "cli_xxx", "secret_yyy", "bad")
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
	c := NewAuthClient(OAuthConfig{HTTPClient: doer})
	tr, err := c.RefreshToken(context.Background(), "x", "y", "ur-old")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if tr.AccessToken != "u-new" {
		t.Errorf("access token: %s", tr.AccessToken)
	}
	if !strings.Contains(doer.lastBody, "refresh_token=ur-old") {
		t.Errorf("body missing refresh_token: %s", doer.lastBody)
	}
	if !strings.Contains(doer.lastBody, "app_id=x") {
		t.Errorf("body missing app_id: %s", doer.lastBody)
	}
	if !strings.Contains(doer.lastBody, "app_secret=y") {
		t.Errorf("body missing app_secret: %s", doer.lastBody)
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
	rec := tr.ToTokenRecord(uid, "cli_xxx")
	if rec.UserID != uid {
		t.Errorf("user id mismatch")
	}
	if rec.AppID != "cli_xxx" {
		t.Errorf("app id: %s", rec.AppID)
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
