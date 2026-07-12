package main

import (
	"context"
	"log"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/delivery"
	"github.com/asyncstarter/agent/internal/feishu"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/queue"
	"github.com/asyncstarter/agent/internal/ratelimit"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/server"
	"github.com/asyncstarter/agent/internal/settings"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Deps struct {
	Cfg                    *config.Config
	Pool                   *pgxpool.Pool
	Trigger                *trigger.Service
	Queue                  *queue.Client
	Syn                    *synthesis.Service
	Deliv                  *delivery.Service
	Auth                   *auth.Service
	AuthMgr                *auth.Manager
	AuthBL                 auth.BlacklistStore
	Matcher                *trigger.Matcher
	SettingsRepo           *settings.Repo
	SettingsFactory        *settings.Factory
	FeishuFactory          *feishu.ClientFactory
	FeishuAuthHandler      *handler.FeishuAuthHandler
	FeishuAppConfigHandler *handler.FeishuAppConfigHandler
	LoginLimiter           *ratelimit.Limiter
}

func Build(ctx context.Context, cfg *config.Config) (*Deps, error) {
	pool, err := repository.Open(ctx, cfg.DSN)
	if err != nil {
		return nil, err
	}

	q, err := queue.NewClient(cfg.RedisURL)
	if err != nil {
		pool.Close()
		return nil, err
	}

	matcher := trigger.NewMatcher(trigger.DefaultMatcherRules())
	trigSvc := trigger.NewService(pool, matcher)

	jwtMgr := auth.NewManager(cfg.JWTSecret, 7*24*time.Hour)
	// 问题 #9: JWT 黑名单迁移到 Redis（替代内存实现），解决重启丢失/多实例不共享问题。
	// NewRedisBlacklist 仅在 redisURL 为空/不可解析时报错（不会因 Redis 暂不可达而失败，
	// 客户端是惰性连接的），失败时回退到内存黑名单以保证服务仍可启动（如本地无 Redis 的开发环境）。
	var authBL auth.BlacklistStore
	if redisBL, err := auth.NewRedisBlacklist(cfg.RedisURL); err != nil {
		log.Printf("[auth] redis blacklist init failed, falling back to in-memory (won't survive restart/multi-instance): %v", err)
		authBL = auth.NewBlacklist()
	} else {
		authBL = redisBL
		log.Println("[auth] blacklist backed by redis")
	}
	authSvc := auth.NewService(pool, bcrypt.DefaultCost)
	if jwtMgr == nil {
		log.Println("[auth] JWT_SECRET empty, /api/v1/auth/* disabled")
	} else {
		log.Println("[auth] manager initialized, ttl=7d")
	}

	ddlH := func(ctx context.Context, taskID, userID, title string) error {
		uid, err := uuid.Parse(userID)
		if err != nil {
			return err
		}
		if _, err := trigSvc.ProcessKeyword(ctx, uid, title); err != nil {
			return err
		}
		_ = taskID
		return nil
	}

	go trigger.RunDDLScheduler(ctx, pool, trigger.NewDDLDetector(), ddlH)

	settingsRepo := settings.NewRepo(pool)

	// 决策 #7 + 用户自建应用凭证: 飞书 OAuth + token 工厂，始终初始化
	// （用户没配凭证时 GetClient 返回 ErrAppNotConfigured，不需要启动时整体开关）
	tokenStore := feishu.NewTokenStore(pool, cfg.DBEncryptionKey)
	appConfigStore := feishu.NewAppConfigStore(pool, cfg.DBEncryptionKey)
	authClient := feishu.NewAuthClient(feishu.OAuthConfig{
		RedirectURL: cfg.FeishuRedirectURL,
	})
	feishuFactory := feishu.NewClientFactory(tokenStore, appConfigStore, authClient)
	feishuAuthHandler := handler.NewFeishuAuthHandler(authClient, tokenStore, appConfigStore)
	feishuAppConfigHandler := &handler.FeishuAppConfigHandler{Store: appConfigStore, TokenStore: tokenStore}
	log.Println("[feishu] client factory initialized (per-user app credentials)")

	settingsFactory := settings.NewFactory(pool, settingsRepo, cfg, feishuFactory)

	synSvc := synthesis.NewService(pool, settingsFactory, "")
	log.Println("[synthesis] service initialized with per-user config factory")

	delivSvc := delivery.NewService(pool, settingsFactory, nil)
	log.Println("[delivery] service initialized with per-user config factory")

	log.Println("[ddl] scheduler started")

	// 问题 #8: 登录接口限流（按用户名维度，5 次/分钟）。Redis 不可达时 Login 内部 fail-open。
	loginLimiter, err := ratelimit.NewLimiter(cfg.RedisURL, 5, time.Minute)
	if err != nil {
		log.Printf("[ratelimit] disabled (login limiter init failed): %v", err)
		loginLimiter = nil
	}

	return &Deps{
		Cfg:                    cfg,
		Pool:                   pool,
		Trigger:                trigSvc,
		Queue:                  q,
		Syn:                    synSvc,
		Deliv:                  delivSvc,
		Auth:                   authSvc,
		AuthMgr:                jwtMgr,
		AuthBL:                 authBL,
		Matcher:                matcher,
		SettingsRepo:           settingsRepo,
		SettingsFactory:        settingsFactory,
		FeishuFactory:          feishuFactory,
		FeishuAuthHandler:      feishuAuthHandler,
		FeishuAppConfigHandler: feishuAppConfigHandler,
		LoginLimiter:           loginLimiter,
	}, nil
}

func (d *Deps) Server() *gin.Engine {
	return server.New(d.Cfg, d.Pool, d.Trigger, d.Syn, d.Deliv, d.Auth, d.AuthMgr, d.AuthBL, d.Matcher, d.SettingsRepo, d.SettingsFactory, d.FeishuAuthHandler, d.FeishuAppConfigHandler, d.LoginLimiter, d.Queue)
}
