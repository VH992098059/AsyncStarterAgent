package auth

import (
	"errors"
	"testing"
	"time"
)

// JWT secret 长度对签发/校验无功能影响，仅做单元测试最小化校验。
const testSecret = "test-jwt-secret-32-bytes-or-longer"

func TestJWT_SignAndParse_RoundTrip(t *testing.T) {
	mgr := NewManager(testSecret, 7*24*time.Hour)

	userID := "11111111-1111-1111-1111-111111111111"
	tok, err := mgr.Sign(userID, "alice")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}

	claims, err := mgr.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("user id: want %s, got %s", userID, claims.UserID)
	}
	if claims.Username != "alice" {
		t.Errorf("username: want alice, got %s", claims.Username)
	}
}

func TestJWT_Parse_InvalidSignature(t *testing.T) {
	mgr := NewManager(testSecret, time.Hour)
	other := NewManager("another-secret", time.Hour)
	tok, _ := other.Sign("u", "bob")

	if _, err := mgr.Parse(tok); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestJWT_Parse_Expired(t *testing.T) {
	mgr := NewManager(testSecret, -time.Second) // 立即过期
	tok, err := mgr.Sign("u", "bob")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := mgr.Parse(tok); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestJWT_Parse_Malformed(t *testing.T) {
	mgr := NewManager(testSecret, time.Hour)
	if _, err := mgr.Parse("not.a.token"); err == nil {
		t.Fatal("expected parse error on malformed token")
	}
}

func TestBlacklist_AddAndCheck(t *testing.T) {
	bl := NewBlacklist()
	if bl.IsRevoked("tok-1") {
		t.Fatal("fresh blacklist should not contain tok-1")
	}
	bl.Revoke("tok-1", time.Hour)
	if !bl.IsRevoked("tok-1") {
		t.Fatal("tok-1 should be revoked")
	}
	if bl.IsRevoked("tok-2") {
		t.Fatal("tok-2 should not be revoked")
	}
}

func TestBlacklist_ExpiredEntries(t *testing.T) {
	bl := NewBlacklist()
	// 50ms TTL → 100ms 后应被清理
	bl.Revoke("tok-exp", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if bl.IsRevoked("tok-exp") {
		t.Fatal("expired entry should not be considered revoked")
	}
}
