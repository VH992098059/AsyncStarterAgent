package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/gin-gonic/gin"
)

func setupAuthRouter(t *testing.T) (*gin.Engine, *auth.Service, *auth.Manager, *auth.Blacklist) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	pool, err := repository.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	gin.SetMode(gin.TestMode)
	svc := auth.NewService(pool, 4)
	mgr := auth.NewManager("test-secret-32-bytes-or-longer", time.Hour)
	bl := auth.NewBlacklist()

	r := gin.New()
	h := &handler.AuthHandler{Svc: svc, Mgr: mgr, BL: bl, TTL: time.Hour}
	r.POST("/api/v1/auth/register", h.Register)
	r.POST("/api/v1/auth/login", h.Login)
	r.POST("/api/v1/auth/logout", h.Logout)
	r.GET("/api/v1/auth/me", auth.Middleware(mgr, bl), h.Me)
	return r, svc, mgr, bl
}

func postJSON(r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_Register_Success(t *testing.T) {
	r, _, _, _ := setupAuthRouter(t)
	uname := "reg_" + time.Now().Format("150405.000000000")
	w := postJSON(r, "/api/v1/auth/register", map[string]string{
		"username": uname, "password": "secret-123",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	data := decodeData(t, w.Body.Bytes())
	if data["token"] == "" || data["token"] == nil {
		t.Fatal("missing token in response")
	}
}

func TestAuth_Register_DuplicateUsername(t *testing.T) {
	r, _, _, _ := setupAuthRouter(t)
	uname := "dup_" + time.Now().Format("150405.000000000")
	_ = postJSON(r, "/api/v1/auth/register", map[string]string{
		"username": uname, "password": "secret-123",
	})
	w := postJSON(r, "/api/v1/auth/register", map[string]string{
		"username": uname, "password": "secret-123",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestAuth_Login_InvalidPassword(t *testing.T) {
	r, _, _, _ := setupAuthRouter(t)
	uname := "login_" + time.Now().Format("150405.000000000")
	_ = postJSON(r, "/api/v1/auth/register", map[string]string{
		"username": uname, "password": "secret-123",
	})
	w := postJSON(r, "/api/v1/auth/login", map[string]string{
		"username": uname, "password": "wrong",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_Me_ValidToken(t *testing.T) {
	r, _, _, _ := setupAuthRouter(t)
	uname := "me_" + time.Now().Format("150405.000000000")
	reg := postJSON(r, "/api/v1/auth/register", map[string]string{
		"username": uname, "password": "secret-123",
	})
	tok := decodeData(t, reg.Body.Bytes())["token"].(string)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuth_Logout_RevokesToken(t *testing.T) {
	r, _, _, bl := setupAuthRouter(t)
	uname := "out_" + time.Now().Format("150405.000000000")
	reg := postJSON(r, "/api/v1/auth/register", map[string]string{
		"username": uname, "password": "secret-123",
	})
	tok := decodeData(t, reg.Body.Bytes())["token"].(string)

	// logout
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout expected 200, got %d", w.Code)
	}

	// 再访问 /me 应该 401（黑名单命中）
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("after logout expected 401, got %d", w2.Code)
	}

	// 黑名单应包含 token signature 段
	parts := splitToken(tok)
	if !bl.IsRevoked(parts[2]) {
		t.Fatal("token should be in blacklist after logout")
	}
}

func splitToken(tok string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(tok); i++ {
		if tok[i] == '.' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(tok[i])
		}
	}
	out = append(out, cur)
	return out
}
