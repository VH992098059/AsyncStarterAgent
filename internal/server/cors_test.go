package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/server"
	"github.com/gin-gonic/gin"
)

// 决策 #5 配套：跨域预检。
// dev 时前端在 http://localhost:1420（Vite）或 tauri://localhost（Tauri 桌面）。
// OPTIONS 必须返回 200 + Access-Control-Allow-* 头，否则浏览器拦截实际请求。
func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Env: "development"}
	return server.New(cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func TestCORS_Preflight_RegisterEndpoint(t *testing.T) {
	r := newTestRouter()
	req, _ := http.NewRequest("OPTIONS", "/api/v1/auth/register", nil)
	req.Header.Set("Origin", "http://localhost:1420")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("expected 200/204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Fatal("missing Access-Control-Allow-Origin header")
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("missing Access-Control-Allow-Methods header")
	}
}

func TestCORS_Preflight_TauriOrigin(t *testing.T) {
	r := newTestRouter()
	req, _ := http.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "tauri://localhost")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("expected 200/204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Fatal("missing Access-Control-Allow-Origin for tauri origin")
	}
}

func TestCORS_ActualRequest_HealthEndpoint(t *testing.T) {
	// GET /health 公共端点：实际请求应带 Access-Control-Allow-Origin
	r := newTestRouter()
	req, _ := http.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://localhost:1420")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Fatal("missing Access-Control-Allow-Origin on actual request")
	}
}
