package harvesting

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sourceAdapter is the interface that all data source adapters must implement.
// T010-T013 adapters (GitHub, Calendar, Feishu, Obsidian) already satisfy this.
type sourceAdapter interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]ContextItem, error)
}

// RuleFilterer abstracts the rule-based filter to avoid import cycle with filter package.
type RuleFilterer interface {
	Apply(items []ContextItem) (kept []ContextItem, rejections map[string]int)
}

// LLMFilterer abstracts the LLM-based filter to avoid import cycle with filter package.
type LLMFilterer interface {
	Apply(ctx context.Context, items []ContextItem) (kept []ContextItem, rejected int)
}

// Pipeline orchestrates the ETL flow: fetch → rule filter → LLM filter → persist (FR-B05 + FR-B06).
type Pipeline struct {
	sync     *SyncStore
	ruleF    RuleFilterer
	llmF     LLMFilterer
	adapters []sourceAdapter
}

// NewPipeline creates a Pipeline with the given DB pool, filters, and adapters.
// Callers should construct filters using the filter sub-package:
//
//	ruleF := filter.NewRuleFilter(filter.DefaultRules())
//	llmF := filter.NewLLMFilter(classifier)
//	pipeline := harvesting.NewPipeline(pool, ruleF, llmF, adapter1, adapter2)
func NewPipeline(pool *pgxpool.Pool, ruleF RuleFilterer, llmF LLMFilterer, adapters ...sourceAdapter) *Pipeline {
	return &Pipeline{
		sync:     NewSyncStore(pool),
		ruleF:    ruleF,
		llmF:     llmF,
		adapters: adapters,
	}
}

// Run executes one full ETL cycle for the given data source:
//  1. Find the adapter by data source ID
//  2. Get last sync timestamp (incremental)
//  3. Fetch raw items from the adapter
//  4. Apply rule-based filter
//  5. Apply LLM-based filter
//  6. Persist kept items
//  7. Update sync timestamp
func (p *Pipeline) Run(ctx context.Context, userID, agentRunID, dataSourceID string) (*ContextSnapshot, error) {
	adapter, dsID, err := p.findAdapter(dataSourceID)
	if err != nil {
		return nil, err
	}
	since, err := p.sync.GetLastSync(ctx, dsID)
	if err != nil {
		return nil, fmt.Errorf("pipeline get last sync: %w", err)
	}
	raw, err := adapter.Fetch(ctx, userID, since)
	if err != nil {
		return nil, fmt.Errorf("pipeline fetch: %w", err)
	}

	// Rule-based filter
	kept, rejections := p.ruleF.Apply(raw)

	// LLM-based filter
	kept2, llmRejected := p.llmF.Apply(ctx, kept)

	// Persist kept items（问题 #12: 批量写入，避免逐条 Exec 的 N+1 网络往返）
	if err := p.sync.UpsertContextItems(ctx, kept2); err != nil {
		log.Printf("[pipeline] upsert: %v", err)
	}

	// Update sync timestamp
	if err := p.sync.UpdateLastSync(ctx, dsID); err != nil {
		return nil, fmt.Errorf("pipeline update sync: %w", err)
	}

	return &ContextSnapshot{
		AgentRunID: agentRunID,
		UserID:     userID,
		Items:      kept2,
		FilterMeta: FilterMeta{
			TotalBeforeFilter: len(raw),
			TotalAfterFilter:  len(kept2),
			RuleRejections:    rejections,
			LLMRejections:     llmRejected,
		},
		SyncedAt: time.Now().UTC(),
	}, nil
}

// findAdapter locates the adapter matching the source part of dataSourceID.
// dataSourceID format: "source_name:qualifier" (e.g. "github:o/r", "obsidian:/path").
func (p *Pipeline) findAdapter(dataSourceID string) (sourceAdapter, string, error) {
	sourceName := splitSource(dataSourceID)
	for _, a := range p.adapters {
		if a.Name() == sourceName {
			return a, dataSourceID, nil
		}
	}
	return nil, "", &NotFoundError{Source: dataSourceID}
}

// NotFoundError is returned when no adapter matches the requested data source.
type NotFoundError struct{ Source string }

func (e *NotFoundError) Error() string { return "adapter not found: " + e.Source }

// splitSource extracts the source name before the first colon.
func splitSource(id string) string {
	for i, c := range id {
		if c == ':' {
			return id[:i]
		}
	}
	return id
}
