package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[panic] %v %s %s", err, c.Request.Method, c.Request.URL.Path)
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}
