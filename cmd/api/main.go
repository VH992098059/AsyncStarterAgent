package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asyncstarter/agent/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps, err := Build(ctx, cfg)
	if err != nil {
		log.Fatalf("wire: %v", err)
	}
	defer deps.Queue.Close()

	// 优雅退出：SIGINT / SIGTERM → srv.Shutdown（停 accept + 等 in-flight）+ cancel（停 DDL scheduler）
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: deps.Server(),
	}

	serveErr := make(chan error, 1)
	go func() {
		fmt.Printf("AsyncStarterAgent API starting on %s (env=%s)\n", srv.Addr, cfg.Env)
		serveErr <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("shutting down on signal %s...", sig)
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
			os.Exit(1)
		}
		return // http.ErrServerClosed：正常路径（无需退出码）
	}

	// shutdown 顺序：先停 HTTP（cancel() 后 DDL scheduler 也会退出，但顺序无关）
	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	if err := srv.Shutdown(shCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
	cancel() // 触发 RunDDLScheduler 退出（goroutine 不泄漏）
}
