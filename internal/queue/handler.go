package queue

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

// HandlerFunc 是用户注册的业务 handler 签名：接收 payload 字符串，返回 error。
type HandlerFunc func(ctx context.Context, payload string) error

// Mux 是 asynq.ServeMux 的薄封装（隐藏 asynq.Task 类型，向上暴露 string payload）。
type Mux struct{ m *asynq.ServeMux }

// NewMux 构造一个新的 Mux。
func NewMux() *Mux { return &Mux{m: asynq.NewServeMux()} }

// HandleFunc 把一个 taskType 注册到指定的 HandlerFunc。
//
// asynq v0.26.0 起 ServeMux.HandleFunc 签名是 `func(ctx, *Task) error`；
// asynq 提供的 ctx 带 deadline / done channel，可直接转给 HandlerFunc 的 ctx 参数。
func (m *Mux) HandleFunc(taskType string, h HandlerFunc) {
	m.m.HandleFunc(taskType, func(ctx context.Context, c *asynq.Task) error {
		return h(ctx, string(c.Payload()))
	})
}

// Server 是 asynq Worker 端。
type Server struct {
	srv *asynq.Server
	mux *asynq.ServeMux
}

// NewServer 构造一个 Server。redisURL 为空时返回 error。mux 不能为 nil。
// queues 当前硬编码 default=5——Phase 1+ 接业务任务时再扩展。
func NewServer(redisURL string, mux *Mux, concurrency int) (*Server, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	srv := asynq.NewServer(opt, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{"default": 5},
	})
	return &Server{srv: srv, mux: mux.m}, nil
}

// Start 阻塞运行 Worker（直到 srv.Stop / srv.Shutdown 被调用或 redis 不可用）。
func (s *Server) Start() error { return s.srv.Start(s.mux) }

// Stop 立刻停止 Worker（不等待 in-flight 任务完成）。
func (s *Server) Stop() { s.srv.Stop() }

// Shutdown 优雅停止 Worker（等 in-flight 任务完成或超时）。
func (s *Server) Shutdown() { s.srv.Shutdown() }
