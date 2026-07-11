// Package ratelimit 提供基于 Redis 的固定窗口计数限流，用于防止登录接口被暴力破解。
package ratelimit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter 限制某个 key 在 window 时间窗口内最多被 Allow 通过 limit 次。
// Redis 不可达或操作失败时 fail-open（放行 + 记日志），避免限流基础设施故障导致
// 所有用户无法登录（可用性优先于限流本身）。
type Limiter struct {
	rdb    *redis.Client
	limit  int64
	window time.Duration
}

// NewLimiter 构造一个 Limiter。redisURL 为空或解析失败返回 error。
func NewLimiter(redisURL string, limit int64, window time.Duration) (*Limiter, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &Limiter{rdb: redis.NewClient(opt), limit: limit, window: window}, nil
}

// Allow 对 key 计数一次访问。返回 true 表示放行，false 表示已超过窗口内的限流阈值。
// Redis 操作失败时 fail-open：记录日志并放行，不因限流基础设施故障阻断正常登录。
func (l *Limiter) Allow(ctx context.Context, key string) bool {
	if l == nil || l.rdb == nil {
		return true
	}
	count, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("[ratelimit] redis incr failed for key=%s, fail-open (allow): %v", key, err)
		return true
	}
	if count == 1 {
		// 首次访问，设置窗口过期时间（若失败则记日志，不影响本次放行判断）
		if err := l.rdb.Expire(ctx, key, l.window).Err(); err != nil {
			log.Printf("[ratelimit] redis expire failed for key=%s: %v", key, err)
		}
	}
	return count <= l.limit
}

// Close 关闭底层 redis 连接。
func (l *Limiter) Close() error {
	if l == nil || l.rdb == nil {
		return nil
	}
	return l.rdb.Close()
}
