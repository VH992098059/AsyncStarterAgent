package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/asyncstarter/agent/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	pgxstd "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open 建立 PostgreSQL 连接池，并在数据库/表不存在时自动创建（幂等）。
// 失败时返回 error，不返回半初始化的 pool（Ping 失败会主动 Close）。
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	// 1. 自动创建数据库（如果不存在）。
	if err := ensureDatabase(ctx, cfg); err != nil {
		return nil, fmt.Errorf("ensure database: %w", err)
	}

	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	// 2. 自动执行迁移创建/更新表结构；已应用过的迁移会跳过。
	if err := runMigrations(dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return pool, nil
}

// ensureDatabase 连接到 postgres 维护库，检查目标库是否存在，不存在则创建。
func ensureDatabase(ctx context.Context, cfg *pgxpool.Config) error {
	dbName := cfg.ConnConfig.Database
	if dbName == "" {
		return fmt.Errorf("database name is required in DSN")
	}

	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"

	conn, err := pgxstd.ConnectConfig(ctx, adminCfg.ConnConfig)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbName,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check database exists: %w", err)
	}
	if exists {
		return nil
	}

	// 创建数据库；使用标识符转义避免 SQL 注入。
	if _, err := conn.Exec(ctx,
		fmt.Sprintf("CREATE DATABASE %s", pgxstd.Identifier{dbName}.Sanitize()),
	); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return nil
}

// runMigrations 使用 golang-migrate 执行 migrations/ 下的 .up.sql 文件。
// 已应用的迁移会自动跳过。
func runMigrations(dsn string) error {
	migrateDSN, err := toMigrateDSN(dsn)
	if err != nil {
		return fmt.Errorf("convert dsn: %w", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, migrateDSN)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// toMigrateDSN 将标准 postgres DSN 转换为 golang-migrate pgx/v5 驱动格式。
func toMigrateDSN(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		u.Scheme = "pgx5"
	}
	return u.String(), nil
}
