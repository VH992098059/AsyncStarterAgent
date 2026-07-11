package auth

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBlacklist 是基于 Redis 的 JWT 黑名单实现（问题 #9：替代内存黑名单，
// 解决服务重启丢失/多实例不共享的问题）。
//
// IsRevoked fail-closed：Redis 不可达时视为已撤销（拒绝请求）。这与登录限流
// （ratelimit.Limiter，fail-open）策略相反——撤销机制关系到"已登出/已知泄露的
// 凭证不能继续使用"这一安全属性，一旦 Redis 故障就放行等同于让撤销形同虚设；
// 相比之下限流只是防刷，故障时放行不影响核心安全性。
type RedisBlacklist struct {
	rdb *redis.Client
}

// NewRedisBlacklist 构造一个 RedisBlacklist。redisURL 为空或解析失败返回 error。
func NewRedisBlacklist(redisURL string) (*RedisBlacklist, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &RedisBlacklist{rdb: redis.NewClient(opt)}, nil
}

func blacklistKey(tokenID string) string { return "jwt_blacklist:" + tokenID }

// Revoke 撤销一个 token，ttl 后自动从 Redis 过期删除（无需额外清理任务）。
func (b *RedisBlacklist) Revoke(tokenID string, ttl time.Duration) {
	if tokenID == "" {
		return
	}
	if err := b.rdb.Set(context.Background(), blacklistKey(tokenID), "1", ttl).Err(); err != nil {
		log.Printf("[auth] redis blacklist revoke failed for token=%s: %v", tokenID, err)
	}
}

// IsRevoked 检查 tokenID 是否在黑名单中。Redis 不可达时 fail-closed（视为已撤销，拒绝请求）。
func (b *RedisBlacklist) IsRevoked(tokenID string) bool {
	if tokenID == "" {
		return false
	}
	n, err := b.rdb.Exists(context.Background(), blacklistKey(tokenID)).Result()
	if err != nil {
		log.Printf("[auth] redis blacklist check failed for token=%s, fail-closed (deny): %v", tokenID, err)
		return true
	}
	return n > 0
}

// Close 关闭底层 redis 连接。
func (b *RedisBlacklist) Close() error { return b.rdb.Close() }
