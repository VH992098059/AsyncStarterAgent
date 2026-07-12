package server

import (
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/delivery"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/middleware"
	"github.com/asyncstarter/agent/internal/queue"
	"github.com/asyncstarter/agent/internal/ratelimit"
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
	authBL auth.BlacklistStore,
	matcher *trigger.Matcher,
	settingsRepo *settings.Repo,
	settingsFactory *settings.Factory,
	feishuAuthHandler *handler.FeishuAuthHandler,
	feishuAppConfigHandler *handler.FeishuAppConfigHandler,
	loginLimiter *ratelimit.Limiter,
	queueClient *queue.Client,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	// 问题 #10: 全局请求体大小限制，防止巨大 body（如 draft markdown）占用大量内存/DB 带宽
	r.Use(middleware.MaxBodyBytes(5 << 20)) // 5 MiB

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
		Svc:          authSvc,
		Mgr:          authMgr,
		BL:           authBL,
		TTL:          7 * 24 * time.Hour,
		LoginLimiter: loginLimiter, // 问题 #8: 按用户名限流登录尝试，防暴力破解
	}
	r.POST("/api/v1/auth/register", ah.Register)
	r.POST("/api/v1/auth/login", ah.Login)
	authMW := auth.Middleware(authMgr, authBL)
	r.POST("/api/v1/auth/logout", authMW, ah.Logout)
	r.GET("/api/v1/auth/me", authMW, ah.Me)

	wh := &handler.WebhookHandler{
		Secret: cfg.TodoistWebhookSecret,
		Svc:    trigSvc,
		Pool:   pool, // 决策 #7: 查 feishu_tokens
		Cfg:    cfg,  // 决策 #7: 读 FeishuVerificationToken
	}
	r.POST("/api/v1/webhook/todoist", wh.Todoist)
	// 决策 #7: 飞书事件订阅 webhook（无需登录中间件，飞书直接推送）
	r.POST("/api/v1/webhook/feishu", wh.HandleFeishuWebhook)

	lh := &handler.ListHandler{Pool: pool, Matcher: matcher}
	r.GET("/api/v1/keywords", authMW, lh.GetKeywords)
	r.GET("/api/v1/datasources", authMW, lh.GetDataSources)
	r.GET("/api/v1/agent-runs", authMW, lh.GetAgentRuns)

	th := &handler.TriggerHandler{Svc: trigSvc, Queue: queueClient} // 问题 #11: 异步入队草稿生成
	r.POST("/api/v1/trigger", authMW, th.ManualTrigger)

	dh := &handler.DraftStreamHandler{Svc: synthSvc, Pool: pool}
	r.GET("/api/v1/drafts/:id/stream", authMW, dh.Stream)

	dlvH := &handler.DeliveryHandler{Svc: delivSvc}
	r.POST("/api/v1/drafts/:id/deliver", authMW, dlvH.Deliver)

	// AgentRun CRUD：详情/删除/取消/重试
	arh := &handler.AgentRunHandler{Pool: pool}
	r.GET("/api/v1/agent-runs/:id", authMW, arh.GetRun)
	r.DELETE("/api/v1/agent-runs/:id", authMW, arh.DeleteRun)
	r.POST("/api/v1/agent-runs/:id/cancel", authMW, arh.CancelRun)
	r.POST("/api/v1/agent-runs/:id/retry", authMW, arh.RetryRun)

	// Draft 详情/更新/mark 解决/交付历史
	ddh := &handler.DraftDetailHandler{Pool: pool, Syn: synthSvc}
	r.GET("/api/v1/drafts/:id", authMW, ddh.GetDraft)
	r.PUT("/api/v1/drafts/:id", authMW, ddh.UpdateDraft)
	r.POST("/api/v1/drafts/:id/marks/:markID/resolve", authMW, ddh.ResolveMark)
	r.GET("/api/v1/drafts/:id/deliveries", authMW, ddh.ListDeliveries)

	// Chat：历史消息 + 流式 LLM 对话（单轮续跑）
	ch := &handler.ChatHandler{Pool: pool, Syn: synthSvc}
	r.GET("/api/v1/agent-runs/:id/messages", authMW, ch.ListMessages)
	r.POST("/api/v1/agent-runs/:id/chat", authMW, ch.Chat)

	sh := &handler.SettingsHandler{Repo: settingsRepo, Fac: settingsFactory, Syn: synthSvc}
	r.GET("/api/v1/settings", authMW, sh.Get)
	r.PUT("/api/v1/settings", authMW, sh.Update)
	r.POST("/api/v1/settings/test-llm", authMW, sh.TestLLM)

	// 决策 #7 + 用户自建应用凭证: 飞书 OAuth 授权流程 + 应用凭证管理
	// Callback 无 authMW：飞书回跳时浏览器无 JWT，靠 state 中的 user_id 恢复身份。
	r.GET("/api/v1/auth/feishu/start", authMW, feishuAuthHandler.StartAuth)
	r.GET("/api/v1/auth/feishu/status", authMW, feishuAuthHandler.Status)
	r.POST("/api/v1/auth/feishu/revoke", authMW, feishuAuthHandler.Revoke)
	r.GET("/api/v1/auth/feishu/callback", feishuAuthHandler.Callback)

	r.GET("/api/v1/feishu/app-config", authMW, feishuAppConfigHandler.Get)
	r.PUT("/api/v1/feishu/app-config", authMW, feishuAppConfigHandler.Put)
	r.DELETE("/api/v1/feishu/app-config", authMW, feishuAppConfigHandler.Delete)

	return r
}
