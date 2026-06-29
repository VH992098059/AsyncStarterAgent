package settings

import (
	"time"

	"github.com/google/uuid"
)

type Settings struct {
	UserID            uuid.UUID `json:"-"`
	LLMAPIKey         string    `json:"llm_api_key"`
	LLMBaseURL        string    `json:"llm_base_url"`
	LLMModel          string    `json:"llm_model"`
	LLMTemperature    float32   `json:"llm_temperature"`
	LLMMaxTokens      int       `json:"llm_max_tokens"`
	EmbedAPIKey       string    `json:"embed_api_key"`
	EmbedBaseURL      string    `json:"embed_base_url"`
	EmbedModel        string    `json:"embed_model"`
	NotionAPIKey      string    `json:"notion_api_key"`
	NotionParentPage  string    `json:"notion_parent_page"`
	ObsidianVaultPath string    `json:"obsidian_vault_path"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func DefaultSettings() *Settings {
	return &Settings{
		LLMTemperature: 0.3,
	}
}

func (s *Settings) MergeWithDefaults(defaultKey, defaultBaseURL, defaultLLMModel,
	defaultEmbedKey, defaultEmbedBaseURL, defaultEmbedModel,
	defaultNotionKey, defaultNotionPage, defaultObsidianPath string) {
	if s.LLMAPIKey == "" {
		s.LLMAPIKey = defaultKey
	}
	if s.LLMBaseURL == "" {
		s.LLMBaseURL = defaultBaseURL
	}
	if s.LLMModel == "" {
		s.LLMModel = defaultLLMModel
	}
	if s.EmbedAPIKey == "" {
		s.EmbedAPIKey = s.LLMAPIKey
		if defaultEmbedKey != "" {
			s.EmbedAPIKey = defaultEmbedKey
		}
	}
	if s.EmbedBaseURL == "" {
		s.EmbedBaseURL = s.LLMBaseURL
		if defaultEmbedBaseURL != "" {
			s.EmbedBaseURL = defaultEmbedBaseURL
		}
	}
	if s.EmbedModel == "" {
		s.EmbedModel = defaultEmbedModel
	}
	if s.NotionAPIKey == "" {
		s.NotionAPIKey = defaultNotionKey
	}
	if s.NotionParentPage == "" {
		s.NotionParentPage = defaultNotionPage
	}
	if s.ObsidianVaultPath == "" {
		s.ObsidianVaultPath = defaultObsidianPath
	}
}

type MaskedSettings struct {
	LLMAPIKey         string  `json:"llm_api_key"`
	LLMBaseURL        string  `json:"llm_base_url"`
	LLMModel          string  `json:"llm_model"`
	LLMTemperature    float32 `json:"llm_temperature"`
	LLMMaxTokens      int     `json:"llm_max_tokens"`
	EmbedAPIKey       string  `json:"embed_api_key"`
	EmbedBaseURL      string  `json:"embed_base_url"`
	EmbedModel        string  `json:"embed_model"`
	NotionAPIKey      string  `json:"notion_api_key"`
	NotionParentPage  string  `json:"notion_parent_page"`
	ObsidianVaultPath string  `json:"obsidian_vault_path"`
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return ""
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func (s *Settings) Masked() *MaskedSettings {
	return &MaskedSettings{
		LLMAPIKey:         maskKey(s.LLMAPIKey),
		LLMBaseURL:        s.LLMBaseURL,
		LLMModel:          s.LLMModel,
		LLMTemperature:    s.LLMTemperature,
		LLMMaxTokens:      s.LLMMaxTokens,
		EmbedAPIKey:       maskKey(s.EmbedAPIKey),
		EmbedBaseURL:      s.EmbedBaseURL,
		EmbedModel:        s.EmbedModel,
		NotionAPIKey:      maskKey(s.NotionAPIKey),
		NotionParentPage:  s.NotionParentPage,
		ObsidianVaultPath: s.ObsidianVaultPath,
	}
}

type UpdateRequest struct {
	LLMAPIKey         *string  `json:"llm_api_key"`
	LLMBaseURL        *string  `json:"llm_base_url"`
	LLMModel          *string  `json:"llm_model"`
	LLMTemperature    *float32 `json:"llm_temperature"`
	LLMMaxTokens      *int     `json:"llm_max_tokens"`
	EmbedAPIKey       *string  `json:"embed_api_key"`
	EmbedBaseURL      *string  `json:"embed_base_url"`
	EmbedModel        *string  `json:"embed_model"`
	NotionAPIKey      *string  `json:"notion_api_key"`
	NotionParentPage  *string  `json:"notion_parent_page"`
	ObsidianVaultPath *string  `json:"obsidian_vault_path"`
}

func (u *UpdateRequest) Apply(s *Settings) {
	if u.LLMAPIKey != nil {
		s.LLMAPIKey = *u.LLMAPIKey
	}
	if u.LLMBaseURL != nil {
		s.LLMBaseURL = *u.LLMBaseURL
	}
	if u.LLMModel != nil {
		s.LLMModel = *u.LLMModel
	}
	if u.LLMTemperature != nil {
		s.LLMTemperature = *u.LLMTemperature
	}
	if u.LLMMaxTokens != nil {
		s.LLMMaxTokens = *u.LLMMaxTokens
	}
	if u.EmbedAPIKey != nil {
		s.EmbedAPIKey = *u.EmbedAPIKey
	}
	if u.EmbedBaseURL != nil {
		s.EmbedBaseURL = *u.EmbedBaseURL
	}
	if u.EmbedModel != nil {
		s.EmbedModel = *u.EmbedModel
	}
	if u.NotionAPIKey != nil {
		s.NotionAPIKey = *u.NotionAPIKey
	}
	if u.NotionParentPage != nil {
		s.NotionParentPage = *u.NotionParentPage
	}
	if u.ObsidianVaultPath != nil {
		s.ObsidianVaultPath = *u.ObsidianVaultPath
	}
}
