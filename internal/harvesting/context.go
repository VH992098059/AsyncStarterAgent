package harvesting

import (
	"context"
	"time"
)

// NoiseClassifier determines whether a ContextItem is noise (FR-B05).
// Implementations may use rule-based logic, LLM calls, or other strategies.
type NoiseClassifier interface {
	IsNoise(ctx context.Context, item ContextItem) (bool, string, error)
}

type ContextItem struct {
	ID         string            `json:"id"`
	UserID     string            `json:"user_id"`     // FR-B06: 关联用户，增量同步 + 入库必需
	Source     string            `json:"source"`     // github / calendar / im / note
	Type       string            `json:"type"`       // commit / pr / review / meeting / message / note
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	URL        string            `json:"url,omitempty"`
	OccurredAt time.Time         `json:"occurred_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type ContextSnapshot struct {
	AgentRunID string        `json:"agent_run_id"`
	UserID     string        `json:"user_id"`
	Items      []ContextItem `json:"items"`
	FilterMeta FilterMeta    `json:"filter_meta"`
	SyncedAt   time.Time     `json:"synced_at"`
}

type FilterMeta struct {
	TotalBeforeFilter int            `json:"total_before_filter"`
	TotalAfterFilter  int            `json:"total_after_filter"`
	RuleRejections    map[string]int `json:"rule_rejections"`
	LLMRejections     int            `json:"llm_rejections"`
}
