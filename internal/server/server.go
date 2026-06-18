package server

import (
	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/handler"
	"github.com/asyncstarter/agent/internal/middleware"
	"github.com/gin-gonic/gin"
)

func New(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	r.Use(func(c *gin.Context) {
		c.Set("env", cfg.Env)
		c.Next()
	})

	r.GET("/health", handler.Health)

	wh := &handler.WebhookHandler{Secret: cfg.TodoistWebhookSecret}
	r.POST("/api/v1/webhook/todoist", wh.Todoist)

	return r
}
