package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Draft 对应 drafts 表（0001_init + 0008 修复）。
// MarkdownContent 是草稿正文（content JSONB 列暂未使用，代码统一读写 markdown_content）。
// Marks 为 JSONB 原始字节，由上层（synthesis.Service）负责序列化/反序列化。
type Draft struct {
	ID              uuid.UUID
	AgentRunID      uuid.UUID
	Title           string
	MarkdownContent string
	Completeness    float32
	Marks           []byte
	Status          string
	IterationCount  int
	QualityScore    float32
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GetDraftByRunID 按 agent_run_id 查询草稿。找不到时返回 ErrDraftNotFound。
func GetDraftByRunID(ctx context.Context, pool *pgxpool.Pool, runID uuid.UUID) (*Draft, error) {
	const q = `SELECT id, agent_run_id, COALESCE(title, ''), COALESCE(markdown_content, ''),
	                  completeness, marks, status, iteration_count, quality_score, created_at, updated_at
	           FROM drafts WHERE agent_run_id = $1 LIMIT 1`
	var d Draft
	row := pool.QueryRow(ctx, q, runID)
	if err := row.Scan(&d.ID, &d.AgentRunID, &d.Title, &d.MarkdownContent,
		&d.Completeness, &d.Marks, &d.Status, &d.IterationCount, &d.QualityScore,
		&d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}

// ErrDraftNotFound 表示按 agent_run_id 未找到草稿。
var ErrDraftNotFound = fmt.Errorf("draft not found")

// UpdateDraftMarkdown 按 agent_run_id 更新草稿正文，并同步重算 marks 与 completeness。
// 调用方应先调 synthesis.ExtractMarks / synthesis.Completeness 算出 marks/comp 传入。
func UpdateDraftMarkdown(ctx context.Context, pool *pgxpool.Pool, runID uuid.UUID, markdown string, marksJSON []byte, completeness float32) error {
	const q = `UPDATE drafts
	           SET markdown_content = $2,
	               marks = $3,
	               completeness = $4,
	               updated_at = NOW()
	           WHERE agent_run_id = $1`
	tag, err := pool.Exec(ctx, q, runID, markdown, marksJSON, completeness)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDraftNotFound
	}
	return nil
}

// MarkJSON 对应 drafts.marks JSONB 数组中的单个元素。导出供上层（synthesis）使用。
type MarkJSON struct {
	ID       string `json:"id"`
	Hint     string `json:"hint"`
	Position int    `json:"position"`
	Resolved bool   `json:"resolved"`
}

// UnmarshalMarks 解析 marks JSONB 字节为 MarkJSON 列表。
// marksJSON 为空或 "[]" 时返回空切片。
func UnmarshalMarks(raw []byte) ([]MarkJSON, error) {
	if len(raw) == 0 {
		return []MarkJSON{}, nil
	}
	var out []MarkJSON
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []MarkJSON{}, nil
	}
	return out, nil
}
