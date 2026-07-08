package repository

import (
	"context"
	"fmt"
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

// GetAgentRunByID 查询单个 AgentRun 详情。userID 用于归属校验，传 uuid.Nil 则不校验。
// 找不到时返回 pgx.ErrNoRows。
func GetAgentRunByID(ctx context.Context, pool *pgxpool.Pool, userID, id uuid.UUID) (*AgentRun, error) {
	const q = `SELECT id, user_id, task_type, status, current_stage, trigger_type,
	                  COALESCE(trigger_source, ''), COALESCE(error_message, ''),
	                  created_at, updated_at, completed_at
	           FROM agent_runs WHERE id = $1`
	var r AgentRun
	row := pool.QueryRow(ctx, q, id)
	if err := row.Scan(&r.ID, &r.UserID, &r.TaskType, &r.Status, &r.CurrentStage,
		&r.TriggerType, &r.TriggerSource, &r.ErrorMessage,
		&r.CreatedAt, &r.UpdatedAt, &r.CompletedAt); err != nil {
		return nil, err
	}
	if userID != uuid.Nil && r.UserID != userID {
		return nil, ErrRunNotOwned
	}
	return &r, nil
}

// ErrRunNotOwned 表示 AgentRun 不属于当前 userID。
var ErrRunNotOwned = fmt.Errorf("agent run not owned by user")

// UpdateAgentRunStatus 更新 AgentRun 的 status / current_stage，并按需回写 error_message / completed_at。
// status 取值：pending / running / completed / failed / cancelled。
// currentStage 取值：ingestion / harvesting / synthesis / delivery。
// 当 status 为 failed 时写入 errMsg；当 status 为 completed/failed/cancelled 时回写 completed_at。
func UpdateAgentRunStatus(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, status, currentStage, errMsg string) error {
	const q = `UPDATE agent_runs
	           SET status = $2,
	               current_stage = $3,
	               error_message = CASE WHEN $2 = 'failed' THEN $4 ELSE error_message END,
	               completed_at = CASE WHEN $2 IN ('completed','failed','cancelled') THEN NOW() ELSE completed_at END,
	               updated_at = NOW()
	           WHERE id = $1`
	tag, err := pool.Exec(ctx, q, id, status, currentStage, errMsg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRunNotFound
	}
	return nil
}

// ErrRunNotFound 表示按 ID 未找到 AgentRun。
var ErrRunNotFound = fmt.Errorf("agent run not found")

// DeleteAgentRun 按 ID 删除 AgentRun。FK CASCADE 会同时清除 drafts / deliveries / agent_run_messages。
func DeleteAgentRun(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	const q = `DELETE FROM agent_runs WHERE id = $1`
	tag, err := pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRunNotFound
	}
	return nil
}
