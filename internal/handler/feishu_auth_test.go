package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/feishu"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// --- state 编码/解码单元测试 ---

// TestBuildAndParseOAuthState 验证 state 编码/解码 roundtrip：
// build → parse 后 csrf / userID 应一致。
func TestBuildAndParseOAuthState(t *testing.T) {
	csrf := "01234567-89ab-cdef-0123-456789abcdef"
	uid := uuid.New()

	state := buildOAuthState(csrf, uid)
	gotCSRF, gotUID, err := parseOAuthState(state)
	if err != nil {
		t.Fatalf("parseOAuthState returned err: %v", err)
	}
	if gotCSRF != csrf {
		t.Errorf("csrf mismatch: want %q got %q", csrf, gotCSRF)
	}
	if gotUID != uid {
		t.Errorf("userID mismatch: want %q got %q", uid, gotUID)
	}
}

// TestParseOAuthState_InvalidFormat 覆盖各种非法 state 输入。
func TestParseOAuthState_InvalidFormat(t *testing.T) {
	cases := []struct {
		name  string
		state string
	}{
		{"no underscore", "no-underscore-here"},
		{"underscore at start (only userid)", "_550e8400-e29b-41d4-a716-446655440000"},
		{"underscore at end (csrf only)", "csrf_only_"},
		{"csrf + not-a-uuid", "csrf_not-a-uuid"},
		{"empty string", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseOAuthState(tc.state)
			if err == nil {
				t.Fatalf("expected error for state=%q, got nil", tc.state)
			}
		})
	}
}

// TestParseOAuthState_ValidUUID 用固定 UUID 验证解析（避免随机 UUID 不可重现）。
func TestParseOAuthState_ValidUUID(t *testing.T) {
	fixedUID, err := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("setup: parse fixed uuid: %v", err)
	}
	state := "abc-csrf-token_550e8400-e29b-41d4-a716-446655440000"

	gotCSRF, gotUID, err := parseOAuthState(state)
	if err != nil {
		t.Fatalf("parseOAuthState returned err: %v", err)
	}
	if gotCSRF != "abc-csrf-token" {
		t.Errorf("csrf mismatch: want %q got %q", "abc-csrf-token", gotCSRF)
	}
	if gotUID != fixedUID {
		t.Errorf("userID mismatch: want %q got %q", fixedUID, gotUID)
	}
}

// --- mock 类型 ---

// fakeOAuthClient mock feishuOAuthClient 接口
type fakeOAuthClient struct {
	tokenResp   *feishu.TokenResponse
	exchangeErr error
}

func (f *fakeOAuthClient) AuthorizeURL(appID, state string) string {
	return "https://open.feishu.cn/open-apis/authen/v1/index?state=" + url.QueryEscape(state)
}

func (f *fakeOAuthClient) ExchangeCode(ctx context.Context, appID, appSecret, code string) (*feishu.TokenResponse, error) {
	if f.exchangeErr != nil {
		return nil, f.exchangeErr
	}
	return f.tokenResp, nil
}

// fakeTokenStore mock feishu.TokenStore 接口
type fakeTokenStore struct {
	saved     []*feishu.TokenRecord
	getRec    *feishu.TokenRecord
	getErr    error
	deleteErr error
}

func (f *fakeTokenStore) Save(ctx context.Context, rec feishu.TokenRecord) error {
	f.saved = append(f.saved, &rec)
	return nil
}

func (f *fakeTokenStore) Get(ctx context.Context, userID uuid.UUID) (*feishu.TokenRecord, error) {
	return f.getRec, f.getErr
}

func (f *fakeTokenStore) Delete(ctx context.Context, userID uuid.UUID) error {
	return f.deleteErr
}

// fakeAppConfigStore mock feishu.AppConfigStore 接口
type fakeAppConfigStore struct {
	rec *feishu.AppConfigRecord
}

func (f *fakeAppConfigStore) Get(ctx context.Context, userID uuid.UUID) (*feishu.AppConfigRecord, error) {
	if f.rec == nil {
		return nil, pgx.ErrNoRows
	}
	return f.rec, nil
}

func (f *fakeAppConfigStore) Upsert(ctx context.Context, rec feishu.AppConfigRecord) error {
	f.rec = &rec
	return nil
}

func (f *fakeAppConfigStore) Delete(ctx context.Context, userID uuid.UUID) error {
	f.rec = nil
	return nil
}

// --- Callback 集成测试 ---

// TestFeishuAuth_Callback_Success 验证完整 OAuth 回调流程：
// StartAuth 签发 state → Callback 消费 state + 换 token + 存储
func TestFeishuAuth_Callback_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeTokenStore{}
	client := &fakeOAuthClient{tokenResp: &feishu.TokenResponse{
		AccessToken: "at-xxx", RefreshToken: "rt-xxx", ExpiresIn: 7200, OpenID: "ou-xxx", Name: "Tester",
	}}
	appCfgStore := &fakeAppConfigStore{rec: &feishu.AppConfigRecord{AppID: "app-x", AppSecret: "secret-y"}}
	h := NewFeishuAuthHandler(client, store, appCfgStore)
	userID := uuid.New()

	// Step 1: StartAuth 签发 state（模拟 auth middleware 已注入 userID）
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/feishu/start", nil)
	c1.Set(auth.ContextUserIDKey, userID.String())
	h.StartAuth(c1)

	if w1.Code != http.StatusOK {
		t.Fatalf("StartAuth: want 200, got %d", w1.Code)
	}
	// 从响应中提取 state
	var resp struct {
		Code int `json:"code"`
		Data struct {
			AuthorizeURL string `json:"authorize_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w1.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse StartAuth resp: %v", err)
	}
	u, err := url.Parse(resp.Data.AuthorizeURL)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("state is empty in authorize url")
	}

	// Step 2: Callback 消费 state
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/feishu/callback?code=testcode&state="+state, nil)
	h.Callback(c2)

	if w2.Code != http.StatusOK {
		t.Errorf("Callback: want 200, got %d, body=%s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "授权成功") {
		t.Errorf("Callback: expected success HTML, got %s", w2.Body.String())
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected 1 saved token, got %d", len(store.saved))
	}
	if store.saved[0].UserID != userID {
		t.Errorf("saved userID: want %s, got %s", userID, store.saved[0].UserID)
	}
	if store.saved[0].AccessToken != "at-xxx" {
		t.Errorf("saved access_token: want at-xxx, got %s", store.saved[0].AccessToken)
	}
}

// TestFeishuAuth_Callback_StateReplay 验证 state 一次性使用（重放被拒）
func TestFeishuAuth_Callback_StateReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeTokenStore{}
	client := &fakeOAuthClient{tokenResp: &feishu.TokenResponse{AccessToken: "at", ExpiresIn: 7200}}
	appCfgStore := &fakeAppConfigStore{rec: &feishu.AppConfigRecord{AppID: "app-x", AppSecret: "secret-y"}}
	h := NewFeishuAuthHandler(client, store, appCfgStore)
	userID := uuid.New()

	// 签发 state
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodGet, "/start", nil)
	c1.Set(auth.ContextUserIDKey, userID.String())
	h.StartAuth(c1)
	var resp struct {
		Data struct {
			AuthorizeURL string `json:"authorize_url"`
		} `json:"data"`
	}
	json.Unmarshal(w1.Body.Bytes(), &resp)
	u, _ := url.Parse(resp.Data.AuthorizeURL)
	state := u.Query().Get("state")

	// 第一次 Callback：成功
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/callback?code=code1&state="+state, nil)
	h.Callback(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("first callback: want 200, got %d", w2.Code)
	}

	// 第二次 Callback（重放同一 state）：应失败
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/callback?code=code2&state="+state, nil)
	h.Callback(c3)
	if w3.Code == http.StatusOK {
		t.Error("replay callback: expected non-200, got 200 (state should be one-time use)")
	}
}

// TestFeishuAuth_Callback_InvalidState 验证无效 state 被拒
func TestFeishuAuth_Callback_InvalidState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeTokenStore{}
	client := &fakeOAuthClient{}
	h := NewFeishuAuthHandler(client, store, &fakeAppConfigStore{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/callback?code=code&state=forged-state", nil)
	h.Callback(c)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 for forged state")
	}
	if !strings.Contains(w.Body.String(), "授权失败") {
		t.Errorf("expected error HTML, got %s", w.Body.String())
	}
}

// TestFeishuAuth_StartAuth_AppNotConfigured 验证用户未配置飞书应用凭证时 StartAuth 报错
func TestFeishuAuth_StartAuth_AppNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewFeishuAuthHandler(&fakeOAuthClient{}, &fakeTokenStore{}, &fakeAppConfigStore{})
	userID := uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/start", nil)
	c.Set(auth.ContextUserIDKey, userID.String())
	h.StartAuth(c)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 when app credentials not configured")
	}
}
