package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config 集中管理应用配置（环境变量加载）
type Config struct {
	Env                     string
	Port                    string
	DSN                     string
	RedisURL                string
	JWTSecret               string
	TodoistWebhookSecret    string
	OpenAIKey               string
	OpenAIModel             string
	OpenAIBaseURL           string
	EmbeddingAPIKey         string
	EmbeddingModel          string
	EmbeddingBaseURL        string
	ObsidianVaultPath       string
	NotionAPIKey            string
	NotionParentPageID      string
	FeishuAppID             string // 决策 #7: 飞书应用 AppID
	FeishuAppSecret         string // 决策 #7: 飞书应用 AppSecret
	FeishuRedirectURL       string // 决策 #7: OAuth 回调地址
	FeishuVerificationToken string // 决策 #7: 飞书事件订阅 Verification Token（webhook 校验）
	DBEncryptionKey         string // 决策 #7: pgcrypto 对称加密密钥
}

// Load 从环境变量加载配置；自动加载 .env 文件；生产环境强制要求 JWT_SECRET
func Load() (*Config, error) {
	// 开发环境自动加载 .env（已设置的环境变量不会被覆盖）
	_ = godotenv.Load()

	env := getEnv("APP_ENV", "development")
	port := getEnv("APP_PORT", "8080")
	dsn := getEnv("DATABASE_URL", "")
	redis := getEnv("REDIS_URL", "redis://localhost:6379/0")
	jwt := getEnv("JWT_SECRET", "")
	todoistSecret := getEnv("TODOIST_WEBHOOK_SECRET", "")

	if env == "production" && jwt == "" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}

	feishuAppID := getEnv("FEISHU_APP_ID", "")
	feishuAppSecret := getEnv("FEISHU_APP_SECRET", "")
	dbEncryptionKey := getEnv("DB_ENCRYPTION_KEY", "")
	if feishuAppID != "" && feishuAppSecret != "" && dbEncryptionKey == "" {
		return nil, fmt.Errorf("DB_ENCRYPTION_KEY is required when FEISHU_APP_ID/FEISHU_APP_SECRET are set")
	}

	return &Config{
		Env:                     env,
		Port:                    port,
		DSN:                     dsn,
		RedisURL:                redis,
		JWTSecret:               jwt,
		TodoistWebhookSecret:    todoistSecret,
		OpenAIKey:               getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:             getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIBaseURL:           getEnv("OPENAI_BASE_URL", ""),
		EmbeddingAPIKey:         getEnv("EMBEDDING_API_KEY", ""),
		EmbeddingModel:          getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
		EmbeddingBaseURL:        getEnv("EMBEDDING_BASE_URL", ""),
		ObsidianVaultPath:       getEnv("OBSIDIAN_VAULT_PATH", ""),
		NotionAPIKey:            getEnv("NOTION_API_KEY", ""),
		NotionParentPageID:      getEnv("NOTION_PARENT_PAGE_ID", ""),
		FeishuAppID:             feishuAppID,
		FeishuAppSecret:         feishuAppSecret,
		FeishuRedirectURL:       getEnv("FEISHU_REDIRECT_URL", "http://localhost:8080/api/v1/auth/feishu/callback"),
		FeishuVerificationToken: getEnv("FEISHU_VERIFICATION_TOKEN", ""),
		DBEncryptionKey:         dbEncryptionKey,
	}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
