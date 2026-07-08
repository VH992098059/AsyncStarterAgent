package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentRunMessage 对应 agent_run_messages 表（0008 迁移新增）。
// Status：sent（用户消息已落库）/ streaming（assistant 占位）/ done（流式结束）/ error（失败）。
// ReasoningContent：LLM 推理内容（DeepSeek thinking），前端默认折叠显示。
type AgentRunMessage struct {
	ID               uuid.UUID
	AgentRunID       uuid.UUID
	UserID           uuid.UUID
	Role             string
	Content          string
	ReasoningContent string
	Status           string
	ErrorMessage     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CreateMessage 插入一条消息，回填 id / created_at / updated_at。
func CreateMessage(ctx context.Context, pool *pgxpool.Pool, m *AgentRunMessage) (uuid.UUID, error) {
	const q = `INSERT INTO agent_run_messages (agent_run_id, user_id, role, content, reasoning_content, status, error_message)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)
	           RETURNING id, created_at, updated_at`
	row := pool.QueryRow(ctx, q, m.AgentRunID, m.UserID, m.Role, m.Content, m.ReasoningContent, m.Status, m.ErrorMessage)
	if err := row.Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return uuid.Nil, err
	}
	return m.ID, nil
}

// ListMessagesByRun 按 agent_run_id 拉取消息（按 created_at ASC）。
// limit <= 0 或 > 200 时取 50。
func ListMessagesByRun(ctx context.Context, pool *pgxpool.Pool, runID uuid.UUID, limit int) ([]AgentRunMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := pool.Query(ctx,
		`SELECT id, agent_run_id, user_id, role, content, reasoning_content, status, COALESCE(error_message, ''), created_at, updated_at
		 FROM agent_run_messages WHERE agent_run_id = $1 ORDER BY created_at ASC LIMIT $2`,
		runID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AgentRunMessage
	for rows.Next() {
		var m AgentRunMessage
		if err := rows.Scan(&m.ID, &m.AgentRunID, &m.UserID, &m.Role, &m.Content, &m.ReasoningContent,
			&m.Status, &m.ErrorMessage, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ErrMessageNotFound 表示按 ID 未找到消息。
var ErrMessageNotFound = fmt.Errorf("message not found")

// CleanStaleStreamingMessages 清理指定 run 下所有 status='streaming' 且 content 为空的 assistant 消息。
// 这些是之前 UpdateMessageStatus SQL bug 留下的脏数据，修了 SQL 后新消息正常，但旧消息仍卡在 streaming。
func CleanStaleStreamingMessages(ctx context.Context, pool *pgxpool.Pool, runID uuid.UUID) (int64, error) {
	const q = `DELETE FROM agent_run_messages
	           WHERE agent_run_id = $1
	             AND role = 'assistant'
	             AND status = 'streaming'
	             AND (content = '' OR content IS NULL)`
	tag, err := pool.Exec(ctx, q, runID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// UpdateMessageStatus 更新消息的 status / content / reasoning_content / error_message。
// 用于流式结束后写入完整 assistant 内容（status='done'），或失败时回写错误（status='error'）。
func UpdateMessageStatus(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, status, content, reasoningContent, errMsg string) error {
	const q = `UPDATE agent_run_messages
	           SET status = $2,
	               content = CASE WHEN $3 = '' THEN content ELSE $3 END,
	               reasoning_content = CASE WHEN $4 = '' THEN reasoning_content ELSE $4 END,
	               error_message = CASE WHEN $6 = 'error' THEN $5 ELSE error_message END,
	               updated_at = NOW()
	           WHERE id = $1`
	tag, err := pool.Exec(ctx, q, id, status, content, reasoningContent, errMsg, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMessageNotFound
	}
	return nil
}
