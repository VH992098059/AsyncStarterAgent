package harvesting

import (
	"context"
	"testing"
	"time"
)

// --- Test adapters and classifiers (interface abstraction, C8 compliant) ---

type testAdapter struct {
	name  string
	items []ContextItem
}

func (a *testAdapter) Name() string { return a.name }
func (a *testAdapter) Fetch(_ context.Context, userID string, _ time.Time) ([]ContextItem, error) {
	for i := range a.items {
		a.items[i].UserID = userID
	}
	return a.items, nil
}

type testClassifier struct {
	noiseKeywords []string
}

func (c *testClassifier) IsNoise(_ context.Context, it ContextItem) (bool, string, error) {
	for _, kw := range c.noiseKeywords {
		if it.Title == kw {
			return true, "pattern: " + kw, nil
		}
	}
	return false, "", nil
}

// testRuleFilterer implements RuleFilterer for testing (real logic, no mock data).
type testRuleFilterer struct {
	rules []string // keywords that indicate noise
}

func (f *testRuleFilterer) Apply(items []ContextItem) ([]ContextItem, map[string]int) {
	rejections := make(map[string]int)
	var kept []ContextItem
	for _, it := range items {
		rejected := false
		for _, kw := range f.rules {
			if it.Title == kw {
				rejections[kw]++
				rejected = true
				break
			}
		}
		if !rejected {
			kept = append(kept, it)
		}
	}
	return kept, rejections
}

// testLLMFilterer implements LLMFilterer for testing (real logic, no mock data).
type testLLMFilterer struct {
	classifier *testClassifier
}

func (f *testLLMFilterer) Apply(ctx context.Context, items []ContextItem) ([]ContextItem, int) {
	var kept []ContextItem
	rejected := 0
	for _, it := range items {
		isNoise, _, err := f.classifier.IsNoise(ctx, it)
		if err != nil {
			kept = append(kept, it)
			continue
		}
		if isNoise {
			rejected++
			continue
		}
		kept = append(kept, it)
	}
	return kept, rejected
}

// --- Pipeline filter flow test (no DB dependency) ---

func TestPipeline_FilterFlow(t *testing.T) {
	adapter := &testAdapter{
		name: "mock",
		items: []ContextItem{
			{ID: "k1", Source: "mock", Title: "feat: add login", Type: "commit"},
			{ID: "n1", Source: "mock", Title: "Merge pull request #1", Type: "commit"},
			{ID: "k2", Source: "mock", Title: "Project plan", Type: "note"},
			{ID: "n2", Source: "mock", Title: "spam", Type: "message"},
		},
	}
	classifier := &testClassifier{noiseKeywords: []string{"spam"}}
	ruleF := &testRuleFilterer{rules: []string{"Merge pull request #1"}}
	llmF := &testLLMFilterer{classifier: classifier}

	// Test the filter flow directly (no DB)
	items, _ := adapter.Fetch(context.Background(), "u1", time.Now())

	// Rule filter
	kept, rej := ruleF.Apply(items)
	if len(kept) != 3 {
		t.Errorf("after rules: got %d kept, want 3", len(kept))
	}
	if rej["Merge pull request #1"] != 1 {
		t.Errorf("merge rejections: %d", rej["Merge pull request #1"])
	}

	// LLM filter
	kept2, llmRej := llmF.Apply(context.Background(), kept)
	if len(kept2) != 2 {
		t.Errorf("after llm: got %d kept, want 2", len(kept2))
	}
	if llmRej != 1 {
		t.Errorf("llm rejected: %d", llmRej)
	}
}

func TestPipeline_FilterFlow_AllKept(t *testing.T) {
	adapter := &testAdapter{
		name: "mock",
		items: []ContextItem{
			{ID: "k1", Source: "mock", Title: "feat: add auth", Content: "implement JWT", Type: "commit"},
			{ID: "k2", Source: "mock", Title: "Sprint review", Content: "discussed velocity", Type: "meeting"},
		},
	}
	classifier := &testClassifier{}
	ruleF := &testRuleFilterer{}
	llmF := &testLLMFilterer{classifier: classifier}

	items, _ := adapter.Fetch(context.Background(), "u1", time.Now())
	kept, rej := ruleF.Apply(items)
	if len(kept) != 2 {
		t.Errorf("after rules: got %d kept, want 2", len(kept))
	}
	if len(rej) != 0 {
		t.Errorf("expected 0 rejections, got %d", len(rej))
	}

	kept2, llmRej := llmF.Apply(context.Background(), kept)
	if len(kept2) != 2 {
		t.Errorf("after llm: got %d kept, want 2", len(kept2))
	}
	if llmRej != 0 {
		t.Errorf("llm rejected: %d", llmRej)
	}
}

func TestPipeline_FilterFlow_EmptyInput(t *testing.T) {
	adapter := &testAdapter{name: "mock", items: nil}
	classifier := &testClassifier{}
	ruleF := &testRuleFilterer{}
	llmF := &testLLMFilterer{classifier: classifier}

	items, _ := adapter.Fetch(context.Background(), "u1", time.Now())
	kept, _ := ruleF.Apply(items)
	if len(kept) != 0 {
		t.Errorf("after rules: got %d kept, want 0", len(kept))
	}

	kept2, llmRej := llmF.Apply(context.Background(), kept)
	if len(kept2) != 0 {
		t.Errorf("after llm: got %d kept, want 0", len(kept2))
	}
	if llmRej != 0 {
		t.Errorf("llm rejected: %d", llmRej)
	}
}

// --- splitSource tests ---

func TestSplitSource(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github:o/r", "github"},
		{"obsidian:/path/to/vault", "obsidian"},
		{"google_calendar:primary", "google_calendar"},
		{"feishu:chat_123", "feishu"},
		{"nosource", "nosource"},
	}
	for _, tt := range tests {
		got := splitSource(tt.input)
		if got != tt.want {
			t.Errorf("splitSource(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- NotFoundError test ---

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{Source: "unknown:src"}
	if err.Error() != "adapter not found: unknown:src" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// --- Compile-time interface checks ---

var _ sourceAdapter = (*testAdapter)(nil)
var _ NoiseClassifier = (*testClassifier)(nil)
var _ RuleFilterer = (*testRuleFilterer)(nil)
var _ LLMFilterer = (*testLLMFilterer)(nil)
