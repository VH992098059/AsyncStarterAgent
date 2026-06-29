// Package auth provides JWT 颁发/校验、内存黑名单、HTTP 中间件和注册/登录服务。
// 范围：决策 #2 扩 MVP 的 auth 子模块（用户已显式同意）。
// 多用户协作/角色/组织仍属 MVP OUT，不在本包引入。
package auth

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)// Claims 是 JWT 自定义 claim：user_id + username + jwt.RegisteredClaims。
type Claims struct {
	UserID   string `json:"sub_id"`
	Username string `json:"usr"`
	jwt.RegisteredClaims
}

// ErrTokenExpired 标记 token 已过期，区别于其他解析错误。
var ErrTokenExpired = errors.New("auth: token expired")

// Manager 负责 JWT 签发与解析。线程安全。
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager 构造 Manager。secret 为空字符串时返回 nil（避免弱密钥）。
func NewManager(secret string, ttl time.Duration) *Manager {
	if secret == "" {
		return nil
	}
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Sign 用 HS256 签发 token。userID 非空即可（handler 在 register 时已生成 UUID，
// 此处不重复校验以避免对将来支持非 UUID 标识符造成阻碍）。
func (m *Manager) Sign(userID, username string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user_id is required")
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			Issuer:    "asyncstarter-agent",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

// Parse 校验签名 + 有效期，返回 claims。过期返回 ErrTokenExpired。
func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// Blacklist 是 JWT 黑名单（按 token jti 撤销）。
// 线程安全；过期条目惰性清理（IsRevoked 时扫描过期）。
// MVP 单实例内存足够；跨实例部署后续替换为 Redis。
type Blacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time
}

// NewBlacklist 构造空黑名单。
func NewBlacklist() *Blacklist {
	return &Blacklist{entries: make(map[string]time.Time)}
}

// Revoke 撤销一个 token。expiresAt 之前 IsRevoked 返回 true。
func (b *Blacklist) Revoke(tokenID string, ttl time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[tokenID] = time.Now().Add(ttl)
}

// IsRevoked 检查 tokenID 是否在黑名单中；过期条目惰性删除。
func (b *Blacklist) IsRevoked(tokenID string) bool {
	if tokenID == "" {
		return false
	}
	b.mu.RLock()
	exp, ok := b.entries[tokenID]
	b.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		b.mu.Lock()
		// 二次确认（其他 goroutine 可能已清理）
		if cur, ok := b.entries[tokenID]; ok && time.Now().After(cur) {
			delete(b.entries, tokenID)
		}
		b.mu.Unlock()
		return false
	}
	return true
}
