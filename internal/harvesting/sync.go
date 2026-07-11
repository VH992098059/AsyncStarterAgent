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

// UpsertContextItems 批量写入多条 context item（问题 #12：替代逐条 Exec 的 N+1 写法）。
// 用 pgx.Batch 把所有 INSERT 语句一次性发给数据库（pipelined），避免同步窗口内数百上千条
// context item 产生等量的网络往返。单条失败不中断整批：记录哪些 item 失败并返回聚合 error，
// 与原逐条实现里"某条失败只记日志不阻断整体"的行为保持一致。
func (s *SyncStore) UpsertContextItems(ctx context.Context, items []ContextItem) error {
	if len(items) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, item := range items {
		metaJSON, _ := json.Marshal(item.Metadata)
		if metaJSON == nil {
			metaJSON = []byte("{}")
		}
		batch.Queue(`
			INSERT INTO context_items
			(user_id, source, external_id, type, title, content, url, occurred_at, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (user_id, source, external_id) DO NOTHING
		`, item.UserID, item.Source, item.ID, item.Type, item.Title, item.Content, item.URL, item.OccurredAt, metaJSON)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()

	var firstErr error
	failed := 0
	for i := 0; i < len(items); i++ {
		if _, err := br.Exec(); err != nil {
			failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("upsert context item %s: %w", items[i].ID, err)
			}
		}
	}
	if firstErr != nil {
		return fmt.Errorf("batch upsert: %d/%d items failed, first error: %w", failed, len(items), firstErr)
	}
	return nil
}
