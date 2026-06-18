package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[http] %d %s %s %v", c.Writer.Status(), c.Request.Method, c.Request.URL.Path, time.Since(start))
	}
}
