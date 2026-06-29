package auth

import (
	"strings"
)

// extractTokenID 提取 token 的 jti 或使用签名后段作为标识。
// 标准 JWT 自带 jti；如果签发时未设 jti，middleware 层用 username 区分。
// 此处简化：取 token 末段（签名 hash 段），足够用于黑名单幂等判断。
func extractTokenID(tok string) string {
	parts := strings.Split(tok, ".")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}
