package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/queue"
	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/settings"
	"github.com/asyncstarter/agent/internal/synthesis"
)

// cmd/worker 是问题 #11 的产物：asynq worker 端入口，消费 queue.TaskDraftGenerate 任务，
// 调用 synthesis.Service.GenerateDraftAsync 在进程外异步生成草稿。
// 依赖构造对齐 cmd/api/wire.go 的 Build（Pool → SettingsRepo/Factory → synthesis.Service），
// 但只构造草稿生成需要的最小依赖集（不含飞书/auth/webhook 等 API 专属依赖）。
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := repository.Open(ctx, cfg.DSN)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer pool.Close()

	settingsRepo := settings.NewRepo(pool)
	// feishuGetter 传 nil：草稿生成（GetLLM/GetEmbedder）不依赖飞书 client。
	settingsFactory := settings.NewFactory(pool, settingsRepo, cfg, nil)
	synSvc := synthesis.NewService(pool, settingsFactory, "")

	mux := queue.NewMux()
	mux.HandleFunc(queue.TaskDraftGenerate, func(ctx context.Context, payload string) error {
		runID := payload
		log.Printf("[worker] draft:generate start run_id=%s", runID)
		if err := synSvc.GenerateDraftAsync(ctx, runID); err != nil {
			log.Printf("[worker] draft:generate failed run_id=%s: %v", runID, err)
			return err
		}
		log.Printf("[worker] draft:generate done run_id=%s", runID)
		return nil
	})

	srv, err := queue.NewServer(cfg.RedisURL, mux, 5)
	if err != nil {
		log.Fatalf("queue server: %v", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Println("AsyncStarterAgent worker starting...")
		serveErr <- srv.Start()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("shutting down on signal %s...", sig)
	case err := <-serveErr:
		if err != nil {
			log.Fatalf("worker failed: %v", err)
		}
		return
	}

	srv.Shutdown()
	cancel()
}
