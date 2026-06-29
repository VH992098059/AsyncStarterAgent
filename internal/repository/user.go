package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User 字段对应 users 表。
type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, username, passwordHash string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)`,
		id, username, passwordHash,
	)
	return id, err
}

func GetUserByUsername(ctx context.Context, pool *pgxpool.Pool, username string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at, updated_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at, updated_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
