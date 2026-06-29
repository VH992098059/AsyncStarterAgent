package auth_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/repository"
)

func TestService_RegisterAndLogin_Integration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	pool, err := repository.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	svc := auth.NewService(pool, 4) // 低 cost 跑测试快
	username := "test_" + time.Now().Format("150405.000000000")
	password := "secret-123"

	id, err := svc.Register(context.Background(), username, password)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if id.String() == "" {
		t.Fatal("empty id")
	}

	// 重名 → 失败
	if _, err := svc.Register(context.Background(), username, password); err != auth.ErrUsernameTaken {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}

	// login 正确
	uid, uname, err := svc.Login(context.Background(), username, password)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if uid != id || uname != username {
		t.Fatalf("login mismatch: %s/%s vs %s/%s", uid, uname, id, username)
	}

	// login 错密码
	if _, _, err := svc.Login(context.Background(), username, "wrong"); err != auth.ErrInvalidCredential {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}

	// login 不存在用户
	if _, _, err := svc.Login(context.Background(), "ghost_user", "x"); err != auth.ErrInvalidCredential {
		t.Fatalf("expected ErrInvalidCredential for ghost, got %v", err)
	}

	// GetByID
	name, err := svc.GetByID(context.Background(), id)
	if err != nil || name != username {
		t.Fatalf("GetByID: %s err=%v", name, err)
	}
}

func TestService_Register_Validation(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	pool, err := repository.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	svc := auth.NewService(pool, 4)

	if _, err := svc.Register(context.Background(), "ab", "secret-123"); err == nil {
		t.Fatal("expected error for short username")
	}
	if _, err := svc.Register(context.Background(), "valid_name", "123"); err == nil {
		t.Fatal("expected error for short password")
	}
}
