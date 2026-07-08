package main

import (
	"context"
	"log"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/delivery"
	"github.com/asyncstarter/agent/internal/feishu"
	"github.com/asyncstarter/agent/internal/queue"
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
	Cfg             *config.Config
	Pool            *pgxpool.Pool
	Trigger         *trigger.Service
	Queue           *queue.Client
	Syn             *synthesis.Service
	Deliv           *delivery.Service
	Auth            *auth.Service
	AuthMgr         *auth.Manager
	AuthBL          *auth.Blacklist
	Matcher         *trigger.Matcher
	SettingsRepo    *settings.Repo
	SettingsFactory *settings.Factory
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
	authBL := auth.NewBlacklist()
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

	// 决策 #7: 飞书 OAuth + token 工厂
	// 注意：必须声明为接口类型 settings.FeishuClientGetter（而非 *feishu.ClientFactory），
	// 这样当飞书未启用时 feishuGetter 保持真正的 nil interface，
	// 否则会触发 Go nil-interface gotcha（nil typed pointer 包在 interface 里 != nil），
	// 导致 settings.Factory.GetFeishuClient 中的 f.feishuCli == nil 判断失效而 panic。
	var feishuGetter settings.FeishuClientGetter
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		tokenStore := feishu.NewTokenStore(pool, cfg.DBEncryptionKey)
		authClient := feishu.NewAuthClient(feishu.OAuthConfig{
			AppID:       cfg.FeishuAppID,
			AppSecret:   cfg.FeishuAppSecret,
			RedirectURL: cfg.FeishuRedirectURL,
		})
		feishuGetter = feishu.NewClientFactory(cfg.FeishuAppID, cfg.FeishuAppSecret, tokenStore, authClient)
		log.Println("[feishu] client factory initialized")
	} else {
		log.Println("[feishu] disabled (FEISHU_APP_ID not set)")
	}

	settingsFactory := settings.NewFactory(pool, settingsRepo, cfg, feishuGetter)

	synSvc := synthesis.NewService(pool, settingsFactory, "")
	log.Println("[synthesis] service initialized with per-user config factory")

	delivSvc := delivery.NewService(pool, settingsFactory, nil)
	log.Println("[delivery] service initialized with per-user config factory")

	log.Println("[ddl] scheduler started")

	return &Deps{
		Cfg:             cfg,
		Pool:            pool,
		Trigger:         trigSvc,
		Queue:           q,
		Syn:             synSvc,
		Deliv:           delivSvc,
		Auth:            authSvc,
		AuthMgr:         jwtMgr,
		AuthBL:          authBL,
		Matcher:         matcher,
		SettingsRepo:    settingsRepo,
		SettingsFactory: settingsFactory,
	}, nil
}

func (d *Deps) Server() *gin.Engine {
	return server.New(d.Cfg, d.Pool, d.Trigger, d.Syn, d.Deliv, d.Auth, d.AuthMgr, d.AuthBL, d.Matcher, d.SettingsRepo, d.SettingsFactory)
}
