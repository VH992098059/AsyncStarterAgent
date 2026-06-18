package handler

import (
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

type HealthResponse struct {
	Status string `json:"status"`
	Env    string `json:"env"`
}

func Health(c *gin.Context) {
	httpx.OK(c, HealthResponse{Status: "ok", Env: c.GetString("env")})
}
