//go:build integration

package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/queue"
)

// TestEnqueueAndHandle 端到端集成测试：Producer Enqueue → Worker Server 拉取 → Handler 触发。
// 需要本地 Redis 在 redis://localhost:6379/0 运行。
// 用法：
//   go test -tags=integration ./internal/queue/...   // 真跑
//   go test ./internal/queue/...                     // 跳过（CI 默认）
func TestEnqueueAndHandle(t *testing.T) {
	redisURL := "redis://localhost:6379/0"

	c, err := queue.NewClient(redisURL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer c.Close()

	handled := make(chan string, 1)
	mux := queue.NewMux()
	mux.HandleFunc("ping", func(ctx context.Context, payload string) error {
		handled <- payload
		return nil
	})

	srv, err := queue.NewServer(redisURL, mux, 1)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	go func() { _ = srv.Start() }()
	defer srv.Stop()

	if err := c.Enqueue(context.Background(), "ping", "hello", 1*time.Minute); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case got := <-handled:
		if got != "hello" {
			t.Fatalf("expected hello, got %s", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler not invoked within 3s")
	}
}
