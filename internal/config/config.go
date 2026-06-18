package config

import (
	"fmt"
	"os"
)

// Config 集中管理应用配置（环境变量加载）
type Config struct {
	Env                  string
	Port                 string
	DSN                  string
	RedisURL             string
	JWTSecret            string
	TodoistWebhookSecret string
}

// Load 从环境变量加载配置；生产环境强制要求 JWT_SECRET
func Load() (*Config, error) {
	env := getEnv("APP_ENV", "development")
	port := getEnv("APP_PORT", "8080")
	dsn := getEnv("DATABASE_URL", "")
	redis := getEnv("REDIS_URL", "redis://localhost:6379/0")
	jwt := getEnv("JWT_SECRET", "")
	todoistSecret := getEnv("TODOIST_WEBHOOK_SECRET", "")

	if env == "production" && jwt == "" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}

	return &Config{
		Env:                  env,
		Port:                 port,
		DSN:                  dsn,
		RedisURL:             redis,
		JWTSecret:            jwt,
		TodoistWebhookSecret: todoistSecret,
	}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
