package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// 业务错误，handler 层映射到 400/401/409。
var (
	ErrUsernameTaken     = errors.New("auth: username already taken")
	ErrInvalidCredential = errors.New("auth: invalid username or password")
	ErrUserNotFound      = errors.New("auth: user not found")
	ErrUsernameLength    = errors.New("auth: username length must be 3-64")
	ErrPasswordLength    = errors.New("auth: password length must be >= 6")
)

// Service 负责 register / login / lookup。
// 依赖 pgxpool；不持有 cache（MVP 单实例，每次查询 DB 可接受）。
type Service struct {
	pool *pgxpool.Pool
	cost int
}

// NewService 构造 auth Service。cost 为 bcrypt cost（4-31），建议 10。
func NewService(pool *pgxpool.Pool, cost int) *Service {
	if cost < bcrypt.MinCost {
		cost = bcrypt.DefaultCost
	}
	return &Service{pool: pool, cost: cost}
}

// Register 创建用户。username 重复返回 ErrUsernameTaken。
func (s *Service) Register(ctx context.Context, username, password string) (uuid.UUID, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 {
		return uuid.Nil, ErrUsernameLength
	}
	if len(password) < 6 {
		return uuid.Nil, ErrPasswordLength
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	_, err = s.pool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)`,
		id, username, string(hash),
	)
	if err != nil {
		// unique violation on username
		if strings.Contains(err.Error(), "users_username_key") {
			return uuid.Nil, ErrUsernameTaken
		}
		return uuid.Nil, err
	}
	return id, nil
}

// Login 校验 username + password，返回 userID。
func (s *Service) Login(ctx context.Context, username, password string) (uuid.UUID, string, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, password_hash FROM users WHERE username = $1`, username,
	)
	var id uuid.UUID
	var hash string
	if err := row.Scan(&id, &hash); err != nil {
		// pgx no rows → 凭证无效（不区分用户不存在/密码错，避免用户名枚举）
		return uuid.Nil, "", ErrInvalidCredential
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return uuid.Nil, "", ErrInvalidCredential
	}
	return id, username, nil
}

// GetByID 拉取 user（中间件用）。不存在返回 ErrUserNotFound。
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (string, error) {
	var username string
	err := s.pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, id).Scan(&username)
	if err != nil {
		return "", ErrUserNotFound
	}
	return username, nil
}
