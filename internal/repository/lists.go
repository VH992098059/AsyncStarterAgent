package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DataSource 字段对应 data_sources 表。
type DataSource struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Type       string
	Name       string
	Config     string
	Status     string
	LastSyncAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ListDataSources 拉取某用户的所有数据源。
func ListDataSources(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]DataSource, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, user_id, type, name, COALESCE(config, ''), status, last_sync_at, created_at, updated_at
		 FROM data_sources WHERE user_id = $1 ORDER BY type`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DataSource
	for rows.Next() {
		var ds DataSource
		if err := rows.Scan(&ds.ID, &ds.UserID, &ds.Type, &ds.Name, &ds.Config, &ds.Status, &ds.LastSyncAt, &ds.CreatedAt, &ds.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ds)
	}
	return out, rows.Err()
}

// ListAgentRuns 拉取某用户的 AgentRun 列表（按 created_at desc）。
// limit 0 → 默认 20。before 非 nil 时只返回 created_at < before 的记录（游标分页）。
func ListAgentRuns(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, limit int, before *time.Time) ([]AgentRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := pool.Query(ctx,
		`SELECT id, user_id, task_type, status, current_stage, trigger_type, COALESCE(trigger_source, ''),
			        COALESCE(error_message, ''), created_at, updated_at, completed_at
			 FROM agent_runs
			 WHERE user_id = $1 AND ($2::timestamptz IS NULL OR created_at < $2)
			 ORDER BY created_at DESC LIMIT $3`,
		userID, before, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AgentRun
	for rows.Next() {
		var r AgentRun
		if err := rows.Scan(&r.ID, &r.UserID, &r.TaskType, &r.Status, &r.CurrentStage, &r.TriggerType, &r.TriggerSource, &r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt, &r.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountAgentRunsByStatus 统计某用户各状态的 AgentRun 数量（今日 + 总计）。
// 今日窗口：created_at >= today UTC 0 点。
func CountAgentRunsByStatus(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) (map[string]int, map[string]int, error) {
	// total
	totalRows, err := pool.Query(ctx,
		`SELECT status, COUNT(*) FROM agent_runs WHERE user_id = $1 GROUP BY status`,
		userID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer totalRows.Close()
	total := make(map[string]int)
	for totalRows.Next() {
		var s string
		var c int
		if err := totalRows.Scan(&s, &c); err != nil {
			return nil, nil, err
		}
		total[s] = c
	}

	// today
	todayRows, err := pool.Query(ctx,
		`SELECT status, COUNT(*) FROM agent_runs WHERE user_id = $1 AND created_at >= date_trunc('day', NOW()) GROUP BY status`,
		userID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer todayRows.Close()
	today := make(map[string]int)
	for todayRows.Next() {
		var s string
		var c int
		if err := todayRows.Scan(&s, &c); err != nil {
			return nil, nil, err
		}
		today[s] = c
	}
	return total, today, nil
}
