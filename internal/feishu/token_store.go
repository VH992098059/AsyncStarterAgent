package feishu

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TokenRecord 存储在 feishu_tokens 表中的 token 记录（解密后）
type TokenRecord struct {
	UserID       uuid.UUID
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	OpenID       string
	Name         string
}

// TokenStore 抽象 token 存储层，便于测试 mock
type TokenStore interface {
	Save(ctx context.Context, rec TokenRecord) error
	Get(ctx context.Context, userID uuid.UUID) (*TokenRecord, error)
	Delete(ctx context.Context, userID uuid.UUID) error
}

// pgTokenStore 基于 PostgreSQL + pgcrypto 的实现
type pgTokenStore struct {
	pool *pgxpool.Pool
	// encKey 是 pgcrypto pgp_sym_encrypt 的对称密钥（来自 DB_ENCRYPTION_KEY 环境变量）
	encKey string
}

// NewTokenStore 创建 token 存储实例
// encKey 不能为空，否则 Save/Get 会返回错误
func NewTokenStore(pool *pgxpool.Pool, encKey string) TokenStore {
	return &pgTokenStore{pool: pool, encKey: encKey}
}

// Save 加密并存储 token 记录（UPSERT）
func (s *pgTokenStore) Save(ctx context.Context, rec TokenRecord) error {
	if s.encKey == "" {
		return fmt.Errorf("feishu token store: DB_ENCRYPTION_KEY is empty")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO feishu_tokens (user_id, access_token, refresh_token, expires_at, open_id, name, updated_at)
		VALUES ($1, pgp_sym_encrypt($2, $3), pgp_sym_encrypt($4, $3), $5, $6, $7, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			expires_at = EXCLUDED.expires_at,
			open_id = EXCLUDED.open_id,
			name = EXCLUDED.name,
			updated_at = NOW()
	`, rec.UserID, rec.AccessToken, s.encKey, rec.RefreshToken, rec.ExpiresAt, rec.OpenID, rec.Name)
	if err != nil {
		return fmt.Errorf("feishu token save: %w", err)
	}
	return nil
}

// Get 读取并解密 token 记录
// 列类型为 BYTEA（见 F001 迁移），无需 ::bytea cast
func (s *pgTokenStore) Get(ctx context.Context, userID uuid.UUID) (*TokenRecord, error) {
	if s.encKey == "" {
		return nil, fmt.Errorf("feishu token store: DB_ENCRYPTION_KEY is empty")
	}
	rec := &TokenRecord{UserID: userID}
	err := s.pool.QueryRow(ctx, `
		SELECT
			pgp_sym_decrypt(access_token, $2) AS access_token,
			pgp_sym_decrypt(refresh_token, $2) AS refresh_token,
			expires_at,
			COALESCE(open_id, ''),
			COALESCE(name, '')
		FROM feishu_tokens WHERE user_id = $1
	`, userID, s.encKey).Scan(&rec.AccessToken, &rec.RefreshToken, &rec.ExpiresAt, &rec.OpenID, &rec.Name)
	if err != nil {
		return nil, fmt.Errorf("feishu token get: %w", err)
	}
	return rec, nil
}

// Delete 删除 token 记录（用户撤销授权时调用）
func (s *pgTokenStore) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM feishu_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("feishu token delete: %w", err)
	}
	return nil
}
