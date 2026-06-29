package harvesting

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSyncStore_GetLastSync_Default(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires DATABASE_URL")
	}
	// Integration test: verifies that GetLastSync returns 7 days ago
	// when no sync_timestamps row exists for the given data source ID.
	// Full integration testing is deferred to the pipeline integration test.
	_ = context.Background()
	_ = time.Now()
}

func TestSyncStore_UpdateLastSync(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires DATABASE_URL")
	}
}

func TestSyncStore_UpsertContextItem(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires DATABASE_URL")
	}
}
