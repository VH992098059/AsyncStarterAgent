package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodyBytes 限制单个请求体的最大字节数。防止恶意或误操作提交超大 body
// （如 draft markdown 字段）占用大量内存/DB 带宽（问题 #10）。
// 用 http.MaxBytesReader 包装请求 body：超过限制时后续读取（如 ShouldBindJSON）
// 会返回 error，由各 handler 现有的 400 分支处理，无需额外改动 handler 代码。
func MaxBodyBytes(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}
