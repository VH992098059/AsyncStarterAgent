package filter

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// --- LLMFilter tests using interface abstraction (C8 compliant) ---

// patternClassifier is a real NoiseClassifier implementation that uses
// pattern matching logic (not hardcoded mock data) to classify items.
type patternClassifier struct{}

func (patternClassifier) IsNoise(_ context.Context, item harvesting.ContextItem) (bool, string, error) {
	title := item.Title
	if len(title) > 4 && (title[:5] == "spam " || title[:5] == "noise") {
		return true, "pattern: noise keyword", nil
	}
	if title == "" && item.Content == "" {
		return true, "pattern: empty content", nil
	}
	return false, "", nil
}

func TestLLMFilter_Apply_WithPatternClassifier(t *testing.T) {
	classifier := &patternClassifier{}
	f := NewLLMFilter(classifier)
	items := []harvesting.ContextItem{
		{ID: "k1", Title: "feat: add login", Type: "commit"},
		{ID: "n1", Title: "spam message", Type: "message"},
		{ID: "k2", Title: "Sprint planning", Content: "Q3 goals", Type: "meeting"},
		{ID: "n2", Title: "noise alert", Type: "message"},
	}
	kept, rejected := f.Apply(context.Background(), items)
	if len(kept) != 2 {
		t.Errorf("kept: got %d, want 2", len(kept))
	}
	if rejected != 2 {
		t.Errorf("rejected: got %d, want 2", rejected)
	}
}

// errClassifier returns an error on every call — tests the conservative
// keep-on-error behavior of LLMFilter.
type errClassifier struct{}

func (errClassifier) IsNoise(context.Context, harvesting.ContextItem) (bool, string, error) {
	return false, "", fmt.Errorf("simulated LLM error")
}

func TestLLMFilter_ErrorKeepsAll(t *testing.T) {
	classifier := &errClassifier{}
	f := NewLLMFilter(classifier)
	items := []harvesting.ContextItem{
		{ID: "1", Title: "item one"},
		{ID: "2", Title: "item two"},
	}
	kept, rejected := f.Apply(context.Background(), items)
	if len(kept) != 2 {
		t.Errorf("on error should keep all, got %d", len(kept))
	}
	if rejected != 0 {
		t.Errorf("on error rejected should be 0, got %d", rejected)
	}
}

func TestLLMFilter_EmptyInput(t *testing.T) {
	classifier := &patternClassifier{}
	f := NewLLMFilter(classifier)
	kept, rejected := f.Apply(context.Background(), nil)
	if len(kept) != 0 {
		t.Errorf("expected 0 kept, got %d", len(kept))
	}
	if rejected != 0 {
		t.Errorf("expected 0 rejected, got %d", rejected)
	}
}

// --- ParseResponse tests ---

func TestParseResponse_Noise(t *testing.T) {
	isNoise, reason, err := ParseResponse(`{"is_noise": true, "reason": "merge commit"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !isNoise {
		t.Error("expected noise=true")
	}
	if reason != "merge commit" {
		t.Errorf("reason: %s", reason)
	}
}

func TestParseResponse_NotNoise(t *testing.T) {
	isNoise, _, err := ParseResponse(`{"is_noise": false, "reason": ""}`)
	if err != nil {
		t.Fatal(err)
	}
	if isNoise {
		t.Error("expected noise=false")
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	_, _, err := ParseResponse(`not json`)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- EinoClassifier integration test (requires OPENAI_API_KEY) ---

func TestEinoClassifier_Integration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("requires OPENAI_API_KEY")
	}
	ctx := context.Background()
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: apiKey,
		Model:  "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("create chat model: %v", err)
	}
	classifier, err := NewEinoClassifier(EinoClassifierConfig{ChatModel: chatModel})
	if err != nil {
		t.Fatalf("create classifier: %v", err)
	}

	isNoise, reason, err := classifier.IsNoise(ctx, harvesting.ContextItem{
		Title:   "Merge pull request #42",
		Content: "Merged feature branch",
		Type:    "commit",
	})
	if err != nil {
		t.Fatalf("is noise: %v", err)
	}
	if !isNoise {
		t.Errorf("expected merge commit to be classified as noise, reason: %s", reason)
	}

	isNoise2, _, err := classifier.IsNoise(ctx, harvesting.ContextItem{
		Title:   "feat: implement user authentication",
		Content: "Added JWT-based auth flow with refresh tokens",
		Type:    "commit",
	})
	if err != nil {
		t.Fatalf("is noise: %v", err)
	}
	if isNoise2 {
		t.Error("expected feature commit to NOT be classified as noise")
	}
}

// --- Compile-time interface check ---

var _ harvesting.NoiseClassifier = (*EinoClassifier)(nil)
var _ harvesting.NoiseClassifier = (*patternClassifier)(nil)
