package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentRun struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	TaskType      string
	Status        string
	CurrentStage  string
	TriggerType   string
	TriggerSource string
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   *time.Time
}

func CreateAgentRun(ctx context.Context, pool *pgxpool.Pool, r *AgentRun) (uuid.UUID, error) {
	const q = `INSERT INTO agent_runs
		(user_id, task_type, status, current_stage, trigger_type, trigger_source)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	row := pool.QueryRow(ctx, q, r.UserID, r.TaskType, r.Status, r.CurrentStage, r.TriggerType, r.TriggerSource)
	if err := row.Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return uuid.Nil, err
	}
	return r.ID, nil
}
