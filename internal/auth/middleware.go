package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// context 字段常量（导出供 handler/service 读取）。
const (
	ContextUserIDKey   = "auth_user_id"
	ContextUsernameKey = "auth_username"
)

// Middleware 校验 Authorization: Bearer <token>，把 user_id 注入 gin.Context。
// 也接受 ?token= 形式（仅用于 SSE，EventSource 不支持自定义 Header）。
// 401 场景：缺失/格式错/签名错/过期/在黑名单。
func Middleware(mgr *Manager, bl *Blacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := extractToken(c)
		if tok == "" {
			abort401(c, "missing authorization")
			return
		}
		claims, err := mgr.Parse(tok)
		if err != nil {
			if errors.Is(err, ErrTokenExpired) {
				abort401(c, "token expired")
			} else {
				abort401(c, "invalid token")
			}
			return
		}
		// 黑名单检查：用 token signature 段作 key（无需 jti）
		if bl != nil {
			parts := strings.Split(tok, ".")
			if len(parts) == 3 && bl.IsRevoked(parts[2]) {
				abort401(c, "token revoked")
				return
			}
		}
		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUsernameKey, claims.Username)
		c.Next()
	}
}

// extractToken 优先从 Authorization 头读，否则从 query string 读（SSE 用）。
func extractToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if q := c.Query("token"); q != "" {
		return q
	}
	return ""
}

func abort401(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    401,
		"message": msg,
	})
}

// MustUserID 从 gin.Context 取出 user_id（middleware 已注入时调用）。
func MustUserID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ContextUserIDKey)
	if !ok {
		return uuid.Nil, false
	}
	s, _ := v.(string)
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// ExtractTokenID 从 Bearer 字符串提取签名段（黑名单 key）。仅 middleware 内部使用。
func ExtractTokenID(tok string) string {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return ""
	}
	return parts[2]
}

// 编译期检查 jwt.ErrTokenExpired 可被本包引用（避免空 import）
var _ = jwt.ErrTokenExpired
