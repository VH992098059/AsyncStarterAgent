package settings

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Get(ctx context.Context, userID uuid.UUID) (*Settings, error) {
	s := &Settings{UserID: userID}
	err := r.pool.QueryRow(ctx,
		`SELECT llm_api_key, llm_base_url, llm_model, llm_temperature, llm_max_tokens,
		        embed_api_key, embed_base_url, embed_model,
		        notion_api_key, notion_parent_page, obsidian_vault_path,
		        created_at, updated_at
		 FROM user_settings WHERE user_id = $1`,
		userID,
	).Scan(
		&s.LLMAPIKey, &s.LLMBaseURL, &s.LLMModel, &s.LLMTemperature, &s.LLMMaxTokens,
		&s.EmbedAPIKey, &s.EmbedBaseURL, &s.EmbedModel,
		&s.NotionAPIKey, &s.NotionParentPage, &s.ObsidianVaultPath,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	return s, nil
}

func (r *Repo) Upsert(ctx context.Context, userID uuid.UUID, req *UpdateRequest) (*Settings, error) {
	existing, err := r.Get(ctx, userID)
	if err != nil {
		existing = DefaultSettings()
		existing.UserID = userID
	}
	req.Apply(existing)

	if existing.LLMTemperature <= 0 {
		existing.LLMTemperature = 0.3
	}
	if existing.LLMTemperature > 2.0 {
		existing.LLMTemperature = 2.0
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO user_settings
		    (user_id, llm_api_key, llm_base_url, llm_model, llm_temperature, llm_max_tokens,
		     embed_api_key, embed_base_url, embed_model,
		     notion_api_key, notion_parent_page, obsidian_vault_path)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 ON CONFLICT (user_id) DO UPDATE SET
		    llm_api_key = EXCLUDED.llm_api_key,
		    llm_base_url = EXCLUDED.llm_base_url,
		    llm_model = EXCLUDED.llm_model,
		    llm_temperature = EXCLUDED.llm_temperature,
		    llm_max_tokens = EXCLUDED.llm_max_tokens,
		    embed_api_key = EXCLUDED.embed_api_key,
		    embed_base_url = EXCLUDED.embed_base_url,
		    embed_model = EXCLUDED.embed_model,
		    notion_api_key = EXCLUDED.notion_api_key,
		    notion_parent_page = EXCLUDED.notion_parent_page,
		    obsidian_vault_path = EXCLUDED.obsidian_vault_path,
		    updated_at = NOW()`,
		userID,
		existing.LLMAPIKey, existing.LLMBaseURL, existing.LLMModel, existing.LLMTemperature, existing.LLMMaxTokens,
		existing.EmbedAPIKey, existing.EmbedBaseURL, existing.EmbedModel,
		existing.NotionAPIKey, existing.NotionParentPage, existing.ObsidianVaultPath,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert settings: %w", err)
	}
	return r.Get(ctx, userID)
}
