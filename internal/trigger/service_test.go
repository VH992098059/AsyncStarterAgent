package trigger_test

import (
	"context"
	"os"
	"testing"

	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/trigger"
	"github.com/google/uuid"
)

func TestProcessKeyword_Integration(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set; skip integration test")
	}
	pool, err := repository.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	svc := trigger.NewService(pool, trigger.NewMatcher(trigger.DefaultMatcherRules()))
	id, err := svc.ProcessKeyword(context.Background(), uuid.New(), "写本周周报")
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("empty run id")
	}
}
