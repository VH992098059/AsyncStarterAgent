package feishu

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AppConfigRecord 存储在 feishu_app_configs 表中的用户级应用凭证（解密后）
type AppConfigRecord struct {
	UserID    uuid.UUID
	AppID     string
	AppSecret string
}

// AppConfigStore 抽象用户级飞书应用凭证存取，便于测试 mock
type AppConfigStore interface {
	Get(ctx context.Context, userID uuid.UUID) (*AppConfigRecord, error)
	Upsert(ctx context.Context, rec AppConfigRecord) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

type pgAppConfigStore struct {
	pool   *pgxpool.Pool
	encKey string
}

// NewAppConfigStore 创建用户级应用凭证存储实例
// encKey 不能为空，否则 Get/Upsert 会返回错误
func NewAppConfigStore(pool *pgxpool.Pool, encKey string) AppConfigStore {
	return &pgAppConfigStore{pool: pool, encKey: encKey}
}

func (s *pgAppConfigStore) Get(ctx context.Context, userID uuid.UUID) (*AppConfigRecord, error) {
	if s.encKey == "" {
		return nil, fmt.Errorf("feishu app config store: DB_ENCRYPTION_KEY is empty")
	}
	rec := &AppConfigRecord{UserID: userID}
	err := s.pool.QueryRow(ctx, `
		SELECT app_id, pgp_sym_decrypt(app_secret, $2)
		FROM feishu_app_configs WHERE user_id = $1
	`, userID, s.encKey).Scan(&rec.AppID, &rec.AppSecret)
	if err != nil {
		return nil, fmt.Errorf("feishu app config get: %w", err)
	}
	return rec, nil
}

// Upsert 加密并存储凭证（UPSERT）
func (s *pgAppConfigStore) Upsert(ctx context.Context, rec AppConfigRecord) error {
	if s.encKey == "" {
		return fmt.Errorf("feishu app config store: DB_ENCRYPTION_KEY is empty")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO feishu_app_configs (user_id, app_id, app_secret, updated_at)
		VALUES ($1, $2, pgp_sym_encrypt($3, $4), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			app_id = EXCLUDED.app_id,
			app_secret = EXCLUDED.app_secret,
			updated_at = NOW()
	`, rec.UserID, rec.AppID, rec.AppSecret, s.encKey)
	if err != nil {
		return fmt.Errorf("feishu app config upsert: %w", err)
	}
	return nil
}

func (s *pgAppConfigStore) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM feishu_app_configs WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("feishu app config delete: %w", err)
	}
	return nil
}
