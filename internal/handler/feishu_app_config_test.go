package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/feishu"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// fakeAppConfigStoreForConfig 复用 feishu_auth_test.go 中的 fakeAppConfigStore（同包）

type fakeTokenStoreForConfig struct {
	deleted bool
}

func (f *fakeTokenStoreForConfig) Save(ctx context.Context, rec feishu.TokenRecord) error { return nil }
func (f *fakeTokenStoreForConfig) Get(ctx context.Context, userID uuid.UUID) (*feishu.TokenRecord, error) {
	return nil, pgx.ErrNoRows
}
func (f *fakeTokenStoreForConfig) Delete(ctx context.Context, userID uuid.UUID) error {
	f.deleted = true
	return nil
}

func newTestConfigContext(userID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(auth.ContextUserIDKey, userID.String())
	return c, w
}

func TestFeishuAppConfig_Get_NotConfigured(t *testing.T) {
	h := &FeishuAppConfigHandler{Store: &fakeAppConfigStore{}, TokenStore: &fakeTokenStoreForConfig{}}
	c, w := newTestConfigContext(uuid.New())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/feishu/app-config", nil)

	h.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Errorf("expected configured:false, got %s", w.Body.String())
	}
}

func TestFeishuAppConfig_Put_SavesAndMasks(t *testing.T) {
	store := &fakeAppConfigStore{}
	tokenStore := &fakeTokenStoreForConfig{}
	h := &FeishuAppConfigHandler{Store: store, TokenStore: tokenStore}
	userID := uuid.New()
	c, w := newTestConfigContext(userID)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/feishu/app-config",
		strings.NewReader(`{"app_id":"cli_abc123","app_secret":"secretvalue123456"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Put(c)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if store.rec == nil || store.rec.AppID != "cli_abc123" {
		t.Fatalf("expected saved app_id cli_abc123, got %+v", store.rec)
	}
	if store.rec.AppSecret != "secretvalue123456" {
		t.Errorf("expected plaintext secret stored via store, got %s", store.rec.AppSecret)
	}
	if strings.Contains(w.Body.String(), "secretvalue123456") {
		t.Error("response should not leak plaintext app_secret")
	}
}

func TestFeishuAppConfig_Put_ClearsOldToken(t *testing.T) {
	store := &fakeAppConfigStore{}
	tokenStore := &fakeTokenStoreForConfig{}
	h := &FeishuAppConfigHandler{Store: store, TokenStore: tokenStore}
	userID := uuid.New()
	c, w := newTestConfigContext(userID)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/feishu/app-config",
		strings.NewReader(`{"app_id":"cli_new","app_secret":"newsecret1234567"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Put(c)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !tokenStore.deleted {
		t.Error("expected old feishu_tokens record to be deleted when app config changes")
	}
}

func TestFeishuAppConfig_Delete(t *testing.T) {
	store := &fakeAppConfigStore{rec: &feishu.AppConfigRecord{AppID: "cli_x", AppSecret: "y"}}
	tokenStore := &fakeTokenStoreForConfig{}
	h := &FeishuAppConfigHandler{Store: store, TokenStore: tokenStore}
	c, w := newTestConfigContext(uuid.New())
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/feishu/app-config", nil)

	h.Delete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if store.rec != nil {
		t.Error("expected app config to be deleted")
	}
	if !tokenStore.deleted {
		t.Error("expected feishu_tokens to be cascaded deleted")
	}
}
