package settings

import (
	"context"
	"fmt"
	"sync"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/delivery"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type cachedLLM struct {
	client synthesis.LLMClient
	key    string
	model  string
	url    string
	temp   float32
}

type cachedEmbedder struct {
	provider *synthesis.EmbeddingProvider
	key      string
	model    string
	url      string
}

type cachedNotion struct {
	adapter *delivery.NotionAdapter
	key     string
	page    string
}

type cachedObsidian struct {
	adapter *delivery.ObsidianAdapter
	path    string
}

type Factory struct {
	pool     *pgxpool.Pool
	repo     *Repo
	defaults *config.Config

	mu        sync.RWMutex
	llmCache  map[uuid.UUID]cachedLLM
	embCache  map[uuid.UUID]cachedEmbedder
	notionMap map[uuid.UUID]cachedNotion
	obsMap    map[uuid.UUID]cachedObsidian
	feishuCli FeishuClientGetter // 决策 #7: 飞书 per-user client 工厂（接口，避免循环依赖）
}

// FeishuClientGetter 由 feishu 包实现，settings 包通过接口依赖，避免循环依赖
type FeishuClientGetter interface {
	GetClient(ctx context.Context, userID uuid.UUID) (*lark.Client, string, error)
	IsAuthorized(ctx context.Context, userID uuid.UUID) bool
}

func NewFactory(pool *pgxpool.Pool, repo *Repo, defaults *config.Config, feishuCli FeishuClientGetter) *Factory {
	return &Factory{
		pool:      pool,
		repo:      repo,
		defaults:  defaults,
		llmCache:  make(map[uuid.UUID]cachedLLM),
		embCache:  make(map[uuid.UUID]cachedEmbedder),
		notionMap: make(map[uuid.UUID]cachedNotion),
		obsMap:    make(map[uuid.UUID]cachedObsidian),
		feishuCli: feishuCli,
	}
}

func (f *Factory) Invalidate(userID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.llmCache, userID)
	delete(f.embCache, userID)
	delete(f.notionMap, userID)
	delete(f.obsMap, userID)
}

func (f *Factory) loadSettings(ctx context.Context, userID uuid.UUID) (*Settings, error) {
	s, err := f.repo.Get(ctx, userID)
	if err != nil {
		s = DefaultSettings()
		s.UserID = userID
	}
	s.MergeWithDefaults(
		f.defaults.OpenAIKey, f.defaults.OpenAIBaseURL, f.defaults.OpenAIModel,
		f.defaults.EmbeddingAPIKey, f.defaults.EmbeddingBaseURL, f.defaults.EmbeddingModel,
		f.defaults.NotionAPIKey, f.defaults.NotionParentPageID, f.defaults.ObsidianVaultPath,
	)
	return s, nil
}

func (f *Factory) GetLLM(ctx context.Context, userID uuid.UUID) (synthesis.LLMClient, error) {
	s, err := f.loadSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.LLMAPIKey == "" {
		return nil, fmt.Errorf("LLM API Key 未配置，请在设置中填写")
	}
	if s.LLMModel == "" {
		s.LLMModel = "gpt-4o-mini"
	}

	f.mu.RLock()
	if c, ok := f.llmCache[userID]; ok && c.key == s.LLMAPIKey && c.model == s.LLMModel && c.url == s.LLMBaseURL && c.temp == s.LLMTemperature {
		f.mu.RUnlock()
		return c.client, nil
	}
	f.mu.RUnlock()

	chatCfg := &openai.ChatModelConfig{
		APIKey: s.LLMAPIKey,
		Model:  s.LLMModel,
	}
	if s.LLMBaseURL != "" {
		chatCfg.BaseURL = s.LLMBaseURL
	}
	chatModel, err := openai.NewChatModel(ctx, chatCfg)
	if err != nil {
		return nil, fmt.Errorf("init LLM: %w", err)
	}
	llm := synthesis.NewEinoLLM(chatModel)

	f.mu.Lock()
	f.llmCache[userID] = cachedLLM{
		client: llm, key: s.LLMAPIKey, model: s.LLMModel, url: s.LLMBaseURL, temp: s.LLMTemperature,
	}
	f.mu.Unlock()

	return llm, nil
}

func (f *Factory) GetEmbedder(ctx context.Context, userID uuid.UUID) (*synthesis.EmbeddingProvider, error) {
	s, err := f.loadSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	embKey := s.EmbedAPIKey
	if embKey == "" {
		embKey = s.LLMAPIKey
	}
	embURL := s.EmbedBaseURL
	if embURL == "" {
		embURL = s.LLMBaseURL
	}
	embModel := s.EmbedModel
	if embModel == "" {
		embModel = "text-embedding-3-small"
	}
	if embKey == "" {
		return nil, fmt.Errorf("Embedding API Key 未配置，请在设置中填写")
	}

	f.mu.RLock()
	if c, ok := f.embCache[userID]; ok && c.key == embKey && c.model == embModel && c.url == embURL {
		f.mu.RUnlock()
		return c.provider, nil
	}
	f.mu.RUnlock()

	embCfg := synthesis.EinoEmbedderConfig{
		APIKey:  embKey,
		Model:   embModel,
		BaseURL: embURL,
		Dim:     1536,
	}
	provider, err := synthesis.NewEinoEmbedder(ctx, embCfg)
	if err != nil {
		return nil, fmt.Errorf("init embedder: %w", err)
	}

	f.mu.Lock()
	f.embCache[userID] = cachedEmbedder{provider: provider, key: embKey, model: embModel, url: embURL}
	f.mu.Unlock()

	return provider, nil
}

func (f *Factory) GetLLMTemperature(ctx context.Context, userID uuid.UUID) float32 {
	s, err := f.loadSettings(ctx, userID)
	if err != nil || s.LLMTemperature <= 0 {
		return 0.3
	}
	return s.LLMTemperature
}

func (f *Factory) GetLLMMaxTokens(ctx context.Context, userID uuid.UUID) int {
	s, err := f.loadSettings(ctx, userID)
	if err != nil {
		return 0
	}
	return s.LLMMaxTokens
}

func (f *Factory) GetNotionAdapter(ctx context.Context, userID uuid.UUID) (*delivery.NotionAdapter, error) {
	s, err := f.loadSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.NotionAPIKey == "" {
		return nil, fmt.Errorf("Notion API Key 未配置，请在设置中填写")
	}

	f.mu.RLock()
	if c, ok := f.notionMap[userID]; ok && c.key == s.NotionAPIKey && c.page == s.NotionParentPage {
		f.mu.RUnlock()
		return c.adapter, nil
	}
	f.mu.RUnlock()

	adapter := delivery.NewNotionAdapter(delivery.NotionConfig{
		APIKey:       s.NotionAPIKey,
		ParentPageID: s.NotionParentPage,
	})

	f.mu.Lock()
	f.notionMap[userID] = cachedNotion{adapter: adapter, key: s.NotionAPIKey, page: s.NotionParentPage}
	f.mu.Unlock()

	return adapter, nil
}

func (f *Factory) GetObsidianAdapter(_ context.Context, userID uuid.UUID) (*delivery.ObsidianAdapter, error) {
	s, err := f.loadSettings(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	if s.ObsidianVaultPath == "" {
		return nil, fmt.Errorf("Obsidian Vault 路径未配置，请在设置中填写")
	}

	f.mu.RLock()
	if c, ok := f.obsMap[userID]; ok && c.path == s.ObsidianVaultPath {
		f.mu.RUnlock()
		return c.adapter, nil
	}
	f.mu.RUnlock()

	adapter := delivery.NewObsidianAdapter(delivery.ObsidianConfig{
		VaultPath: s.ObsidianVaultPath,
	})

	f.mu.Lock()
	f.obsMap[userID] = cachedObsidian{adapter: adapter, path: s.ObsidianVaultPath}
	f.mu.Unlock()

	return adapter, nil
}

// GetFeishuClient 返回 per-user 的飞书 lark.Client + user_access_token
// 决策 #7: 所有飞书 API 调用必须走此方法，确保以用户身份调用
func (f *Factory) GetFeishuClient(ctx context.Context, userID uuid.UUID) (*lark.Client, string, error) {
	if f.feishuCli == nil {
		return nil, "", fmt.Errorf("飞书集成未启用（未配置 FEISHU_APP_ID）")
	}
	return f.feishuCli.GetClient(ctx, userID)
}

// IsFeishuAuthorized 检查用户是否已授权飞书
func (f *Factory) IsFeishuAuthorized(ctx context.Context, userID uuid.UUID) bool {
	if f.feishuCli == nil {
		return false
	}
	return f.feishuCli.IsAuthorized(ctx, userID)
}
