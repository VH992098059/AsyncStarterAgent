package feishu

import (
	"context"
	"errors"
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

// TestClientFactory_ErrRefreshFailed_Unwrap 验证 ErrRefreshFailed 支持 errors.Is 链式判断
func TestClientFactory_ErrRefreshFailed_Unwrap(t *testing.T) {
	cause := errors.New("underlying")
	err := &ErrRefreshFailed{UserID: uuid.New(), Cause: cause}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should traverse Cause via Unwrap")
	}
}

// TestClientFactory_IsAuthorized_ExpiredToken 验证 IsAuthorized 对过期 token 仍返回 true
// （过期 access_token 可能通过 refresh 恢复，故视为已授权）
func TestClientFactory_IsAuthorized_ExpiredToken(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{
		UserID:      uid,
		AccessToken: "expired",
		ExpiresAt:   time.Now().Add(-1 * time.Minute), // 已过期
	})
	auth := NewAuthClient(OAuthConfig{AppID: "x", AppSecret: "y"})
	factory := NewClientFactory("x", "y", store, auth)
	if !factory.IsAuthorized(context.Background(), uid) {
		t.Error("expired token should still count as authorized (refresh may recover)")
	}
}
