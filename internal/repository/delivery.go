package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Delivery 对应 deliveries 表（0001_init）。
// 每次交付独立一行，target_type 为 notion / obsidian，status 为 pending / success / failed。
type Delivery struct {
	ID           uuid.UUID
	DraftID      uuid.UUID
	TargetType   string
	TargetURL    string
	Status       string
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ListDeliveriesByRun 按 agent_run_id 查询交付历史（JOIN drafts，按 created_at DESC）。
func ListDeliveriesByRun(ctx context.Context, pool *pgxpool.Pool, runID uuid.UUID) ([]Delivery, error) {
	rows, err := pool.Query(ctx,
		`SELECT d.id, d.draft_id, d.target_type, COALESCE(d.target_url, ''),
		        d.status, COALESCE(d.error_message, ''), d.created_at, d.updated_at
		 FROM deliveries d
		 JOIN drafts dr ON d.draft_id = dr.id
		 WHERE dr.agent_run_id = $1
		 ORDER BY d.created_at DESC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Delivery
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.ID, &d.DraftID, &d.TargetType, &d.TargetURL,
			&d.Status, &d.ErrorMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
