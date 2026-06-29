package server

import (
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/delivery"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/middleware"
	"github.com/asyncstarter/agent/internal/settings"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var allowedOrigins = []string{
	"http://localhost:1420",
	"http://localhost:5173",
	"tauri://localhost",
	"http://tauri.localhost",
	"https://tauri.localhost",
}

func allowOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func New(
	cfg *config.Config,
	pool *pgxpool.Pool,
	trigSvc *trigger.Service,
	synthSvc *synthesis.Service,
	delivSvc *delivery.Service,
	authSvc *auth.Service,
	authMgr *auth.Manager,
	authBL *auth.Blacklist,
	matcher *trigger.Matcher,
	settingsRepo *settings.Repo,
	settingsFactory *settings.Factory,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	r.Use(cors.New(cors.Config{
		AllowOriginFunc:  allowOrigin,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(func(c *gin.Context) {
		c.Set("env", cfg.Env)
		c.Next()
	})

	r.GET("/health", handler.Health)

	ah := &handler.AuthHandler{
		Svc: authSvc,
		Mgr: authMgr,
		BL:  authBL,
		TTL: 7 * 24 * time.Hour,
	}
	r.POST("/api/v1/auth/register", ah.Register)
	r.POST("/api/v1/auth/login", ah.Login)
	authMW := auth.Middleware(authMgr, authBL)
	r.POST("/api/v1/auth/logout", authMW, ah.Logout)
	r.GET("/api/v1/auth/me", authMW, ah.Me)

	wh := &handler.WebhookHandler{
		Secret: cfg.TodoistWebhookSecret,
		Svc:    trigSvc,
	}
	r.POST("/api/v1/webhook/todoist", wh.Todoist)

	lh := &handler.ListHandler{Pool: pool, Matcher: matcher}
	r.GET("/api/v1/keywords", authMW, lh.GetKeywords)
	r.GET("/api/v1/datasources", authMW, lh.GetDataSources)
	r.GET("/api/v1/agent-runs", authMW, lh.GetAgentRuns)

	th := &handler.TriggerHandler{Svc: trigSvc}
	r.POST("/api/v1/trigger", authMW, th.ManualTrigger)

	dh := &handler.DraftStreamHandler{Svc: synthSvc}
	r.GET("/api/v1/drafts/:id/stream", authMW, dh.Stream)

	dlvH := &handler.DeliveryHandler{Svc: delivSvc}
	r.POST("/api/v1/drafts/:id/deliver", authMW, dlvH.Deliver)

	sh := &handler.SettingsHandler{Repo: settingsRepo, Fac: settingsFactory, Syn: synthSvc}
	r.GET("/api/v1/settings", authMW, sh.Get)
	r.PUT("/api/v1/settings", authMW, sh.Update)
	r.POST("/api/v1/settings/test-llm", authMW, sh.TestLLM)

	return r
}
