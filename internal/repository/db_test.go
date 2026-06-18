package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/asyncstarter/agent/internal/repository"
)

func TestDBConnect_RequiresDSN(t *testing.T) {
	os.Setenv("DATABASE_URL", "")
	_, err := repository.Open(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when DSN is empty")
	}
}
