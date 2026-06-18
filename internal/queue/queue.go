// Package queue 提供基于 asynq 的 Redis 任务队列基础设施（Producer / Worker / Mux）。
//
// 入口：
//   - NewClient(redisURL)  构造 Producer
//   - Enqueue(ctx, type, payload, ttl) 入队
//   - NewMux()  构造任务路由表
//   - mux.HandleFunc(type, handler) 注册 handler
//   - NewServer(redisURL, mux, concurrency) 构造 Worker
//   - srv.Start() / srv.Stop() / srv.Shutdown() 控制生命周期
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Client 是 asynq 生产者端的薄封装。
type Client struct {
	cli *asynq.Client
}

// NewClient 构造一个 Client。redisURL 为空时返回 error（避免后面再 nil-deref）。
//
// asynq v0.26.0 起 RedisClientOpt 不再含 URL 字段；改用 asynq.ParseRedisURI
// 把 `redis://host:port/db` 字符串解析成 RedisConnOpt（支持 redis / rediss /
// redis-socket / redis-sentinel 4 种 scheme）。
func NewClient(redisURL string) (*Client, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	cli := asynq.NewClient(opt)
	return &Client{cli: cli}, nil
}

// Close 关闭底层 asynq.Client（释放 redis 连接池）。忽略关闭错误。
func (c *Client) Close() { _ = c.cli.Close() }

// Enqueue 入队一个任务。taskType 是任务类型，payload 是字符串载荷，ttl 是单次执行超时。
// 重试策略硬编码为 MaxRetry(3)——后续 Phase 如需更复杂策略再扩展。
func (c *Client) Enqueue(ctx context.Context, taskType string, payload string, ttl time.Duration) error {
	t := asynq.NewTask(taskType, []byte(payload), asynq.MaxRetry(3), asynq.Timeout(ttl))
	_, err := c.cli.EnqueueContext(ctx, t)
	return err
}
