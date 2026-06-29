package harvesting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncStore manages incremental sync state and context item persistence (FR-B06).
type SyncStore struct {
	pool *pgxpool.Pool
}

// NewSyncStore creates a SyncStore backed by the given connection pool.
func NewSyncStore(pool *pgxpool.Pool) *SyncStore { return &SyncStore{pool: pool} }

// GetLastSync returns the last sync timestamp for the given data source.
// If no record exists (first sync), it returns 7 days ago as the default.
func (s *SyncStore) GetLastSync(ctx context.Context, dataSourceID string) (time.Time, error) {
	var ts time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT last_sync_at FROM sync_timestamps WHERE data_source_id = $1`,
		dataSourceID,
	).Scan(&ts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return time.Now().Add(-7 * 24 * time.Hour), nil
		}
		return time.Time{}, fmt.Errorf("get last sync: %w", err)
	}
	return ts, nil
}

// UpdateLastSync upserts the sync timestamp for the given data source to NOW().
func (s *SyncStore) UpdateLastSync(ctx context.Context, dataSourceID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sync_timestamps (data_source_id, last_sync_at)
		VALUES ($1, NOW())
		ON CONFLICT (data_source_id) DO UPDATE SET last_sync_at = NOW()
	`, dataSourceID)
	if err != nil {
		return fmt.Errorf("update sync ts: %w", err)
	}
	return nil
}

// UpsertContextItem inserts a context item or ignores on conflict (user_id, source, external_id).
func (s *SyncStore) UpsertContextItem(ctx context.Context, item ContextItem) error {
	metaJSON, _ := json.Marshal(item.Metadata)
	if metaJSON == nil {
		metaJSON = []byte("{}")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO context_items
		(user_id, source, external_id, type, title, content, url, occurred_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, source, external_id) DO NOTHING
	`, item.UserID, item.Source, item.ID, item.Type, item.Title, item.Content, item.URL, item.OccurredAt, metaJSON)
	if err != nil {
		return fmt.Errorf("upsert context item: %w", err)
	}
	return nil
}
