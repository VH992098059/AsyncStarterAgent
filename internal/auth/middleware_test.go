package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter(mgr *Manager, bl *Blacklist) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(mgr, bl))
	r.GET("/protected", func(c *gin.Context) {
		uid, _ := c.Get(ContextUserIDKey)
		uname, _ := c.Get(ContextUsernameKey)
		c.JSON(http.StatusOK, gin.H{"user_id": uid, "username": uname})
	})
	return r
}

func TestMiddleware_ValidToken(t *testing.T) {
	mgr := NewManager("test-secret-32-bytes-or-longer", 1e9)
	tok, _ := mgr.Sign("11111111-1111-1111-1111-111111111111", "alice")
	r := newTestRouter(mgr, NewBlacklist())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestMiddleware_MissingHeader(t *testing.T) {
	mgr := NewManager("test-secret-32-bytes-or-longer", 1e9)
	r := newTestRouter(mgr, NewBlacklist())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	mgr := NewManager("test-secret-32-bytes-or-longer", 1e9)
	r := newTestRouter(mgr, NewBlacklist())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMiddleware_RevokedToken(t *testing.T) {
	mgr := NewManager("test-secret-32-bytes-or-longer", 1e9)
	bl := NewBlacklist()
	tok, _ := mgr.Sign("11111111-1111-1111-1111-111111111111", "alice")
	bl.Revoke(extractTokenID(tok), 1e9)

	r := newTestRouter(mgr, bl)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (revoked), got %d", w.Code)
	}
}
