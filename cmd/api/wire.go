package main

import (
	"context"
	"log"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/queue"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/server"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Deps 集中持有 main 装配好的运行期依赖。
// main 用完通过 defer Close 释放资源（目前只有 Queue 需要显式 Close）。
type Deps struct {
	Cfg     *config.Config
	Trigger *trigger.Service
	Queue   *queue.Client
}

// Build 构造所有依赖。
//
// 启动顺序：
//  1. Postgres pool（repository.Open）— 失败立即返回
//  2. Redis queue client（queue.NewClient）— 失败立即返回（同时关闭已开的 pool，避免连接泄漏）
//  3. Matcher + Trigger Service（T007 已实现）
//  4. DDL Scheduler 后台 goroutine（T008 已实现）— 启动时立即跑一次 + 15 分钟轮询
//
// 优雅退出：ctx cancel → RunDDLScheduler 返回，goroutine 不泄漏。
func Build(ctx context.Context, cfg *config.Config) (*Deps, error) {
	pool, err := repository.Open(ctx, cfg.DSN)
	if err != nil {
		return nil, err
	}

	q, err := queue.NewClient(cfg.RedisURL)
	if err != nil {
		// 关闭已开的 pool 避免连接泄漏。
		pool.Close()
		return nil, err
	}

	matcher := trigger.NewMatcher(trigger.DefaultMatcherRules())
	trigSvc := trigger.NewService(pool, matcher)

	// 注册 DDL 调度器回调：DDL 触发 → 关键词匹配 → 创建 AgentRun。
	// 路径：user_tasks.deadline_at 临近 → RunDDLScheduler 拉取 → 本回调 →
	//       trigSvc.ProcessKeyword(uid, title) → repository.CreateAgentRun
	// taskID 在 wire 范围内不传（plan 范围内不传 AgentRun ← user_task 关联）。
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
	log.Println("[ddl] scheduler started")

	return &Deps{Cfg: cfg, Trigger: trigSvc, Queue: q}, nil
}

// Server 由 Deps 装配 trigger svc 注入到 router。
// 返回 *gin.Engine（T002 server.New 已实现），main 调用 .Run(addr) 启动。
func (d *Deps) Server() *gin.Engine {
	return server.New(d.Cfg, d.Trigger)
}
